package http_utils

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCachePath(t *testing.T) {
	cases := []struct {
		dir      string
		key      string
		expected string
	}{
		{"/tmp/cache", "abc123", "/tmp/cache/abc123"},
		{".", "xyz789", "xyz789"},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, cache_path(c.dir, c.key))
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

// returns a test server counting its requests and answering with `status` and `body`.
func counting_server(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	count := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, count
}

func TestFileCachingRequest__fresh_entry_hits(t *testing.T) {
	srv, count := counting_server(t, 200, "hello")
	now := time.Now()
	client := &http.Client{Transport: &FileCachingRequest{Dir: t.TempDir(), Now: func() time.Time { return now }}}

	for range 2 {
		resp, err := Download(client, srv.URL+"/thing", nil)
		assert.NoError(t, err)
		assert.Equal(t, "hello", resp.Text)
	}
	assert.Equal(t, int32(1), count.Load())
}

func TestFileCachingRequest__expired_entry_misses(t *testing.T) {
	srv, count := counting_server(t, 200, "hello")
	now := time.Now()
	x := &FileCachingRequest{Dir: t.TempDir(), Now: func() time.Time { return now }}
	client := &http.Client{Transport: x}

	_, err := Download(client, srv.URL+"/thing", nil)
	assert.NoError(t, err)

	now = now.Add(2 * time.Hour)
	_, err = Download(client, srv.URL+"/thing", nil)
	assert.NoError(t, err)

	assert.Equal(t, int32(2), count.Load())
}

// an expired entry is fetched again and the cache holds the new response.
func TestFileCachingRequest__expired_entry_replaced(t *testing.T) {
	body := "hello"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	now := time.Now()
	client := &http.Client{Transport: &FileCachingRequest{Dir: t.TempDir(), Now: func() time.Time { return now }}}

	_, err := Download(client, srv.URL+"/thing", nil)
	assert.NoError(t, err)

	body = "hello again"
	now = now.Add(2 * time.Hour)
	actual, err := Download(client, srv.URL+"/thing", nil)
	assert.NoError(t, err)
	assert.Equal(t, "hello again", actual.Text)

	// fresh again, the replaced entry is served from the cache
	body = "not requested"
	actual, err = Download(client, srv.URL+"/thing", nil)
	assert.NoError(t, err)
	assert.Equal(t, "hello again", actual.Text)
}

func TestFileCachingRequest__non_2xx_not_cached(t *testing.T) {
	srv, count := counting_server(t, 500, "boom")
	dir := t.TempDir()
	client := &http.Client{Transport: &FileCachingRequest{Dir: dir}}

	for range 2 {
		resp, err := Download(client, srv.URL+"/thing", nil)
		assert.NoError(t, err)
		assert.Equal(t, 500, resp.StatusCode)
	}
	assert.Equal(t, int32(2), count.Load())
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestPruneCache(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old_path := filepath.Join(dir, "old")
	new_path := filepath.Join(dir, "new")
	os.WriteFile(old_path, []byte("x"), 0o644)
	os.WriteFile(new_path, []byte("x"), 0o644)
	os.Chtimes(old_path, now.Add(-2*time.Hour), now.Add(-2*time.Hour))

	actual, err := PruneCache(dir, time.Hour, now)
	assert.NoError(t, err)
	assert.Equal(t, 1, actual)
	assert.NoFileExists(t, old_path)
	assert.FileExists(t, new_path)
}

func TestPruneCache__missing_dir(t *testing.T) {
	actual, err := PruneCache(filepath.Join(t.TempDir(), "nope"), time.Hour, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, 0, actual)
}

func TestDownloadFile(t *testing.T) {
	srv, _ := counting_server(t, 200, "zipbytes")
	dir := t.TempDir()
	dest := filepath.Join(dir, "addon.zip")
	client := &http.Client{Transport: &FileCachingRequest{Dir: filepath.Join(dir, "cache")}}

	err := DownloadFile(client, srv.URL+"/addon.zip", dest, nil)
	assert.NoError(t, err)

	actual, _ := os.ReadFile(dest)
	assert.Equal(t, "zipbytes", string(actual))
	// downloaded files are not cached, and no temporary file is left behind
	assert.NoDirExists(t, filepath.Join(dir, "cache"))
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
}

func TestDownloadFile__not_found_creates_nothing(t *testing.T) {
	srv, _ := counting_server(t, 404, "nope")
	dir := t.TempDir()
	dest := filepath.Join(dir, "addon.zip")

	err := DownloadFile(&http.Client{}, srv.URL+"/addon.zip", dest, nil)
	assert.Error(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}

func TestDownloadFile__failure_preserves_existing(t *testing.T) {
	// the server promises more bytes than it sends, so the transfer fails part way
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(200)
		w.Write([]byte("partial"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "catalogue.json")
	os.WriteFile(dest, []byte("previous"), 0o644)

	err := DownloadFile(&http.Client{}, srv.URL+"/catalogue.json", dest, nil)
	assert.Error(t, err)
	actual, _ := os.ReadFile(dest)
	assert.Equal(t, "previous", string(actual))
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
}

func TestUserAgent(t *testing.T) {
	var actual string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	prev := UserAgent()
	defer func() { user_agent = prev }()
	SetUserAgent("strongbox", "8.0.0", "https://github.com/ogri-la/strongbox")

	Download(&http.Client{}, srv.URL, nil)
	assert.Equal(t, "strongbox/8.0.0 (https://github.com/ogri-la/strongbox)", actual)
}

func TestFixtureTransport(t *testing.T) {
	ft := NewFixtureTransport(map[string]Fixture{
		"https://example.org/a": {Body: []byte("a")},
	})
	client := &http.Client{Transport: ft}

	resp, err := Download(client, "https://example.org/a", nil)
	assert.NoError(t, err)
	assert.Equal(t, "a", resp.Text)

	_, err = Download(client, "https://example.org/b", nil)
	assert.Error(t, err)
	assert.Equal(t, []string{"https://example.org/b"}, ft.Unrouted())

	ft.Set("https://example.org/b", Fixture{Status: 404})
	resp, err = Download(client, "https://example.org/b", nil)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, []string{"https://example.org/a", "https://example.org/b", "https://example.org/b"}, ft.Requested())
}
