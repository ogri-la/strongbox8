package http_utils

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// convenience wrapper around a `http.Response`.
type ResponseWrapper struct {
	*http.Response
	Bytes []byte
	Text  string
}

// logs whether the HTTP request's underlying TCP connection was re-used.
func trace_context() context.Context {
	client_tracer := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			slog.Debug("HTTP connection reuse", "reused", info.Reused, "remote", info.Conn.RemoteAddr())
		},
	}
	return httptrace.WithClientTrace(context.Background(), client_tracer)
}

// --- caching

// returns a path to the cache directory.
func cache_dir(cwd string) string {
	return filepath.Join(cwd, "http-cache") // "/current/working/dir/http-cache"
}

// returns a path to the given `cache_key`.
func cache_path(cwd string, cache_key string) string {
	return filepath.Join(cache_dir(cwd), cache_key) // "/current/working/dir/http-cache/711f20df1f76da140218e51445a6fc47"
}

// returns a list of cache keys found in the cache directory.
// each key in list can be read with `read_cache_key`.
func cache_entry_list(cwd string) []string {
	empty_response := []string{}
	dir_entry_list, err := os.ReadDir(cache_dir(cwd))
	if err != nil {
		slog.Error("failed to list cache directory", "error", err)
		return empty_response
	}
	file_list := []string{}
	for _, dir_entry := range dir_entry_list {
		if !dir_entry.IsDir() {
			file_list = append(file_list, dir_entry.Name())
		}
	}
	return file_list
}

// creates a key that is unique to the given `req` URL (including query parameters),
// hashed to an MD5 string and prefixed, suffixed.
// the result can be safely used as a filename.
func make_cache_key(req *http.Request) string {
	// inconsistent case and url params etc will cause cache misses
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

// reads the cached response as if it were the result of `httputil.Dumpresponse`,
// a status code, followed by a series of headers, followed by the response body.
func read_cache_entry(cwd string, cache_key string) (*http.Response, error) {
	fh, err := os.Open(cache_path(cwd, cache_key))
	if err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(fh), nil)
}

// deletes a cache entry from the cache directory using the given `cache_key`.
func remove_cache_entry(cwd string, cache_key string) error {
	return os.Remove(cache_path(cwd, cache_key))
}

// returns true if the given `path` hasn't been modified for a certain duration.
// different paths have different durations.
// assumes `path` exists.
// returns `true` when an error occurs stat'ing `path`.
func cache_expired(path string, use_expired_cache bool) bool {
	if true || use_expired_cache {
		return false
	}

	default_cache_duration := 1 // hrs

	bits := strings.Split(filepath.Base(path), "-") // "/foo/bar-baz" => [bar, baz]
	suffix := ""
	if len(bits) == 2 {
		suffix = bits[1]
	}

	var cache_duration_hrs int
	switch suffix {
	/*
		case "-search":
			cache_duration_hrs = CACHE_DURATION_SEARCH
		case "-zip":
			cache_duration_hrs = CACHE_DURATION_ZIP
		case "-release.json":
			cache_duration_hrs = CACHE_DURATION_RELEASE_JSON
	*/
	default:
		cache_duration_hrs = default_cache_duration
	}

	if cache_duration_hrs == -1 {
		return false // cache at given `path` never expires
	}

	stat, err := os.Stat(path)
	if err != nil {
		slog.Warn("failed to stat cache file, assuming missing/bad cache file", "cache-path", path, "expired", true)
		return true
	}

	//diff := STATE.RunStart.Sub(stat.ModTime())
	run_start := time.Now()
	diff := run_start.Sub(stat.ModTime())
	hours := int(math.Floor(diff.Hours()))
	return hours >= cache_duration_hrs
}

type FileCachingRequest struct {
	CWD             string
	UseExpiredCache bool
}

// limit global concurrent HTTP requests
var HTTPSem = make(chan int, 50)

func take_http_token() {
	HTTPSem <- 1
}

func release_http_token() {
	<-HTTPSem
}

func (x FileCachingRequest) RoundTrip(req *http.Request) (*http.Response, error) {

	cache_key := make_cache_key(req)           // "711f20df1f76da140218e51445a6fc47"
	cache_path := cache_path(x.CWD, cache_key) // "/current/working/dir/output/711f20df1f76da140218e51445a6fc47"
	cached_resp, err := read_cache_entry(x.CWD, cache_key)
	if err == nil && !cache_expired(cache_path, x.UseExpiredCache) {
		// a cache entry was found and it's still valid, use that.
		slog.Debug("HTTP GET cache HIT", "url", req.URL, "cache-path", cache_path)
		return cached_resp, nil
	}
	slog.Debug("HTTP GET cache MISS", "url", req.URL, "cache-path", cache_path, "error", err)

	panic("no uncached http requests")

	take_http_token()
	defer release_http_token()

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		// do not cache error responses
		slog.Error("error with transport", "url", req.URL)
		return resp, err
	}

	if resp.StatusCode == 301 || resp.StatusCode == 302 {
		// we've been redirected to another location.
		// follow the redirect and save it's response under the original cache key.
		new_url, err := resp.Location()
		if err != nil {
			slog.Error("error with redirect request, no location given", "resp", resp)
			return resp, err
		}
		slog.Debug("request redirected", "requested-url", req.URL, "redirected-to", new_url)

		// make another request, update the `resp`, cache as normal.
		// this allows us to cache regular file like `release.json`.

		// but what happens when the redirect is also redirected?
		// the `client` below isn't attached to this `RoundTrip` transport,
		// so it will keep following redirects.
		// the downside is it will probably create a new connection.
		client := http.Client{}
		resp, err = client.Get(new_url.String())
		if err != nil {
			slog.Error("error with transport handling redirect", "requested-url", req.URL, "redirected-to", new_url, "error", err)
			return resp, err
		}
	}

	if resp.StatusCode > 299 {
		// non-2xx response, skip cache
		bdy, _ := io.ReadAll(resp.Body)
		slog.Debug("request unsuccessful, skipping cache", "code", resp.StatusCode, "body", string(bdy))
		return resp, nil
	}

	fh, err := os.Create(cache_path)
	if err != nil {
		slog.Warn("failed to open cache file for writing", "error", err)
		return resp, nil
	}
	defer fh.Close()

	dumped_bytes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		slog.Warn("failed to dump response to bytes", "error", err)
		return resp, nil
	}

	_, err = fh.Write(dumped_bytes)
	if err != nil {
		slog.Warn("failed to write all bytes in response to cache file", "error", err)
		return resp, nil
	}

	cached_resp, err = read_cache_entry(x.CWD, cache_key)
	if err != nil {
		slog.Warn("failed to read cache file", "error", err)
		return resp, nil
	}
	return cached_resp, nil
}

func user_agent() string {
	return fmt.Sprintf("%v/%v (%v)", "foo", "0.0.1", "https://github.com/bar/baz")
}

func Download(client *http.Client, url string, headers map[string]string) (ResponseWrapper, error) {
	slog.Debug("HTTP GET", "url", url)
	empty_response := ResponseWrapper{}

	// ---

	req, err := http.NewRequestWithContext(trace_context(), http.MethodGet, url, nil)
	if err != nil {
		return empty_response, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", user_agent())

	for header, header_val := range headers {
		req.Header.Set(header, header_val)
	}

	// ---

	resp, err := client.Do(req)
	if err != nil {
		return empty_response, fmt.Errorf("failed to fetch '%s': %w", url, err)
	}
	defer resp.Body.Close()

	// ---

	content_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty_response, fmt.Errorf("failed to read response body: %w", err)
	}

	return ResponseWrapper{
		Response: resp,
		Bytes:    content_bytes,
		Text:     string(content_bytes),
	}, nil
}

func DownloadFile(remote string, output_path string) error {
	/*
	   if file_exists(output_path) {
	           return errors.New("output path exists")
	   }
	*/

	out, err := os.Create(output_path)
	if err != nil {
		return err
	}
	defer out.Close()

	slog.Info("downloading file to disk", "url", remote, "output-path", output_path)
	resp, err := http.Get(remote)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200 response requesting file, refusing to write response to disk: %d", resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// github wrangling

/*

// returns `true` if given `resp` was github_throttled.
func github_throttled(resp ResponseWrapper) bool {
	return resp.StatusCode == 403
}

// inspects `resp` and determines how long to github_wait. then waits.
func github_wait(resp ResponseWrapper) {
	default_pause := float64(60) // seconds.
	pause := default_pause

	// inspect cache to see an example of this value
	val := resp.Header.Get("X-RateLimit-Reset")
	if val == "" {
		slog.Debug("rate limited but no 'X-RateLimit-Reset' header present.", "headers", resp.Header)
	} else {
		int_val, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			slog.Error("failed to convert value of 'X-RateLimit-Reset' header to an integer", "val", val)
		} else {
			pause = math.Ceil(time.Until(time.Unix(int_val, 0)).Seconds())
			if pause > 120 {
				slog.Warn("received unusual wait time, using default instead", "X-RateLimit-Reset", val, "wait-time", pause, "default-wait-time", default_pause)
				pause = default_pause
			}
		}
	}
	if pause > 0 {
		slog.Info("throttled", "pause", pause)
		time.Sleep(time.Duration(pause) * time.Second)
	}
}


// just like `download` but adds an 'authorization' header to the request.
func github_download(url string) (ResponseWrapper, error) {
	headers := map[string]string{
		"Authorization": "token " + STATE.GithubToken,
	}
	return download(url, headers)
}

func github_zip_download(url string, zipped_file_filter func(string) bool) (map[string][]byte, error) {
	headers := map[string]string{
		"Authorization": "token " + STATE.GithubToken,
	}
	return download_zip(url, headers, zipped_file_filter)
}

func github_download_with_retries_and_backoff(url string) (ResponseWrapper, error) {
	var resp ResponseWrapper
	var err error
	num_attempts := 5

	for i := 1; i <= num_attempts; i++ {
		resp, err = github_download(url)
		if err != nil {
			return ResponseWrapper{}, err
		}

		if resp.StatusCode == 404 {
			return ResponseWrapper{}, errors.New("not found")
		}

		if github_throttled(resp) {
			github_wait(resp)
			continue
		}

		if resp.StatusCode != 200 {
			slog.Warn("unsuccessful response from github, waiting and trying again", "url", url, "response", resp.StatusCode, "attempt", i)
			github_wait(resp)
			continue
		}

		return resp, nil
	}

	slog.Error("failed to download url after a number of attempts", "url", url, "num-attempts", num_attempts, "last-resp", resp.StatusCode)
	return ResponseWrapper{}, errors.New("failed to download url: " + url)
}
*/
