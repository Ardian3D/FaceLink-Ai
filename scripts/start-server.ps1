# Menjalankan backend dengan secret SMTP dari ciphertext DPAPI.
# Secret tidak pernah ditulis ke environment shell secara permanen dan tidak
# dicetak. Gunakan -Check untuk memvalidasi konfigurasi tanpa membuka port.
param(
  [string]$ConfigDir = (Join-Path $env:LOCALAPPDATA "FaceLink\secrets"),
  [string]$DbPath = "",
  [switch]$Check
)

$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Script ini memakai DPAPI Windows." }
Add-Type -AssemblyName System.Security

$configPath = Join-Path $ConfigDir "smtp.config.json"
$secretPath = Join-Path $ConfigDir "smtp-auth.secret"
if (-not (Test-Path -LiteralPath $configPath)) { throw "Konfigurasi SMTP belum ada. Jalankan scripts/setup-smtp.ps1." }
if (-not (Test-Path -LiteralPath $secretPath)) { throw "Secret SMTP belum ada. Jalankan scripts/setup-smtp.ps1." }

$cfg = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
foreach ($field in @("host", "port", "username", "from", "replyTo", "otpKeyPath")) {
  if (-not ($cfg.PSObject.Properties.Name -contains $field)) { throw "smtp.config.json tidak memuat $field." }
}
if (-not (Test-Path -LiteralPath $cfg.otpKeyPath)) { throw "Key OTP tidak ditemukan. Jalankan scripts/new-otp-key.ps1." }

$cipher = [IO.File]::ReadAllBytes($secretPath)
$plainBytes = [Security.Cryptography.ProtectedData]::Unprotect($cipher, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
try {
  $secret = [Text.Encoding]::UTF8.GetString($plainBytes)
} finally {
  [Array]::Clear($plainBytes, 0, $plainBytes.Length)
}
if ([string]::IsNullOrWhiteSpace($secret)) { throw "Secret SMTP kosong." }

$root = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $root "backend"
if (-not (Test-Path -LiteralPath $backend)) { throw "Direktori backend tidak ditemukan." }

$tlsCert = $env:FACELINK_TLS_CERT
$tlsKey = $env:FACELINK_TLS_KEY
if ([string]::IsNullOrWhiteSpace($tlsCert) -or [string]::IsNullOrWhiteSpace($tlsKey)) {
  $tlsCert = Join-Path $root "backend\dev\certs\server.pem"
  $tlsKey = Join-Path $root "backend\dev\certs\server-key.pem"
  if (-not (Test-Path -LiteralPath $tlsCert) -or -not (Test-Path -LiteralPath $tlsKey)) {
    throw "Sertifikat TLS dev tidak ditemukan. Jalankan: bash scripts/gen-dev-tls.sh"
  }
}

$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
  $fallback = Join-Path $env:ProgramFiles "Go\bin\go.exe"
  if (Test-Path -LiteralPath $fallback) { $go = Get-Item -LiteralPath $fallback }
}
if (-not $go) { throw "Perintah go tidak ditemukan di PATH." }
$goExe = if ($go -is [System.Management.Automation.ApplicationInfo]) { $go.Source } else { $go.FullName }

$previous = @{}
$names = @("SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_AUTH_SECRET", "SMTP_FROM", "SMTP_REPLY_TO", "SMTP_ALLOWED_FROM_DOMAINS", "FACELINK_OTP_HMAC_KEY_FILE", "FACELINK_TLS_CERT", "FACELINK_TLS_KEY", "FACELINK_DB_PATH")
foreach ($n in $names) { $previous[$n] = [Environment]::GetEnvironmentVariable($n) }

try {
  $env:SMTP_HOST = [string]$cfg.host
  $env:SMTP_PORT = [string]$cfg.port
  $env:SMTP_USERNAME = [string]$cfg.username
  $env:SMTP_AUTH_SECRET = $secret
  $env:SMTP_FROM = [string]$cfg.from
  $env:SMTP_REPLY_TO = [string]$cfg.replyTo
  $env:SMTP_ALLOWED_FROM_DOMAINS = (@($cfg.allowedFromDomains) -join ",")
  $env:FACELINK_OTP_HMAC_KEY_FILE = [string]$cfg.otpKeyPath
  $env:FACELINK_TLS_CERT = $tlsCert
  $env:FACELINK_TLS_KEY = $tlsKey
  if (-not [string]::IsNullOrWhiteSpace($DbPath)) { $env:FACELINK_DB_PATH = $DbPath }

  Write-Output ("SMTP: {0}:{1}; From: {2}; OTP key: DPAPI" -f $cfg.host, $cfg.port, $cfg.from)
  Push-Location $backend
  try {
    if ($Check) { & $goExe run ./cmd/server -check-config }
    else { & $goExe run ./cmd/server }
    exit $LASTEXITCODE
  } finally { Pop-Location }
} finally {
  foreach ($n in $names) {
    if ($null -eq $previous[$n]) { Remove-Item -Path "Env:$n" -ErrorAction SilentlyContinue }
    else { Set-Item -Path "Env:$n" -Value $previous[$n] }
  }
  $secret = $null
}
