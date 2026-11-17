package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	StashKindLink     = "link"
	StashKindBilibili = "bilibili"
	StashKindGitHub   = "github"
	StashKindNote     = "note"

	StashStatusPending    = "pending"
	StashStatusProcessing = "processing"
	StashStatusReady      = "ready"
	StashStatusFailed     = "failed"
	StashStatusArchived   = "archived"

	MaxStashExcerptRunes = 12000
)

type StashItem struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	URL            string   `json:"url"`
	Domain         string   `json:"domain"`
	Title          string   `json:"title"`
	RawInput       string   `json:"raw_input"`
	UserNote       string   `json:"user_note"`
	AITldr         string   `json:"ai_tldr"`
	AIPoints       string   `json:"ai_points"`
	Points         []string `json:"points"`
	ContentExcerpt string   `json:"content_excerpt"`
	Tags           string   `json:"tags"`
	Status         string   `json:"status"`
	ErrorMsg       string   `json:"error_msg,omitempty"`
	Pinned         bool     `json:"pinned"`
	ReadCount      int      `json:"read_count"`
	LastReviewedAt string   `json:"last_reviewed_at,omitempty"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type StashTagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

var (
	reFirstHTTPURL   = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `\]）】》]+`)
	reMarkdownLink   = regexp.MustCompile(`\[([^\]]+)\]\(\s*(https?://[^\s)]+)\s*\)`)
	reMultiBlankLine = regexp.MustCompile(`\s+`)

	stashTrackingQueryKeys = map[string]bool{
		"spm":          true,
		"spm_id_from":  true,
		"vd_source":    true,
		"from_spmid":   true,
		"from":         true,
		"seid":         true,
		"timestamp":    true,
		"unique_k":     true,
		"buvid":        true,
		"is_story_h5":  true,
		"up_id":        true,
		"mid":          true,
		"plat_id":      true,
		"bbid":         true,
		"ts":           true,
		"fbclid":       true,
		"gclid":        true,
		"igshid":       true,
		"si":           true,
		"feature":      true,
		"ref_src":      true,
		"ref_url":      true,
		"mkt_tok":      true,
		"mc_cid":       true,
		"mc_eid":       true,
		"vero_id":      true,
		"wickedid":     true,
		"yclid":        true,
		"_hsenc":       true,
		"_hsmi":        true,
		"nsukey":       true,
		"isappinstalled": true,
	}
)

func NormalizeStashKind(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case StashKindBilibili:
		return StashKindBilibili
	case StashKindGitHub:
		return StashKindGitHub
	case StashKindNote:
		return StashKindNote
	case StashKindLink:
		return StashKindLink
	default:
		return StashKindLink
	}
}

func NormalizeStashStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case StashStatusPending:
		return StashStatusPending
	case StashStatusProcessing:
		return StashStatusProcessing
	case StashStatusReady:
		return StashStatusReady
	case StashStatusFailed:
		return StashStatusFailed
	case StashStatusArchived:
		return StashStatusArchived
	default:
		return StashStatusPending
	}
}

func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

func NormalizeStashTags(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	replacer := strings.NewReplacer("，", ",", "、", ",", "；", ",", ";", ",", "|", ",", "#", "")
	cleaned := replacer.Replace(raw)
	seen := make(map[string]bool)
	out := make([]string, 0, 6)
	for _, part := range strings.Split(cleaned, ",") {
		t := strings.TrimSpace(part)
		if t == "" {
			continue
		}
		t = TruncateRunes(t, 24)
		lower := strings.ToLower(t)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, t)
		if len(out) >= 8 {
			break
		}
	}
	return strings.Join(out, ",")
}

func MergeStashTags(existing, incoming string) string {
	if strings.TrimSpace(incoming) == "" {
		return NormalizeStashTags(existing)
	}
	if strings.TrimSpace(existing) == "" {
		return NormalizeStashTags(incoming)
	}
	return NormalizeStashTags(existing + "," + incoming)
}

func isTrackingQueryParam(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return true
	}
	if strings.HasPrefix(k, "utm_") || strings.HasPrefix(k, "share_") || strings.HasPrefix(k, "spm_") {
		return true
	}
	return stashTrackingQueryKeys[k]
}

// NormalizeStashURL strips tracking parameters, canonicalizes host/path casing, and classifies kind & domain.
func NormalizeStashURL(rawURL string) (cleanURL, domain, kind string, ok bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", StashKindNote, false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", "", StashKindNote, false
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if host == "m.bilibili.com" {
		host = "www.bilibili.com"
	}
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = host + ":" + port
	} else {
		parsed.Host = host
	}

	if parsed.RawQuery != "" {
		q := parsed.Query()
		for k := range q {
			if isTrackingQueryParam(k) {
				q.Del(k)
			}
		}
		parsed.RawQuery = q.Encode()
	}

	if len(parsed.Path) > 1 {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}
	if !strings.HasPrefix(parsed.Fragment, "/") {
		parsed.Fragment = ""
	}

	domain = strings.TrimPrefix(host, "www.")
	switch {
	case host == "b23.tv" || host == "bilibili.com" || strings.HasSuffix(host, ".bilibili.com"):
		kind = StashKindBilibili
	case host == "github.com" || host == "gist.github.com":
		kind = StashKindGitHub
	default:
		kind = StashKindLink
	}
	return parsed.String(), domain, kind, true
}

func trimTrailingURLPunctuation(u string) string {
	const trailingPunct = ",.;:!?，。；：！？、）】》\"'"
	out := strings.TrimSpace(u)
	for len(out) > 0 {
		r, size := utf8.DecodeLastRuneInString(out)
		if strings.ContainsRune(trailingPunct, r) {
			out = out[:len(out)-size]
			continue
		}
		if r == ')' && strings.Count(out, ")") > strings.Count(out, "(") {
			out = out[:len(out)-size]
			continue
		}
		break
	}
	return out
}

// ParseStashInput separates mobile share text (e.g. "【UP主】标题 https://b23.tv/xxx") into URL, candidate title, domain, and kind.
// If no HTTP/HTTPS URL is present, it classifies the input as a plain-text flash note (kind = "note").
func ParseStashInput(rawInput string) (cleanURL, domain, candidateTitle, kind string) {
	trimmed := strings.TrimSpace(rawInput)
	if trimmed == "" {
		return "", "", "", StashKindNote
	}

	// Check Markdown [title](url) syntax first
	if m := reMarkdownLink.FindStringSubmatch(trimmed); len(m) == 3 {
		u := trimTrailingURLPunctuation(m[2])
		if normURL, dom, k, ok := NormalizeStashURL(u); ok {
			cand := strings.TrimSpace(m[1])
			return normURL, dom, TruncateRunes(cand, 120), k
		}
	}

	loc := reFirstHTTPURL.FindStringIndex(trimmed)
	if loc == nil {
		firstLine := trimmed
		if idx := strings.IndexAny(firstLine, "\r\n"); idx >= 0 {
			firstLine = strings.TrimSpace(firstLine[:idx])
		}
		return "", "", TruncateRunes(firstLine, 64), StashKindNote
	}

	rawMatched := trimmed[loc[0]:loc[1]]
	trimmedURL := trimTrailingURLPunctuation(rawMatched)
	normURL, dom, k, ok := NormalizeStashURL(trimmedURL)
	if !ok {
		firstLine := trimmed
		if idx := strings.IndexAny(firstLine, "\r\n"); idx >= 0 {
			firstLine = strings.TrimSpace(firstLine[:idx])
		}
		return "", "", TruncateRunes(firstLine, 64), StashKindNote
	}

	prefix := strings.TrimSpace(trimmed[:loc[0]])
	suffix := strings.TrimSpace(trimmed[loc[0]+len(trimmedURL):])
	cand := prefix
	if cand == "" {
		cand = suffix
	}
	cand = reMultiBlankLine.ReplaceAllString(cand, " ")
	cand = strings.Trim(cand, " -|—_·:：")

	if cand == "" {
		if k == StashKindGitHub {
			if u, err := url.Parse(normURL); err == nil {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
					cand = parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
				}
			}
		}
		if cand == "" {
			cand = dom
		}
	}
	return normURL, dom, TruncateRunes(cand, 120), k
}

const stashSelectCols = `s.id, s.kind, s.url, s.domain, s.title, s.raw_input, s.user_note,
	s.ai_tldr, s.ai_points, s.content_excerpt, s.tags, s.status, s.error_msg,
	s.pinned, s.read_count, s.last_reviewed_at, s.created_at, s.updated_at`

func decodeStashPoints(raw string) []string {
	pts := make([]string, 0)
	if strings.TrimSpace(raw) == "" {
		return pts
	}
	if err := json.Unmarshal([]byte(raw), &pts); err != nil || pts == nil {
		return []string{}
	}
	return pts
}

func encodeStashPoints(points []string, fallbackJSON string) string {
	if len(points) > 0 {
		clean := make([]string, 0, len(points))
		for _, p := range points {
			t := strings.TrimSpace(p)
			if t != "" {
				clean = append(clean, t)
			}
		}
		if b, err := json.Marshal(clean); err == nil {
			return string(b)
		}
	}
	if strings.TrimSpace(fallbackJSON) != "" {
		var check []string
		if json.Unmarshal([]byte(fallbackJSON), &check) == nil {
			if b, err := json.Marshal(check); err == nil {
				return string(b)
			}
		}
	}
	return "[]"
}

func scanStashRow(scanner interface{ Scan(dest ...any) error }) (StashItem, error) {
	var item StashItem
	var pinInt int
	err := scanner.Scan(
		&item.ID,
		&item.Kind,
		&item.URL,
		&item.Domain,
		&item.Title,
		&item.RawInput,
		&item.UserNote,
		&item.AITldr,
		&item.AIPoints,
		&item.ContentExcerpt,
		&item.Tags,
		&item.Status,
		&item.ErrorMsg,
		&pinInt,
		&item.ReadCount,
		&item.LastReviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	item.Pinned = pinInt == 1
	item.Points = decodeStashPoints(item.AIPoints)
	return item, nil
}

func (s *Store) GetStashItem(id string) (*StashItem, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil, nil
	}
	row := s.db.QueryRow(`SELECT `+stashSelectCols+` FROM stash_items s WHERE s.id = ?`, trimmed)
	item, err := scanStashRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) FindUnarchivedStashByURL(normURL string) (*StashItem, error) {
	u := strings.TrimSpace(normURL)
	if u == "" {
		return nil, nil
	}
	row := s.db.QueryRow(`SELECT `+stashSelectCols+` FROM stash_items s WHERE s.url = ? AND s.status != 'archived' ORDER BY s.updated_at DESC LIMIT 1`, u)
	item, err := scanStashRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// CaptureStashItem saves a newly captured input (<50ms path) or merges with an existing unarchived item sharing the normalized URL.
// Returns (item, deduplicated, shouldDistill, error).
func (s *Store) CaptureStashItem(rawInput, userNote, tags string, forceReprocess bool) (StashItem, bool, bool, error) {
	trimmedRaw := strings.TrimSpace(rawInput)
	if trimmedRaw == "" {
		return StashItem{}, false, false, errors.New("raw_input is required")
	}

	normURL, domain, candTitle, kind := ParseStashInput(trimmedRaw)
	cleanNote := strings.TrimSpace(userNote)
	cleanTags := NormalizeStashTags(tags)
	now := NowShanghai()

	if kind != StashKindNote && normURL != "" {
		existing, err := s.FindUnarchivedStashByURL(normURL)
		if err != nil {
			return StashItem{}, false, false, err
		}
		if existing != nil {
			if cleanNote != "" {
				if existing.UserNote == "" {
					existing.UserNote = cleanNote
				} else if !strings.Contains(existing.UserNote, cleanNote) {
					existing.UserNote = existing.UserNote + "\n" + cleanNote
				}
			}
			existing.Tags = MergeStashTags(existing.Tags, cleanTags)
			if existing.Title == "" && candTitle != "" {
				existing.Title = candTitle
			}
			shouldDistill := false
			if forceReprocess || existing.Status == StashStatusFailed || existing.Status == StashStatusPending {
				existing.Status = StashStatusProcessing
				existing.ErrorMsg = ""
				shouldDistill = true
			}
			existing.UpdatedAt = now
			saved, uErr := s.UpsertStashItemFull(*existing)
			return saved, true, shouldDistill, uErr
		}
	}

	excerpt := ""
	if kind == StashKindNote {
		excerpt = TruncateRunes(trimmedRaw, MaxStashExcerptRunes)
	}

	item := StashItem{
		ID:             newAssistantID("st"),
		Kind:           kind,
		URL:            normURL,
		Domain:         domain,
		Title:          candTitle,
		RawInput:       TruncateRunes(trimmedRaw, MaxStashExcerptRunes),
		UserNote:       cleanNote,
		AITldr:         "",
		AIPoints:       "[]",
		Points:         []string{},
		ContentExcerpt: excerpt,
		Tags:           cleanTags,
		Status:         StashStatusProcessing,
		ErrorMsg:       "",
		Pinned:         false,
		ReadCount:      0,
		LastReviewedAt: "",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	saved, err := s.UpsertStashItemFull(item)
	if err != nil {
		return StashItem{}, false, false, err
	}
	return saved, false, true, nil
}

// UpsertStashItemFull inserts or updates a StashItem row (also used by backup import and async distillation).
func (s *Store) UpsertStashItemFull(item StashItem) (StashItem, error) {
	now := NowShanghai()
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" {
		item.ID = newAssistantID("st")
	}
	item.Kind = NormalizeStashKind(item.Kind)
	item.Status = NormalizeStashStatus(item.Status)
	item.URL = strings.TrimSpace(item.URL)
	item.Domain = strings.TrimSpace(item.Domain)
	if item.URL != "" && item.Domain == "" {
		if normURL, dom, k, ok := NormalizeStashURL(item.URL); ok {
			item.URL = normURL
			item.Domain = dom
			if item.Kind == "" || item.Kind == StashKindLink {
				item.Kind = k
			}
		}
	}
	item.Title = TruncateRunes(strings.TrimSpace(item.Title), 240)
	item.RawInput = TruncateRunes(strings.TrimSpace(item.RawInput), MaxStashExcerptRunes)
	item.UserNote = TruncateRunes(strings.TrimSpace(item.UserNote), 4000)
	item.AITldr = TruncateRunes(strings.TrimSpace(item.AITldr), 300)
	item.AIPoints = encodeStashPoints(item.Points, item.AIPoints)
	item.Points = decodeStashPoints(item.AIPoints)
	item.ContentExcerpt = TruncateRunes(strings.TrimSpace(item.ContentExcerpt), MaxStashExcerptRunes)
	item.Tags = NormalizeStashTags(item.Tags)
	item.ErrorMsg = TruncateRunes(strings.TrimSpace(item.ErrorMsg), 500)
	if item.ReadCount < 0 {
		item.ReadCount = 0
	}
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	if item.UpdatedAt == "" {
		item.UpdatedAt = now
	}

	pinInt := 0
	if item.Pinned {
		pinInt = 1
	}

	_, err := s.db.Exec(`INSERT INTO stash_items (
			id, kind, url, domain, title, raw_input, user_note,
			ai_tldr, ai_points, content_excerpt, tags, status, error_msg,
			pinned, read_count, last_reviewed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			kind = excluded.kind,
			url = excluded.url,
			domain = excluded.domain,
			title = excluded.title,
			raw_input = excluded.raw_input,
			user_note = excluded.user_note,
			ai_tldr = excluded.ai_tldr,
			ai_points = excluded.ai_points,
			content_excerpt = excluded.content_excerpt,
			tags = excluded.tags,
			status = excluded.status,
			error_msg = excluded.error_msg,
			pinned = excluded.pinned,
			read_count = excluded.read_count,
			last_reviewed_at = excluded.last_reviewed_at,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at`,
		item.ID, item.Kind, item.URL, item.Domain, item.Title, item.RawInput, item.UserNote,
		item.AITldr, item.AIPoints, item.ContentExcerpt, item.Tags, item.Status, item.ErrorMsg,
		pinInt, item.ReadCount, item.LastReviewedAt, item.CreatedAt, item.UpdatedAt,
	)
	return item, err
}

// DeleteStashItem deletes a stash item by ID.
func (s *Store) DeleteStashItem(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM stash_items WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RebuildStashFTS forces SQLite FTS5 to rebuild its trigram inverted index from stash_items (used after backup import).
func (s *Store) RebuildStashFTS() error {
	_, err := s.db.Exec(`INSERT INTO stash_fts(stash_fts) VALUES('rebuild')`)
	return err
}

func buildTrigramFTSMatchExpr(trimmedQuery string) (string, bool) {
	tokens := strings.Fields(trimmedQuery)
	if len(tokens) == 0 {
		return "", false
	}
	quoted := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		cleanTok := strings.Trim(tok, `"'“”‘’`)
		if utf8.RuneCountInString(cleanTok) < 3 {
			// Any token shorter than 3 UTF-8 characters cannot match a 3-gram index; use LIKE path
			return "", false
		}
		escaped := strings.ReplaceAll(cleanTok, `"`, `""`)
		quoted = append(quoted, `"`+escaped+`"`)
	}
	return strings.Join(quoted, " "), true
}

func buildStashFilterClauses(filter, tag, kind string) ([]string, []any) {
	var conds []string
	var args []any

	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "", "all", "active":
		conds = append(conds, "s.status != 'archived'")
	case "unread":
		conds = append(conds, "s.status != 'archived'", "s.read_count = 0")
	case "pinned":
		conds = append(conds, "s.status != 'archived'", "s.pinned = 1")
	case "note", "notes":
		conds = append(conds, "s.status != 'archived'", "s.kind = 'note'")
	case "archived":
		conds = append(conds, "s.status = 'archived'")
	case "ready", "processing", "pending", "failed":
		conds = append(conds, "s.status = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(filter)))
	case "everything":
		// Includes archived items (used by full backup export)
	default:
		conds = append(conds, "s.status != 'archived'")
	}

	if k := strings.ToLower(strings.TrimSpace(kind)); k != "" && k != "all" {
		conds = append(conds, "s.kind = ?")
		args = append(args, NormalizeStashKind(k))
	}

	if t := strings.TrimSpace(tag); t != "" {
		conds = append(conds, "(',' || lower(s.tags) || ',') LIKE ?")
		args = append(args, "%,"+strings.ToLower(t)+",%")
	}

	return conds, args
}

// SearchStashItems searches stash_items with hybrid FTS5 trigram (>=3 runes) and parameterized LIKE fallback (1-2 rune short words or FTS syntax fallback).
func (s *Store) SearchStashItems(query, filter, tag string, limit int) ([]StashItem, error) {
	return s.SearchStashItemsWithKind(query, filter, tag, "", limit)
}

func (s *Store) SearchStashItemsWithKind(query, filter, tag, kind string, limit int) ([]StashItem, error) {
	if limit <= 0 {
		limit = 80
	}
	if limit > 2000 {
		limit = 2000
	}

	baseConds, baseArgs := buildStashFilterClauses(filter, tag, kind)
	trimmedQ := strings.TrimSpace(query)

	// No search query: list directly using B-Tree index
	if trimmedQ == "" {
		where := ""
		if len(baseConds) > 0 {
			where = "WHERE " + strings.Join(baseConds, " AND ")
		}
		args := append(append([]any{}, baseArgs...), limit)
		sqlStr := `SELECT ` + stashSelectCols + ` FROM stash_items s ` + where + `
			ORDER BY s.pinned DESC, s.updated_at DESC LIMIT ?`
		return s.queryStashRows(sqlStr, args...)
	}

	// Path 1: UTF-8 rune count >= 3 (and every whitespace token >= 3 runes) -> FTS5 trigram MATCH with BM25 ranking
	if utf8.RuneCountInString(trimmedQ) >= 3 {
		if matchExpr, ok := buildTrigramFTSMatchExpr(trimmedQ); ok {
			ftsConds := append([]string{"stash_fts MATCH ?"}, baseConds...)
			ftsArgs := append([]any{matchExpr}, baseArgs...)
			ftsArgs = append(ftsArgs, limit)
			ftsSQL := `SELECT ` + stashSelectCols + `
				FROM stash_fts
				JOIN stash_items s ON s.rowid = stash_fts.rowid
				WHERE ` + strings.Join(ftsConds, " AND ") + `
				ORDER BY s.pinned DESC, bm25(stash_fts), s.updated_at DESC
				LIMIT ?`
			if ftsItems, err := s.queryStashRows(ftsSQL, ftsArgs...); err == nil && len(ftsItems) > 0 {
				return ftsItems, nil
			}
		}
	}

	// Path 2: 1-2 char short words (e.g. "双栈", "备份") or FTS fallback -> parameterized LIKE across all indexed fields + URL
	tokens := strings.Fields(trimmedQ)
	likeConds := append([]string{}, baseConds...)
	likeArgs := append([]any{}, baseArgs...)
	for _, tok := range tokens {
		cleanTok := strings.Trim(tok, `"'“”‘’`)
		if cleanTok == "" {
			cleanTok = tok
		}
		pat := "%" + cleanTok + "%"
		likeConds = append(likeConds, `(s.title LIKE ? OR s.ai_tldr LIKE ? OR s.ai_points LIKE ? OR s.tags LIKE ? OR s.user_note LIKE ? OR s.content_excerpt LIKE ? OR s.url LIKE ?)`)
		likeArgs = append(likeArgs, pat, pat, pat, pat, pat, pat, pat)
	}
	likeArgs = append(likeArgs, limit)

	where := ""
	if len(likeConds) > 0 {
		where = "WHERE " + strings.Join(likeConds, " AND ")
	}
	likeSQL := `SELECT ` + stashSelectCols + ` FROM stash_items s ` + where + `
		ORDER BY s.pinned DESC, s.updated_at DESC LIMIT ?`
	return s.queryStashRows(likeSQL, likeArgs...)
}

func (s *Store) queryStashRows(sqlStr string, args ...any) ([]StashItem, error) {
	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]StashItem, 0)
	for rows.Next() {
		if item, sErr := scanStashRow(rows); sErr == nil {
			list = append(list, item)
		}
	}
	return list, rows.Err()
}

// ListTopStashTags aggregates the most frequent tags across unarchived stash items for UI filter pills.
func (s *Store) ListTopStashTags(limit int) ([]StashTagCount, error) {
	if limit <= 0 || limit > 40 {
		limit = 12
	}
	rows, err := s.db.Query(`SELECT tags FROM stash_items WHERE status != 'archived' AND tags != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	display := make(map[string]string)
	for rows.Next() {
		var raw string
		if rows.Scan(&raw) == nil {
			for _, part := range strings.Split(raw, ",") {
				t := strings.TrimSpace(part)
				if t == "" {
					continue
				}
				key := strings.ToLower(t)
				counts[key]++
				if _, exists := display[key]; !exists {
					display[key] = t
				}
			}
		}
	}

	out := make([]StashTagCount, 0, len(counts))
	for k, c := range counts {
		out = append(out, StashTagCount{Tag: display[k], Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PickStashForReview selects 1 ready item created >= 3 days ago, prioritizing unread (read_count = 0) or earliest last_reviewed_at.
func (s *Store) PickStashForReview() (*StashItem, error) {
	threshold := time.Now().In(ShanghaiLocation()).Add(-72 * time.Hour).Format("2006-01-02 15:04:05")
	row := s.db.QueryRow(`SELECT `+stashSelectCols+`
		FROM stash_items s
		WHERE s.status = 'ready' AND s.created_at <= ?
		ORDER BY CASE WHEN s.read_count = 0 THEN 0 ELSE 1 END ASC,
		         CASE WHEN s.last_reviewed_at = '' THEN 0 ELSE 1 END ASC,
		         s.last_reviewed_at ASC,
		         s.pinned DESC,
		         s.created_at ASC
		LIMIT 1`, threshold)
	item, err := scanStashRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) MarkStashReviewed(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE stash_items SET last_reviewed_at = ? WHERE id = ?`, NowShanghai(), trimmed)
	return err
}

// migrateStashToolOnce appends "manage_stash" one time to existing assistant/researcher bots
// whose allowed_tools already include "manage_todos" or "manage_notes".
func (s *Store) migrateStashToolOnce() {
	const markerKey = "migration_stash_tool_v232"
	if v, _ := s.GetConfig(markerKey); v == "1" {
		return
	}
	bots, err := s.ListBots()
	if err != nil {
		return
	}
	for _, b := range bots {
		if b.IsGroup {
			continue
		}
		tools := b.AllowedToolList()
		if len(tools) == 0 {
			continue
		}
		hasAssistant := false
		hasStash := false
		for _, t := range tools {
			if t == "manage_todos" || t == "manage_notes" {
				hasAssistant = true
			}
			if t == "manage_stash" {
				hasStash = true
			}
		}
		if hasAssistant && !hasStash {
			tools = append(tools, "manage_stash")
			if raw, mErr := json.Marshal(tools); mErr == nil {
				b.AllowedTools = string(raw)
				_ = s.SaveBot(b)
			}
		}
	}
	_ = s.SetConfig(markerKey, "1")
}
