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
func New(w io.Writer, level string) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(handler)
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
