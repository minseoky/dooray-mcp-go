package register

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// InstallDirectory returns the stable per-user location the binary is copied
// to before it is recorded in an MCP client configuration.
func InstallDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(localAppData, "Programs", "dooray-mcp"), nil
	}
	return filepath.Join(home, ".local", "bin"), nil
}

// binaryName is the file name used inside the install directory.
func binaryName() string {
	if runtime.GOOS == "windows" {
		return "dooray-mcp.exe"
	}
	return "dooray-mcp"
}

// InstallTarget returns the path InstallSelf would write to, without copying
// anything. It backs the preview that --print produces.
func InstallTarget(directory string) (string, error) {
	if directory == "" {
		resolved, err := InstallDirectory()
		if err != nil {
			return "", err
		}
		directory = resolved
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, binaryName()), nil
}

// InstallSelf copies the running executable into directory and returns the
// installed path.
//
// This exists so that an MCP client never launches the binary straight out of
// an npm or npx cache. Doing that makes the process that unpacked the
// executable the same process that runs it, which endpoint protection reads as
// a dropper executing its own payload. Installing once, from a separate
// command, keeps the writer and the eventual parent process distinct.
//
// An identical file already in place is left alone, so re-registering does not
// have to overwrite a binary that a running client may hold open.
func InstallSelf(directory string) (string, error) {
	source, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not resolve this binary's path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	}

	if directory == "" {
		directory, err = InstallDirectory()
		if err != nil {
			return "", err
		}
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", err
	}

	target := filepath.Join(directory, binaryName())
	if sameFile(source, target) {
		return target, nil
	}

	content, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}

	if identicalContent(target, content) {
		return target, nil
	}

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}

	if err := writeExecutableAtomic(target, content); err != nil {
		return "", fmt.Errorf("could not install the binary to %s: %w", target, err)
	}
	return target, nil
}

func sameFile(source, target string) bool {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return false
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		return false
	}
	return os.SameFile(sourceInfo, targetInfo)
}

func identicalContent(target string, content []byte) bool {
	existing, err := os.ReadFile(target)
	if err != nil {
		return false
	}
	if len(existing) != len(content) {
		return false
	}
	for index := range existing {
		if existing[index] != content[index] {
			return false
		}
	}
	return true
}

// writeExecutableAtomic stages the binary next to its destination and renames
// it into place, so a client never sees a half-written executable.
func writeExecutableAtomic(target string, content []byte) error {
	directory := filepath.Dir(target)
	temporary, err := os.CreateTemp(directory, ".dooray-mcp-*")
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
	if err := os.Chmod(temporaryPath, 0o755); err != nil {
		os.Remove(temporaryPath)
		return err
	}

	// Windows refuses to rename onto an existing file, and refuses to remove
	// one that a running client still holds open.
	if runtime.GOOS == "windows" {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			os.Remove(temporaryPath)
			return fmt.Errorf("%w; close Claude Desktop and run register again", err)
		}
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		os.Remove(temporaryPath)
		return err
	}
	return nil
}
