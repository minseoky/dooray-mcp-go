# dooray-mcp-go (npm wrapper)

This package ships the prebuilt `dooray-mcp` Go binaries and a launcher that selects the one matching the current platform, so the server can be installed and registered with a single `npx` command.

Run the server directly:

```sh
DOORAY_TOKEN="{personal-token}" npx -y dooray-mcp-go
```

This package is not an installer. `register` is refused when run from the npx cache, because a cache path is not worth recording and copying the binary elsewhere would make it a self-replicating executable.

To install and configure Claude Desktop, use the `.mcpb` bundle or an install script from the [repository README](https://github.com/minseoky/dooray-mcp-go#install). Do not put `npx` in an MCP config as the launch command either: every start would unpack the executable and spawn it from one process, which endpoint protection reads as a dropper launching its payload.

Full documentation lives in the [repository README](https://github.com/minseoky/dooray-mcp-go#readme).

The binaries are committed into the tarball rather than fetched by a postinstall script, so installing works behind a proxy that only allows the npm registry.
