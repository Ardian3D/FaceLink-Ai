# Menyiapkan kredensial SMTP Gmail + OTP HMAC key secara lokal (Windows).
#
# Secret TIDAK ditulis ke Git dan TIDAK ditampilkan. App Password disimpan
# sebagai ciphertext DPAPI CurrentUser di %LOCALAPPDATA%\FaceLink\secrets.
#
# ACL hanya mengubah DACL (scripts/lib/secure-local.ps1); tidak ada Set-Acl,
# SetOwner, atau SACL, sehingga tidak memerlukan SeSecurityPrivilege.
#
# Prasyarat: 2-Step Verification aktif dan App Password dibuat sendiri di
# https://myaccount.google.com/apppasswords
#
# Contoh:
#   powershell -ExecutionPolicy Bypass -File scripts/setup-smtp.ps1 `
#     -Username "smtp-sender@example.test" -From "support@facelink.online"
param(
  [Parameter(Mandatory = $true)][string]$Username,
  [string]$From = "support@facelink.online",
  [string]$ReplyTo = "support@facelink.online",
  [string]$SmtpHost = "smtp.gmail.com",
  [ValidateSet(465, 587)][int]$Port = 587,
  [string]$ConfigDir = (Join-Path $env:LOCALAPPDATA "FaceLink\secrets"),
  [string]$OtpKeyPath = (Join-Path $env:LOCALAPPDATA "FaceLink\secrets\email-otp.key"),
  [switch]$Force
)

$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Script ini memakai DPAPI Windows." }
. (Join-Path $PSScriptRoot "lib\secure-local.ps1")
Add-Type -AssemblyName System.Security

function Assert-Alamat([string]$nilai, [string]$nama) {
  if ([string]::IsNullOrWhiteSpace($nilai) -or $nilai -match "[\r\n]") { throw "$nama tidak sah." }
  try { $a = [Net.Mail.MailAddress]::new($nilai) } catch { throw "$nama bukan alamat email yang sah." }
  if (-not [string]::IsNullOrEmpty($a.DisplayName) -or $a.Address -ne $nilai) { throw "$nama harus berupa alamat email tanpa display name." }
}

Assert-Alamat $Username "SMTP_USERNAME"
Assert-Alamat $From "SMTP_FROM"
Assert-Alamat $ReplyTo "SMTP_REPLY_TO"
if ($SmtpHost -notmatch "^[A-Za-z0-9.-]+$") { throw "SMTP_HOST tidak sah." }

$fromDomain = ($From -split "@")[-1].ToLowerInvariant()
$allowed = @()
if (-not [string]::Equals($From, $Username, [StringComparison]::OrdinalIgnoreCase)) { $allowed += $fromDomain }

# Gagal di sini berarti belum ada secret/config yang ditulis.
Protect-SecretDirectory -Path $ConfigDir

$secretPath = Join-Path $ConfigDir "smtp-auth.secret"
$configPath = Join-Path $ConfigDir "smtp.config.json"
if ((Test-Path -LiteralPath $secretPath) -and -not $Force) {
  throw "Secret SMTP sudah ada. Gunakan -Force hanya untuk rotasi App Password."
}

if (-not (Test-Path -LiteralPath $OtpKeyPath)) {
  & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "new-otp-key.ps1") -Path $OtpKeyPath
  if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $OtpKeyPath)) { throw "Key OTP gagal dibuat." }
}

# Tulis konfigurasi non-rahasia lebih dahulu, lalu secret. Bila penulisan
# secret gagal, tidak ada klaim sukses dan tidak ada secret lama yang hilang.
$config = [ordered]@{
  host               = $SmtpHost
  port               = $Port
  username           = $Username
  from               = $From
  replyTo            = $ReplyTo
  allowedFromDomains = @($allowed)
  otpKeyPath         = $OtpKeyPath
}
$configTmp = Join-Path $ConfigDir ("smtp.config.json.tmp." + [Guid]::NewGuid().ToString("N"))
try {
  $config | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $configTmp -Encoding UTF8
  Move-Item -LiteralPath $configTmp -Destination $configPath -Force
  # Konfigurasi memuat alamat/PII; samakan perlindungannya dengan secret.
  Protect-SecretFile -Path $configPath
} catch {
  if (Test-Path -LiteralPath $configTmp) { Remove-Item -LiteralPath $configTmp -Force -ErrorAction SilentlyContinue }
  throw
}

$secure = Read-Host -Prompt "Gmail App Password (16 karakter, input tersembunyi)" -AsSecureString
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try {
  $plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
  $plain = ($plain -replace "\s", "")
  if ($plain.Length -ne 16 -or $plain -notmatch "^[A-Za-z0-9]+$") {
    throw "App Password harus 16 karakter alfanumerik. Pastikan 2-Step Verification aktif."
  }
  # Fungsi ini menulis ciphertext DPAPI secara atomik dan hanya menyentuh DACL.
  Write-ProtectedSecret -Path $secretPath -PlainText $plain -Force:$Force
} finally {
  $plain = $null
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
}

Write-Output "Konfigurasi SMTP + key OTP disimpan di $ConfigDir (di luar Git)."
Write-Output "Secret disimpan sebagai ciphertext DPAPI CurrentUser; tidak dicetak."
Write-Output ""
Write-Output "Langkah berikutnya:"
Write-Output "  1. powershell -ExecutionPolicy Bypass -File scripts/start-server.ps1 -Check"
Write-Output "  2. powershell -ExecutionPolicy Bypass -File scripts/send-smtp-test.ps1 -To admin@facelink.online"
Write-Output "  3. Unduh email uji sebagai .eml, lalu jalankan scripts/check-email-headers.ps1 -Path <file.eml>"
