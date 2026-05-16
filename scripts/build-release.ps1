$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$release = Join-Path $root 'release'
New-Item -ItemType Directory -Force $release | Out-Null

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

Copy-Item (Join-Path $root 'bin\client\mirage-client.exe') (Join-Path $release 'mirage-client.exe') -Force
Copy-Item (Join-Path $root 'bin\server\mirage-server-linux-amd64') (Join-Path $release 'mirage-server-linux-amd64') -Force
Copy-Item (Join-Path $root 'scripts\install-server.sh') (Join-Path $release 'install-server.sh') -Force

Push-Location $release
Get-FileHash -Algorithm SHA256 .\mirage-client.exe, .\mirage-server-linux-amd64, .\install-server.sh |
  ForEach-Object { "$($_.Hash.ToLower())  $([System.IO.Path]::GetFileName($_.Path))" } |
  Set-Content -Encoding ascii .\SHA256SUMS
Pop-Location

$textArtifacts = @((Join-Path $release 'install-server.sh'), (Join-Path $release 'SHA256SUMS'))
$bad = Select-String -Path $textArtifacts -Pattern 'mirage://','client_private_key','private_key:','PRIVATE KEY','ssh-key-' -SimpleMatch -ErrorAction SilentlyContinue
if ($bad) {
  $bad | ForEach-Object { Write-Error "Secret-like release text content: $($_.Path):$($_.LineNumber)" }
}

Write-Host "Release artifacts ready: $release"
