# Mengirim SATU email uji (bukan OTP) memakai kredensial SMTP lokal.
# Tujuannya memeriksa From/SPF/DKIM/DMARC pada header yang benar-benar diterima.
# Secret dibaca dari ciphertext DPAPI dan tidak dicetak.
param(
  [Parameter(Mandatory = $true)][string]$To,
  [string]$ConfigDir = (Join-Path $env:LOCALAPPDATA "FaceLink\secrets"),
  [string]$Subject = "",
  [switch]$DryRun
)

$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Script ini memakai DPAPI Windows." }
Add-Type -AssemblyName System.Security
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$configPath = Join-Path $ConfigDir "smtp.config.json"
$secretPath = Join-Path $ConfigDir "smtp-auth.secret"
if (-not (Test-Path -LiteralPath $configPath)) { throw "Konfigurasi SMTP belum ada. Jalankan scripts/setup-smtp.ps1." }
$cfg = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json

try { $tujuan = [Net.Mail.MailAddress]::new($To) } catch { throw "Alamat tujuan tidak sah." }
if (-not [string]::IsNullOrEmpty($tujuan.DisplayName) -or $tujuan.Address -ne $To) { throw "Alamat tujuan harus tanpa display name." }

$id = [Guid]::NewGuid().ToString("N")
if ([string]::IsNullOrWhiteSpace($Subject)) { $Subject = "[FaceLink SMTP test] $id" }

if ($DryRun) {
  Write-Output ("DRY RUN: {0}:{1}; From={2}; To={3}; Subject={4}" -f $cfg.host, $cfg.port, $cfg.from, $To, $Subject)
  exit 0
}
if (-not (Test-Path -LiteralPath $secretPath)) { throw "Secret SMTP belum ada. Jalankan scripts/setup-smtp.ps1." }

$cipher = [IO.File]::ReadAllBytes($secretPath)
$plainBytes = [Security.Cryptography.ProtectedData]::Unprotect($cipher, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
try { $secret = [Text.Encoding]::UTF8.GetString($plainBytes) } finally { [Array]::Clear($plainBytes, 0, $plainBytes.Length) }
if ([string]::IsNullOrWhiteSpace($secret)) { throw "Secret SMTP kosong." }

$message = New-Object Net.Mail.MailMessage
$message.From = New-Object Net.Mail.MailAddress([string]$cfg.from)
$message.To.Add($tujuan)
if (-not [string]::IsNullOrWhiteSpace([string]$cfg.replyTo)) {
  $message.ReplyToList.Add([string]$cfg.replyTo)
}
$message.Subject = $Subject
$message.IsBodyHtml = $false
$message.Headers.Add("X-FaceLink-Test", $id)
$message.Body = @"
Ini pesan uji FaceLink AI. Pesan ini TIDAK berisi kode OTP.

ID uji : $id
Waktu  : $([DateTime]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ"))
From   : $($cfg.from)
Tujuan : $To

Periksa header email ini (Show original) untuk From, Return-Path,
Authentication-Results, SPF, DKIM, dan DMARC. Jangan membalas pesan ini.
"@

$client = New-Object Net.Mail.SmtpClient([string]$cfg.host, [int]$cfg.port)
$client.EnableSsl = $true
$client.DeliveryMethod = [Net.Mail.SmtpDeliveryMethod]::Network
$client.Timeout = 20000
$client.Credentials = New-Object Net.NetworkCredential([string]$cfg.username, $secret)

try {
  $client.Send($message)
  Write-Output "SMTP menerima pesan (bukan bukti inbox). ID=$id"
  Write-Output "From yang diminta: $($cfg.from)"
  Write-Output "Langkah verifikasi: buka email di inbox, Show original, unduh .eml,"
  Write-Output "lalu jalankan scripts/check-email-headers.ps1 -Path <file.eml>."
} catch [Net.Mail.SmtpException] {
  Write-Output ("SMTP MENOLAK: status={0}" -f $_.Exception.StatusCode)
  Write-Output ("Pesan: {0}" -f $_.Exception.Message)
  Write-Output "Jangan mengubah From secara manual. Laporkan hasil ini; alternatif pengirim harus mendukung domain sendiri."
  exit 2
} finally {
  $message.Dispose()
  $client.Dispose()
  $secret = $null
}
