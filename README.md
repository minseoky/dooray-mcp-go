# dooray-mcp-go

Dooray MCP server implemented in Go. It exposes Dooray's account, calendar, project, post, attachment, and messenger APIs to MCP clients such as Claude Desktop, and ships as a single static binary with no runtime dependency.

## Requirements

- A Dooray personal API token
- Nothing else at runtime. Go 1.21+ is needed only to build from source, and Node.js only for the `npx` install route.

## Official Documentation

- [Dooray API official documentation](https://helpdesk.dooray.com/share/pages/9wWo-xwiR66BO5LGshgVTg/2939987729788437786)

## Scope

This MCP server does not wrap every Dooray API. It focuses on frequently used account, calendar, project, post, attachment, and messenger APIs.

Most `/admin/v1` and `/admin/v2` administration APIs are intentionally not exposed, especially write-capable administration endpoints.

## What the MCP client receives

Every tool returns the Dooray API response as-is. A task body, comment, calendar entry, or member record is passed to the MCP client unchanged, and from there to whatever model backs it — including anything the original Dooray content happens to contain, such as personal data or material your organization classifies as confidential.

The server does not classify, redact, or filter that content, and it cannot tell which posts are sensitive. Deciding what may leave Dooray is the caller's responsibility:

- Scope requests to the projects and posts that actually need to be read, rather than sweeping whole projects.
- Treat `dooray_post_file_download` the same way. It writes attachments, including inline body images, to a local temporary directory that is not cleaned up automatically.
- Run with `--mode read-only` when a session only needs to read, so no tool can write back to Dooray.
- Check your organization's policy before pointing this at projects holding personal or confidential data.

## Write tools require confirmation

The four write-capable tools — `dooray_messenger`, `dooray_calendar_post_event`, `dooray_post_log_create`, and `dooray_post_log_update` — take a required `confirm` boolean. The handler refuses the call unless it is exactly `true`, before any request reaches Dooray, so passing schema validation is not on its own enough to send a message or post a comment. Set it only after the user has confirmed the specific change.

## Read-only mode

Dooray personal API tokens are not issued with separate read-only and write permissions. A token that can call write APIs may still have those permissions at the Dooray API level.

For safer installations, this server provides a read-only mode at the MCP tool layer. In read-only mode, write-capable tools are not exposed in `tools/list` and cannot be called through `tools/call`.

## Install

Pick whichever route fits how the machine is already set up. All of them end at the same binary.

### 1. npx

The published `dooray-mcp-go` npm package contains the prebuilt binaries for every supported platform and a launcher that picks the right one. Nothing is compiled or downloaded at install time beyond the package itself. This route needs only Node.js 16 or newer.

```bash
npx -y dooray-mcp-go --token "{personal-token}"
```

### 2. Install script

macOS and Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/minseoky/dooray-mcp-go/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/minseoky/dooray-mcp-go/main/scripts/install.ps1 | iex
```

Both scripts download `SHA256SUMS` from the same release, verify the archive against it, and abort without installing anything if the checksum is missing or does not match. They then drop `dooray-mcp` into a per-user directory and put that directory on `PATH`, so the MCP config can use the bare command name on either OS.

Checksum verification protects against a corrupted or substituted download. It does not by itself prove the release is genuine, since the checksum file comes from the same release; the binaries are not signed with GPG or cosign.

Install and register with Claude Desktop in one go:

```sh
curl -fsSL https://raw.githubusercontent.com/minseoky/dooray-mcp-go/main/scripts/install.sh | sh
"$HOME/.local/bin/dooray-mcp" register --token "{personal-token}"
```

```powershell
irm https://raw.githubusercontent.com/minseoky/dooray-mcp-go/main/scripts/install.ps1 | iex
& "$env:LOCALAPPDATA\Programs\dooray-mcp\dooray-mcp.exe" register --token "{personal-token}"
```

### 3. Go toolchain

```bash
go install github.com/minseoky/dooray-mcp-go@latest
```

`go install` names the binary after the module, so this installs `dooray-mcp-go` into `$(go env GOPATH)/bin`. Use that name in the MCP config, or rename it to `dooray-mcp`.

### 4. Direct download

Grab the archive for your platform from the [releases page](https://github.com/minseoky/dooray-mcp-go/releases), unpack it, and point the MCP config at the absolute path. Verify it against `SHA256SUMS` from the same release first:

```sh
shasum -a 256 -c SHA256SUMS --ignore-missing
```

```powershell
(Get-FileHash .\dooray-mcp_windows_amd64.zip -Algorithm SHA256).Hash
```

The binaries are not code-signed. Downloading them through a browser attaches a quarantine flag, so the first run is blocked by Gatekeeper on macOS and by SmartScreen on Windows. Clear it once:

```sh
xattr -d com.apple.quarantine ./dooray-mcp
```

```powershell
Unblock-File .\dooray-mcp.exe
```

Routes 1 to 3 do not set that flag, because npm, curl, and the Go toolchain are not quarantine-aware downloaders.

## Run

```bash
dooray-mcp --token "{personal-token}"
```

Using an environment variable instead:

```bash
DOORAY_TOKEN="{personal-token}" dooray-mcp
```

Exposing only read-only tools:

```bash
DOORAY_TOKEN="{personal-token}" dooray-mcp --mode read-only
```

## Claude Desktop

Claude Desktop has no CLI, so it is configured by merging into `claude_desktop_config.json`. The binary can do that merge itself, which avoids hand-editing JSON and works the same on macOS and Windows.

### Register from the shell

```sh
dooray-mcp register --token "{personal-token}"
```

Without installing anything first, through the npm wrapper:

```sh
npx -y dooray-mcp-go register --token "{personal-token}"
```

Register a second, read-only server alongside it:

```sh
dooray-mcp register --name dooray-read-only --mode read-only --token "{personal-token}"
```

What the command does:

- Resolves the Claude Desktop config path for the current platform, and prints the file it wrote so you can confirm the location.
- Copies the current file to `claude_desktop_config.json.bak` before writing.
- Merges only the `mcpServers.<name>` entry, leaving every other server and top-level setting untouched.
- Records this binary's absolute path as `command`, or `npx -y dooray-mcp-go@<version>` when it was started through the npm wrapper.
- Refuses to overwrite an existing server of the same name unless `--force` is passed.
- Writes a new config as owner-only (`0600`), because the file holds the API token.

Restart Claude Desktop afterwards to load the server.

### If the server fails to start

Claude Desktop launches MCP servers without your shell profile, so its `PATH` is the bare system one. A config whose `command` is `npx`, `node`, or `dooray-mcp` fails with a spawn error when those live in a Homebrew, nvm, or per-user directory that the login shell adds.

Registering from the shell without the npm wrapper sidesteps this, because it records the binary's absolute path:

```sh
dooray-mcp register --force --token "{personal-token}"
```

To keep an npx-based config, record the absolute path of `npx`. Run this through the wrapper so the package arguments are kept — `--command` replaces only the executable:

```sh
npx -y dooray-mcp-go register --force --command "$(command -v npx)" --token "{personal-token}"
```

Preview the JSON without touching the file:

```sh
dooray-mcp register --print --token "{personal-token}"
```

`dooray-mcp register --help` lists every option, including `--config <path>` to merge into a different file and `--command <path>` to record a specific command.

### Register by hand

Via npx, which works the same on Windows and macOS:

```json
{
  "mcpServers": {
    "dooray": {
      "command": "npx",
      "args": ["-y", "dooray-mcp-go@0.1.0"],
      "env": {
        "DOORAY_TOKEN": "{personal-token}"
      }
    }
  }
}
```

Via a binary on `PATH`, after running one of the install scripts:

```json
{
  "mcpServers": {
    "dooray": {
      "command": "dooray-mcp",
      "env": {
        "DOORAY_TOKEN": "{personal-token}"
      }
    }
  }
}
```

Read-only configuration:

```json
{
  "mcpServers": {
    "dooray-read-only": {
      "command": "dooray-mcp",
      "args": ["--mode", "read-only"],
      "env": {
        "DOORAY_TOKEN": "{personal-token}"
      }
    }
  }
}
```

On Windows, a config that points at an absolute path must use `dooray-mcp.exe` and escaped backslashes, for example `"C:\\Users\\me\\AppData\\Local\\Programs\\dooray-mcp\\dooray-mcp.exe"`.

## Claude Code

```bash
claude mcp add dooray --env DOORAY_TOKEN="{personal-token}" -- dooray-mcp
```

Read-only registration:

```bash
claude mcp add dooray-read-only --env DOORAY_TOKEN="{personal-token}" -- dooray-mcp --mode read-only
```

## Codex

```bash
codex mcp add dooray --env DOORAY_TOKEN="$DOORAY_TOKEN" -- dooray-mcp
```

```bash
codex mcp add dooray-read-only --env DOORAY_TOKEN="$DOORAY_TOKEN" -- dooray-mcp --mode read-only
```

## Project Structure

```text
dooray-mcp-go/
├── main.go                        # entry point: config, client, registry, stdio server
├── internal/
│   ├── config/config.go           # flags and environment variables
│   ├── dooray/client.go           # authenticated API client and attachment download
│   ├── dooray/filename.go         # Content-Disposition parsing and file name sanitizing
│   ├── jsonschema/schema.go       # ordered JSON Schema builders
│   ├── mcp/server.go              # JSON-RPC 2.0 stdio transport and MCP methods
│   ├── register/                  # `register` subcommand and config merging
│   └── tools/                     # tool definitions, handlers, argument helpers
├── npm/                           # npm wrapper that ships the prebuilt binaries
├── scripts/install.sh             # macOS and Linux installer
├── scripts/install.ps1            # Windows installer
├── .github/workflows/release.yml  # test, cross-compile, release, npm publish
├── Makefile
└── README.md
```

## Build

```bash
make            # gofmt, vet, test, and build ./dooray-mcp
make test
make release    # cross-compile every platform into dist/ with SHA256SUMS
make npm-stage  # stage the binaries into the npm wrapper package
```

`make release` builds `darwin/arm64`, `darwin/amd64`, `windows/amd64`, `windows/arm64`, `linux/amd64`, and `linux/arm64` with `CGO_ENABLED=0`, so each binary is static and runs on a clean machine.

Tagging `vX.Y.Z` runs `.github/workflows/release.yml`, which tests, cross-compiles, attaches the archives to a GitHub release, and publishes the npm wrapper.

## Tools

- `dooray_messenger` (`confirm` must be `true`)
- `dooray_calendar_calendars`
- `dooray_calendar_events`
- `dooray_calendar_post_event` (`confirm` must be `true`)
- `dooray_account_members`
- `dooray_account_member`
- `dooray_project`
- `dooray_posts` (finds task posts and exposes the task body plus `fileIdList`, which can contain inline body images/files)
- `dooray_post_logs` (find comments and activity logs for a post)
- `dooray_post_log` (find one comment or activity log by ID)
- `dooray_post_log_create` (add a comment to a post; body is `{ "mimeType": "text/x-markdown", "content": "..." }`, and `confirm` must be `true`)
- `dooray_post_log_update` (update a comment or activity log with the same body format, and `confirm` must be `true`)
- `dooray_post_files` (lists regular attachments; an empty result or `AUTH_FORBIDDEN_ERROR` does not determine whether `fileIdList` items can be downloaded)
- `dooray_post_file_download` (downloads IDs from `dooray_posts.fileIdList`, including inline body images, or regular attachment file IDs with `media=raw`; authorization is limited to the configured API origin and HTTPS `file-api.dooray.com`)
- `os`

### Reading task URLs and body files

Use this workflow when a request asks for a Dooray task body, an image embedded in the body, or a file referenced by the body. Comments and activity logs are a separate resource and are not required for this workflow.

1. Parse the task URL. `/task/{projectId}/{postId}` provides both IDs directly. The legacy `/project/tasks/{postId}` form provides only `postId`, so its trailing number must not be used as `projectId`.
2. Resolve the project only for the legacy URL form. Call `dooray_project` with `operation=find_projects` and the required `type`, `scope`, and `state` filters. Repeat the relevant filter combinations and continue through `page` values with `size` up to `100` until every result page has been checked.
3. Search for the matching post. Call `dooray_posts` with the project ID from the new URL form or the candidate project IDs discovered for a legacy URL. Use `size` up to `100`, continue through `page` values as needed, and match the returned post ID to the URL's `postId`. A lookup under one incorrect project ID or only the first result page is not evidence that the task or its files are unavailable.
4. Read the task body from the matching `dooray_posts` result. Do not call `dooray_post_logs` unless the user explicitly asks for comments or activity history.
5. If the matching post contains `fileIdList`, call `dooray_post_file_download` once for every listed ID using the same verified `projectId` and `postId`. These IDs commonly represent images embedded in the task body, but can represent other body files as well.
6. Inspect the returned local `filePath` with an image viewer or an appropriate document parser. The download result also includes `fileName`, `mimeType`, `size`, and `temporary`.

`dooray_post_files` is exposed for listing regular attachments, but it is separate from the `fileIdList` path used by body files. It can return an empty list or `AUTH_FORBIDDEN_ERROR` even when direct downloads from `dooray_posts.fileIdList` work. Therefore, neither outcome should be reported as proof that a body image or file is inaccessible. A transport failure or timeout from `dooray_post_file_download` is also not a permission result, and should be retried or reported separately. Report a file as forbidden or missing only when the direct download returns a terminal Dooray response such as `403` or `404` with the verified `projectId`, `postId`, and `fileId`.

### What the MCP client receives

Every tool returns the Dooray API response as-is. A task body, comment, calendar entry, or member record is passed to the MCP client unchanged, and from there to whatever model backs it — including anything the original Dooray content happens to contain, such as personal data or material your organization classifies as confidential.

The server does not classify, redact, or filter that content, and it cannot tell which posts are sensitive. Deciding what may leave Dooray is the caller's responsibility:

- Scope requests to the projects and posts that actually need to be read, rather than sweeping whole projects.
- Treat `dooray_post_file_download` the same way. It writes attachments, including inline body images, to a local temporary directory that is not cleaned up automatically.
- Run with `--mode read-only` when a session only needs to read, so no tool can write back to Dooray.
- Check your organization's policy before pointing this at projects holding personal or confidential data.

## Write tools require confirmation

The four write-capable tools — `dooray_messenger`, `dooray_calendar_post_event`, `dooray_post_log_create`, and `dooray_post_log_update` — take a required `confirm` boolean. The handler refuses the call unless it is exactly `true`, before any request reaches Dooray, so passing schema validation is not on its own enough to send a message or post a comment. Set it only after the user has confirmed the specific change.

## Read-only mode tools

When `--mode read-only` or `DOORAY_MCP_MODE=read-only` is set, write-capable Dooray tools are not exposed in `tools/list` and cannot be called through `tools/call`.

Exposed in read-only mode:

- `dooray_calendar_calendars`
- `dooray_calendar_events`
- `dooray_account_members`
- `dooray_account_member`
- `dooray_project`
- `dooray_posts`
- `dooray_post_logs`
- `dooray_post_log`
- `dooray_post_files`
- `dooray_post_file_download`
- `os`

Hidden in read-only mode:

- `dooray_messenger`
- `dooray_calendar_post_event`
- `dooray_post_log_create`
- `dooray_post_log_update`

## Options

- `--token`: Dooray personal API token. Defaults to `DOORAY_TOKEN`.
- `--endpoint`: Dooray API endpoint. Defaults to `DOORAY_ENDPOINT`, then `https://api.dooray.com`.
- `--mode`: tool exposure mode. Use `full` or `read-only`. Defaults to `DOORAY_MCP_MODE` or `full`.
- `--help`: print usage and exit.

## Subcommands

- `register`: merge this server into the Claude Desktop configuration. See [Claude Desktop](#claude-desktop).

## Environment

- `DOORAY_TOKEN`: Dooray personal API token.
- `DOORAY_ENDPOINT`: Dooray API endpoint, default `https://api.dooray.com`.
- `DOORAY_MCP_MODE`: `full` or `read-only`, default `full`.
- `DOORAY_REQUEST_TIMEOUT_MS`: per-request timeout in milliseconds, default `30000`.
- `DOORAY_DOWNLOAD_DIR`: attachment download directory, default `<system temp>/dooray-mcp`.

## Notes on cross-platform behavior

- Downloaded file names are stripped of directory components and of the characters Windows rejects (`\ / : * ? " < > |`), plus trailing dots and spaces.
- The download directory defaults to the platform temp directory: `%TEMP%\dooray-mcp` on Windows, `$TMPDIR/dooray-mcp` on macOS.
- The stdio transport accepts both LF and CRLF line endings, so Windows MCP clients work without configuration.
- Binaries are built with `CGO_ENABLED=0`, so they have no libc or runtime dependency.
