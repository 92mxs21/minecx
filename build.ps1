# Builds minecx for Windows and Linux (amd64 + arm64) into .\dist
$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

$out = 'dist'
New-Item -ItemType Directory -Force -Path $out | Out-Null

$env:CGO_ENABLED = '0'
$ldflags = '-s -w'

function Build-Target($goos, $goarch, $name) {
    Write-Host "==> $goos/$goarch -> $out\$name"
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    go build -trimpath -ldflags $ldflags -o "$out\$name" .
    if ($LASTEXITCODE -ne 0) { throw "build failed for $goos/$goarch" }
}

Build-Target 'windows' 'amd64' 'minecx-windows-amd64.exe'
Build-Target 'linux'   'amd64' 'minecx-linux-amd64'
Build-Target 'linux'   'arm64' 'minecx-linux-arm64'

Remove-Item Env:GOOS       -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH     -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host ''
Write-Host 'Built:'
Get-ChildItem $out | Select-Object Name, Length | Format-Table -AutoSize
