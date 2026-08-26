package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterGet(t *testing.T) {
	r := New[string]()
	require.NoError(t, r.Register("a", "1"))
	value, ok := r.Get("a")
	require.True(t, ok)
	require.Equal(t, "1", value)
	_, ok = r.Get("missing")
	require.False(t, ok)
}

func TestRegisterDuplicate(t *testing.T) {
	r := New[string]()
	require.NoError(t, r.Register("a", "1"))
	require.ErrorIs(t, r.Register("a", "2"), ErrExists)
}

func TestGetOrCreate(t *testing.T) {
	r := New[int]()
	calls := 0
	create := func() int {
		calls++
		return 42
	}
	require.Equal(t, 42, r.GetOrCreate("k", create))
	require.Equal(t, 42, r.GetOrCreate("k", create))
	require.Equal(t, 1, calls)
}

func TestDeleteAndReset(t *testing.T) {
	r := New[string]()
	require.NoError(t, r.Register("a", "1"))
	r.Delete("a")
	_, ok := r.Get("a")
	require.False(t, ok)
	require.NoError(t, r.Register("a", "1"))
	r.Reset()
	_, ok = r.Get("a")
	require.False(t, ok)
}
