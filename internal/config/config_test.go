package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFlagsBeatEnvironment(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")
	t.Setenv("DOORAY_ENDPOINT", "https://env.example.com")

	settings, err := Load([]string{"--token", "flag-token", "--endpoint=https://flag.example.com/"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if settings.Token != "flag-token" {
		t.Errorf("Token = %q", settings.Token)
	}
	if settings.Endpoint != "https://flag.example.com" {
		t.Errorf("Endpoint = %q, trailing slash must be trimmed", settings.Endpoint)
	}
}

func TestEnvironmentDefaults(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")

	settings, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if settings.Endpoint != DefaultEndpoint {
		t.Errorf("Endpoint = %q", settings.Endpoint)
	}
	if settings.Mode != ModeFull {
		t.Errorf("Mode = %q", settings.Mode)
	}
	if settings.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v", settings.RequestTimeout)
	}
	if !filepath.IsAbs(settings.DownloadDirectory) {
		t.Errorf("DownloadDirectory = %q must be absolute", settings.DownloadDirectory)
	}
}

func TestMissingTokenIsRejected(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "")

	if _, err := Load(nil); err == nil || !strings.Contains(err.Error(), "token must be set") {
		t.Errorf("error = %v", err)
	}
}

func TestModeValidation(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")

	settings, err := Load([]string{"--mode", ModeReadOnly})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if settings.Mode != ModeReadOnly {
		t.Errorf("Mode = %q", settings.Mode)
	}

	if _, err := Load([]string{"--mode", "write-only"}); err == nil {
		t.Error("expected an error for an unknown mode")
	}
}

func TestModeFromEnvironment(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")
	t.Setenv("DOORAY_MCP_MODE", ModeReadOnly)

	settings, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if settings.Mode != ModeReadOnly {
		t.Errorf("Mode = %q", settings.Mode)
	}
}

func TestTimeoutValidation(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")

	t.Setenv("DOORAY_REQUEST_TIMEOUT_MS", "1500")
	settings, err := Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if settings.RequestTimeout != 1500*time.Millisecond {
		t.Errorf("RequestTimeout = %v", settings.RequestTimeout)
	}

	for _, invalid := range []string{"0", "-1", "abc", "1.5"} {
		t.Setenv("DOORAY_REQUEST_TIMEOUT_MS", invalid)
		if _, err := Load(nil); err == nil {
			t.Errorf("DOORAY_REQUEST_TIMEOUT_MS=%q must be rejected", invalid)
		}
	}
}

func TestHelpShortCircuits(t *testing.T) {
	settings, err := Load([]string{"--help"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !settings.Help {
		t.Error("Help = false")
	}
}

func TestFlagWithoutValueIsRejected(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")

	if _, err := Load([]string{"--token"}); err == nil {
		t.Error("expected an error for --token without a value")
	}
}
