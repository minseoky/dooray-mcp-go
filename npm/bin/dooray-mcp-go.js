#!/usr/bin/env node
// Thin launcher that runs the prebuilt Go binary for the current platform.
// The binaries ship inside the package, so installing needs no network access
// beyond the npm registry itself.

import { spawn } from "node:child_process";
import { chmodSync, existsSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import process from "node:process";

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const platformNames = {
  darwin: "darwin",
  win32: "windows",
  linux: "linux",
};

const architectureNames = {
  x64: "amd64",
  arm64: "arm64",
};

function resolveBinaryPath() {
  const platform = platformNames[process.platform];
  const architecture = architectureNames[process.arch];

  if (!platform || !architecture) {
    throw new Error(
      `dooray-mcp-go has no prebuilt binary for ${process.platform}/${process.arch}. ` +
        "Build from source with: go install github.com/minseoky/dooray-mcp-go@latest",
    );
  }

  const extension = platform === "windows" ? ".exe" : "";
  const binaryPath = path.join(
    packageRoot,
    "binaries",
    `dooray-mcp_${platform}_${architecture}${extension}`,
  );

  if (!existsSync(binaryPath)) {
    throw new Error(`dooray-mcp-go binary is missing: ${binaryPath}`);
  }

  return binaryPath;
}

let binaryPath;
try {
  binaryPath = resolveBinaryPath();
} catch (error) {
  console.error(error.message);
  process.exit(1);
}

if (process.platform !== "win32") {
  // npm does not always preserve the executable bit through a tarball.
  try {
    chmodSync(binaryPath, 0o755);
  } catch {
    // A read-only install directory is fine as long as the bit is already set.
  }
}

// `register` records the command an MCP client should spawn. Running through
// npx means the binary lives in a cache npx may evict, so the child is told to
// record the npx invocation instead of its own path.
function npmSpec() {
  try {
    const manifest = JSON.parse(readFileSync(path.join(packageRoot, "package.json"), "utf8"));
    return `${manifest.name}@${manifest.version}`;
  } catch {
    return "dooray-mcp-go";
  }
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
  env: { ...process.env, DOORAY_MCP_NPM_SPEC: npmSpec() },
});

child.on("error", (error) => {
  console.error(`failed to start dooray-mcp: ${error.message}`);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => child.kill(signal));
}
