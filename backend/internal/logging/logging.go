// Package logging menyediakan logger terstruktur JSON untuk backend.
//
// Batas yang ditegakkan di sini (AGENTS.md §3, docs/SECURITY_PRIVACY.md §2):
// JPEG mentah, embedding, kunci, token, dan detail biometrik tidak boleh
// mencapai log. Paket ini tidak menyediakan jalur apa pun untuk mencetak nilai
// tersebut; pemanggil harus memilih atribut yang aman.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New membuat logger JSON pada writer yang diberikan dengan level yang sudah
// divalidasi konfigurasi.
//
// Penyensoran dipasang di handler, bukan diserahkan kepada pemanggil: setiap
// atribut string — termasuk `msg` yang dibangun handler sendiri — dan setiap
// atribut error dilewatkan Redact sebelum ditulis. Dengan begitu satu catatan
// baru yang lupa memanggil Redact tidak dapat membocorkan nilai mentah
// (FL-011, D-036, docs/SECURITY_PRIVACY.md §2).
func New(w io.Writer, level string) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       parseLevel(level),
		ReplaceAttr: sensor,
	})
	return slog.New(handler)
}

// sensor adalah ReplaceAttr yang menyensor teks bebas pada atribut log.
//
// Yang diganti: nilai string apa pun (termasuk `msg`) dan nilai error apa pun
// yang handler simpan sebagai KindAny. Nilai error diubah menjadi teks
// err.Error() yang sudah disensor — sama seperti yang akan ditulis handler JSON
// untuk error biasa, sehingga keluaran tidak berubah bentuk.
//
// Yang dibiarkan apa adanya: `time`, `level`, dan `source` bawaan handler
// (jenisnya Time/Any non-error, bukan string, sehingga tidak pernah masuk
// cabang penyensoran), angka, bool, durasi, waktu, nilai non-error apa pun, dan
// nilai KindGroup itu sendiri.
//
// Biaya dijaga tetap terbatas: tepat satu panggilan Redact per atribut string
// dan per atribut error, tanpa penelusuran berulang ke dalam struktur nilai.
// Atribut di dalam grup tetap tersensor karena handler yang menelusuri grup
// (log/slog.handleState.appendAttr memanggil appendAttrs untuk tiap anaknya)
// dan memanggil sensor untuk tiap anak yang bukan KindGroup; fungsi ini sendiri
// tidak pernah menyusuri grup, jadi tidak ada biaya rekursif di sini.
//
// Sisa celah yang disadari:
//   - Nilai KindAny yang bukan error tidak diubah menjadi teks (memang tidak
//     boleh), sehingga struktur, peta, dan nilai yang mengimplementasikan
//     json.Marshaler tetap ditulis apa adanya. Pemanggil tidak boleh menaruh
//     rahasia di sana.
//   - Nama kunci dan nama grup tidak disensor; kunci adalah pengenal tetap dari
//     kode, bukan nilai bebas, sehingga rahasia tidak boleh dijadikan nama
//     kunci.
func sensor(_ []string, a slog.Attr) slog.Attr {
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Redact(a.Value.String()))
	case slog.KindAny:
		if err, ok := a.Value.Any().(error); ok {
			return slog.String(a.Key, Redact(err.Error()))
		}
		return a
	default:
		// Angka, bool, durasi, waktu, dan grup tidak memuat teks bebas.
		return a
	}
}

// NewDiscard mengembalikan logger yang membuang seluruh keluaran; dipakai oleh
// pengujian agar hasil uji tidak tercemar dan tidak ada nilai yang bocor ke
// berkas log.
func NewDiscard() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
