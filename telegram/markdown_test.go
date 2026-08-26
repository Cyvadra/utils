package telegram

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		chunks []string
		err    error
	}{
		{name: "empty", text: "", err: ErrEmptyText},
		{name: "plain text", text: "hello world", chunks: []string{"hello world"}},
		{name: "bold", text: "*bold*", chunks: []string{"*bold*"}},
		{name: "italic", text: "_italic_", chunks: []string{"_italic_"}},
		{name: "underline", text: "__underline__", chunks: []string{"__underline__"}},
		{name: "strikethrough", text: "~strike~", chunks: []string{"~strike~"}},
		{name: "spoiler", text: "||spoiler||", chunks: []string{"||spoiler||"}},
		{name: "inline code", text: "`code`", chunks: []string{"`code`"}},
		{name: "fenced code", text: "```\ncode\n```", chunks: []string{"```\ncode\n```"}},
		{name: "link", text: "[label](https://example.com)", chunks: []string{"[label](https://example.com)"}},
		{name: "nested", text: "*bold _italic_*", chunks: []string{"*bold _italic_*"}},
		{name: "escaped underscore", text: `snake\_case`, chunks: []string{`snake\_case`}},
		{name: "single underscore in word", text: "snake_case", err: ErrInvalidMarkup},
		// Literal delimiters surrounded by whitespace are valid MarkdownV2.
		{name: "literal underscore between spaces", text: "hello _ world", chunks: []string{"hello _ world"}},
		{name: "literal star between spaces", text: "hello * world", chunks: []string{"hello * world"}},
		// Unclosed or malformed formatting is rejected before delivery.
		{name: "unclosed bold", text: "*bold", err: ErrInvalidMarkup},
		{name: "unclosed italic", text: "_italic", err: ErrInvalidMarkup},
		{name: "trailing escape", text: `abc\`, err: ErrInvalidMarkup},
		{name: "unclosed inline code", text: "`code", err: ErrInvalidMarkup},
		{name: "unclosed fenced code", text: "```\ncode", err: ErrInvalidMarkup},
		{name: "invalid link", text: "[no link]", err: ErrInvalidMarkup},
		{name: "unclosed link", text: "[label](https://example.com", err: ErrInvalidMarkup},
		{name: "invalid utf8", text: string([]byte{0xff, 0xfe}), err: ErrInvalidMarkup},
		{name: "oversized code block", text: "```" + strings.Repeat("a", TextLimit+1) + "```", err: ErrCannotSplit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks, err := splitMarkdown(test.text)
			require.ErrorIs(t, err, test.err)
			if test.err != nil {
				return
			}
			require.Equal(t, test.chunks, chunks)
			require.Equal(t, test.text, joinChunks(chunks))
			for _, chunk := range chunks {
				require.LessOrEqual(t, renderedLength(chunk), TextLimit)
			}
		})
	}
}

func TestSplitMarkdownLongMessageCloseAndReopen(t *testing.T) {
	// A long bold message is split into chunks, each with formatting closed
	// and reopened so every chunk is independently valid.
	text := "*" + strings.Repeat("a", TextLimit-2) + " " + strings.Repeat("b", TextLimit) + "*"
	chunks, err := splitMarkdown(text)
	require.NoError(t, err)
	require.Greater(t, len(chunks), 1)

	var joined string
	for _, chunk := range chunks {
		require.True(t, strings.HasPrefix(chunk, "*"))
		require.True(t, strings.HasSuffix(chunk, "*"))
		joined += strings.TrimSuffix(strings.TrimPrefix(chunk, "*"), "*")
	}
	require.Equal(t, strings.Repeat("a", TextLimit-2)+" "+strings.Repeat("b", TextLimit), joined)
}

func TestSplitMarkdownUnderscoreInsideWordIsItalic(t *testing.T) {
	// Telegram treats an unescaped underscore inside a word as italic.
	chunks, err := splitMarkdown("foo_bar_baz")
	require.NoError(t, err)
	require.Equal(t, []string{"foo_bar_baz"}, chunks)
}

func TestSplitMarkdownCountsVisibleNotRawLength(t *testing.T) {
	// The splitter measures the visible label, not the URL, so the raw chunk
	// length can exceed TextLimit. This documents the known limitation.
	text := "[" + strings.Repeat("x", TextLimit) + "](https://e.example)"
	chunks, err := splitMarkdown(text)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	require.Greater(t, len(chunks[0]), TextLimit)
}
