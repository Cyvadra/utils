package telegram

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
		err      error
	}{
		{name: "empty", text: "", err: ErrEmptyText},
		{name: "at limit", text: strings.Repeat("a", TextLimit), expected: []string{strings.Repeat("a", TextLimit)}},
		{name: "prefers newline", text: strings.Repeat("a", TextLimit-2) + "\n" + "tail", expected: []string{strings.Repeat("a", TextLimit-2) + "\n", "tail"}},
		{name: "unicode", text: strings.Repeat("你", TextLimit+1), expected: []string{strings.Repeat("你", TextLimit), "你"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks, err := splitText(test.text)
			require.ErrorIs(t, err, test.err)
			if test.err != nil {
				return
			}
			require.Equal(t, test.expected, chunks)
			require.Equal(t, test.text, joinChunks(chunks))
			for _, chunk := range chunks {
				require.LessOrEqual(t, renderedLength(chunk), TextLimit)
			}
		})
	}
}
