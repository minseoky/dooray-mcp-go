// Package dooray implements the authenticated HTTP client used by the MCP
// tools, including the redirect-aware attachment download.
package dooray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// trustedDownloadAuthHosts receive the Dooray token on an HTTPS redirect even
// though they are not the configured API origin.
var trustedDownloadAuthHosts = map[string]bool{
	"file-api.dooray.com": true,
}

const maxDownloadRedirects = 3

// Client talks to the Dooray REST API with a personal API token.
type Client struct {
	endpoint          string
	token             string
	timeout           time.Duration
	downloadDirectory string
	httpClient        *http.Client
}

// Options configures a Client.
type Options struct {
	Endpoint          string
	Token             string
	Timeout           time.Duration
	DownloadDirectory string
}

// New builds a Client that never follows redirects automatically, so that the
// Authorization header is only replayed to trusted origins.
func New(options Options) *Client {
	return &Client{
		endpoint:          options.Endpoint,
		token:             options.Token,
		timeout:           options.Timeout,
		downloadDirectory: options.DownloadDirectory,
		httpClient: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// DownloadResult is the payload returned to the MCP client after a successful
// attachment download.
type DownloadResult struct {
	FilePath  string `json:"filePath"`
	FileName  string `json:"fileName"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	Temporary bool   `json:"temporary"`
}

// Request performs a JSON API call and returns the raw response body, or "{}"
// when the response has no body.
func (c *Client) Request(ctx context.Context, method, path string, query url.Values, body any) (string, error) {
	target, err := url.Parse(c.endpoint + path)
	if err != nil {
		return "", err
	}
	if len(query) > 0 {
		target.RawQuery = query.Encode()
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		payload = strings.NewReader(string(encoded))
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestCtx, method, target.String(), payload)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "dooray-api "+c.token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json;charset=utf-8")
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", c.wrapTransportError(err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return "", c.wrapTransportError(err)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("Dooray API %d %s: %s", response.StatusCode, statusText(response), string(raw))
	}

	if len(raw) == 0 {
		return "{}", nil
	}
	return string(raw), nil
}

// Download streams an attachment to the download directory, forwarding the
// Dooray token only to the API origin and trusted HTTPS download hosts.
func (c *Client) Download(ctx context.Context, requestPath, requestedFileName, fallbackFileName string) (*DownloadResult, error) {
	endpointURL, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, err
	}
	endpointOrigin := originOf(endpointURL)

	response, err := c.downloadRequest(ctx, c.endpoint+requestPath, true)
	if err != nil {
		return nil, err
	}

	for redirectCount := 0; redirectCount < maxDownloadRedirects && isRedirect(response.StatusCode); redirectCount++ {
		location := response.Header.Get("Location")
		if location == "" {
			response.Body.Close()
			return nil, errors.New("Dooray download redirect did not provide a Location header.")
		}

		redirectURL, err := response.Request.URL.Parse(location)
		response.Body.Close()
		if err != nil {
			return nil, err
		}

		response, err = c.downloadRequest(ctx, redirectURL.String(), shouldSendDownloadAuthorization(redirectURL, endpointOrigin))
		if err != nil {
			return nil, err
		}
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		raw, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("Dooray API %d %s from %s: %s",
			response.StatusCode, statusText(response), originOf(response.Request.URL), string(raw))
	}

	if err := os.MkdirAll(c.downloadDirectory, 0o755); err != nil {
		return nil, err
	}

	fileName := sanitizeFileName(firstNonEmpty(
		contentDispositionFileName(response.Header.Get("Content-Disposition")),
		requestedFileName,
		fallbackFileName,
	))
	filePath := filepath.Join(c.downloadDirectory, fileName)

	size, err := writeStream(filePath, response.Body)
	if err != nil {
		os.Remove(filePath)
		return nil, c.wrapTransportError(err)
	}

	mimeType := response.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return &DownloadResult{
		FilePath:  filePath,
		FileName:  fileName,
		MimeType:  mimeType,
		Size:      size,
		Temporary: true,
	}, nil
}

func (c *Client) downloadRequest(ctx context.Context, target string, withAuthorization bool) (*http.Response, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)

	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	request.Header.Set("Accept", "application/octet-stream")
	if withAuthorization {
		request.Header.Set("Authorization", "dooray-api "+c.token)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		cancel()
		return nil, c.wrapTransportError(err)
	}

	// The per-request timeout must stay alive while the body is streamed, so
	// the cancel func is attached to the body instead of being deferred here.
	response.Body = &cancelOnCloseBody{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

func (c *Client) wrapTransportError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("Dooray request timed out after %dms", c.timeout.Milliseconds())
	}
	return err
}

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func writeStream(filePath string, source io.Reader) (int64, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}

	size, err := io.Copy(file, source)
	closeErr := file.Close()
	if err != nil {
		return size, err
	}
	return size, closeErr
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func shouldSendDownloadAuthorization(target *url.URL, endpointOrigin string) bool {
	if originOf(target) == endpointOrigin {
		return true
	}
	return target.Scheme == "https" && trustedDownloadAuthHosts[target.Hostname()]
}

func originOf(target *url.URL) string {
	return target.Scheme + "://" + target.Host
}

func statusText(response *http.Response) string {
	// Go keeps the numeric prefix in Status, e.g. "404 Not Found".
	if text := strings.TrimSpace(strings.TrimPrefix(response.Status, fmt.Sprint(response.StatusCode))); text != "" {
		return text
	}
	return http.StatusText(response.StatusCode)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
