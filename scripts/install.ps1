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
} finally {
    Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
}
