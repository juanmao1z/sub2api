//go:build unit && embed

package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTMLCacheRejectsWriteFromInvalidatedSettingsGeneration(t *testing.T) {
	cache := NewHTMLCache()
	cache.SetBaseHTML([]byte("base"))
	version := cache.Version()
	cache.Invalidate()
	require.False(t, cache.SetIfVersion([]byte("stale"), []byte(`{"site":"old"}`), version))
	require.Nil(t, cache.Get())
	require.True(t, cache.SetIfVersion([]byte("fresh"), []byte(`{"site":"new"}`), cache.Version()))
	require.Equal(t, []byte("fresh"), cache.Get().Content)
}
