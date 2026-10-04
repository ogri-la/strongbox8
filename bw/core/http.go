package core

import (
	"bw/http_utils"
	"fmt"
	"log/slog"
)

// downloads files on the app's behalf.
// swap the app's implementation to control responses during testing.
type IDownloader interface {
	Download(app *App, url string, headers map[string]string) (*http_utils.ResponseWrapper, error)
	DownloadFile(app *App, url string, output_path string) error
}

type HTTPDownloader struct{}

var _ IDownloader = (*HTTPDownloader)(nil)

// --- convenience. wraps whatever Downloader the app has set to avoid passing the `app` around.

func (app *App) Download(url string, headers map[string]string) (*http_utils.ResponseWrapper, error) {
	return app.Downloader.Download(app, url, headers)
}

func (app *App) DownloadFile(url string, output_path string) error {
	return app.Downloader.DownloadFile(app, url, output_path)
}

// --- actual IDownloader implementation that wraps the lower level bw.http_utils

func (d *HTTPDownloader) Download(app *App, url string, headers map[string]string) (*http_utils.ResponseWrapper, error) {
	slog.Info("downloading", "url", url)
	return http_utils.Download(app.HTTPClient, url, headers)
}

func (d *HTTPDownloader) DownloadFile(app *App, url string, output_path string) error {
	slog.Info("downloading file", "url", url, "local", output_path)
	return http_utils.DownloadFile(url, output_path)
}

// --- dummy IDownloader implementation to control responses during testing

type DummyDownloader struct {
	Response *http_utils.ResponseWrapper
	Error    error
}

var _ IDownloader = (*DummyDownloader)(nil)

// returns a `DummyDownloader` that fails every request with the given `err`.
func MakeDummyDownloaderError(err error) *DummyDownloader {
	return &DummyDownloader{Error: err}
}

// returns a `DummyDownloader` that answers every `Download` request with `resp`.
// `DownloadFile` writes nothing and returns nil.
func MakeDummyDownloader(resp *http_utils.ResponseWrapper) *DummyDownloader {
	return &DummyDownloader{Response: resp}
}

func (d *DummyDownloader) Download(app *App, url string, headers map[string]string) (*http_utils.ResponseWrapper, error) {
	empty_response := &http_utils.ResponseWrapper{}
	if d.Error != nil {
		return empty_response, d.Error
	}
	if d.Response == nil {
		d.Response = empty_response
	}
	return d.Response, nil
}

func (d *DummyDownloader) DownloadFile(app *App, url string, output_path string) error {
	if d.Error != nil {
		return d.Error
	}
	return nil
}

// --- IDownloader implementation that answers each URL differently

// a `DummyDownloader` answers every request the same way.
// this one matches the requested URL against `ResponseMap`, which suits code that makes
// more than one request.
// a URL that is not in the map is an error, so a test fails loudly rather than seeing an
// empty response.
type MapDownloader struct {
	ResponseMap map[string]*http_utils.ResponseWrapper

	// every URL requested, in order, for asserting on what was and wasn't fetched.
	RequestedURLList []string
}

var _ IDownloader = (*MapDownloader)(nil)

// returns a `MapDownloader` that answers requests using `response_map`.
func MakeMapDownloader(response_map map[string]*http_utils.ResponseWrapper) *MapDownloader {
	return &MapDownloader{
		ResponseMap:      response_map,
		RequestedURLList: []string{},
	}
}

// returns a `MapDownloader` that answers requests with the given `body_map` of URL to
// response body.
func MakeMapDownloaderBytes(body_map map[string][]byte) *MapDownloader {
	response_map := map[string]*http_utils.ResponseWrapper{}
	for url, body := range body_map {
		response_map[url] = &http_utils.ResponseWrapper{Bytes: body, Text: string(body)}
	}
	return MakeMapDownloader(response_map)
}

func (d *MapDownloader) Download(app *App, url string, headers map[string]string) (*http_utils.ResponseWrapper, error) {
	empty_response := &http_utils.ResponseWrapper{}
	d.RequestedURLList = append(d.RequestedURLList, url)
	resp, present := d.ResponseMap[url]
	if !present {
		return empty_response, fmt.Errorf("no response configured for url: %s", url)
	}
	return resp, nil
}

func (d *MapDownloader) DownloadFile(app *App, url string, output_path string) error {
	d.RequestedURLList = append(d.RequestedURLList, url)
	_, present := d.ResponseMap[url]
	if !present {
		return fmt.Errorf("no response configured for url: %s", url)
	}
	return nil
}
