// Command dooray-mcp runs the Dooray Model Context Protocol server over stdio.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/minseoky/dooray-mcp-go/internal/config"
	"github.com/minseoky/dooray-mcp-go/internal/dooray"
	"github.com/minseoky/dooray-mcp-go/internal/mcp"
	"github.com/minseoky/dooray-mcp-go/internal/register"
	"github.com/minseoky/dooray-mcp-go/internal/tools"
)

const serverName = "dooray"

// version is the reported server version. Release builds override it with
// -ldflags "-X main.version=...".
var version = "0.1.0"

func main() {
	// `register` is a setup subcommand; every other invocation speaks MCP over
	// stdio, so it must be intercepted before the server flags are parsed.
	if len(os.Args) > 1 && os.Args[1] == "register" {
		os.Exit(register.Run(os.Args[2:], os.Stdout, os.Stderr))
	}

	settings, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if settings.Help {
		fmt.Print(config.HelpText())
		return
	}

	client := dooray.New(dooray.Options{
		Endpoint:          settings.Endpoint,
		Token:             settings.Token,
		Timeout:           settings.RequestTimeout,
		DownloadDirectory: settings.DownloadDirectory,
	})

	registry := tools.NewRegistry(client, settings.Mode == config.ModeReadOnly)
	server := mcp.NewServer(serverName, version, registry.Tools, registry.Handlers, os.Stdout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := server.Serve(ctx, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
