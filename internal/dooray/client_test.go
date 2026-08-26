package dooray

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

func newTestClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	return New(Options{
		Endpoint:          endpoint,
		Token:             "test-token",
		Timeout:           5 * time.Second,
		DownloadDirectory: t.TempDir(),
	})
}

func TestRequestSendsTokenAndQuery(t *testing.T) {
	var gotAuth, gotQuery, gotBody, gotContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery
		gotContentType = r.Header.Get("Content-Type")
		raw := make([]byte, r.ContentLength)
		r.Body.Read(raw)
		gotBody = string(raw)
		w.Write([]byte(`{"header":{"isSuccessful":true}}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	body, err := client.Request(context.Background(), http.MethodPost, "/x", url.Values{"page": {"0"}}, map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}

	if body != `{"header":{"isSuccessful":true}}` {
		t.Errorf("body = %q", body)
	}
	if gotAuth != "dooray-api test-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotQuery != "page=0" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotBody != `{"a":1}` {
		t.Errorf("request body = %q", gotBody)
	}
	if gotContentType != "application/json;charset=utf-8" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
}

func TestRequestErrorIncludesStatusAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"header":{"resultCode":403}}`))
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL).Request(context.Background(), http.MethodGet, "/x", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Dooray API 403 Forbidden") || !strings.Contains(err.Error(), "resultCode") {
		t.Errorf("error = %v", err)
	}
}

func TestRequestEmptyBodyBecomesEmptyObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	body, err := newTestClient(t, server.URL).Request(context.Background(), http.MethodGet, "/x", nil, nil)
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if body != "{}" {
		t.Errorf("body = %q, want {}", body)
	}
}

func TestDownloadFollowsRedirectAndStripsUntrustedAuthorization(t *testing.T) {
	var storageAuth string
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''%ED%95%9C%EA%B8%80.png`)
		w.Write([]byte("png-bytes"))
	}))
	defer storage.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "dooray-api test-token" {
			t.Errorf("api Authorization = %q", r.Header.Get("Authorization"))
		}
		http.Redirect(w, r, storage.URL+"/blob", http.StatusFound)
	}))
	defer api.Close()

	client := newTestClient(t, api.URL)
	result, err := client.Download(context.Background(), "/project/v1/projects/1/posts/2/files/3?media=raw", "", "3")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	// The redirect target is neither the API origin nor file-api.dooray.com,
	// so the Dooray token must not be replayed to it.
	if storageAuth != "" {
		t.Errorf("token leaked to redirect target: %q", storageAuth)
	}
	if result.FileName != "한글.png" {
		t.Errorf("FileName = %q", result.FileName)
	}
	if result.MimeType != "image/png" {
		t.Errorf("MimeType = %q", result.MimeType)
	}
	if result.Size != int64(len("png-bytes")) {
		t.Errorf("Size = %d", result.Size)
	}
	if !result.Temporary {
		t.Error("Temporary = false")
	}

	saved, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(saved) != "png-bytes" {
		t.Errorf("saved content = %q", saved)
	}
	if filepath.Base(result.FilePath) != "한글.png" {
		t.Errorf("FilePath = %q", result.FilePath)
	}
}

func TestDownloadFallsBackToFileIDName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer server.Close()

	result, err := newTestClient(t, server.URL).Download(context.Background(), "/files/9", "", "9")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if result.FileName != "9" {
		t.Errorf("FileName = %q, want 9", result.FileName)
	}
	if result.MimeType == "" {
		t.Error("MimeType must not be empty")
	}
}

func TestDownloadErrorReportsOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("missing"))
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL).Download(context.Background(), "/files/9", "", "9")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Dooray API 404 Not Found from "+server.URL) {
		t.Errorf("error = %v", err)
	}
}

func TestRequestTimeoutMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	client := New(Options{
		Endpoint:          server.URL,
		Token:             "test-token",
		Timeout:           20 * time.Millisecond,
		DownloadDirectory: t.TempDir(),
	})

	_, err := client.Request(context.Background(), http.MethodGet, "/x", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Dooray request timed out after 20ms") {
		t.Errorf("error = %v", err)
	}
}
