package probe

import "unicode"

const minReasonableLatencyMs = 50

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// isContentMeaningful returns false if content is empty, whitespace-only,
// zero-width characters only, non-breaking spaces only, or has fewer than
// 1 visible character after stripping all whitespace.
func isContentMeaningful(content string) bool {
	visibleCount := 0
	for _, r := range content {
		if r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff' || r == '\u00a0' {
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		visibleCount++
	}
	return visibleCount >= 1
}

// stripBOM removes a leading UTF-8 BOM (0xEF 0xBB 0xBF) from body if present.
// Some API proxies (especially Chinese OneAPI-based providers) prepend a BOM to
// JSON responses, which causes json.Unmarshal to fail.
func stripBOM(body []byte) []byte {
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		return body[3:]
	}
	return body
}
