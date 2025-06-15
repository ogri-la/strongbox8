package core

import (
	"bw/http_utils"
	"log/slog"
)

//
// http related application interface that wraps lower level bw.http_utils
//

func Download(app *App, url string, headers map[string]string) (http_utils.ResponseWrapper, error) {
	slog.Info("downloading", "url", url)
	return http_utils.Download(app.HTTPClient, url, headers)
}

func DownloadFile(app *App, remote string, output_path string) error {
	slog.Info("downloading file", "url", remote, "local", output_path)
	return http_utils.DownloadFile(remote, output_path)
}
