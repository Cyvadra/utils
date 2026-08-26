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
	open := make([]string, 0)
	prevNonSpace := false
	for index := 0; index < len(text); {
		// Escaped character: emit as a single literal unit.
		if text[index] == '\\' {
			if index+1 >= len(text) {
				return nil, fmt.Errorf("%w: trailing MarkdownV2 escape", ErrInvalidMarkup)
			}
			escaped, width := utf8.DecodeRuneInString(text[index+1:])
			units = append(units, markupUnit{raw: text[index : index+1+width], visible: 1})
			prevNonSpace = !unicode.IsSpace(escaped)
			index += 1 + width
			continue
		}
		// Fenced code block.
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
			units = append(units, markupUnit{raw: raw, visible: renderedLength(content), safe: true})
			prevNonSpace = lastRuneNonSpace(content)
			index += len(raw)
			continue
		}
		// Inline code span.
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
			prevNonSpace = lastRuneNonSpace(content)
			index += len(raw)
			continue
		}
		// Inline link.
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
			prevNonSpace = lastRuneNonSpace(label)
			index = rawEnd
			continue
		}

		// Formatting delimiter. Whether it opens, closes, or is a literal
		// character follows Telegram's MarkdownV2 word-boundary rules, so a
		// bare delimiter surrounded by spaces (e.g. "hello _ world") is literal.
		if delimiter := markdownDelimiter(text[index:]); delimiter != "" {
			unit, closer := markdownDelimiterUnit(text, index, delimiter, open, prevNonSpace)
			switch {
			case closer:
				open = open[:len(open)-1]
			case unit.open != "":
				open = append(open, delimiter)
			default:
				prevNonSpace = true
			}
			units = append(units, unit)
			index += len(delimiter)
			continue
		}

		runeValue, width := utf8.DecodeRuneInString(text[index:])
		if runeValue == utf8.RuneError && width == 1 {
			return nil, fmt.Errorf("%w: invalid UTF-8", ErrInvalidMarkup)
		}
		units = append(units, markupUnit{raw: text[index : index+width], visible: 1, safe: unicode.IsSpace(runeValue)})
		prevNonSpace = !unicode.IsSpace(runeValue)
		index += width
	}
	if len(open) != 0 {
		return nil, fmt.Errorf("%w: unclosed MarkdownV2 delimiter", ErrInvalidMarkup)
	}
	return units, nil
}

// markdownDelimiterUnit classifies a delimiter as an opener, a closer, or a
// literal character. It returns the unit and whether it closed an open entity.
// An opener requires a following non-whitespace character; a closer requires
// the open entity's content to end with a non-whitespace character.
func markdownDelimiterUnit(text string, index int, delimiter string, open []string, prevNonSpace bool) (markupUnit, bool) {
	topOpen := len(open) > 0 && open[len(open)-1] == delimiter
	nextNonSpace := nextCharNonSpace(text[index+len(delimiter):])
	unit := markupUnit{raw: delimiter, visible: 1}
	switch {
	case topOpen && prevNonSpace:
		unit.open, unit.close = delimiter, delimiter
		return unit, true
	case !topOpen && nextNonSpace:
		unit.open, unit.close = delimiter, delimiter
		return unit, false
	default:
		return unit, false
	}
}

func nextCharNonSpace(text string) bool {
	if text == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text)
	return !unicode.IsSpace(r)
}

func lastRuneNonSpace(text string) bool {
	if text == "" {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(text)
	return !unicode.IsSpace(last)
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
		hasContent := false
		working := append([]string(nil), active...)
		for end < len(units) {
			unit := units[end]
			if visible+unit.visible > TextLimit {
				break
			}
			visible += unit.visible
			if unit.open == "" {
				hasContent = true
			} else if len(working) > 0 && working[len(working)-1] == unit.open {
				working = working[:len(working)-1]
			} else {
				working = append(working, unit.open)
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
		if !hasContent {
			// A run of formatting delimiters with no visible content only
			// closes the active state; the previous chunk already emitted the
			// matching closing delimiters, so nothing is appended.
			active = applyMarkupState(active, units[start:end])
			start = end
			continue
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
