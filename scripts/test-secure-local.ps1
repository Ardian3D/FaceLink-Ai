# Regresi ACL secret pada Windows non-elevated.
# Tidak memakai App Password asli dan tidak mencetak isi key/secret.
param()

$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Test ini khusus Windows." }
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if ($principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  throw "Jalankan test ini dari PowerShell non-elevated agar SeSecurityPrivilege tidak menutupi masalah."
}
. (Join-Path $PSScriptRoot "lib\secure-local.ps1")
Add-Type -AssemblyName System.Security

$root = Join-Path $env:TEMP ("fl-secure-local-test." + [Guid]::NewGuid().ToString("N"))
try {
  $dir = Join-Path $root "secrets"
  [IO.Directory]::CreateDirectory($dir) | Out-Null
  $ownerBefore = (Get-Acl -LiteralPath $dir).Owner
  Protect-SecretDirectory -Path $dir
  $ownerAfter = (Get-Acl -LiteralPath $dir).Owner
  if ($ownerBefore -ne $ownerAfter) { throw "Owner folder berubah; seharusnya tidak disentuh." }

  $dummy = Join-Path $dir "dummy.secret"
  [IO.File]::WriteAllBytes($dummy, [Text.Encoding]::UTF8.GetBytes("bukan-secret-nyata"))
  Protect-SecretFile -Path $dummy

  $sid = Get-CurrentUserSid
  # Assert-DaclAman juga dipanggil oleh helper; ulangi di sini sebagai bukti.
  Assert-DaclAman -Security ([IO.DirectoryInfo]::new($dir).GetAccessControl([Security.AccessControl.AccessControlSections]::Access)) -Sid $sid
  Assert-DaclAman -Security ([IO.FileInfo]::new($dummy).GetAccessControl([Security.AccessControl.AccessControlSections]::Access)) -Sid $sid

  # Identitas lain pada DACL harus ditolak, bukan diabaikan.
  $extra = Join-Path $dir "extra.secret"
  [IO.File]::WriteAllBytes($extra, [Text.Encoding]::UTF8.GetBytes("bukan-secret-nyata"))
  Protect-SecretFile -Path $extra
  $users = [Security.Principal.SecurityIdentifier]::new("S-1-5-32-545")
  $info = [IO.FileInfo]::new($extra)
  $aclExtra = $info.GetAccessControl([Security.AccessControl.AccessControlSections]::Access)
  $aclExtra.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new($users, "Read", "None", "None", "Allow"))
  $info.SetAccessControl($aclExtra)
  $rejected = $false
  try { Assert-DaclAman -Security $info.GetAccessControl([Security.AccessControl.AccessControlSections]::Access) -Sid $sid } catch { $rejected = $true }
  if (-not $rejected) { throw "ACE untuk identitas lain tidak ditolak." }

  # Writer secret: DPAPI, ACL, rotasi atomik, tanpa menimpa tanpa izin.
  $secretPath = Join-Path $dir "dummy-auth.secret"
  Write-ProtectedSecret -Path $secretPath -PlainText "dummySMTPpass123"
  $cipher = [IO.File]::ReadAllBytes($secretPath)
  $plain = [Security.Cryptography.ProtectedData]::Unprotect($cipher, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
  try {
    if ([Text.Encoding]::UTF8.GetString($plain) -ne "dummySMTPpass123") { throw "DPAPI secret roundtrip gagal." }
  } finally { [Array]::Clear($plain, 0, $plain.Length) }

  $hashBefore = (Get-FileHash -LiteralPath $secretPath -Algorithm SHA256).Hash
  $refused = $false
  try { Write-ProtectedSecret -Path $secretPath -PlainText "dummySMTPpass456" } catch { $refused = $true }
  if (-not $refused) { throw "Writer menimpa secret tanpa -Force." }
  $hashAfter = (Get-FileHash -LiteralPath $secretPath -Algorithm SHA256).Hash
  if ($hashBefore -ne $hashAfter) { throw "Secret lama berubah setelah penolakan." }

  Write-ProtectedSecret -Path $secretPath -PlainText "dummySMTPpass456" -Force
  $cipher = [IO.File]::ReadAllBytes($secretPath)
  $plain = [Security.Cryptography.ProtectedData]::Unprotect($cipher, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
  try {
    if ([Text.Encoding]::UTF8.GetString($plain) -ne "dummySMTPpass456") { throw "Rotasi secret gagal." }
  } finally { [Array]::Clear($plain, 0, $plain.Length) }
  if (@(Get-ChildItem -LiteralPath $dir -Filter "secret.bak.*" -File -ErrorAction SilentlyContinue).Count -ne 0) {
    throw "Backup rotasi tertinggal setelah sukses."
  }

  $keyDir = Join-Path $root "otp"
  $keyPath = Join-Path $keyDir "email-otp.key"
  $keyScript = Join-Path $PSScriptRoot "new-otp-key.ps1"
  function Invoke-KeyScript([string]$path) {
    $out = Join-Path $root ("child-out." + [Guid]::NewGuid().ToString("N"))
    $err = Join-Path $root ("child-err." + [Guid]::NewGuid().ToString("N"))
    $proc = Start-Process -FilePath "powershell" -ArgumentList @(
      "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $keyScript, "-Path", $path
    ) -Wait -PassThru -RedirectStandardOutput $out -RedirectStandardError $err
    return $proc.ExitCode
  }
  if ((Invoke-KeyScript $keyPath) -ne 0 -or -not (Test-Path -LiteralPath $keyPath)) { throw "Pembuatan key OTP gagal pada akun non-elevated." }
  Assert-DaclAman -Security ([IO.FileInfo]::new($keyPath).GetAccessControl([Security.AccessControl.AccessControlSections]::Access)) -Sid $sid

  $cipher = [IO.File]::ReadAllBytes($keyPath)
  $plain = [Security.Cryptography.ProtectedData]::Unprotect($cipher, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
  try {
    if ($plain.Length -ne 32) { throw "DPAPI roundtrip tidak menghasilkan 32 byte." }
  } finally { [Array]::Clear($plain, 0, $plain.Length) }

  $hashBefore = (Get-FileHash -LiteralPath $keyPath -Algorithm SHA256).Hash
  if ((Invoke-KeyScript $keyPath) -eq 0) { throw "Pembuatan kedua seharusnya ditolak, bukan menimpa key." }
  $hashAfter = (Get-FileHash -LiteralPath $keyPath -Algorithm SHA256).Hash
  if ($hashBefore -ne $hashAfter) { throw "Key lama berubah setelah percobaan kedua." }

  Write-Output "PASSED: DACL-only (user FullControl + SYSTEM Read), DPAPI secret/key, rotasi aman, non-elevated."
} finally {
  if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue }
}
