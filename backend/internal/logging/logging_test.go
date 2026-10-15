package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// markers adalah nilai yang menyerupai data sensitif. Nama field sudah jelas
// terlarang; nilainya juga dipakai untuk memastikan tidak ada jalur lain yang
// membocorkannya (AGENTS.md §3, docs/SECURITY_PRIVACY.md §6).
var markers = []string{
	"jpeg-bytes-mentah",
	"embedding-0.123-0.456",
	"api-key-rahasiia",
	"internal-token-abc",
	"kata-sandi-operator",
	"face-template-terenkripsi",
}

func TestLoggerTidakPernahMencetakRahasiaYangTidakDiminta(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "debug")

	// Catatan yang sah: hanya metadata operasional.
	log.Debug("konfigurasi dimuat", "addr", "127.0.0.1:8080", "log_level", "debug")
	log.Info("server mendengarkan", "addr", "127.0.0.1:8080")
	log.Warn("mesin AI belum siap", "ai_engine", "unavailable")
	log.Error("gagal membuka basis data", "error", "file is not a database")

	rendered := buf.String()
	if rendered == "" {
		t.Fatal("logger menghasilkan keluaran kosong")
	}
	for _, m := range markers {
		if strings.Contains(rendered, m) {
			t.Errorf("log memuat penanda sensitif %q:\n%s", m, rendered)
		}
	}
}

func TestLoggerMenghasilkanJSONYangDapatDibacaMesin(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "info")

	log.Info("migrasi selesai", "current_version", 1, "applied", 1, "available", 1)

	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("baris log bukan JSON yang sah (%q): %v", line, err)
		}
		if record["msg"] != "migrasi selesai" {
			t.Errorf("msg = %v, ingin %q", record["msg"], "migrasi selesai")
		}
		if record["level"] != "INFO" {
			t.Errorf("level = %v, ingin INFO", record["level"])
		}
		if record["current_version"] != float64(1) {
			t.Errorf("current_version = %v, ingin 1", record["current_version"])
		}
	}
}

func TestLoggerMenghormatiLevel(t *testing.T) {
	cases := []struct {
		level   string
		wantLog bool
	}{
		{"debug", true},
		{"info", false},
		{"warn", false},
		{"error", false},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			var buf bytes.Buffer
			New(&buf, tc.level).Debug("catatan debug")

			if got := buf.Len() > 0; got != tc.wantLog {
				t.Errorf("level %q mencatat debug = %v, ingin %v", tc.level, got, tc.wantLog)
			}
		})
	}
}

func TestLevelTidakDikenalJatuhKeInfo(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "verbose")

	log.Debug("tidak boleh tampil")
	if buf.Len() != 0 {
		t.Errorf("level tidak dikenal tidak boleh menurunkan ambang ke debug:\n%s", buf.String())
	}

	log.Info("harus tampil")
	if !strings.Contains(buf.String(), "harus tampil") {
		t.Errorf("level tidak dikenal seharusnya berperilaku seperti info:\n%s", buf.String())
	}
}

func TestNewDiscardTidakMenulisApaPun(t *testing.T) {
	log := NewDiscard()
	if log == nil {
		t.Fatal("NewDiscard() mengembalikan nil")
	}
	log.Error("tidak boleh muncul di mana pun")
	if log.Enabled(nil, slog.LevelError) == false {
		t.Error("logger buang seharusnya tetap mengaktifkan level error")
	}
}
