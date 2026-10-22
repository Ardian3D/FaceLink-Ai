#!/usr/bin/env bash
# Membuat CA development dan sertifikat server FaceLink AI untuk pengujian
# emulator Android. Berkas kunci TIDAK boleh masuk Git (lihat .gitignore:
# *.pem, *.key). CA ini hanya untuk build debug; build release tidak
# mempercayainya sama sekali.
#
# Pemakaian:
#   bash scripts/gen-dev-tls.sh
#
# Keluaran (di backend/dev/certs/):
#   dev-ca.pem        sertifikat CA (publik; dipasang di aplikasi debug saja)
#   dev-ca-key.pem    kunci privat CA (rahasia lokal)
#   server.pem        sertifikat server (SAN: 127.0.0.1, 10.0.2.2, localhost)
#   server-key.pem    kunci privat server (rahasia lokal)
#
# Sertifikat berlaku 397 hari karena Android menolak masa berlaku yang lebih
# panjang pada trust anchor yang dipasang pengguna/aplikasi.

set -euo pipefail

out_dir="$(cd "$(dirname "$0")/.." && pwd)/backend/dev/certs"

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl tidak ditemukan di PATH" >&2
  exit 1
fi

mkdir -p "$out_dir"
cd "$out_dir"

if [ -f dev-ca.pem ] && [ -f server.pem ]; then
  echo "Sertifikat sudah ada di $out_dir; hapus dulu bila ingin membuat ulang."
  openssl x509 -in dev-ca.pem -noout -subject -enddate
  exit 0
fi

openssl genrsa -out dev-ca-key.pem 4096 2>/dev/null
openssl req -x509 -new -key dev-ca-key.pem -sha256 -days 397 \
  -subj "/C=ID/O=FaceLink AI Development/CN=FaceLink AI Dev CA" \
  -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" \
  -out dev-ca.pem

openssl genrsa -out server-key.pem 2048 2>/dev/null
openssl req -new -key server-key.pem \
  -subj "/C=ID/O=FaceLink AI Development/CN=127.0.0.1" \
  -out server.csr

cat > server.ext <<'EOF'
basicConstraints = critical,CA:FALSE
keyUsage = critical,digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt
[alt]
IP.1 = 127.0.0.1
IP.2 = 10.0.2.2
DNS.1 = localhost
EOF

openssl x509 -req -in server.csr -CA dev-ca.pem -CAkey dev-ca-key.pem \
  -CAcreateserial -days 397 -sha256 -extfile server.ext -out server.pem 2>/dev/null

rm -f server.csr server.ext dev-ca.srl

echo "Sertifikat dibuat di $out_dir"
openssl x509 -in server.pem -noout -subject -issuer -enddate
openssl x509 -in server.pem -noout -ext subjectAltName
