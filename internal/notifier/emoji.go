package notifier

import (
	"strings"
	"unicode"
)

// StripEmoji removes Unicode emoji/pictograph runes and variation selectors to
// enforce the STRICT NO EMOJI policy across all push dispatches.
func StripEmoji(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isEmojiRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isEmojiRune(r rune) bool {
	if r == 0xFE0F || r == 0xFE0E || r == 0x200D || r == 0x20E3 || r == 0x203C || r == 0x2049 {
		return true
	}
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF: // Misc symbols, pictographs, emoticons, transport, supplemental
		return true
	case r >= 0x1F000 && r <= 0x1F0FF: // Mahjong, Domino, Playing Cards
		return true
	case r >= 0x2600 && r <= 0x27BF: // Misc symbols & Dingbats
		return true
	case r >= 0x2300 && r <= 0x23FF: // Misc technical (watches, hourglasses, etc.)
		return true
	case r >= 0x2B00 && r <= 0x2BFF: // Misc symbols and arrows
		return true
	case unicode.In(r, unicode.So) && r > 0x2000:
		return true
	}
	return false
}
