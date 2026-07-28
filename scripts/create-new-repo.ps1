param(
    [Parameter(Mandatory = $false)]
    [string]$Destination = "$HOME\src\gereh-platform"
)

$ErrorActionPreference = "Stop"

if (Test-Path $Destination) {
    throw "Destination already exists: $Destination"
}

New-Item -ItemType Directory -Force -Path $Destination | Out-Null

$SourceRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Get-ChildItem -Path $SourceRoot -Force |
    Where-Object { $_.Name -ne ".git" } |
    Copy-Item -Destination $Destination -Recurse -Force

Set-Location $Destination

git init
git add .
git commit -m "Bootstrap Gereh cloud platform"
git branch -M main

Write-Host "Created repository at $Destination"
Write-Host "Open it from WSL for best Docker/Dev Container performance."
