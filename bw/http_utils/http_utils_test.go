package http_utils

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheDir(t *testing.T) {
	tests := []struct {
		cwd      string
		expected string
	}{
		{"/tmp", "/tmp/http-cache"},
		{"/home/user", "/home/user/http-cache"},
		{".", "http-cache"},
		{"", "http-cache"},
	}

	for _, test := range tests {
		result := cache_dir(test.cwd)
		assert.Equal(t, test.expected, result, "Wrong cache dir for cwd: %s", test.cwd)
	}
}

func TestCachePath(t *testing.T) {
	tests := []struct {
		cwd       string
		cacheKey  string
		expected  string
	}{
		{"/tmp", "abc123", "/tmp/http-cache/abc123"},
		{"/home/user", "def456", "/home/user/http-cache/def456"},
		{".", "xyz789", "http-cache/xyz789"},
	}

	for _, test := range tests {
		result := cache_path(test.cwd, test.cacheKey)
		assert.Equal(t, test.expected, result, "Wrong cache path for cwd: %s, key: %s", test.cwd, test.cacheKey)
	}
}

func TestMakeCacheKey(t *testing.T) {
	tests := []struct {
		urlStr     string
		pathSuffix string
		expectsOk  bool
	}{
		{"https://example.com/test", "", true},
		{"https://example.com/search?q=test", "-search", true},
		{"https://example.com/file.zip", "-zip", true},
		{"https://example.com/api/release.json", "-release.json", true},
		{"https://example.com/other/file.zip", "-zip", true},
	}

	for _, test := range tests {
		parsedURL, err := url.Parse(test.urlStr)
		assert.NoError(t, err, "Failed to parse URL: %s", test.urlStr)

		req := &http.Request{URL: parsedURL}
		result := make_cache_key(req)

		// Check that result is a hex string (32 chars for MD5)
		if test.pathSuffix == "" {
			assert.Len(t, result, 32, "Cache key should be 32 chars for URL: %s", test.urlStr)
		} else {
			assert.True(t, len(result) > 32, "Cache key should be longer than 32 chars for URL: %s", test.urlStr)
			assert.Contains(t, result, test.pathSuffix, "Cache key should contain suffix for URL: %s", test.urlStr)
		}

		// Check that it's hexadecimal
		for _, char := range result[:32] {
			assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'),
				"Cache key should be hexadecimal, got char: %c", char)
		}
	}
}

func TestMakeCacheKeyConsistent(t *testing.T) {
	// Same URL should produce same cache key
	urlStr := "https://example.com/test?param=value"
	parsedURL, err := url.Parse(urlStr)
	assert.NoError(t, err)

	req := &http.Request{URL: parsedURL}
	key1 := make_cache_key(req)
	key2 := make_cache_key(req)

	assert.Equal(t, key1, key2, "Same URL should produce same cache key")
}

func TestMakeCacheKeyDifferent(t *testing.T) {
	// Different URLs should produce different cache keys
	url1, _ := url.Parse("https://example.com/test1")
	url2, _ := url.Parse("https://example.com/test2")

	req1 := &http.Request{URL: url1}
	req2 := &http.Request{URL: url2}

	key1 := make_cache_key(req1)
	key2 := make_cache_key(req2)

	assert.NotEqual(t, key1, key2, "Different URLs should produce different cache keys")
}