package ezhttp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"
	"github.com/topi314/gobin/gobin"
)

var defaultClient = &http.Client{
	Timeout: 10 * time.Second,
}

func Do(method string, path string, token string, body io.Reader) (*http.Response, error) {
	server := viper.GetString("server")
	request, err := http.NewRequest(method, server+path, body)
	if err != nil {
		return nil, err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return defaultClient.Do(request)
}

func Get(path string) (*http.Response, error) {
	return Do(http.MethodGet, path, "", nil)
}

func Post(path string, body io.Reader) (*http.Response, error) {
	return Do(http.MethodPost, path, "", body)
}

func Patch(path string, token string, body io.Reader) (*http.Response, error) {
	return Do(http.MethodPatch, path, token, body)
}

func Delete(path string, token string) (*http.Response, error) {
	return Do(http.MethodDelete, path, token, nil)
}

func ProcessBody(method string, rs *http.Response, body any) error {
	if rs.StatusCode >= 200 && rs.StatusCode <= 299 {
		if err := json.NewDecoder(rs.Body).Decode(body); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		return nil
	}
	var errRs gobin.ErrorResponse
	if err := json.NewDecoder(rs.Body).Decode(&errRs); err != nil {
		return fmt.Errorf("failed to decode error response: %w", err)
	}
	return fmt.Errorf("failed to %s: %s", method, errRs.Message)
}
