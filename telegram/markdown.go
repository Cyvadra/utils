package telegram

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type markupUnit struct {
	raw     string
	visible int
	open    string
	close   string
	safe    bool
}

func splitMarkdown(text string) ([]string, error) {
	if text == "" {
		return nil, ErrEmptyText
	}
	units, err := markdownUnits(text)
	if err != nil {
		return nil, err
	}
	return splitMarkupUnits(units)
}

func markdownUnits(text string) ([]markupUnit, error) {
	units := make([]markupUnit, 0, len(text))
	for index := 0; index < len(text); {
		if text[index] == '\\' {
			if index+1 >= len(text) {
				return nil, fmt.Errorf("%w: trailing MarkdownV2 escape", ErrInvalidMarkup)
			}
			_, width := utf8.DecodeRuneInString(text[index+1:])
			units = append(units, markupUnit{raw: text[index : index+1+width], visible: 1})
			index += 1 + width
			continue
		}
		if strings.HasPrefix(text[index:], "```") {
			end := strings.Index(text[index+3:], "```")
			if end < 0 {
				return nil, fmt.Errorf("%w: unclosed MarkdownV2 code block", ErrInvalidMarkup)
			}
			raw := text[index : index+3+end+3]
			content := text[index+3 : index+3+end]
			if renderedLength(content) > TextLimit {
				return nil, ErrCannotSplit
			}
			units = append(units, markupUnit{raw: raw, visible: renderedLength(content)})
			index += len(raw)
			continue
		}
		if text[index] == '`' {
			end := strings.IndexByte(text[index+1:], '`')
			if end < 0 {
				return nil, fmt.Errorf("%w: unclosed MarkdownV2 inline code", ErrInvalidMarkup)
			}
			raw := text[index : index+end+2]
			content := text[index+1 : index+end+1]
			if renderedLength(content) > TextLimit {
				return nil, ErrCannotSplit
			}
			units = append(units, markupUnit{raw: raw, visible: renderedLength(content)})
			index += len(raw)
			continue
		}
		if text[index] == '[' {
			endLabel := strings.Index(text[index:], "](")
			if endLabel < 0 {
				return nil, fmt.Errorf("%w: invalid MarkdownV2 link", ErrInvalidMarkup)
			}
			urlStart := index + endLabel + 2
			urlEnd := strings.IndexByte(text[urlStart:], ')')
			if urlEnd < 0 {
				return nil, fmt.Errorf("%w: unclosed MarkdownV2 link", ErrInvalidMarkup)
			}
			rawEnd := urlStart + urlEnd + 1
			label := text[index+1 : index+endLabel]
			if renderedLength(label) > TextLimit {
				return nil, ErrCannotSplit
			}
			units = append(units, markupUnit{raw: text[index:rawEnd], visible: renderedLength(label)})
			index = rawEnd
			continue
		}

		delimiter := markdownDelimiter(text[index:])
		if delimiter != "" {
			units = append(units, markupUnit{raw: delimiter, open: delimiter, close: delimiter})
			index += len(delimiter)
			continue
		}
		runeValue, width := utf8.DecodeRuneInString(text[index:])
		if runeValue == utf8.RuneError && width == 1 {
			return nil, fmt.Errorf("%w: invalid UTF-8", ErrInvalidMarkup)
		}
		units = append(units, markupUnit{raw: text[index : index+width], visible: 1, safe: unicode.IsSpace(runeValue)})
		index += width
	}
	return units, nil
}

func markdownDelimiter(text string) string {
	for _, delimiter := range []string{"||", "__", "*", "_", "~"} {
		if strings.HasPrefix(text, delimiter) {
			return delimiter
		}
	}
	return ""
}

func splitMarkupUnits(units []markupUnit) ([]string, error) {
	var chunks []string
	active := make([]string, 0)
	for start := 0; start < len(units); {
		visible := 0
		end := start
		lastSafe := -1
		working := append([]string(nil), active...)
		for end < len(units) {
			unit := units[end]
			if visible+unit.visible > TextLimit {
				break
			}
			visible += unit.visible
			if unit.open != "" {
				if len(working) > 0 && working[len(working)-1] == unit.open {
					working = working[:len(working)-1]
				} else {
					working = append(working, unit.open)
				}
			}
			end++
			if unit.safe {
				lastSafe = end
			}
		}
		if end == start {
			return nil, ErrCannotSplit
		}
		if end < len(units) && lastSafe > start {
			end = lastSafe
			working = applyMarkupState(active, units[start:end])
		}

		var builder strings.Builder
		for _, delimiter := range active {
			builder.WriteString(delimiter)
		}
		for _, unit := range units[start:end] {
			builder.WriteString(unit.raw)
		}
		for index := len(working) - 1; index >= 0; index-- {
			builder.WriteString(working[index])
		}
		chunks = append(chunks, builder.String())
		active = working
		start = end
	}
	if len(active) != 0 {
		return nil, fmt.Errorf("%w: unclosed MarkdownV2 delimiter", ErrInvalidMarkup)
	}
	return chunks, nil
}

func applyMarkupState(active []string, units []markupUnit) []string {
	state := append([]string(nil), active...)
	for _, unit := range units {
		if unit.open == "" {
			continue
		}
		if len(state) > 0 && state[len(state)-1] == unit.open {
			state = state[:len(state)-1]
		} else {
			state = append(state, unit.open)
		}
	}
	return state
}
