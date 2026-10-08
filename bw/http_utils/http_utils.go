// HTTP requests with an on-disk response cache.
// use `Download` for a request whose body is wanted in memory, and `DownloadFile` to
// stream a response straight to disk.
// caching is applied by installing `FileCachingRequest` as a client's transport.
package http_utils

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// how long a cached response is used before it is fetched again.
const DEFAULT_CACHE_MAX_AGE = 1 * time.Hour

// sent with every request. set it with `SetUserAgent`.
var user_agent = "bw/unreleased (https://github.com/ogri-la/strongbox)"

// sets the User-Agent sent with every request to "name/version (url)".
func SetUserAgent(name string, version string, url string) {
	user_agent = fmt.Sprintf("%s/%s (%s)", name, version, url)
}

// returns the User-Agent sent with every request.
func UserAgent() string {
	return user_agent
}

// convenience wrapper around a `http.Response`.
type ResponseWrapper struct {
	*http.Response
	Bytes []byte
	Text  string
}

// logs whether the HTTP request's underlying TCP connection was re-used.
func trace_context(ctx context.Context) context.Context {
	client_tracer := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			slog.Debug("HTTP connection reuse", "reused", info.Reused, "remote", info.Conn.RemoteAddr())
		},
	}
	return httptrace.WithClientTrace(ctx, client_tracer)
}

// returns a transport that gives up on hosts that do not connect or respond in time.
// there is no overall deadline: large files take as long as they take.
func DefaultTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = 10 * time.Second
	t.ResponseHeaderTimeout = 30 * time.Second
	return t
}

// --- caching

// a context key marking a request whose response must not be cached.
type no_cache_key struct{}

// returns `ctx` marked so `FileCachingRequest` neither reads nor writes the cache.
func NoCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, no_cache_key{}, true)
}

// returns `true` when `ctx` was marked with `NoCache`.
func is_no_cache(ctx context.Context) bool {
	val, _ := ctx.Value(no_cache_key{}).(bool)
	return val
}

// returns a path to the given `cache_key` in the cache directory `dir`.
func cache_path(dir string, cache_key string) string {
	return filepath.Join(dir, cache_key) // "/path/to/cache/711f20df1f76da140218e51445a6fc47"
}

// returns a cache key unique to the given `req` URL, including its query parameters.
// the key is an MD5 hash, so it is safe to use as a filename, suffixed by request type so
// each type can expire on its own schedule.
// the URL is hashed as-is: inconsistent casing or parameter order causes cache misses.
func make_cache_key(req *http.Request) string {
	key := req.URL.String()
	md5sum := md5.Sum([]byte(key))
	cache_key := hex.EncodeToString(md5sum[:]) // fb9f36f59023fbb3681a895823ae9ba0
	if strings.HasPrefix(req.URL.Path, "/search") {
		return cache_key + "-search" // fb9f36f59023fbb3681a895823ae9ba0-search
	}
	if strings.HasSuffix(req.URL.Path, ".zip") {
		return cache_key + "-zip"
	}
	if strings.HasSuffix(req.URL.Path, "/release.json") {
		return cache_key + "-release.json"
	}
	return cache_key
}

// reads the cached response as if it were the result of `httputil.DumpResponse`,
// a status code, followed by a series of headers, followed by the response body.
// the entry is read into memory so no file handle outlives the call.
func read_cache_entry(dir string, cache_key string) (*http.Response, error) {
	b, err := os.ReadFile(cache_path(dir, cache_key))
	if err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewReader(b)), nil)
}

// returns `true` when the cache entry at `path` was written `max_age` or more before
// `now`, or cannot be inspected.
func cache_expired(path string, max_age time.Duration, now time.Time) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return true
	}
	return now.Sub(stat.ModTime()) >= max_age
}

// deletes every cache entry in `dir` written `max_age` or more before `now`.
// returns the number of entries deleted.
// a missing cache directory is not an error, there is nothing to prune.
func PruneCache(dir string, max_age time.Duration, now time.Time) (int, error) {
	entry_list, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	pruned := 0
	for _, entry := range entry_list {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if cache_expired(path, max_age, now) {
			if err := os.Remove(path); err != nil {
				return pruned, err
			}
			pruned++
		}
	}
	return pruned, nil
}

// a `http.RoundTripper` that caches successful responses on disk for `MaxAge`.
// set it as a `http.Client`'s transport to cache that client's requests.
type FileCachingRequest struct {
	Dir    string            // the cache directory. empty disables caching.
	MaxAge time.Duration     // zero is `DEFAULT_CACHE_MAX_AGE`
	Next   http.RoundTripper // makes the real requests. nil is `DefaultTransport()`
	Now    func() time.Time  // nil is `time.Now`
}

func (x *FileCachingRequest) max_age() time.Duration {
	if x.MaxAge == 0 {
		return DEFAULT_CACHE_MAX_AGE
	}
	return x.MaxAge
}

func (x *FileCachingRequest) now() time.Time {
	if x.Now == nil {
		return time.Now()
	}
	return x.Now()
}

func (x *FileCachingRequest) next() http.RoundTripper {
	if x.Next == nil {
		x.Next = DefaultTransport()
	}
	return x.Next
}

// limit global concurrent HTTP requests
var HTTPSem = make(chan int, 50)

func take_http_token() {
	HTTPSem <- 1
}

func release_http_token() {
	<-HTTPSem
}

// serves `req` from the on-disk cache when a fresh entry exists, otherwise makes the real
// request.
// a redirect is followed and the final response is stored under the original cache key,
// so a redirected file such as a `release.json` still caches.
// error responses, non-2xx responses and requests marked with `NoCache` are never cached.
// a failure to write the cache is logged, not returned: the response is still usable.
// concurrent real requests are capped at 50.
func (x *FileCachingRequest) RoundTrip(req *http.Request) (*http.Response, error) {
	caching := x.Dir != "" && !is_no_cache(req.Context())
	cache_key := make_cache_key(req)           // "711f20df1f76da140218e51445a6fc47"
	cache_path := cache_path(x.Dir, cache_key) // "/path/to/cache/711f20df1f76da140218e51445a6fc47"

	if caching && !cache_expired(cache_path, x.max_age(), x.now()) {
		cached_resp, err := read_cache_entry(x.Dir, cache_key)
		if err == nil {
			slog.Debug("HTTP GET cache HIT", "url", req.URL, "cache-path", cache_path)
			return cached_resp, nil
		}
	}
	slog.Debug("HTTP GET cache MISS", "url", req.URL, "caching", caching)

	take_http_token()
	defer release_http_token()

	resp, err := x.next().RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		new_url, err := resp.Location()
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("redirected without a location: %w", err)
		}
		slog.Debug("request redirected", "requested-url", req.URL, "redirected-to", new_url)

		// this client follows any further redirects itself, through the same transport.
		redirect_req, err := http.NewRequestWithContext(req.Context(), http.MethodGet, new_url.String(), nil)
		if err != nil {
			return nil, err
		}
		redirect_req.Header = req.Header.Clone()
		client := http.Client{Transport: x.next()}
		resp, err = client.Do(redirect_req)
		if err != nil {
			return resp, err
		}
	}

	if !caching || resp.StatusCode > 299 {
		return resp, nil
	}

	if err := os.MkdirAll(x.Dir, 0o755); err != nil {
		slog.Warn("failed to create cache directory", "error", err)
		return resp, nil
	}

	dumped_bytes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		slog.Warn("failed to dump response to bytes", "error", err)
		return resp, nil
	}

	if err := os.WriteFile(cache_path, dumped_bytes, 0o644); err != nil {
		slog.Warn("failed to write cache file", "error", err)
		return resp, nil
	}

	cached_resp, err := read_cache_entry(x.Dir, cache_key)
	if err != nil {
		slog.Warn("failed to read cache file", "error", err)
		return resp, nil
	}
	return cached_resp, nil
}

// --- requests

// fetches `url` with the given `client` and reads the whole body into memory.
// a non-2xx response is not an error, check `ResponseWrapper.StatusCode`.
// caching depends on the client's transport, see `FileCachingRequest`.
func Download(client *http.Client, url string, headers map[string]string) (*ResponseWrapper, error) {
	slog.Debug("HTTP GET", "url", url)
	empty_response := &ResponseWrapper{}

	req, err := http.NewRequestWithContext(trace_context(context.Background()), http.MethodGet, url, nil)
	if err != nil {
		return empty_response, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", user_agent)
	for header, header_val := range headers {
		req.Header.Set(header, header_val)
	}

	resp, err := client.Do(req)
	if err != nil {
		return empty_response, fmt.Errorf("failed to fetch '%s': %w", url, err)
	}
	defer resp.Body.Close()

	content_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty_response, fmt.Errorf("failed to read response body: %w", err)
	}

	return &ResponseWrapper{
		Response: resp,
		Bytes:    content_bytes,
		Text:     string(content_bytes),
	}, nil
}

// streams `url` to `output_path` with the given `client`, bypassing any response cache.
// the body is written to a temporary file beside `output_path` and renamed into place
// only once complete, so a failure never leaves a partial file and never touches an
// existing `output_path`.
// returns an error on any non-200 response.
func DownloadFile(client *http.Client, url string, output_path string, headers map[string]string) error {
	slog.Debug("HTTP GET file", "url", url, "output-path", output_path)

	req, err := http.NewRequestWithContext(NoCache(trace_context(context.Background())), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", user_agent)
	for header, header_val := range headers {
		req.Header.Set(header, header_val)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch '%s': %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch '%s': HTTP %d", url, resp.StatusCode)
	}

	tmp, err := os.CreateTemp(filepath.Dir(output_path), filepath.Base(output_path)+".*.part")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmp_path := tmp.Name()

	_, copy_err := io.Copy(tmp, resp.Body)
	close_err := tmp.Close()
	if copy_err != nil || close_err != nil {
		os.Remove(tmp_path)
		if copy_err != nil {
			return fmt.Errorf("failed to download '%s': %w", url, copy_err)
		}
		return fmt.Errorf("failed to write '%s': %w", tmp_path, close_err)
	}

	if err := os.Rename(tmp_path, output_path); err != nil {
		os.Remove(tmp_path)
		return fmt.Errorf("failed to move download into place: %w", err)
	}
	return nil
}
