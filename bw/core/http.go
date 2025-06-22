package core

import (
	"bw/http_utils"
	"log/slog"
)

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

// returns a `DummyDownload` struct that will respond to `Download` requests with a HTTP 404 response
func MakeDummyDownloaderError(err error) *DummyDownloader {
	return &DummyDownloader{Error: err}
}

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
