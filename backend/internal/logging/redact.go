package logging

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"unicode/utf8"
)

// Batas penyensoran dan pemotongan. Nilainya dipilih agar rahasia yang lazim
// dipakai backend tertangkap, sementara penautan log tetap utuh.
const (
	// panjangDeretRahasia adalah panjang minimum deret karakter tanpa pemisah
	// yang diperlakukan sebagai token. Token akses/penyegar FaceLink adalah 32
	// byte acak dalam base64url tanpa padding: 43 karakter. UUID (36 karakter)
	// dan ID permintaan Chi (17 karakter) berada di bawah batas ini sehingga
	// tetap terbaca dan korelasi antar baris log tidak rusak.
	panjangDeretRahasia = 40
	// panjangPesanPanik adalah batas teks panic yang dicatat.
	panjangPesanPanik = 300
	// panjangTumpukan adalah batas jejak tumpukan yang dicatat.
	panjangTumpukan = 2048
)

// pola menyensor bentuk rahasia yang lazim muncul pada nilai panic dan pesan
// galat. Urutan penerapannya penting: JWT lebih dahulu supaya ketiga bagiannya
// diganti sekaligus, dan nilai tajuk diganti sebelum pasangan kunci.
var (
	polaJWT      = regexp.MustCompile(`[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)
	polaTajuk    = regexp.MustCompile(`(?i)\b(bearer|basic)\s+\S{6,}`)
	polaPasangan = regexp.MustCompile(`(?i)\b(access[_ ]?token|refresh[_ ]?token|id[_ ]?token|token|secret|client[_ ]?secret|password|passwd|api[_ -]?key|authorization|cookie|nonce)\b("?\s*[:=]\s*"?)[^\s",;}<>]{1,}"?`)
	polaSurel    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	polaDeret    = regexp.MustCompile(`[A-Za-z0-9_=-]{40,}`)
)

// Redact menyensor bentuk rahasia yang mungkin terbawa pada teks bebas — nilai
// panic, pesan galat, atau atribut log lain — sebelum teks itu mencapai log.
//
// Yang diganti: token JWT, nilai tajuk otorisasi/cookie, nilai pada pasangan
// kunci sensitif (`token=…`, `nonce: …`), alamat surel, dan deret karakter acak
// sepanjang token. Kalimat diagnostik, kode galat, nama kunci, waktu, dan angka
// dibiarkan apa adanya supaya log tetap berguna.
//
// Fungsi ini adalah jaring pengaman untuk nilai yang tidak dikendalikan backend
// (D-036, docs/SECURITY_PRIVACY.md §2); rahasia tetap tidak boleh sengaja
// dicatat meski hasilnya akan tersensor.
func Redact(teks string) string {
	teks = polaJWT.ReplaceAllString(teks, "<jwt>")
	teks = polaTajuk.ReplaceAllString(teks, "<tajuk-otorisasi>")
	teks = polaPasangan.ReplaceAllString(teks, "$1$2<nilai>")
	teks = polaSurel.ReplaceAllString(teks, "<surel>")
	return polaDeret.ReplaceAllString(teks, "<token>")
}

// PanicSummary mengubah nilai yang ditangkap recover() menjadi teks yang aman
// dicatat. Jenis nilai selalu disimpan sebagai diagnostik; isinya disensor dan
// dipotong. Nilai yang bukan string, error, maupun Stringer hanya dilaporkan
// jenisnya sehingga struktur besar atau data biner tidak pernah masuk log.
func PanicSummary(rec any) string {
	jenis := fmt.Sprintf("%T", rec)
	switch nilai := rec.(type) {
	case nil:
		return jenis + ": <kosong>"
	case string:
		return jenis + ": " + potong(Redact(nilai), panjangPesanPanik)
	case error:
		return jenis + ": " + potong(Redact(nilai.Error()), panjangPesanPanik)
	case fmt.Stringer:
		return jenis + ": " + potong(Redact(nilai.String()), panjangPesanPanik)
	default:
		return jenis + ": <isi tidak dicatat>"
	}
}

// StackTrace mengembalikan jejak tumpukan goroutine saat ini dalam bentuk yang
// dipotong. Jejak tumpukan Go memuat nama fungsi, berkas, dan nomor baris —
// bukan isi nilai — sehingga aman dicatat dan tetap berguna untuk diagnosis.
func StackTrace() string {
	return potong(string(debug.Stack()), panjangTumpukan)
}

// potong memangkas teks pada batas maks dan menandainya, tanpa memutus rune
// sehingga JSON yang dihasilkan tetap sah.
func potong(teks string, maks int) string {
	if len(teks) <= maks {
		return teks
	}
	terpotong := teks[:maks]
	for len(terpotong) > 0 && !utf8.ValidString(terpotong) {
		terpotong = terpotong[:len(terpotong)-1]
	}
	return strings.TrimRight(terpotong, " ") + "…(dipotong)"
}
