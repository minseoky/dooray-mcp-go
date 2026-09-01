# dooray-mcp-go (npm wrapper)

This package ships the prebuilt `dooray-mcp` Go binaries and a launcher that selects the one matching the current platform, so the server can be installed and registered with a single `npx` command.

Register it with Claude Desktop in one command:

```sh
npx -y --package=dooray-mcp-go -- dooray-mcp-go register --token "{personal-token}" --force
```

That writes the configuration with `npx` as the launch command:

```json
{
  "mcpServers": {
    "dooray": {
      "command": "npx",
      "args": ["-y", "--package=dooray-mcp-go@0.1.3", "--", "dooray-mcp-go"],
      "env": { "DOORAY_TOKEN": "{personal-token}" }
    }
  }
}
```

Nothing is copied or installed by the binary itself. Every start does unpack and spawn the executable in one step, which endpoint protection can read as a dropper launching its payload; the [repository README](https://github.com/minseoky/dooray-mcp-go#what-is-and-is-not-written-as-an-executable) covers that tradeoff and the routes that avoid it.

Full documentation lives in the [repository README](https://github.com/minseoky/dooray-mcp-go#readme).

The binaries are committed into the tarball rather than fetched by a postinstall script, so installing works behind a proxy that only allows the npm registry.
