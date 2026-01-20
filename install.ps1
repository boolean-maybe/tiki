param(
  [string]$InstallDir = "$env:LOCALAPPDATA\Programs\tiki\bin"
)

$ErrorActionPreference = "Stop"

$repoOwner = "boolean-maybe"
$repoName = "tiki"

function Write-Info {
  param([string]$Message)
  Write-Host $Message
}

function Get-Arch {
  switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { return "amd64" }
    "ARM64" { return "arm64" }
    default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
  }
}

$arch = Get-Arch
$apiUrl = "https://api.github.com/repos/$repoOwner/$repoName/releases/latest"
$release = Invoke-RestMethod -Uri $apiUrl
$tag = $release.tag_name
if (-not $tag) {
  throw "failed to resolve latest release tag"
}

$version = $tag.TrimStart("v")
$asset = "tiki_{0}_windows_{1}.zip" -f $version, $arch
$baseUrl = "https://github.com/$repoOwner/$repoName/releases/download/$tag"

$tempDir = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath()) -Name ("tiki-" + [System.Guid]::NewGuid().ToString("N"))
$zipPath = Join-Path $tempDir $asset
$checksumsPath = Join-Path $tempDir "checksums.txt"

Write-Info "downloading $asset"
Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile $zipPath
Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumsPath

$checksumLine = Get-Content $checksumsPath | Where-Object { $_ -match (" {0}$" -f [Regex]::Escape($asset)) } | Select-Object -First 1
if (-not $checksumLine) {
  throw "checksum not found for $asset"
}

$expectedChecksum = ($checksumLine -split "\s+")[0]
$actualChecksum = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($expectedChecksum.ToLowerInvariant() -ne $actualChecksum) {
  throw "checksum mismatch"
}

Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force
$exePath = Join-Path $tempDir "tiki.exe"
if (-not (Test-Path $exePath)) {
  throw "tiki.exe not found in archive"
}

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -Path $exePath -Destination (Join-Path $InstallDir "tiki.exe") -Force

Write-Info "installed tiki to $InstallDir\tiki.exe"
Write-Info "add to path: setx PATH `"$InstallDir;$env:PATH`""
Write-Info "run: tiki --version"
