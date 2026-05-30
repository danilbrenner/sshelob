$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$Repo = "danilbrenner/sshelob"

function Get-Arch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Main {
    if (-not $IsWindows) {
        throw "This script is intended for Windows. Use ./get.sh on Linux/macOS."
    }

    $goos = "windows"
    $goarch = Get-Arch

    $headers = @{
        "Accept" = "application/vnd.github+json"
        "User-Agent" = "sshelob-installer"
    }

    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers $headers
    if (-not $release.tag_name) {
        throw "Could not parse latest release tag"
    }

    $tag = [string]$release.tag_name
    $asset = "sshelob_${tag}_${goos}_${goarch}.zip"
    $url = "https://github.com/$Repo/releases/download/$tag/$asset"

    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("sshelob-get-" + [System.Guid]::NewGuid().ToString("N"))
    New-Item -Path $tempDir -ItemType Directory | Out-Null

    try {
        $archivePath = Join-Path $tempDir $asset
        Write-Host "Downloading $url"
        Invoke-WebRequest -Uri $url -OutFile $archivePath

        Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force

        $binary = Get-ChildItem -Path $tempDir -Recurse -File -Filter "sshelob.exe" | Select-Object -First 1
        if (-not $binary) {
            throw "Expected binary sshelob.exe not found in archive"
        }

        $installPath = Join-Path (Get-Location) "sshelob.exe"
        Copy-Item -Path $binary.FullName -Destination $installPath -Force

        Write-Host "Installed $installPath from $tag"
    }
    finally {
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force
        }
    }
}

Main
