# dooray-mcp-go (npm wrapper)

This package ships the prebuilt `dooray-mcp` Go binaries and a launcher that selects the one matching the current platform, so the server can be installed and registered with a single `npx` command.

Register it with Claude Desktop in one command:

```sh
npx -y dooray-mcp-go register --token "{personal-token}"
```

That copies the binary into a stable per-user directory and writes a config pointing at it.

Do not leave `npx` itself as the launch command in an MCP config. Every start would then unpack the executable and spawn it from one process, which endpoint protection reads as a dropper launching its own payload.

Full documentation lives in the [repository README](https://github.com/minseoky/dooray-mcp-go#readme).

The binaries are committed into the tarball rather than fetched by a postinstall script, so installing works behind a proxy that only allows the npm registry.
