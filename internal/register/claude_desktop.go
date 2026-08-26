// Package register writes this server into an MCP client's configuration
// file. Claude Desktop has no CLI, so its JSON config is merged in place.
package register

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ServerEntry is one entry under "mcpServers".
type ServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Request describes the entry to write and how to write it.
type Request struct {
	Name       string
	Entry      ServerEntry
	ConfigPath string
	Force      bool
}

// Result reports what the merge did.
type Result struct {
	ConfigPath string
	BackupPath string
	Replaced   bool
}

// ErrAlreadyExists is returned when the target server name is taken and the
// caller did not ask to replace it.
var ErrAlreadyExists = errors.New("server name already exists")

// ClaudeDesktopConfigPath returns the platform-specific configuration path.
func ClaudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil
	default:
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		return filepath.Join(configHome, "Claude", "claude_desktop_config.json"), nil
	}
}

// ClaudeDesktop merges the entry into the Claude Desktop configuration,
// keeping every other key and server untouched and backing up the original.
func ClaudeDesktop(request Request) (*Result, error) {
	configPath := request.ConfigPath
	if configPath == "" {
		resolved, err := ClaudeDesktopConfigPath()
		if err != nil {
			return nil, err
		}
		configPath = resolved
	}

	existing, mode, err := readConfig(configPath)
	if err != nil {
		return nil, err
	}

	servers, err := serversOf(existing)
	if err != nil {
		return nil, err
	}

	_, replaced := servers[request.Name]
	if replaced && !request.Force {
		return nil, fmt.Errorf("%w: %q in %s; pass --force to replace it", ErrAlreadyExists, request.Name, configPath)
	}

	entry, err := json.Marshal(request.Entry)
	if err != nil {
		return nil, err
	}
	servers[request.Name] = entry

	encodedServers, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	existing["mcpServers"] = encodedServers

	encoded, err := marshalIndentedObject(existing)
	if err != nil {
		return nil, err
	}

	result := &Result{ConfigPath: configPath, Replaced: replaced}
	if mode != 0 {
		backupPath, err := backup(configPath)
		if err != nil {
			return nil, err
		}
		result.BackupPath = backupPath
	} else {
		// A fresh config holds an API token, so it starts owner-only.
		mode = 0o600
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return nil, err
	}
	if err := writeFileAtomic(configPath, encoded, mode); err != nil {
		return nil, err
	}

	return result, nil
}

// readConfig returns the parsed object and the existing file mode, or a zero
// mode when the file does not exist yet.
func readConfig(configPath string) (map[string]json.RawMessage, os.FileMode, error) {
	info, err := os.Stat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, 0, err
	}
	if len(raw) == 0 {
		return map[string]json.RawMessage{}, info.Mode().Perm(), nil
	}

	parsed := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, 0, fmt.Errorf("%s is not a JSON object: %w", configPath, err)
	}
	return parsed, info.Mode().Perm(), nil
}

func serversOf(config map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	raw, ok := config["mcpServers"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return map[string]json.RawMessage{}, nil
	}

	servers := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &servers); err != nil {
		return nil, fmt.Errorf("mcpServers is not a JSON object: %w", err)
	}
	return servers, nil
}

// marshalIndentedObject renders the config with the two-space indentation
// Claude Desktop writes, without escaping the HTML characters that can appear
// in a token or a Windows path.
func marshalIndentedObject(config map[string]json.RawMessage) ([]byte, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded, "", "  "); err != nil {
		return nil, err
	}
	return append(indented.Bytes(), '\n'), nil
}

func backup(configPath string) (string, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	backupPath := configPath + ".bak"
	if err := os.WriteFile(backupPath, raw, 0o600); err != nil {
		return "", err
	}
	return backupPath, nil
}

// writeFileAtomic writes through a temporary file in the same directory so a
// failure never leaves a truncated configuration behind.
func writeFileAtomic(configPath string, content []byte, mode os.FileMode) error {
	directory := filepath.Dir(configPath)
	temporary, err := os.CreateTemp(directory, ".claude_desktop_config-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()

	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		os.Remove(temporaryPath)
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return err
	}
	if err := os.Chmod(temporaryPath, mode); err != nil {
		os.Remove(temporaryPath)
		return err
	}

	// Windows refuses to rename onto an existing file.
	if runtime.GOOS == "windows" {
		os.Remove(configPath)
	}
	if err := os.Rename(temporaryPath, configPath); err != nil {
		os.Remove(temporaryPath)
		return err
	}
	return nil
}
