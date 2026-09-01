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

func TestRunRecordsTheNpxInvocationFromAPackageCache(t *testing.T) {
	// The cache path is not stable, so the launch command reproduces it through
	// npx. That the client then unpacks and spawns in one step is accepted.
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.2")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	server := configOf(t, stdout)["dooray"]
	if server.Command != "npx" {
		t.Errorf("command = %q", server.Command)
	}
	// Windows npx treats a bare name@version as the command to run, so the
	// spec goes through --package and the bin is named after --.
	if strings.Join(server.Args, " ") != "-y --package=dooray-mcp-go@0.1.2 -- dooray-mcp-go" {
		t.Errorf("args = %v", server.Args)
	}
}

func TestPackageNameOf(t *testing.T) {
	cases := map[string]string{
		"dooray-mcp-go@0.1.2": "dooray-mcp-go",
		"dooray-mcp-go":       "dooray-mcp-go",
		"@scope/pkg@1.2.3":    "@scope/pkg",
		"@scope/pkg":          "@scope/pkg",
		"pkg@^1.0.0":          "pkg",
	}
	for spec, want := range cases {
		if got := packageNameOf(spec); got != want {
			t.Errorf("packageNameOf(%q) = %q, want %q", spec, got, want)
		}
	}
}

func TestRunFromAPackageCacheStillHonoursCommandOverride(t *testing.T) {
	t.Setenv(npmLauncherEnv, "dooray-mcp-go@0.1.2")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok", "--command", "/usr/local/bin/npx")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	server := configOf(t, stdout)["dooray"]
	if server.Command != "/usr/local/bin/npx" {
		t.Errorf("command = %q", server.Command)
	}
	if strings.Join(server.Args, " ") != "-y --package=dooray-mcp-go@0.1.2 -- dooray-mcp-go" {
		t.Errorf("args = %v, the package arguments must survive --command", server.Args)
	}
}

func TestRunRecordsItsOwnAbsolutePath(t *testing.T) {
	t.Setenv(npmLauncherEnv, "")

	code, stdout, stderr := runCommand(t, "--print", "--token", "tok")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	command := configOf(t, stdout)["dooray"].Command
	if !filepath.IsAbs(command) {
		t.Errorf("command = %q must be absolute", command)
	}
	if command == "npx" {
		t.Error("command must not be npx")
	}
}

func TestRunWritesNoExecutable(t *testing.T) {
	// The server binary must never produce another executable; that is what
	// made endpoint protection report self-replication.
	t.Setenv(npmLauncherEnv, "")
	configDir := t.TempDir()

	code, _, stderr := runCommand(t, "--token", "tok", "--config", filepath.Join(configDir, "claude_desktop_config.json"))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}

	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, dirEntry := range entries {
		info, err := dirEntry.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", dirEntry.Name(), err)
		}
		if info.Mode().Perm()&0o111 != 0 {
			t.Errorf("register produced an executable file: %s", dirEntry.Name())
		}
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
