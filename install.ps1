<#
.SYNOPSIS
Install the latest (or a pinned) rtdd release binary on Windows.

.DESCRIPTION
    irm https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.ps1 | iex

install.sh is the Linux/macOS route and also works under Git Bash, MSYS2 and Cygwin. This
script is the native-PowerShell route, for a shell with no POSIX sh behind it.

Environment variables:
  RTDD_VERSION      tag to install, e.g. v1.2.3 (default: latest release)
  RTDD_INSTALL_DIR  install destination (default: %LOCALAPPDATA%\Programs\rtdd)
  RTDD_NO_SKILL     set to any value to skip installing the machine-wide agent skill
  RTDD_BASE_URL     override the release-assets base URL (for testing; undocumented)
  RTDD_API_URL      override the GitHub "latest release" API URL (same purpose)
#>

#Requires -Version 5.1
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Repo = 'VocanicZ/rtdd'
$BaseUrl = if ($env:RTDD_BASE_URL) { $env:RTDD_BASE_URL } else { "https://github.com/$Repo/releases/download" }
$ApiUrl = if ($env:RTDD_API_URL) { $env:RTDD_API_URL } else { "https://api.github.com/repos/$Repo/releases/latest" }

function Write-Log($Message) { Write-Host $Message }
function Stop-Install($Message) { Write-Host "install.ps1: $Message" -ForegroundColor Red; exit 1 }

# PROCESSOR_ARCHITECTURE reports the architecture of the PROCESS, not the machine: a 32-bit
# PowerShell on 64-bit Windows reports x86 and puts the real answer in PROCESSOR_ARCHITEW6432.
# Reading only the first would install a 32-bit binary rtdd does not ship.
$machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
switch ($machine) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' {
        # .goreleaser.yaml ignores windows/arm64, so there is no asset to fetch. Saying so
        # beats a 404 on the download.
        Stop-Install "rtdd does not ship a windows/arm64 binary; build from source with: go build ./cmd/rtdd"
    }
    default { Stop-Install "unsupported architecture: $machine; rtdd ships a windows/amd64 binary only" }
}

$version = $env:RTDD_VERSION
if (-not $version) {
    Write-Log 'resolving the latest rtdd release...'
    try {
        $release = Invoke-RestMethod -Uri $ApiUrl -UseBasicParsing
    } catch {
        Stop-Install "could not reach $ApiUrl : $($_.Exception.Message)"
    }
    $version = $release.tag_name
    if (-not $version) { Stop-Install "could not resolve the latest release version from $ApiUrl" }
}

$versionNum = $version -replace '^v', ''
$archive = "rtdd_${versionNum}_windows_${arch}.zip"

$installDir = $env:RTDD_INSTALL_DIR
if (-not $installDir) { $installDir = Join-Path $env:LOCALAPPDATA 'Programs\rtdd' }
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$work = Join-Path ([System.IO.Path]::GetTempPath()) ("rtdd-install-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $work | Out-Null

try {
    $archivePath = Join-Path $work $archive
    $sumsPath = Join-Path $work 'checksums.txt'

    Write-Log "downloading $archive ($version)..."
    # Invoke-WebRequest on Windows PowerShell 5.1 uses the Internet Explorer engine unless
    # -UseBasicParsing, which fails outright on a machine where IE was never configured.
    Invoke-WebRequest -Uri "$BaseUrl/$version/$archive" -OutFile $archivePath -UseBasicParsing
    Invoke-WebRequest -Uri "$BaseUrl/$version/checksums.txt" -OutFile $sumsPath -UseBasicParsing

    Write-Log 'verifying checksum...'
    $line = Get-Content $sumsPath | Where-Object { $_ -match ('\s' + [regex]::Escape($archive) + '$') } | Select-Object -First 1
    if (-not $line) { Stop-Install "no checksum entry for $archive in checksums.txt" }
    $expected = ($line -split '\s+')[0]
    $actual = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash
    # Get-FileHash returns uppercase; sha256sum writes lowercase.
    if ($expected -ine $actual) {
        Stop-Install "checksum mismatch for $archive : expected $expected, got $actual"
    }

    Write-Log 'extracting...'
    Expand-Archive -Path $archivePath -DestinationPath $work -Force
    $exe = Join-Path $work 'rtdd.exe'
    if (-not (Test-Path $exe)) { Stop-Install "$archive did not contain rtdd.exe" }

    $target = Join-Path $installDir 'rtdd.exe'
    Move-Item -Path $exe -Destination $target -Force
    Write-Log "rtdd $version installed to $target"
} finally {
    Remove-Item -Path $work -Recurse -Force -ErrorAction SilentlyContinue
}

# Windows has no /usr/local/bin, so an install directory nothing points at leaves `rtdd`
# unresolvable in a fresh shell. The user PATH is per-user and needs no admin rights, and it
# is only appended to when the directory is genuinely absent — re-running the installer must
# not grow PATH by one copy each time.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $userPath) { $userPath = '' }
$parts = $userPath -split ';' | Where-Object { $_ -ne '' }
if ($parts -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', (($parts + $installDir) -join ';'), 'User')
    Write-Log "added $installDir to your user PATH — open a new terminal for it to take effect"
}
# The current session's PATH is a copy made at launch, so it needs updating separately for
# the skill install below (and for the user's next command in this same window) to resolve.
if (($env:Path -split ';') -notcontains $installDir) { $env:Path = "$env:Path;$installDir" }

# Install the machine-wide agent front-ends, so an agent knows rtdd exists without the user
# having to find and run `rtdd init` first. The installed binary is invoked rather than this
# script writing the files, because the front-ends are generated from protocol/PROTOCOL.md
# and embedded in the binary: a copy pasted in here would be a second source of truth.
#
# Deliberately non-fatal, for the same reason as in install.sh: the binary install is what
# the user asked for and it has already succeeded.
if (-not $env:RTDD_NO_SKILL) {
    $exePath = Join-Path $installDir 'rtdd.exe'
    # & does not throw on a non-zero exit code, so $LASTEXITCODE is the only signal here.
    & $exePath skill install
    if ($LASTEXITCODE -ne 0) {
        Write-Log 'note: could not install the agent skill; rtdd itself is installed and usable.'
        Write-Log '      run `rtdd skill install` to retry, or `rtdd skill prompt` to install it by hand.'
    }
}
