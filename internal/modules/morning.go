package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type MorningModule struct {
	store      *store.Store
	push       *notifier.Client
	http       *http.Client
	mu         sync.RWMutex
	summarizer LightSummarizer
}

func NewMorningModule(st *store.Store, push *notifier.Client) *MorningModule {
	return &MorningModule{store: st, push: push, http: NewHTTPClient(15 * time.Second)}
}

func (m *MorningModule) SetSummarizer(sum LightSummarizer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summarizer = sum
}

func (m *MorningModule) getSummarizer() LightSummarizer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.summarizer
}

type weatherInfo struct {
	City        string
	Condition   string
	CurrentTemp float64
	MinTemp     float64
	MaxTemp     float64
	PrecipProb  int
	WindSpeed   float64
}

func (m *MorningModule) Run(ctx context.Context) (RunResult, error) {
	return m.RunWithTask(ctx, store.Task{
		ID:   "morning",
		Name: "每日早报小秘书",
		Type: "morning",
	})
}

func (m *MorningModule) RunWithTask(ctx context.Context, task store.Task) (RunResult, error) {
	var params map[string]any
	_ = json.Unmarshal([]byte(task.Params), &params)

	city := ""
	if params != nil {
		if c, ok := params["city"].(string); ok {
			city = strings.TrimSpace(c)
		}
	}
	if city == "" {
		city, _ = m.store.GetConfig(store.KeyMorningCity)
	}
	if strings.TrimSpace(city) == "" {
		city = "Shanghai"
	}

	nezhaURL := ""
	if params != nil {
		if n, ok := params["nezha_url"].(string); ok {
			nezhaURL = strings.TrimSpace(n)
		}
	}
	if nezhaURL == "" {
		nezhaURL, _ = m.store.GetConfig(store.KeyNezhaURL)
	}

	taskName := task.Name
	if taskName == "" {
		taskName = "BotHub 晨间简报"
	}

	now := time.Now().In(ShanghaiLocation())
	dateStr := now.Format("2006-01-02")
	weekday := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[now.Weekday()]

	w, wErr := m.fetchWeatherWithFallback(ctx, city)
	nodePulse := ""
	if strings.TrimSpace(nezhaURL) != "" {
		nodePulse = m.fetchNodePulse(ctx, nezhaURL)
	}

	openTodos, _ := m.store.ListTodos("open", "", 20)
	overdueCount := 0
	dueTodayCount := 0
	for _, td := range openTodos {
		if d := strings.TrimSpace(td.DueAt); d != "" {
			day := d
			if len(day) >= 10 {
				day = day[:10]
			}
			if day < dateStr {
				overdueCount++
			} else if day == dateStr {
				dueTodayCount++
			}
		}
	}
	expiring, _ := m.store.GetExpiringAssets(7)
	unreadStashes, _ := m.store.SearchStashItems("", "unread", "", 50)
	reviewItem, rErr := m.store.PickStashForReview()

	// Build structured detail sections first so AI executive summary can synthesize ALL real data and sit at the very top
	var detailBuf strings.Builder

	if len(openTodos) > 0 {
		detailBuf.WriteString("#### 今日行动焦点（待办）\n")
		shown := 0
		for _, td := range openTodos {
			if shown >= 8 {
				detailBuf.WriteString(fmt.Sprintf("- 其余 %d 项待办请在行动台查看\n", len(openTodos)-8))
				break
			}
			dueNote := ""
			if strings.TrimSpace(td.DueAt) != "" {
				dueDay := td.DueAt
				if len(dueDay) >= 10 {
					dueDay = dueDay[:10]
				}
				if dueDay < dateStr {
					dueNote = fmt.Sprintf(" [已逾期 %s]", td.DueAt)
				} else if dueDay == dateStr {
					dueNote = fmt.Sprintf(" [今日截止 %s]", td.DueAt)
				} else {
					dueNote = fmt.Sprintf(" [截止 %s]", td.DueAt)
				}
			}
			detailBuf.WriteString(fmt.Sprintf("- **[%s]** %s%s\n", td.Priority, td.Title, dueNote))
			shown++
		}
		detailBuf.WriteString("\n")
	}

	if len(expiring) > 0 {
		detailBuf.WriteString("#### 临期数字资产（7 天内）\n")
		for _, a := range expiring {
			costPart := ""
			if p := strings.TrimSpace(a.Price); p != "" {
				costPart = " · " + p
			}
			detailBuf.WriteString(fmt.Sprintf("- **%s**（%s%s · 剩余 %d 天）\n", a.Name, a.Spec, costPart, a.DaysLeft))
		}
		detailBuf.WriteString("\n")
	}

	detailBuf.WriteString("#### 气象与出行\n")
	if wErr == nil {
		detailBuf.WriteString(fmt.Sprintf("- **%s**：%s，实时 %.1f°C（今日 %.1f°C ~ %.1f°C），降水概率 %d%%，风速 %.1f km/h\n", w.City, w.Condition, w.CurrentTemp, w.MinTemp, w.MaxTemp, w.PrecipProb, w.WindSpeed))
		detailBuf.WriteString(fmt.Sprintf("- **出行建议**：%s\n\n", buildAdvice(w)))
	} else {
		detailBuf.WriteString(fmt.Sprintf("- **%s**：气象接口暂不可用（%v）\n\n", city, wErr))
	}

	if nodePulse != "" {
		detailBuf.WriteString("#### 节点脉搏\n")
		detailBuf.WriteString(nodePulse)
		detailBuf.WriteString("\n\n")
	}

	if rErr == nil && reviewItem != nil {
		detailBuf.WriteString("#### 收藏匣记忆重温\n")
		titleText := notifier.StripEmoji(strings.TrimSpace(reviewItem.Title))
		if titleText == "" {
			titleText = "未命名收藏条目"
		}
		detailBuf.WriteString(fmt.Sprintf("- **标题**：%s\n", titleText))
		if tldr := notifier.StripEmoji(strings.TrimSpace(reviewItem.AITldr)); tldr != "" {
			detailBuf.WriteString(fmt.Sprintf("- **核心结论**：%s\n", tldr))
		}
		if len(reviewItem.Points) > 0 {
			maxPts := len(reviewItem.Points)
			if maxPts > 3 {
				maxPts = 3
			}
			for i := 0; i < maxPts; i++ {
				pt := notifier.StripEmoji(strings.TrimSpace(reviewItem.Points[i]))
				if pt != "" {
					detailBuf.WriteString(fmt.Sprintf("  - %s\n", pt))
				}
			}
		}
		if u := strings.TrimSpace(reviewItem.URL); u != "" {
			detailBuf.WriteString(fmt.Sprintf("- **原链接**：%s\n", u))
		}
		detailBuf.WriteString("\n")
		_ = m.store.MarkStashReviewed(reviewItem.ID)
	}

	// Build compact 1-line overview strip
	var overviewParts []string
	if wErr == nil {
		overviewParts = append(overviewParts, fmt.Sprintf("%s %s %.0f~%.0f°C", w.City, w.Condition, w.MinTemp, w.MaxTemp))
	}
	if len(openTodos) > 0 {
		todoStat := fmt.Sprintf("待办 %d 项", len(openTodos))
		if overdueCount > 0 || dueTodayCount > 0 {
			todoStat += fmt.Sprintf("（逾期 %d / 今日 %d）", overdueCount, dueTodayCount)
		}
		overviewParts = append(overviewParts, todoStat)
	} else {
		overviewParts = append(overviewParts, "今日无积压待办")
	}
	if len(expiring) > 0 {
		overviewParts = append(overviewParts, fmt.Sprintf("临期资产 %d 项", len(expiring)))
	}
	if len(unreadStashes) > 0 {
		overviewParts = append(overviewParts, fmt.Sprintf("未读收藏 %d 条", len(unreadStashes)))
	}

	// Feed recent 3 morning briefs to AI summarizer so it avoids repeating phrasing across consecutive days
	var pastHint strings.Builder
	if recent, _ := m.store.ListRecentNotificationsByModule(task.ID, 3); len(recent) > 0 {
		pastHint.WriteString("\n\n[近几日早报摘要（禁止重复相同措辞与模板）]\n")
		for i, n := range recent {
			pastHint.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, n.CreatedAt, strings.TrimSpace(n.Summary)))
		}
	}

	sysPrompt := "你是 BotHub 的晨间行动参谋。请基于提供的今日真实数据（待办优先级与逾期项、临期资产、气象出行、节点状态、收藏匣），直接输出 2-3 条高信息密度、立刻可执行的「今日行动建议」（总字数不超过 180 字）。严格规则：1) 禁止寒暄问候与鸡汤格言（严禁出现“早安”“新的一天”“加油”等空话）；2) 优先点名最紧急的逾期/今日待办或临期资产，给出具体先后顺序；3) 严禁输出任何 Emoji。"
	aiInput := fmt.Sprintf("日期：%s (%s)\n概览：%s\n\n%s%s", dateStr, weekday, strings.Join(overviewParts, " · "), detailBuf.String(), pastHint.String())

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### %s · %s (%s)\n\n", taskName, dateStr, weekday))
	sb.WriteString(fmt.Sprintf("> **今日概览**：%s\n\n", strings.Join(overviewParts, " · ")))

	if aiInsight, aiErr := summarizeModuleContent(ctx, m.store, m.getSummarizer(), sysPrompt, aiInput); aiErr == nil && aiInsight != "" {
		sb.WriteString("#### 今日核心行动建议\n")
		sb.WriteString(aiInsight)
		sb.WriteString("\n\n")
	}

	sb.WriteString(strings.TrimSpace(detailBuf.String()))

	title := fmt.Sprintf("%s · %s %s", taskName, dateStr, city)
	if wErr == nil {
		title = fmt.Sprintf("%s · %s %s %.0f~%.0f°C", taskName, dateStr, w.City, w.MinTemp, w.MaxTemp)
	}
	if overdueCount > 0 {
		title += fmt.Sprintf("（%d 项逾期）", overdueCount)
	} else if len(expiring) > 0 {
		title += fmt.Sprintf("（%d 项临期）", len(expiring))
	}

	if m.push == nil {
		return RunResult{Summary: fmt.Sprintf("已生成 %s（城市: %s, 日期: %s）", taskName, city, dateStr)}, nil
	}
	if err := m.push.SendMarkdown(ctx, task.ID, title, sb.String()); err != nil {
		return RunResult{Summary: "推送失败: " + err.Error()}, err
	}
	return RunResult{Summary: fmt.Sprintf("已推送 %s（城市: %s, 日期: %s）", taskName, city, dateStr), ItemsPushed: 1}, nil
}

func (m *MorningModule) fetchWeather(ctx context.Context, city string) (weatherInfo, error) {
	// 1. Geocoding
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh&format=json", url.QueryEscape(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geoURL, nil)
	if err != nil {
		return weatherInfo{}, err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return weatherInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return weatherInfo{}, fmt.Errorf("geocoding HTTP %d", resp.StatusCode)
	}

	var geo struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&geo); err != nil {
		return weatherInfo{}, err
	}
	if len(geo.Results) == 0 {
		return weatherInfo{}, fmt.Errorf("未找到城市: %s", city)
	}
	loc := geo.Results[0]

	// 2. Forecast
	fURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,wind_speed_10m&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max&timezone=Asia%%2FShanghai&forecast_days=1",
		loc.Latitude, loc.Longitude,
	)
	fReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fURL, nil)
	fResp, err := m.http.Do(fReq)
	if err != nil {
		return weatherInfo{}, err
	}
	defer fResp.Body.Close()
	if fResp.StatusCode >= 400 {
		return weatherInfo{}, fmt.Errorf("open-meteo HTTP %d", fResp.StatusCode)
	}

	var fc struct {
		Current struct {
			Temp      float64 `json:"temperature_2m"`
			Code      int     `json:"weather_code"`
			WindSpeed float64 `json:"wind_speed_10m"`
		} `json:"current"`
		Daily struct {
			MaxTemp    []float64 `json:"temperature_2m_max"`
			MinTemp    []float64 `json:"temperature_2m_min"`
			PrecipProb []int     `json:"precipitation_probability_max"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(io.LimitReader(fResp.Body, 512<<10)).Decode(&fc); err != nil || len(fc.Daily.MaxTemp) == 0 {
		return weatherInfo{}, fmt.Errorf("解析气象预报失败")
	}

	precip := 0
	if len(fc.Daily.PrecipProb) > 0 {
		precip = fc.Daily.PrecipProb[0]
	}
	name := loc.Name
	if name == "" {
		name = city
	}

	return weatherInfo{
		City:        name,
		Condition:   wmoCodeCN(fc.Current.Code),
		CurrentTemp: fc.Current.Temp,
		MinTemp:     fc.Daily.MinTemp[0],
		MaxTemp:     fc.Daily.MaxTemp[0],
		PrecipProb:  precip,
		WindSpeed:   fc.Current.WindSpeed,
	}, nil
}

func (m *MorningModule) fetchWeatherWithFallback(ctx context.Context, city string) (weatherInfo, error) {
	// 1. Primary source: Open-Meteo
	w, err := m.fetchWeather(ctx, city)
	if err == nil {
		return w, nil
	}
	// 2. Secondary fallback source: wttr.in (zero key, resilient)
	wFallback, fbErr := m.fetchWeatherWttrIn(ctx, city)
	if fbErr == nil {
		return wFallback, nil
	}
	return weatherInfo{}, fmt.Errorf("主备气象接口均不可用 (open-meteo: %v; wttr.in: %v)", err, fbErr)
}

func (m *MorningModule) fetchWeatherWttrIn(ctx context.Context, city string) (weatherInfo, error) {
	u := fmt.Sprintf("https://wttr.in/%s?format=j1", url.QueryEscape(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return weatherInfo{}, err
	}
	req.Header.Set("User-Agent", "curl/7.68.0")
	resp, err := m.http.Do(req)
	if err != nil {
		return weatherInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return weatherInfo{}, fmt.Errorf("wttr.in HTTP %d", resp.StatusCode)
	}

	var data struct {
		Current []struct {
			TempC        string `json:"temp_C"`
			Windspeed    string `json:"windspeedKmph"`
			WeatherDesc  []struct{ Value string `json:"value"` } `json:"weatherDesc"`
		} `json:"current_condition"`
		Weather []struct {
			MaxTempC string `json:"maxtempC"`
			MinTempC string `json:"mintempC"`
			Hourly   []struct {
				ChanceOfRain string `json:"chanceofrain"`
			} `json:"hourly"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&data); err != nil || len(data.Current) == 0 || len(data.Weather) == 0 {
		return weatherInfo{}, fmt.Errorf("解析 wttr.in 失败")
	}

	curr := data.Current[0]
	today := data.Weather[0]
	cTemp, _ := strconv.ParseFloat(curr.TempC, 64)
	mxTemp, _ := strconv.ParseFloat(today.MaxTempC, 64)
	mnTemp, _ := strconv.ParseFloat(today.MinTempC, 64)
	wSpeed, _ := strconv.ParseFloat(curr.Windspeed, 64)

	precip := 0
	if len(today.Hourly) > 0 {
		precip, _ = strconv.Atoi(today.Hourly[0].ChanceOfRain)
	}
	cond := "晴朗/多云"
	if len(curr.WeatherDesc) > 0 && curr.WeatherDesc[0].Value != "" {
		cond = notifier.StripEmoji(strings.TrimSpace(curr.WeatherDesc[0].Value))
	}

	return weatherInfo{
		City:        city,
		Condition:   cond,
		CurrentTemp: cTemp,
		MinTemp:     mnTemp,
		MaxTemp:     mxTemp,
		PrecipProb:  precip,
		WindSpeed:   wSpeed,
	}, nil
}

func (m *MorningModule) fetchNodePulse(ctx context.Context, nezhaURL string) string {
	base := strings.TrimRight(strings.TrimSpace(nezhaURL), "/")
	if base == "" {
		return "- **监控地址**：未配置探针地址"
	}
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/v1/server/list", nil)
	req.Header.Set("User-Agent", "Vesper-BotHub/1.0")
	if resp, err := m.http.Do(req); err == nil {
		defer resp.Body.Close()
		var res struct {
			Result []struct {
				LastActive int64 `json:"last_active"`
			} `json:"result"`
		}
		if resp.StatusCode < 400 && json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&res) == nil && len(res.Result) > 0 {
			online := 0
			nowUnix := time.Now().Unix()
			for _, n := range res.Result {
				if nowUnix-n.LastActive <= 120 {
					online++
				}
			}
			return fmt.Sprintf("- **哪吒探针**：在线 %d / %d 节点（延迟 %d ms）\n- **面板入口**：%s", online, len(res.Result), time.Since(start).Milliseconds(), base)
		}
	}
	return fmt.Sprintf("- **探针入口**：%s（状态检测中）", base)
}

func buildAdvice(w weatherInfo) string {
	var parts []string
	switch {
	case w.MaxTemp < 12:
		parts = append(parts, "气温偏冷，建议厚大衣保暖")
	case w.MaxTemp < 20:
		parts = append(parts, "气温微凉舒适，适合风衣、夹克或针织外搭")
	case w.MaxTemp < 27:
		parts = append(parts, "体感温和，适合轻便长短袖")
	default:
		parts = append(parts, "气温偏高，注意防晒补水")
	}
	if w.WindSpeed >= 30 {
		parts = append(parts, fmt.Sprintf("风力偏大（%.0f km/h），注意防风保暖", w.WindSpeed))
	}
	if w.PrecipProb >= 40 {
		parts = append(parts, fmt.Sprintf("降水概率 %d%%，出门请带伞", w.PrecipProb))
	} else {
		parts = append(parts, "降水概率低，无需带伞")
	}
	return strings.Join(parts, "；") + "。"
}

func wmoCodeCN(code int) string {
	switch {
	case code == 0:
		return "晴朗"
	case code <= 3:
		return "多云/阴天"
	case code <= 48:
		return "有雾"
	case code <= 67:
		return "降雨"
	case code <= 77:
		return "降雪"
	case code <= 82:
		return "阵雨"
	default:
		return "阴"
	}
}
