# Installs the dooray-mcp binary for Windows into a directory on PATH.
#
#   irm https://raw.githubusercontent.com/minseoky/dooray-mcp-go/main/scripts/install.ps1 | iex
#
# Parameters can also be supplied as environment variables:
#   DOORAY_MCP_VERSION  release tag to install, default: latest
#   DOORAY_MCP_BIN_DIR  install directory, default: %LOCALAPPDATA%\Programs\dooray-mcp

$ErrorActionPreference = 'Stop'

$repo = 'minseoky/dooray-mcp-go'
$version = if ($env:DOORAY_MCP_VERSION) { $env:DOORAY_MCP_VERSION } else { 'latest' }
$binDir = if ($env:DOORAY_MCP_BIN_DIR) { $env:DOORAY_MCP_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\dooray-mcp' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

$asset = "dooray-mcp_windows_$arch.zip"
$checksumFile = 'SHA256SUMS'
$baseUrl = if ($version -eq 'latest') {
    "https://github.com/$repo/releases/latest/download"
} else {
    "https://github.com/$repo/releases/download/$version"
}

$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    Write-Host "downloading $baseUrl/$asset"
    $archivePath = Join-Path $tempDir $asset
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $archivePath -UseBasicParsing

    # The release publishes SHA256SUMS alongside the archives. A mismatch means
    # the download was corrupted or tampered with, so installation must stop.
    Write-Host "downloading $baseUrl/$checksumFile"
    $checksumPath = Join-Path $tempDir $checksumFile
    Invoke-WebRequest -Uri "$baseUrl/$checksumFile" -OutFile $checksumPath -UseBasicParsing

    $expected = $null
    foreach ($line in Get-Content $checksumPath) {
        $fields = $line -split '\s+', 2
        if ($fields.Count -eq 2 -and $fields[1].Trim().TrimStart('*') -eq $asset) {
            $expected = $fields[0].Trim()
            break
        }
    }
    if (-not $expected) {
        throw "$checksumFile has no entry for $asset; refusing to install."
    }

    $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash
    if ($expected -ne $actual) {
        throw "checksum mismatch for ${asset}: expected $expected, got $actual"
    }
    Write-Host "checksum verified: $actual"

    Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force

    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    $target = Join-Path $binDir 'dooray-mcp.exe'
    Move-Item -Path (Join-Path $tempDir "dooray-mcp_windows_$arch.exe") -Destination $target -Force

    Write-Host "installed: $target"

    # Append the install directory to the per-user PATH when it is missing, so
    # an MCP config can use the bare command name.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$binDir*") {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$binDir", 'User')
        Write-Host "added $binDir to your user PATH; restart your terminal to pick it up."
    }

    # The configuration is written here rather than by launching the binary. A
    # process that has just written an executable and then runs it is what a
    # dropper does, and endpoint protection scores it that way, so the freshly
    # installed file is left alone until Claude Desktop starts it.
    if (-not $env:DOORAY_TOKEN) {
        Write-Host ""
        Write-Host "Set DOORAY_TOKEN and re-run to configure Claude Desktop automatically."
        Write-Host "Otherwise add this under mcpServers in claude_desktop_config.json:"
        Write-Host ""
        Write-Host "    dooray: command = $target, env.DOORAY_TOKEN = <your token>"
        return
    }

    # Claude Desktop does not keep its configuration in the same directory on
    # every machine: a packaged install has its Roaming writes redirected into
    # a per-package LocalCache. Prefer a file that already exists.
    $candidates = @(Join-Path $env:APPDATA "Claude\claude_desktop_config.json")
    $packages = Join-Path $env:LOCALAPPDATA "Packages"
    if (Test-Path $packages) {
        $candidates += Get-ChildItem -Path $packages -Directory -ErrorAction SilentlyContinue |
            ForEach-Object { Join-Path $_.FullName "LocalCache\Roaming\Claude\claude_desktop_config.json" }
    }
    $candidates += Join-Path $env:LOCALAPPDATA "Claude\claude_desktop_config.json"

    $configPath = $candidates | Where-Object { Test-Path -PathType Leaf $_ } | Select-Object -First 1
    if (-not $configPath) {
        $configPath = $candidates[0]
        Write-Host "no existing Claude Desktop configuration was found, so this one was created."
        Write-Host "locations searched:"
        $candidates | ForEach-Object { Write-Host "  $_" }
    }

    $config = @{}
    if (Test-Path -PathType Leaf $configPath) {
        Copy-Item $configPath "$configPath.bak" -Force
        $raw = Get-Content -Raw $configPath
        if ($raw.Trim()) { $config = $raw | ConvertFrom-Json -AsHashtable }
    } else {
        New-Item -ItemType Directory -Path (Split-Path -Parent $configPath) -Force | Out-Null
    }

    if (-not $config.ContainsKey("mcpServers")) { $config["mcpServers"] = @{} }
    $config["mcpServers"]["dooray"] = @{
        command = $target
        env     = @{ DOORAY_TOKEN = $env:DOORAY_TOKEN }
    }

    $config | ConvertTo-Json -Depth 10 | Set-Content -Path $configPath -Encoding utf8
    Write-Host "registered MCP server 'dooray' in $configPath"
    Write-Host "restart Claude Desktop to load the server."
} finally {
    Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
}
