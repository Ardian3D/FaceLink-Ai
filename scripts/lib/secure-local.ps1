# Helper ACL minimum untuk secret lokal di Windows, tanpa Administrator.
#
# Aturan:
#  - Hanya bagian DACL yang dibaca/ditulis. Owner, group, dan SACL tidak disentuh.
#  - Tidak memakai Set-Acl/SetOwner; Set-Acl dapat menulis seluruh security
#    descriptor termasuk SACL dan meminta SeSecurityPrivilege.
#  - Tidak menambahkan Everyone. ACE lain yang sudah eksplisit tidak dihapus
#    secara buta; hasil akhir diverifikasi dan proses gagal bila tidak aman.
Set-StrictMode -Version 2.0

function Get-CurrentUserSid {
  return [Security.Principal.WindowsIdentity]::GetCurrent().User
}

function Assert-OutsideRepository {
  param([Parameter(Mandatory = $true)][string]$Path)
  $ancestor = [IO.Path]::GetFullPath($Path)
  while ($ancestor) {
    if (Test-Path -LiteralPath (Join-Path $ancestor ".git")) {
      throw "Secret wajib disimpan di luar repository Git."
    }
    $parent = Split-Path -Parent $ancestor
    if ($parent -eq $ancestor) { break }
    $ancestor = $parent
  }
}

function Assert-DaclAman {
  param(
    [Parameter(Mandatory = $true)]$Security,
    [Parameter(Mandatory = $true)][Security.Principal.SecurityIdentifier]$Sid
  )
  if (-not $Security.AreAccessRulesProtected) {
    throw "DACL secret masih mewarisi izin induk; proses dihentikan."
  }
  $rules = $Security.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier])
  $systemSid = [Security.Principal.SecurityIdentifier]::new("S-1-5-18")
  $milikSendiri = @($rules | Where-Object {
      -not $_.IsInherited -and
      $_.IdentityReference.Value -eq $Sid.Value -and
      ($_.FileSystemRights -band [Security.AccessControl.FileSystemRights]::FullControl) -eq [Security.AccessControl.FileSystemRights]::FullControl
    })
  if ($milikSendiri.Count -eq 0) {
    throw "DACL secret tidak memberikan FullControl eksplisit kepada pengguna saat ini; proses dihentikan."
  }
  $systemRead = @($rules | Where-Object {
      -not $_.IsInherited -and
      $_.AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and
      $_.IdentityReference.Value -eq $systemSid.Value -and
      ($_.FileSystemRights -band [Security.AccessControl.FileSystemRights]::Read) -eq [Security.AccessControl.FileSystemRights]::Read
    })
  if ($systemRead.Count -eq 0) {
    throw "DACL secret tidak memberikan Read eksplisit kepada SYSTEM; proses dihentikan."
  }
  $everyone = [Security.Principal.SecurityIdentifier]::new("S-1-1-0")
  if (@($rules | Where-Object { $_.IdentityReference.Value -eq $everyone.Value }).Count -gt 0) {
    throw "DACL secret memuat Everyone; proses dihentikan."
  }
  # Hanya SID pengguna aktif dan SYSTEM yang boleh memiliki ACE Allow. ACE
  # eksplisit untuk identitas lain (grup/Administrators) membuat proses gagal.
  $izinLain = @($rules | Where-Object {
      $_.AccessControlType -eq [Security.AccessControl.AccessControlType]::Allow -and
      $_.IdentityReference.Value -ne $Sid.Value -and
      $_.IdentityReference.Value -ne $systemSid.Value
    })
  if ($izinLain.Count -gt 0) {
    $sidLain = @($izinLain | ForEach-Object { $_.IdentityReference.Value } | Select-Object -Unique) -join ", "
    throw "DACL secret memberi izin Allow kepada identitas selain pengguna aktif/SYSTEM ($sidLain); proses dihentikan."
  }
}

function Protect-SecretDirectory {
  param([Parameter(Mandatory = $true)][string]$Path)

  if ($env:OS -ne "Windows_NT") { throw "Perlindungan secret ini memakai Windows ACL." }
  $full = [IO.Path]::GetFullPath($Path)
  Assert-OutsideRepository $full
  if (-not (Test-Path -LiteralPath $full)) {
    [IO.Directory]::CreateDirectory($full) | Out-Null
  }

  $sid = Get-CurrentUserSid
  $dir = [IO.DirectoryInfo]::new($full)
  try {
    # Hanya DACL: tidak ada owner/SACL di objek ini.
    $acl = $dir.GetAccessControl([Security.AccessControl.AccessControlSections]::Access)
    $acl.SetAccessRuleProtection($true, $false)
    $rule = [Security.AccessControl.FileSystemAccessRule]::new(
      $sid, "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow")
    $acl.AddAccessRule($rule)
    # Hak sistem minimum: SYSTEM boleh membaca untuk backup/scanning, bukan
    # mengubah. Ini DACL, bukan SACL; auditing tidak disentuh.
    $systemSid = [Security.Principal.SecurityIdentifier]::new("S-1-5-18")
    $systemRule = [Security.AccessControl.FileSystemAccessRule]::new(
      $systemSid, "Read", "ContainerInherit,ObjectInherit", "None", "Allow")
    $acl.AddAccessRule($systemRule)
    $dir.SetAccessControl($acl)
  } catch {
    throw "Gagal membatasi DACL folder secret. Tidak ada perubahan owner/SACL. Jalankan sebagai pemilik folder, bukan Administrator. Detail: $($_.Exception.Message)"
  }
  Assert-DaclAman -Security $dir.GetAccessControl([Security.AccessControl.AccessControlSections]::Access) -Sid $sid
}

function Protect-SecretFile {
  param([Parameter(Mandatory = $true)][string]$Path)

  if ($env:OS -ne "Windows_NT") { throw "Perlindungan secret ini memakai Windows ACL." }
  $full = [IO.Path]::GetFullPath($Path)
  Assert-OutsideRepository $full
  if (-not (Test-Path -LiteralPath $full)) { throw "Berkas secret tidak ditemukan." }

  $sid = Get-CurrentUserSid
  $file = [IO.FileInfo]::new($full)
  try {
    $acl = $file.GetAccessControl([Security.AccessControl.AccessControlSections]::Access)
    $acl.SetAccessRuleProtection($true, $false)
    $rule = [Security.AccessControl.FileSystemAccessRule]::new(
      $sid, "FullControl", "None", "None", "Allow")
    $acl.AddAccessRule($rule)
    $systemSid = [Security.Principal.SecurityIdentifier]::new("S-1-5-18")
    $systemRule = [Security.AccessControl.FileSystemAccessRule]::new(
      $systemSid, "Read", "None", "None", "Allow")
    $acl.AddAccessRule($systemRule)
    $file.SetAccessControl($acl)
  } catch {
    throw "Gagal membatasi DACL berkas secret. Tidak ada perubahan owner/SACL. Detail: $($_.Exception.Message)"
  }
  Assert-DaclAman -Security $file.GetAccessControl([Security.AccessControl.AccessControlSections]::Access) -Sid $sid
}

# Menulis ciphertext DPAPI CurrentUser secara atomik. Plaintext hanya ada di
# memori pemanggil; fungsi ini tidak pernah mencetaknya.
function Write-ProtectedSecret {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$PlainText,
    [switch]$Force
  )

  if ($env:OS -ne "Windows_NT") { throw "Perlindungan secret ini memakai Windows DPAPI." }
  Add-Type -AssemblyName System.Security
  $full = [IO.Path]::GetFullPath($Path)
  Assert-OutsideRepository $full
  if ([string]::IsNullOrEmpty($PlainText)) { throw "Secret kosong." }
  $dir = Split-Path -Parent $full
  Protect-SecretDirectory -Path $dir
  if ((Test-Path -LiteralPath $full) -and -not $Force) {
    throw "Secret sudah ada; gunakan -Force hanya untuk rotasi."
  }

  $bytes = [Text.Encoding]::UTF8.GetBytes($PlainText)
  $tmp = Join-Path $dir ("secret.tmp." + [Guid]::NewGuid().ToString("N"))
  $backup = $null
  try {
    $cipher = [Security.Cryptography.ProtectedData]::Protect($bytes, $null, [Security.Cryptography.DataProtectionScope]::CurrentUser)
    $stream = [IO.File]::Open($tmp, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
    try { $stream.Write($cipher, 0, $cipher.Length) } finally { $stream.Dispose() }
    [Array]::Clear($cipher, 0, $cipher.Length)
    Protect-SecretFile -Path $tmp

    if (Test-Path -LiteralPath $full) {
      $backup = Join-Path $dir ("secret.bak." + [Guid]::NewGuid().ToString("N"))
      [IO.File]::Replace($tmp, $full, $backup)
      try {
        Protect-SecretFile -Path $full
        Remove-Item -LiteralPath $backup -Force
        $backup = $null
      } catch {
        $restoreError = $null
        try {
          [IO.File]::Replace($backup, $full, $null)
          $backup = $null
        } catch { $restoreError = $_ }
        if ($restoreError) {
          throw "Rotasi secret gagal dan pemulihan backup juga gagal. Backup lama dipertahankan; jangan dihapus sebelum diperiksa."
        }
        throw
      }
    } else {
      [IO.File]::Move($tmp, $full)
    }
  } catch {
    if (Test-Path -LiteralPath $tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
    throw
  } finally {
    [Array]::Clear($bytes, 0, $bytes.Length)
  }
}
