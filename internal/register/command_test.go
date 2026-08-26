package register

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCommand(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(argv, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunPrintsConfigBlock(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "")
	t.Setenv(npmLauncherEnv, "")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--command", "dooray-mcp", "--mode", "read-only")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	var parsed struct {
		MCPServers map[string]ServerEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}

	server := parsed.MCPServers["dooray"]
	if server.Command != "dooray-mcp" {
		t.Errorf("command = %q", server.Command)
	}
	if strings.Join(server.Args, " ") != "--mode read-only" {
		t.Errorf("args = %v", server.Args)
	}
	if server.Env["DOORAY_TOKEN"] != "tok" {
		t.Errorf("env = %v", server.Env)
	}
}

func TestRunPrintDoesNotWriteConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")

	code, _, stderr := runCommand(t, "--print", "--token", "tok", "--config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("--print must not create the config file")
	}
}

func TestRunUsesNpxWhenLaunchedThroughNpm(t *testing.T) {
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.0")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, `"npx"`) || !strings.Contains(stdout, "dooray-mcp-go@0.1.0") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestRunFallsBackToTokenEnvironment(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "env-token")
	t.Setenv(npmLauncherEnv, "")

	code, stdout, stderr := runCommand(t, "--print", "--command", "dooray-mcp")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "env-token") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestRunRequiresToken(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "")

	code, _, stderr := runCommand(t, "--print")
	if code == 0 {
		t.Fatal("expected a non-zero exit")
	}
	if !strings.Contains(stderr, "token must be set") {
		t.Errorf("stderr = %s", stderr)
	}
}

func TestRunWritesConfig(t *testing.T) {
	t.Setenv(npmLauncherEnv, "")
	configPath := filepath.Join(t.TempDir(), "claude_desktop_config.json")

	code, stdout, stderr := runCommand(t, "--token", "tok", "--command", "dooray-mcp", "--config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "registered MCP server") || !strings.Contains(stdout, "restart Claude Desktop") {
		t.Errorf("stdout = %s", stdout)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config was not written: %v", err)
	}
}

func TestRunRejectsBadOptions(t *testing.T) {
	cases := [][]string{
		{"--mode", "write-only", "--token", "tok"},
		{"--client", "codex", "--token", "tok"},
		{"--name", "", "--token", "tok"},
		{"--token"},
		{"--nope"},
	}

	for _, argv := range cases {
		if code, _, _ := runCommand(t, argv...); code == 0 {
			t.Errorf("%v must be rejected", argv)
		}
	}
}

func TestRunHelp(t *testing.T) {
	code, stdout, _ := runCommand(t, "--help")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "Usage: dooray-mcp register") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestRunCommandOverrideKeepsNpxArguments(t *testing.T) {
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.0")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--command", "/opt/homebrew/bin/npx")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	var parsed struct {
		MCPServers map[string]ServerEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}

	server := parsed.MCPServers["dooray"]
	if server.Command != "/opt/homebrew/bin/npx" {
		t.Errorf("command = %q", server.Command)
	}
	if strings.Join(server.Args, " ") != "-y dooray-mcp-go@0.1.0" {
		t.Errorf("args = %v, the package arguments must survive --command", server.Args)
	}
}

func TestRunCommandOverrideOutsideNpmWrapper(t *testing.T) {
	t.Setenv(npmLauncherEnv, "")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--command", "/usr/local/bin/dooray-mcp", "--mode", "read-only")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	var parsed struct {
		MCPServers map[string]ServerEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}

	server := parsed.MCPServers["dooray"]
	if server.Command != "/usr/local/bin/dooray-mcp" {
		t.Errorf("command = %q", server.Command)
	}
	if strings.Join(server.Args, " ") != "--mode read-only" {
		t.Errorf("args = %v", server.Args)
	}
}
