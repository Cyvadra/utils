package wecom

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestSplitContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		limit    int
		expected []string
	}{
		{
			name:     "empty content",
			content:  "",
			limit:    1,
			expected: []string{""},
		},
		{
			name:     "content at limit",
			content:  "hello",
			limit:    5,
			expected: []string{"hello"},
		},
		{
			name:     "prefers newline boundary",
			content:  "first\nsecond",
			limit:    8,
			expected: []string{"first\n", "second"},
		},
		{
			name:     "splits a long unicode line at rune boundaries",
			content:  strings.Repeat("你", 5),
			limit:    7,
			expected: []string{"你你", "你你", "你"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks := splitContent(test.content, test.limit)
			require.Equal(t, test.expected, chunks)
			require.Equal(t, test.content, strings.Join(chunks, ""))
			for _, chunk := range chunks {
				require.LessOrEqual(t, len(chunk), test.limit)
				require.True(t, utf8.ValidString(chunk))
			}
		})
	}
}
