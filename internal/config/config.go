// Package config parses the command line flags and environment variables that
// configure the Dooray MCP server.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultEndpoint is the public Dooray API origin.
	DefaultEndpoint = "https://api.dooray.com"
	// ModeFull exposes every implemented tool.
	ModeFull = "full"
	// ModeReadOnly hides write-capable tools.
	ModeReadOnly = "read-only"

	defaultTimeout = 30000 * time.Millisecond
)

// Config holds the resolved runtime settings.
type Config struct {
	Token             string
	Endpoint          string
	Mode              string
	RequestTimeout    time.Duration
	DownloadDirectory string
	Help              bool
}

type flags struct {
	token    string
	endpoint string
	mode     string
	help     bool
}

// Load resolves the configuration from the given arguments and the process
// environment. It returns ErrHelp semantics through Config.Help.
func Load(argv []string) (*Config, error) {
	parsed, err := parseArgs(argv)
	if err != nil {
		return nil, err
	}

	if parsed.help {
		return &Config{Help: true}, nil
	}

	token := firstNonEmpty(parsed.token, os.Getenv("DOORAY_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("token must be set. Use --token <token> or DOORAY_TOKEN")
	}

	endpoint := firstNonEmpty(parsed.endpoint, os.Getenv("DOORAY_ENDPOINT"), DefaultEndpoint)
	endpoint = strings.TrimRight(endpoint, "/")

	mode, err := normalizeMode(firstNonEmpty(parsed.mode, os.Getenv("DOORAY_MCP_MODE"), ModeFull))
	if err != nil {
		return nil, err
	}

	timeout, err := parseTimeout(os.Getenv("DOORAY_REQUEST_TIMEOUT_MS"))
	if err != nil {
		return nil, err
	}

	downloadDirectory := os.Getenv("DOORAY_DOWNLOAD_DIR")
	if downloadDirectory == "" {
		downloadDirectory = filepath.Join(os.TempDir(), "dooray-mcp")
	}
	downloadDirectory, err = filepath.Abs(downloadDirectory)
	if err != nil {
		return nil, err
	}

	return &Config{
		Token:             token,
		Endpoint:          endpoint,
		Mode:              mode,
		RequestTimeout:    timeout,
		DownloadDirectory: downloadDirectory,
	}, nil
}

func parseArgs(argv []string) (flags, error) {
	var parsed flags

	takeValue := func(index *int, name string) (string, error) {
		if *index+1 >= len(argv) {
			return "", fmt.Errorf("%s requires a value", name)
		}
		*index++
		return argv[*index], nil
	}

	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		var err error

		switch {
		case arg == "--help" || arg == "-h":
			parsed.help = true
		case arg == "--token":
			parsed.token, err = takeValue(&index, "--token")
		case strings.HasPrefix(arg, "--token="):
			parsed.token = strings.TrimPrefix(arg, "--token=")
		case arg == "--endpoint":
			parsed.endpoint, err = takeValue(&index, "--endpoint")
		case strings.HasPrefix(arg, "--endpoint="):
			parsed.endpoint = strings.TrimPrefix(arg, "--endpoint=")
		case arg == "--mode":
			parsed.mode, err = takeValue(&index, "--mode")
		case strings.HasPrefix(arg, "--mode="):
			parsed.mode = strings.TrimPrefix(arg, "--mode=")
		}

		if err != nil {
			return flags{}, err
		}
	}

	return parsed, nil
}

func normalizeMode(value string) (string, error) {
	if value == ModeFull || value == ModeReadOnly {
		return value, nil
	}
	return "", fmt.Errorf("DOORAY_MCP_MODE or --mode must be one of: %s, %s", ModeFull, ModeReadOnly)
}

func parseTimeout(value string) (time.Duration, error) {
	if value == "" {
		return defaultTimeout, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("DOORAY_REQUEST_TIMEOUT_MS must be a positive integer")
	}
	return time.Duration(parsed) * time.Millisecond, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// HelpText is the usage message printed for --help.
func HelpText() string {
	return fmt.Sprintf(`Usage: dooray-mcp --token <dooray-token> [--endpoint %s] [--mode %s|%s]

Subcommands:
  register           Add this server to the Claude Desktop configuration.
                     Run "dooray-mcp register --help" for its options.

Environment:
  DOORAY_TOKEN       Dooray personal API token
  DOORAY_ENDPOINT    Dooray API endpoint, default %s
  DOORAY_MCP_MODE    tool exposure mode: %s or %s, default %s
  DOORAY_REQUEST_TIMEOUT_MS  request timeout, default 30000
  DOORAY_DOWNLOAD_DIR        attachment directory, default system temp directory
`, DefaultEndpoint, ModeFull, ModeReadOnly, DefaultEndpoint, ModeFull, ModeReadOnly, ModeFull)
}
