package probe

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripBOM removes a leading UTF-8 BOM (0xEF 0xBB 0xBF) from body if present.
// Some API proxies (especially Chinese OneAPI-based providers) prepend a BOM to
// JSON responses, which causes json.Unmarshal to fail.
func stripBOM(body []byte) []byte {
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		return body[3:]
	}
	return body
}
