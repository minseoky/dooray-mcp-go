package register

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func entry(token string) ServerEntry {
	return ServerEntry{
		Command: "dooray-mcp",
		Env:     map[string]string{"DOORAY_TOKEN": token},
	}
}

func readBack(t *testing.T, configPath string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return parsed
}

func TestClaudeDesktopCreatesConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "claude_desktop_config.json")

	result, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("tok"), ConfigPath: configPath})
	if err != nil {
		t.Fatalf("ClaudeDesktop: %v", err)
	}
	if result.Replaced {
		t.Error("Replaced = true for a new config")
	}
	if result.BackupPath != "" {
		t.Errorf("BackupPath = %q for a new config", result.BackupPath)
	}

	servers := readBack(t, configPath)["mcpServers"].(map[string]any)
	dooray := servers["dooray"].(map[string]any)
	if dooray["command"] != "dooray-mcp" {
		t.Errorf("command = %v", dooray["command"])
	}
	if dooray["env"].(map[string]any)["DOORAY_TOKEN"] != "tok" {
		t.Errorf("env = %v", dooray["env"])
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, a new config holding a token must be owner-only", info.Mode().Perm())
		}
	}
}

func TestClaudeDesktopPreservesOtherKeysAndServers(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")
	original := `{"globalShortcut":"Alt+Space","mcpServers":{"filesystem":{"command":"npx","args":["-y","x"]}}}`
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	result, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("tok"), ConfigPath: configPath})
	if err != nil {
		t.Fatalf("ClaudeDesktop: %v", err)
	}
	if result.BackupPath != configPath+".bak" {
		t.Errorf("BackupPath = %q", result.BackupPath)
	}

	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup = %q, want the untouched original", backup)
	}

	config := readBack(t, configPath)
	if config["globalShortcut"] != "Alt+Space" {
		t.Errorf("globalShortcut = %v", config["globalShortcut"])
	}
	servers := config["mcpServers"].(map[string]any)
	if len(servers) != 2 || servers["filesystem"] == nil || servers["dooray"] == nil {
		t.Errorf("mcpServers = %v", servers)
	}
}

func TestClaudeDesktopRefusesDuplicateWithoutForce(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")

	if _, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("first"), ConfigPath: configPath}); err != nil {
		t.Fatalf("first register: %v", err)
	}

	_, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("second"), ConfigPath: configPath})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}

	servers := readBack(t, configPath)["mcpServers"].(map[string]any)
	token := servers["dooray"].(map[string]any)["env"].(map[string]any)["DOORAY_TOKEN"]
	if token != "first" {
		t.Errorf("token = %v, the refused write must not change the file", token)
	}
}

func TestClaudeDesktopForceReplaces(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")

	if _, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("first"), ConfigPath: configPath}); err != nil {
		t.Fatalf("first register: %v", err)
	}

	result, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("second"), ConfigPath: configPath, Force: true})
	if err != nil {
		t.Fatalf("force register: %v", err)
	}
	if !result.Replaced {
		t.Error("Replaced = false")
	}

	servers := readBack(t, configPath)["mcpServers"].(map[string]any)
	if token := servers["dooray"].(map[string]any)["env"].(map[string]any)["DOORAY_TOKEN"]; token != "second" {
		t.Errorf("token = %v", token)
	}
}

func TestClaudeDesktopRejectsMalformedConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")
	if err := os.WriteFile(configPath, []byte("not json"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	_, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("tok"), ConfigPath: configPath})
	if err == nil || !strings.Contains(err.Error(), "is not a JSON object") {
		t.Errorf("error = %v", err)
	}
}

func TestClaudeDesktopHandlesEmptyFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")
	if err := os.WriteFile(configPath, nil, 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	if _, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("tok"), ConfigPath: configPath}); err != nil {
		t.Fatalf("ClaudeDesktop: %v", err)
	}
	if servers := readBack(t, configPath)["mcpServers"].(map[string]any); servers["dooray"] == nil {
		t.Errorf("mcpServers = %v", servers)
	}
}

func TestClaudeDesktopConfigCandidatesAreUsable(t *testing.T) {
	candidates, err := ClaudeDesktopConfigCandidates()
	if err != nil {
		t.Fatalf("ClaudeDesktopConfigCandidates: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("no candidates returned")
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		if !filepath.IsAbs(candidate) {
			t.Errorf("candidate %q must be absolute", candidate)
		}
		if filepath.Base(candidate) != configFileName {
			t.Errorf("candidate %q must end in %s", candidate, configFileName)
		}
		if seen[candidate] {
			t.Errorf("candidate %q is listed twice", candidate)
		}
		seen[candidate] = true
	}
}

func TestClaudeDesktopConfigPathPrefersAnExistingFile(t *testing.T) {
	// The resolved path has to be one of the candidates, so a machine that
	// keeps its configuration elsewhere is never silently written to.
	configPath, err := ClaudeDesktopConfigPath()
	if err != nil {
		t.Fatalf("ClaudeDesktopConfigPath: %v", err)
	}

	candidates, err := ClaudeDesktopConfigCandidates()
	if err != nil {
		t.Fatalf("ClaudeDesktopConfigCandidates: %v", err)
	}

	found := false
	for _, candidate := range candidates {
		if candidate == configPath {
			found = true
		}
	}
	if !found {
		t.Errorf("resolved %q is not among the candidates %v", configPath, candidates)
	}
}

func TestClaudeDesktopReportsCreationVersusMerge(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), configFileName)

	result, err := ClaudeDesktop(Request{Name: "dooray", Entry: entry("tok"), ConfigPath: configPath})
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	if !result.Created {
		t.Error("Created = false for a config that did not exist")
	}

	result, err = ClaudeDesktop(Request{Name: "other", Entry: entry("tok"), ConfigPath: configPath})
	if err != nil {
		t.Fatalf("second register: %v", err)
	}
	if result.Created {
		t.Error("Created = true for a config that already existed")
	}
}
