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
	// Created is true when no configuration existed at any known location, so
	// the file was written fresh. A client that keeps its configuration
	// somewhere else would show up this way.
	Created bool
}

// ErrAlreadyExists is returned when the target server name is taken and the
// caller did not ask to replace it.
var ErrAlreadyExists = errors.New("server name already exists")

// configFileName is the file every Claude Desktop install uses, wherever the
// containing directory turns out to be.
const configFileName = "claude_desktop_config.json"

// ClaudeDesktopConfigCandidates lists the locations Claude Desktop is known to
// keep its configuration, most specific first.
//
// The directory is not the same on every Windows machine. A packaged (MSIX or
// Store) install has its AppData writes redirected into a per-package
// LocalCache, so the file can sit under LOCALAPPDATA\Packages\<package>\
// instead of APPDATA. Rather than commit to one of those, the caller searches
// for a file that already exists.
func ClaudeDesktopConfigCandidates() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, "Library", "Application Support", "Claude", configFileName),
		}, nil

	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}

		candidates := []string{filepath.Join(appData, "Claude", configFileName)}

		// A packaged install redirects Roaming into its own LocalCache. The
		// package family name varies, so every package directory is examined.
		packaged, _ := filepath.Glob(filepath.Join(localAppData, "Packages", "*", "LocalCache", "Roaming", "Claude", configFileName))
		candidates = append(candidates, packaged...)

		return append(candidates, filepath.Join(localAppData, "Claude", configFileName)), nil

	default:
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(home, ".config")
		}
		return []string{filepath.Join(configHome, "Claude", configFileName)}, nil
	}
}

// ClaudeDesktopConfigPath returns the configuration file to merge into: the
// first candidate that already exists, or the default location to create when
// Claude Desktop has not written one yet.
func ClaudeDesktopConfigPath() (string, error) {
	candidates, err := ClaudeDesktopConfigCandidates()
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("no Claude Desktop configuration location is known for this platform")
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return candidates[0], nil
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

	result := &Result{ConfigPath: configPath, Replaced: replaced, Created: mode == 0}
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
