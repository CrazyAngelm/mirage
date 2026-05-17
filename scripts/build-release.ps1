$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$release = Join-Path $root 'release'
$sidecarCache = Join-Path $root 'bin\sidecar-cache'
$windowsStaging = Join-Path $release 'mirage-windows-amd64'
New-Item -ItemType Directory -Force $release | Out-Null
New-Item -ItemType Directory -Force $sidecarCache | Out-Null

function Get-CheckedDownload {
  param(
    [Parameter(Mandatory = $true)][string]$Url,
    [Parameter(Mandatory = $true)][string]$Sha256,
    [Parameter(Mandatory = $true)][string]$OutputPath
  )
  if (-not (Test-Path $OutputPath)) {
    Invoke-WebRequest -Uri $Url -OutFile $OutputPath
  }
  $actual = (Get-FileHash -Algorithm SHA256 $OutputPath).Hash.ToLower()
  if ($actual -ne $Sha256.ToLower()) {
    Remove-Item $OutputPath -Force
    throw "Checksum mismatch for $OutputPath`: got $actual want $Sha256"
  }
}

function Expand-SidecarBinary {
  param(
    [Parameter(Mandatory = $true)][string]$ArchivePath,
    [Parameter(Mandatory = $true)][string]$BinaryName,
    [Parameter(Mandatory = $true)][string]$OutputPath
  )
  $extractDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Force $extractDir | Out-Null
  try {
    Expand-Archive -Path $ArchivePath -DestinationPath $extractDir -Force
    $match = Get-ChildItem -Path $extractDir -Recurse -File -Filter $BinaryName | Select-Object -First 1
    if (-not $match) { throw "$BinaryName not found in $ArchivePath" }
    New-Item -ItemType Directory -Force (Split-Path -Parent $OutputPath) | Out-Null
    Copy-Item $match.FullName $OutputPath -Force
  } finally {
    Remove-Item $extractDir -Recurse -Force -ErrorAction SilentlyContinue
  }
}

$mingw = 'C:\Users\redmi\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin'
if (Test-Path $mingw) { $env:Path = $mingw + ';' + $env:Path }
$env:CGO_ENABLED = '1'
go -C $root test ./...
go -C $root build -ldflags '-H=windowsgui' -o .\bin\client\mirage-client.exe .\cmd\mirage-client
$env:GOOS = 'linux'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go -C $root build -o .\bin\server\mirage-server-linux-amd64 .\cmd\mirage-server
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

$singBoxUrl = 'https://github.com/SagerNet/sing-box/releases/download/v1.13.11/sing-box-1.13.11-windows-amd64.zip'
$singBoxSha256 = '30ecceaebb659195aa67d0a9a398c75c42fb263e079f5499a5f1dcecfa138507'
$xrayUrl = 'https://github.com/XTLS/Xray-core/releases/download/v26.3.27/Xray-windows-64.zip'
$xraySha256 = 'd004c39288ce9ada487c6f398c7c545f7d749e44bdfdd59dbc9f865afba4e1ad'
$singBoxArchive = Join-Path $sidecarCache 'sing-box-1.13.11-windows-amd64.zip'
$xrayArchive = Join-Path $sidecarCache 'Xray-windows-64.zip'
Get-CheckedDownload -Url $singBoxUrl -Sha256 $singBoxSha256 -OutputPath $singBoxArchive
Get-CheckedDownload -Url $xrayUrl -Sha256 $xraySha256 -OutputPath $xrayArchive

Remove-Item $windowsStaging -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force (Join-Path $windowsStaging 'bin') | Out-Null
Copy-Item (Join-Path $root 'bin\client\mirage-client.exe') (Join-Path $windowsStaging 'mirage-client.exe') -Force
Expand-SidecarBinary -ArchivePath $singBoxArchive -BinaryName 'sing-box.exe' -OutputPath (Join-Path $windowsStaging 'bin\sing-box.exe')
Expand-SidecarBinary -ArchivePath $xrayArchive -BinaryName 'xray.exe' -OutputPath (Join-Path $windowsStaging 'bin\xray.exe')

$windowsZip = Join-Path $release 'mirage-windows-amd64.zip'
Remove-Item $windowsZip -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $windowsStaging '*') -DestinationPath $windowsZip -Force

Copy-Item (Join-Path $root 'bin\client\mirage-client.exe') (Join-Path $release 'mirage-client.exe') -Force
Copy-Item (Join-Path $root 'bin\server\mirage-server-linux-amd64') (Join-Path $release 'mirage-server-linux-amd64') -Force
Copy-Item (Join-Path $root 'scripts\install-server.sh') (Join-Path $release 'install-server.sh') -Force

Push-Location $release
Get-FileHash -Algorithm SHA256 .\mirage-client.exe, .\mirage-server-linux-amd64, .\install-server.sh, .\mirage-windows-amd64.zip |
  ForEach-Object { "$($_.Hash.ToLower())  $([System.IO.Path]::GetFileName($_.Path))" } |
  Set-Content -Encoding ascii .\SHA256SUMS
Pop-Location

$textArtifacts = @((Join-Path $release 'install-server.sh'), (Join-Path $release 'SHA256SUMS'))
$bad = Select-String -Path $textArtifacts -Pattern 'mirage://','client_private_key','private_key:','PRIVATE KEY','ssh-key-' -SimpleMatch -ErrorAction SilentlyContinue
if ($bad) {
  $bad | ForEach-Object { Write-Error "Secret-like release text content: $($_.Path):$($_.LineNumber)" }
}

Write-Host "Release artifacts ready: $release"
