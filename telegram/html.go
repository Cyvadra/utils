package telegram

import (
	"fmt"
	"html"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

type htmlTag struct {
	name string
	raw  string
}

var supportedHTMLTags = map[string]bool{
	"a": true, "b": true, "blockquote": true, "code": true, "del": true,
	"em": true, "i": true, "ins": true, "pre": true, "s": true,
	"span": true, "strike": true, "strong": true, "tg-emoji": true,
	"tg-spoiler": true, "tg-time": true, "u": true,
}

func splitHTML(text string) ([]string, error) {
	if text == "" {
		return nil, ErrEmptyText
	}
	units, err := htmlUnits(text)
	if err != nil {
		return nil, err
	}
	return splitHTMLUnits(units)
}

func htmlUnits(text string) ([]markupUnit, error) {
	tokenizer := xhtml.NewTokenizer(strings.NewReader(text))
	units := make([]markupUnit, 0, len(text))
	stack := make([]htmlTag, 0)
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case xhtml.ErrorToken:
			if tokenizer.Err() != nil && tokenizer.Err().Error() != "EOF" {
				return nil, fmt.Errorf("%w: %v", ErrInvalidMarkup, tokenizer.Err())
			}
			if len(stack) != 0 {
				return nil, fmt.Errorf("%w: unclosed HTML tag", ErrInvalidMarkup)
			}
			return units, nil
		case xhtml.TextToken:
			textUnits, err := htmlTextUnits(string(tokenizer.Raw()))
			if err != nil {
				return nil, err
			}
			units = append(units, textUnits...)
		case xhtml.StartTagToken:
			token := tokenizer.Token()
			if !supportedHTMLTags[token.Data] {
				return nil, fmt.Errorf("%w: unsupported HTML tag <%s>", ErrInvalidMarkup, token.Data)
			}
			raw := string(tokenizer.Raw())
			stack = append(stack, htmlTag{name: token.Data, raw: raw})
			units = append(units, markupUnit{raw: raw, open: raw, close: "</" + token.Data + ">"})
		case xhtml.EndTagToken:
			token := tokenizer.Token()
			if len(stack) == 0 || stack[len(stack)-1].name != token.Data {
				return nil, fmt.Errorf("%w: mismatched HTML closing tag", ErrInvalidMarkup)
			}
			stack = stack[:len(stack)-1]
			units = append(units, markupUnit{raw: string(tokenizer.Raw()), open: "</" + token.Data + ">", close: ""})
		default:
			return nil, fmt.Errorf("%w: unsupported HTML token", ErrInvalidMarkup)
		}
	}
}

func htmlTextUnits(text string) ([]markupUnit, error) {
	units := make([]markupUnit, 0, len(text))
	for index := 0; index < len(text); {
		if text[index] == '&' {
			end := strings.IndexByte(text[index:], ';')
			if end > 0 {
				raw := text[index : index+end+1]
				decoded := html.UnescapeString(raw)
				if decoded != raw {
					units = append(units, markupUnit{raw: raw, visible: renderedLength(decoded), safe: strings.TrimSpace(decoded) == ""})
					index += len(raw)
					continue
				}
			}
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

func splitHTMLUnits(units []markupUnit) ([]string, error) {
	var chunks []string
	active := make([]htmlTag, 0)
	for start := 0; start < len(units); {
		visible := 0
		end := start
		lastSafe := -1
		working := append([]htmlTag(nil), active...)
		for end < len(units) {
			unit := units[end]
			if visible+unit.visible > TextLimit {
				break
			}
			visible += unit.visible
			updateHTMLState(&working, unit)
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
			working = applyHTMLState(active, units[start:end])
		}

		var builder strings.Builder
		for _, tag := range active {
			builder.WriteString(tag.raw)
		}
		for _, unit := range units[start:end] {
			builder.WriteString(unit.raw)
		}
		for index := len(working) - 1; index >= 0; index-- {
			builder.WriteString("</")
			builder.WriteString(working[index].name)
			builder.WriteString(">")
		}
		chunks = append(chunks, builder.String())
		active = working
		start = end
	}
	if len(active) != 0 {
		return nil, fmt.Errorf("%w: unclosed HTML tag", ErrInvalidMarkup)
	}
	return chunks, nil
}

func updateHTMLState(state *[]htmlTag, unit markupUnit) {
	if unit.open == "" {
		return
	}
	if strings.HasPrefix(unit.open, "</") {
		*state = (*state)[:len(*state)-1]
		return
	}
	tokenizer := xhtml.NewTokenizer(strings.NewReader(unit.open))
	if tokenizer.Next() == xhtml.StartTagToken {
		*state = append(*state, htmlTag{name: tokenizer.Token().Data, raw: unit.raw})
	}
}

func applyHTMLState(active []htmlTag, units []markupUnit) []htmlTag {
	state := append([]htmlTag(nil), active...)
	for _, unit := range units {
		updateHTMLState(&state, unit)
	}
	return state
}
