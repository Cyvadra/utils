package wecom

import (
	"strings"
	"unicode/utf8"
)

func splitContent(content string, maxBytes int) []string {
	if len(content) <= maxBytes {
		return []string{content}
	}

	chunks := make([]string, 0, len(content)/maxBytes+1)
	for len(content) > maxBytes {
		cut := strings.LastIndex(content[:maxBytes], "\n") + 1
		if cut == 0 {
			cut = runeBoundary(content, maxBytes)
		}
		if cut == 0 {
			// Guard against byte limits smaller than a single multibyte rune:
			// consume one full rune so progress is guaranteed.
			_, width := utf8.DecodeRuneInString(content)
			cut = width
		}
		chunks = append(chunks, content[:cut])
		content = content[cut:]
	}
	return append(chunks, content)
}

func runeBoundary(content string, limit int) int {
	for limit > 0 && !utf8.RuneStart(content[limit]) {
		limit--
	}
	return limit
}
