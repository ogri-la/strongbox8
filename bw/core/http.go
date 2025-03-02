package core

import (
	"bw/http_utils"
)

//
// http related application interface that wraps lower level bw.http_utils
//

func Download(app *App, url string, headers map[string]string) (http_utils.ResponseWrapper, error) {
	return http_utils.Download(app.state.HTTPClient, url, headers)
}

func DownloadFile(app *App, remote string, output_path string) error {
	return http_utils.DownloadFile(remote, output_path)
}
