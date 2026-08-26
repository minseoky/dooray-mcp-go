# dooray-mcp-go (npm wrapper)

This package ships the prebuilt `dooray-mcp` Go binaries and a launcher that selects the one matching the current platform, so the server can be installed and registered with a single `npx` command.

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

The package can also write that config for you:

```sh
npx -y dooray-mcp-go register --token "{personal-token}"
```

Full documentation lives in the [repository README](https://github.com/minseoky/dooray-mcp-go#readme).

The binaries are committed into the tarball rather than fetched by a postinstall script, so installing works behind a proxy that only allows the npm registry.
