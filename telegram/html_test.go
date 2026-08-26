package telegram

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitHTML(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		chunks []string
		err    error
	}{
		{name: "empty", text: "", err: ErrEmptyText},
		{name: "plain", text: "hello", chunks: []string{"hello"}},
		{name: "bold", text: "<b>bold</b>", chunks: []string{"<b>bold</b>"}},
		{name: "nested", text: "<b><i>nested</i></b>", chunks: []string{"<b><i>nested</i></b>"}},
		{name: "link with attribute", text: `<a href="https://example.com">link</a>`, chunks: []string{`<a href="https://example.com">link</a>`}},
		{name: "entity", text: "a &amp; b", chunks: []string{"a &amp; b"}},
		{name: "unsupported tag", text: "<br>", err: ErrInvalidMarkup},
		{name: "mismatched close", text: "<b>x</i>", err: ErrInvalidMarkup},
		{name: "unclosed tag", text: "<b>x", err: ErrInvalidMarkup},
		{name: "comment", text: "<!-- x -->", err: ErrInvalidMarkup},
		{name: "invalid utf8", text: string([]byte{0xff}), err: ErrInvalidMarkup},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks, err := splitHTML(test.text)
			require.ErrorIs(t, err, test.err)
			if test.err != nil {
				return
			}
			require.Equal(t, test.chunks, chunks)
			require.Equal(t, test.text, joinChunks(chunks))
		})
	}
}

func TestSplitHTMLLongMessageCloseAndReopen(t *testing.T) {
	text := "<b>" + strings.Repeat("a", TextLimit-4) + " " + strings.Repeat("b", TextLimit) + "</b>"
	chunks, err := splitHTML(text)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	var joined string
	for _, chunk := range chunks {
		require.True(t, strings.HasPrefix(chunk, "<b>"))
		require.True(t, strings.HasSuffix(chunk, "</b>"))
		joined += strings.TrimSuffix(strings.TrimPrefix(chunk, "<b>"), "</b>")
	}
	require.Equal(t, strings.Repeat("a", TextLimit-4)+" "+strings.Repeat("b", TextLimit), joined)
}

func TestSplitHTMLLinkPreservesAttribute(t *testing.T) {
	text := `<a href="https://example.com">` + strings.Repeat("x", TextLimit) + `</a>`
	chunks, err := splitHTML(text)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Equal(t, text, chunks[0])
}
