package register

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCommand always pins --install-dir to a temporary directory unless the
// case supplies its own, so a test can never write into the real home.
func runCommand(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()

	pinned := true
	for _, arg := range argv {
		if arg == "--install-dir" || strings.HasPrefix(arg, "--install-dir=") {
			pinned = false
		}
	}
	if pinned {
		argv = append([]string{"--install-dir", t.TempDir()}, argv...)
	}

	var stdout, stderr bytes.Buffer
	code := Run(argv, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// configOf extracts the printed JSON block, ignoring any human-facing lines
// that precede it.
func configOf(t *testing.T, stdout string) map[string]ServerEntry {
	t.Helper()

	start := strings.Index(stdout, "{")
	if start < 0 {
		t.Fatalf("no JSON in stdout: %s", stdout)
	}

	var parsed struct {
		MCPServers map[string]ServerEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(stdout[start:]), &parsed); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout)
	}
	return parsed.MCPServers
}

func TestRunPrintsConfigBlock(t *testing.T) {
	t.Setenv("DOORAY_TOKEN", "")
	t.Setenv(npmLauncherEnv, "")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--command", "dooray-mcp", "--mode", "read-only")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	server := configOf(t, stdout)["dooray"]
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

func TestRunRecordsInstalledPathInsteadOfNpx(t *testing.T) {
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.0")
	installDir := t.TempDir()

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--install-dir", installDir)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	// Launching through npx would make the process that unpacks the binary the
	// same one that runs it, so the config must point at a stable path.
	server := configOf(t, stdout)["dooray"]
	if server.Command == "npx" || strings.Contains(strings.Join(server.Args, " "), "dooray-mcp-go@") {
		t.Errorf("config still launches through npx: %+v", server)
	}
	if filepath.Dir(server.Command) != installDir {
		t.Errorf("command = %q, want it inside %q", server.Command, installDir)
	}
}

func TestRunNoInstallKeepsNpxLaunch(t *testing.T) {
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.0")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--no-install")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	server := configOf(t, stdout)["dooray"]
	if server.Command != "npx" {
		t.Errorf("command = %q", server.Command)
	}
	if strings.Join(server.Args, " ") != "-y dooray-mcp-go@0.1.0" {
		t.Errorf("args = %v", server.Args)
	}
}

func TestPrintDoesNotInstallTheBinary(t *testing.T) {
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.0")
	installDir := t.TempDir()

	code, _, stderr := runCommand(t, "--print", "--token", "tok", "--install-dir", installDir)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatalf("read install dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("--print wrote %d file(s) into the install directory", len(entries))
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

	// --command names the executable; with --no-install the package arguments
	// npx needs must still survive it.
	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--no-install", "--command", "/opt/homebrew/bin/npx")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	server := configOf(t, stdout)["dooray"]
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

	server := configOf(t, stdout)["dooray"]
	if server.Command != "/usr/local/bin/dooray-mcp" {
		t.Errorf("command = %q", server.Command)
	}
	if strings.Join(server.Args, " ") != "--mode read-only" {
		t.Errorf("args = %v", server.Args)
	}
}
