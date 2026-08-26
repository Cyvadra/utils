package telegram

import "strings"

func splitText(text string) ([]string, error) {
	if text == "" {
		return nil, ErrEmptyText
	}
	characters := []rune(text)
	if len(characters) <= TextLimit {
		return []string{text}, nil
	}

	chunks := make([]string, 0, len(characters)/TextLimit+1)
	for len(characters) > TextLimit {
		cut := TextLimit
		for index := TextLimit - 1; index >= 0; index-- {
			if characters[index] == '\n' {
				cut = index + 1
				break
			}
		}
		chunks = append(chunks, string(characters[:cut]))
		characters = characters[cut:]
	}
	return append(chunks, string(characters)), nil
}

func renderedLength(text string) int {
	return len([]rune(text))
}

func joinChunks(chunks []string) string {
	return strings.Join(chunks, "")
}
