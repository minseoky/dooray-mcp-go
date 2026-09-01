package register

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultServerName = "dooray"
	// npmLauncherEnv is set by the npm wrapper so that a config written from
	// `npx -y dooray-mcp-go register` points back at npx instead of at the
	// binary inside the npm cache, which npx may evict.
	npmLauncherEnv = "DOORAY_MCP_NPM_SPEC"
)

type options struct {
	client     string
	name       string
	token      string
	mode       string
	command    string
	configPath string
	force      bool
	print      bool
	help       bool
}

// Run executes the `register` subcommand. argv excludes the subcommand name.
func Run(argv []string, stdout, stderr io.Writer) int {
	parsed, err := parseOptions(argv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, HelpText())
		return 1
	}

	if parsed.help {
		fmt.Fprint(stdout, HelpText())
		return 0
	}

	if parsed.client != "claude-desktop" {
		fmt.Fprintf(stderr, "unsupported --client %q; only claude-desktop is supported\n", parsed.client)
		return 1
	}

	token := parsed.token
	if token == "" {
		token = os.Getenv("DOORAY_TOKEN")
	}
	if token == "" {
		fmt.Fprintln(stderr, "token must be set. Use --token <token> or DOORAY_TOKEN.")
		return 1
	}

	entry, err := buildEntry(parsed, token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if parsed.print {
		encoded, err := printableConfig(parsed.name, entry)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, encoded)
		return 0
	}

	result, err := ClaudeDesktop(Request{
		Name:       parsed.name,
		Entry:      entry,
		ConfigPath: parsed.configPath,
		Force:      parsed.force,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	action := "registered"
	if result.Replaced {
		action = "replaced"
	}
	fmt.Fprintf(stdout, "%s MCP server %q in %s\n", action, parsed.name, result.ConfigPath)
	if result.BackupPath != "" {
		fmt.Fprintf(stdout, "previous configuration backed up to %s\n", result.BackupPath)
	}

	// Creating the file means no existing configuration was found. That is
	// normal before Claude Desktop has ever been configured, but it is also
	// what happens when this install keeps its configuration somewhere the
	// search does not cover, so the searched locations are worth showing.
	if result.Created && parsed.configPath == "" {
		fmt.Fprintln(stdout, "no existing Claude Desktop configuration was found, so this one was created.")
		if candidates, err := ClaudeDesktopConfigCandidates(); err == nil && len(candidates) > 1 {
			fmt.Fprintln(stdout, "locations searched:")
			for _, candidate := range candidates {
				fmt.Fprintf(stdout, "  %s\n", candidate)
			}
			fmt.Fprintln(stdout, "if Claude Desktop reads a different file, re-run with --config <path>.")
		}
	}

	fmt.Fprintln(stdout, "restart Claude Desktop to load the server.")
	return 0
}

// buildEntry decides what the MCP client should spawn.
func buildEntry(parsed options, token string) (ServerEntry, error) {
	var command string
	var args []string

	if spec := os.Getenv(npmLauncherEnv); spec != "" {
		// Started through the npm wrapper, whose cache path is not stable, so
		// the launch command is the npx invocation that reproduces it.
		//
		// This makes the client unpack and spawn the executable in one step.
		// Endpoint protection can read that as a dropper launching its payload;
		// it is accepted here deliberately. What is not acceptable is the
		// alternative of copying the binary somewhere stable, because an
		// executable writing a copy of its own image is self-replication, which
		// scores far worse and is never done by this binary.
		command = "npx"
		args = append(args, "-y", spec)
	} else {
		executable, err := os.Executable()
		if err != nil {
			return ServerEntry{}, fmt.Errorf("could not resolve this binary's path: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		command = executable
	}

	// --command replaces only the executable, so an absolute path to npx keeps
	// the package arguments the wrapper needs.
	if parsed.command != "" {
		command = parsed.command
	}

	if parsed.mode != "" {
		args = append(args, "--mode", parsed.mode)
	}

	return ServerEntry{
		Command: command,
		Args:    args,
		Env:     map[string]string{"DOORAY_TOKEN": token},
	}, nil
}

func printableConfig(name string, entry ServerEntry) (string, error) {
	encoded, err := json.MarshalIndent(map[string]any{
		"mcpServers": map[string]ServerEntry{name: entry},
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func parseOptions(argv []string) (options, error) {
	parsed := options{client: "claude-desktop", name: defaultServerName}

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
		case arg == "--force":
			parsed.force = true
		case arg == "--print":
			parsed.print = true
		case arg == "--client":
			parsed.client, err = takeValue(&index, "--client")
		case strings.HasPrefix(arg, "--client="):
			parsed.client = strings.TrimPrefix(arg, "--client=")
		case arg == "--name":
			parsed.name, err = takeValue(&index, "--name")
		case strings.HasPrefix(arg, "--name="):
			parsed.name = strings.TrimPrefix(arg, "--name=")
		case arg == "--token":
			parsed.token, err = takeValue(&index, "--token")
		case strings.HasPrefix(arg, "--token="):
			parsed.token = strings.TrimPrefix(arg, "--token=")
		case arg == "--mode":
			parsed.mode, err = takeValue(&index, "--mode")
		case strings.HasPrefix(arg, "--mode="):
			parsed.mode = strings.TrimPrefix(arg, "--mode=")
		case arg == "--command":
			parsed.command, err = takeValue(&index, "--command")
		case strings.HasPrefix(arg, "--command="):
			parsed.command = strings.TrimPrefix(arg, "--command=")
		case arg == "--config":
			parsed.configPath, err = takeValue(&index, "--config")
		case strings.HasPrefix(arg, "--config="):
			parsed.configPath = strings.TrimPrefix(arg, "--config=")
		default:
			return options{}, fmt.Errorf("unknown option: %s", arg)
		}

		if err != nil {
			return options{}, err
		}
	}

	if parsed.name == "" {
		return options{}, fmt.Errorf("--name must not be empty")
	}
	if parsed.mode != "" && parsed.mode != "full" && parsed.mode != "read-only" {
		return options{}, fmt.Errorf("--mode must be one of: full, read-only")
	}

	return parsed, nil
}

// HelpText is the usage message for the register subcommand.
func HelpText() string {
	return `Usage: dooray-mcp register [--token <dooray-token>] [options]

Merges this server into the Claude Desktop configuration, keeping every other
server and setting in the file. The previous file is backed up as
claude_desktop_config.json.bak.

The recorded command is the npx invocation when this runs through the npm
wrapper, and otherwise this binary's own absolute path. No executable is ever
written by this command.

Options:
  --token <token>     Dooray personal API token. Defaults to DOORAY_TOKEN.
  --name <name>       MCP server name in the config. Default: dooray
  --mode read-only    Register the server with only read-only tools exposed.
  --client <client>   Target client. Only claude-desktop is supported.
  --command <path>    Executable to record. Defaults to this binary's own
                      absolute path.
  --config <path>     Configuration file to merge into. Defaults to the
                      platform's Claude Desktop config path.
  --force             Replace an existing server with the same name.
  --print             Print the JSON block instead of writing the file.
  -h, --help          Print this message.
`
}
