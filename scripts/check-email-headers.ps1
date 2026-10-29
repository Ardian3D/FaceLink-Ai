# Membaca header email mentah (.eml) dan mencetak HANYA ringkasan autentikasi.
# Body tidak dibaca/dicetak. Multiple Authentication-Results dipisah per hop
# agar hasil Cloudflare dan hasil penerima akhir tidak tercampur.
param([Parameter(Mandatory = $true)][string]$Path)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $Path)) { throw "Berkas tidak ditemukan." }

$lines = Get-Content -LiteralPath $Path -Encoding UTF8
$headers = @{}
$current = $null
foreach ($line in $lines) {
  if ($line -eq "") { break }
  if ($line -match "^[ \t]" -and $current) {
    $headers[$current] = $headers[$current] + " " + $line.Trim()
    continue
  }
  $i = $line.IndexOf(":")
  if ($i -gt 0) {
    $current = $line.Substring(0, $i).Trim()
    $value = $line.Substring($i + 1).Trim()
    if ($headers.ContainsKey($current)) { $headers[$current] = $headers[$current] + "`n" + $value }
    else { $headers[$current] = $value }
  }
}

function Get-Header([string]$name) { if ($headers.ContainsKey($name)) { return [string]$headers[$name] } return "" }
function Get-Domain([string]$value) {
  if ($value -match "@([A-Za-z0-9.-]+)") { return $matches[1].TrimEnd(".").ToLowerInvariant() }
  if ($value -match "^([A-Za-z0-9.-]+)$") { return $matches[1].TrimEnd(".").ToLowerInvariant() }
  return ""
}
function Get-Tag([string]$value, [string]$tag) {
  $m = [regex]::Match($value, "(?i)(?:^|[;\s])$tag=([^;\s]+)")
  if ($m.Success) { return $m.Groups[1].Value }
  return ""
}
function Normalize-Domain([string]$value) { return $value.Trim().TrimStart("@").TrimEnd(".").ToLowerInvariant() }

# Satu blok Authentication-Results dapat memuat beberapa metode. Jangan
# menggabungkan blok dari hop berbeda.
function Parse-AuthBlock([string]$value) {
  $parts = @($value -split ";" | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" })
  $authserv = ""
  foreach ($p in $parts) {
    if ($p -match "^i=\d+$") { continue }
    $authserv = $p
    break
  }
  $spf = ""; $mailFrom = ""; $dkim = ""; $dkimDomains = @(); $dmarc = ""; $dmarcFrom = ""
  if ($value -match "(?i)\bspf=(\w+)") { $spf = $matches[1].ToLowerInvariant() }
  if ($value -match "(?i)smtp\.mailfrom=([^;\s]+)") { $mailFrom = Normalize-Domain $matches[1] }
  $dkimMatches = [regex]::Matches($value, "(?i)\bdkim=(\w+)")
  if ($dkimMatches.Count -gt 0) { $dkim = $dkimMatches[0].Groups[1].Value.ToLowerInvariant() }
  foreach ($m in [regex]::Matches($value, "(?i)header\.d=([^;\s]+)")) {
    $d = Normalize-Domain $m.Groups[1].Value
    if ($d -and $dkimDomains -notcontains $d) { $dkimDomains += $d }
  }
  if ($value -match "(?i)\bdmarc=(\w+)") { $dmarc = $matches[1].ToLowerInvariant() }
  if ($value -match "(?i)dmarc=[^;]*header\.from=([^;\s]+)") { $dmarcFrom = Normalize-Domain $matches[1] }
  [pscustomobject]@{
    Authserv = $authserv; SPF = $spf; MailFrom = $mailFrom; DKIM = $dkim
    DkimDomains = $dkimDomains; DMARC = $dmarc; DmarcFrom = $dmarcFrom
  }
}

function Format-Block([string]$label, $b, [string]$fromDomain) {
  $spfAligned = if ($b.MailFrom -and $fromDomain) { $b.MailFrom -eq $fromDomain } else { $false }
  $dkimAligned = $b.DkimDomains -contains $fromDomain
  $dmarcFromCocok = if ($b.DmarcFrom -and $fromDomain) { $b.DmarcFrom -eq $fromDomain } else { $false }
  "{0} {1} | spf={2} mailfrom={3} | dkim={4} d={5} | dmarc={6} header.from={7} | SPF aligned={8} DKIM aligned={9} DMARC from cocok={10}" -f `
    $label, $(if ($b.Authserv) { $b.Authserv } else { "(tanpa authserv)" }),
    $(if ($b.SPF) { $b.SPF } else { "tidak ada" }),
    $(if ($b.MailFrom) { $b.MailFrom } else { "tidak ada" }),
    $(if ($b.DKIM) { $b.DKIM } else { "tidak ada" }),
    $(if ($b.DkimDomains.Count -gt 0) { $b.DkimDomains -join "," } else { "tidak ada" }),
    $(if ($b.DMARC) { $b.DMARC } else { "tidak ada" }),
    $(if ($b.DmarcFrom) { $b.DmarcFrom } else { "tidak ada" }),
    $(if ($spfAligned) { "ya" } else { "tidak" }),
    $(if ($dkimAligned) { "ya" } else { "tidak" }),
    $(if ($dmarcFromCocok) { "ya" } else { "tidak" })
}

$from = Get-Header "From"
$returnPath = Get-Header "Return-Path"
$replyTo = Get-Header "Reply-To"
$sender = Get-Header "Sender"
$auth = Get-Header "Authentication-Results"
$arc = Get-Header "ARC-Authentication-Results"
$receivedSpf = Get-Header "Received-SPF"
$dkim = Get-Header "DKIM-Signature"

$fromDomain = Get-Domain $from
$returnDomain = Get-Domain $returnPath
$dkimDomains = @()
foreach ($m in [regex]::Matches($dkim, "(?i)(?:^|[;\s])d=([^;\s]+)")) {
  $d = Normalize-Domain $m.Groups[1].Value
  if ($dkimDomains -notcontains $d) { $dkimDomains += $d }
}

$authBlocks = @($auth -split "`n" | Where-Object { $_.Trim() -ne "" })
$arcBlocks = @($arc -split "`n" | Where-Object { $_.Trim() -ne "" })
$parsed = @()
for ($i = 0; $i -lt $authBlocks.Count; $i++) { $parsed += Parse-AuthBlock $authBlocks[$i] }
$parsedArc = @()
for ($i = 0; $i -lt $arcBlocks.Count; $i++) { $parsedArc += Parse-AuthBlock $arcBlocks[$i] }

$received = Get-Header "Received"
$adaCloudflare = $received -match "(?i)cloudflare"
$adaGoogle = $received -match "(?i)google"
$jalur = "tidak diketahui"
if ($adaCloudflare -and $adaGoogle) { $jalur = "Cloudflare-forwarded dari Google" }
elseif ($adaCloudflare) { $jalur = "Cloudflare forwarding/relay" }
elseif ($adaGoogle) { $jalur = "Google SMTP langsung" }

$spfFallback = ""
if ($receivedSpf -match "(?i)^(\w+)") { $spfFallback = $matches[1].ToLowerInvariant() }

Write-Output "=== Ringkasan autentikasi ==="
Write-Output ("Jalur terdeteksi : {0}" -f $jalur)
Write-Output ("From              : {0}" -f $from)
Write-Output ("From domain       : {0}" -f $fromDomain)
Write-Output ("Return-Path domain: {0}" -f $returnDomain)
Write-Output ("DKIM d= (signature): {0}" -f $(if ($dkimDomains.Count -gt 0) { $dkimDomains -join ", " } else { "tidak ada" }))
if ($parsed.Count -eq 0) {
  Write-Output ("SPF (Received-SPF): {0}" -f $(if ($spfFallback) { $spfFallback } else { "tidak ditemukan" }))
  Write-Output "Authentication-Results: tidak ditemukan"
} else {
  Write-Output ("Authentication-Results blocks: {0}" -f $parsed.Count)
  for ($i = 0; $i -lt $parsed.Count; $i++) {
    $line = Format-Block -label ("AR[{0}]" -f ($i + 1)) -b $parsed[$i] -fromDomain $fromDomain
    Write-Output $line
  }
  Write-Output "Catatan: AR[1] biasanya dari penerima akhir; AR berikutnya dari hop sebelumnya. Jangan menyimpulkan outbound DMARC dari AR forwarding."
}
for ($i = 0; $i -lt $parsedArc.Count; $i++) {
  $line = Format-Block -label ("ARC[{0}]" -f ($i + 1)) -b $parsedArc[$i] -fromDomain $fromDomain
  Write-Output $line
}

Write-Output ""
Write-Output "=== Header relevan (body tidak dicetak) ==="
Write-Output ("From: {0}" -f $from)
Write-Output ("Return-Path: {0}" -f $returnPath)
if ($replyTo) { Write-Output ("Reply-To: {0}" -f $replyTo) }
if ($sender) { Write-Output ("Sender: {0}" -f $sender) }
if ($receivedSpf) { Write-Output ("Received-SPF result: {0}" -f $(if ($spfFallback) { $spfFallback } else { "tidak ditemukan" })) }
if ($dkim) {
  $ringkas = @()
  foreach ($tag in @("v", "a", "s", "d", "i")) {
    $v = Get-Tag $dkim $tag
    if ($v) { $ringkas += "$tag=$v" }
  }
  Write-Output ("DKIM-Signature: {0}; ..." -f ($ringkas -join "; "))
}
