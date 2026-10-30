# Jalankan sebagai pengguna biasa, BUKAN Administrator. Tidak mencetak key.
#
# ACL: hanya DACL folder/berkas yang diubah (scripts/lib/secure-local.ps1).
# Tidak ada Set-Acl, SetOwner, atau penulisan SACL, sehingga tidak memerlukan
# SeSecurityPrivilege. Key tetap dilindungi DPAPI CurrentUser.
param([string]$Path = (Join-Path $env:LOCALAPPDATA "FaceLink\secrets\email-otp.key"))

$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "lib\secure-local.ps1")
Add-Type -AssemblyName System.Security

$full = [IO.Path]::GetFullPath($Path)
if (Test-Path -LiteralPath $full) { throw "Key sudah ada; tidak ditimpa. Rotasi key adalah operasi terpisah." }
$dir = Split-Path -Parent $full
Protect-SecretDirectory -Path $dir
if (Test-Path -LiteralPath $full) { throw "Key muncul bersamaan dari proses lain; tidak ditimpa." }

$key = New-Object byte[] 32
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
$tmp = Join-Path $dir ("email-otp.key.tmp." + [Guid]::NewGuid().ToString("N"))
try {
  $rng.GetBytes($key)
  $cipher = [Security.Cryptography.ProtectedData]::Protect($key, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
  $file = [IO.File]::Open($tmp, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
  try { $file.Write($cipher, 0, $cipher.Length) } finally { $file.Dispose() }
  [Array]::Clear($cipher, 0, $cipher.Length)
  Protect-SecretFile -Path $tmp
  # Move pada volume yang sama; destinasi tidak pernah ditimpa.
  [IO.File]::Move($tmp, $full)
} catch {
  if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
  throw
} finally {
  [Array]::Clear($key, 0, $key.Length)
  $rng.Dispose()
}
Write-Output "Key OTP DPAPI CurrentUser dibuat. Atur FACELINK_OTP_HMAC_KEY_FILE ke path tersebut. Jangan commit atau bagikan file ini."
