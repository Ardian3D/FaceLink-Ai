# Regression test parser header multi-hop dengan fixture sintetis.
# Tidak memakai data email nyata dan tidak mencetak body.
param()

$ErrorActionPreference = "Stop"
$root = Join-Path $env:TEMP ("fl-header-test." + [Guid]::NewGuid().ToString("N"))
[IO.Directory]::CreateDirectory($root) | Out-Null
$eml = Join-Path $root "multi-hop.eml"
try {
  @"
From: FaceLink Support <support@facelink.online>
Reply-To: support@facelink.online
Return-Path: <srs-forwarder@relay.example.test>
Received: from route1.mx.cloudflare.net (route1.mx.cloudflare.net [104.30.8.131]) by mx.google.com; Fri, 23 Oct 2026 12:00:00 +0000
Received: from mail-sor-f41.google.com (mail-sor-f41.google.com [209.85.220.41]) by route1.mx.cloudflare.net; Fri, 23 Oct 2026 11:59:59 +0000
Authentication-Results: mx.google.com;
 spf=pass (google.com: domain of srs-forwarder@relay.example.test designates 104.30.8.131 as permitted sender) smtp.mailfrom=srs-forwarder@relay.example.test;
 dkim=pass header.d=facelink.online header.s=cf2024-1;
 dmarc=pass (p=NONE sp=NONE) header.from=facelink.online
Authentication-Results: cloudflare-email-routing; spf=pass smtp.mailfrom=smtp-sender@example.test; dkim=pass header.d=gmail.com header.s=20230601; dmarc=fail header.from=facelink.online
ARC-Authentication-Results: i=1; mx.google.com; spf=pass smtp.mailfrom=srs-forwarder@relay.example.test; dkim=pass header.d=facelink.online; dmarc=pass header.from=facelink.online
DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=facelink.online; s=cf2024-1; h=from:to:subject; bh=abc; b=def
Received-SPF: pass (google.com: domain of srs-forwarder@relay.example.test designates 104.30.8.131 as permitted sender) client-ip=104.30.8.131;
Subject: synthetic multi-hop

Body ini tidak boleh tercetak. Kode palsu: 12345678
"@ | Set-Content -LiteralPath $eml -Encoding UTF8

  $out = (& powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "check-email-headers.ps1") -Path $eml) -join "`n"
  $wajib = @(
    "Jalur terdeteksi : Cloudflare-forwarded dari Google",
    "Return-Path domain: relay.example.test",
    "AR[1] mx.google.com",
    "AR[2] cloudflare-email-routing",
    "dmarc=pass",
    "dmarc=fail",
    "ARC[1] mx.google.com",
    "DKIM d= (signature): facelink.online",
    "DMARC from cocok=ya"
  )
  foreach ($teks in $wajib) {
    if ($out -notmatch [regex]::Escape($teks)) { throw "Output tidak memuat: $teks`n$out" }
  }
  foreach ($terlarang in @("12345678", "Body ini tidak boleh", "client-ip=")) {
    if ($out -match [regex]::Escape($terlarang)) { throw "Output membocorkan: $terlarang" }
  }

  $direct = Join-Path $root "direct.eml"
  @"
From: FaceLink Support <support@facelink.online>
Reply-To: support@facelink.online
Return-Path: <smtp-sender@example.test>
Received: from mail-sor-f41.google.com (mail-sor-f41.google.com [209.85.220.41]) by mx.google.com; Fri, 23 Oct 2026 12:10:00 +0000
Authentication-Results: mx.google.com; spf=pass (google.com: domain of smtp-sender@example.test designates 209.85.220.41 as permitted sender) smtp.mailfrom=smtp-sender@example.test; dkim=pass header.d=gmail.com header.s=20230601; dmarc=fail (p=NONE sp=NONE) header.from=facelink.online
DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=gmail.com; s=20230601; h=from:to:subject; bh=abc; b=def
Received-SPF: pass (google.com: domain of smtp-sender@example.test designates 209.85.220.41 as permitted sender) client-ip=209.85.220.41;
Subject: synthetic direct

Body ini tidak boleh tercetak. Kode palsu: 87654321
"@ | Set-Content -LiteralPath $direct -Encoding UTF8

  $outDirect = (& powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "check-email-headers.ps1") -Path $direct) -join "`n"
  $wajibDirect = @(
    "Jalur terdeteksi : Google SMTP langsung",
    "Return-Path domain: example.test",
    "AR[1] mx.google.com",
    "dmarc=fail",
    "DKIM d= (signature): gmail.com",
    "SPF aligned=tidak",
    "DKIM aligned=tidak"
  )
  foreach ($teks in $wajibDirect) {
    if ($outDirect -notmatch [regex]::Escape($teks)) { throw "Output direct tidak memuat: $teks`n$outDirect" }
  }
  foreach ($terlarang in @("87654321", "Body ini tidak boleh", "client-ip=")) {
    if ($outDirect -match [regex]::Escape($terlarang)) { throw "Output direct membocorkan: $terlarang" }
  }

  Write-Output "PASSED: multi-hop AR/ARC, folded header, direct control, no body/client-ip."
} finally {
  if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue }
}
