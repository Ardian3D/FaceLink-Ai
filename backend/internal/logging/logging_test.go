package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
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

// Nilai uji yang dipakai berulang oleh uji jaring pengaman ReplaceAttr.
const (
	// tokenAksesUji adalah 43 karakter base64url tanpa padding — bentuk token
	// akses/penyegar FaceLink yang sesungguhnya.
	tokenAksesUji = "ZmFjZWxpbmstcmVmcmVzaC10b2tlbi11amktMzJieXQ"
	// jwtUji menyerupai ID token Google: tiga bagian base64url.
	jwtUji = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImsxIn0.eyJzdWIiOiJzdWItMTIzNDU2Nzg5MCJ9.dGFuZGEtdGFuZ2Fu"
	// surelUji adalah alamat surel pengguna yang harus disensor.
	surelUji = "budi.santoso@kampus.example"
	// idSesiUji adalah UUID v4 36 karakter; bukan rahasia dan harus tetap utuh
	// supaya korelasi log tidak rusak.
	idSesiUji = "550e8400-e29b-41d4-a716-446655440000"
	// idPermintaanUji adalah ID permintaan gaya Chi (17 karakter).
	idPermintaanUji = "AbCdEfGhIj-000123"
	// ruteUji adalah rute hasil normalisasi Chi.
	ruteUji = "/api/v1/classes/{classID}/students"
)

// barisLog membuang catatan ke buf dan mengembalikan baris JSON terakhir
// sebagai peta, sehingga uji dapat memeriksa nilai yang benar-benar tertulis.
func barisLog(t *testing.T, tulis func(log *slog.Logger)) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	tulis(New(&buf, "debug"))

	baris := strings.TrimSpace(buf.String())
	if baris == "" {
		t.Fatal("logger tidak menulis apa pun")
	}
	terakhir := baris[strings.LastIndex(baris, "\n")+1:]

	var rekaman map[string]any
	if err := json.Unmarshal([]byte(terakhir), &rekaman); err != nil {
		t.Fatalf("keluaran bukan JSON yang sah: %v\n%s", err, terakhir)
	}
	return rekaman
}

// teksLog memastikan nilai pada kunci tertentu berupa string dan
// mengembalikannya. Kegagalan di sini berarti handler mengubah jenis nilai,
// bukan sekadar salah menyensor.
func teksLog(t *testing.T, rekaman map[string]any, kunci string) string {
	t.Helper()

	nilai, ada := rekaman[kunci]
	if !ada {
		t.Fatalf("kunci %q tidak ada pada catatan: %v", kunci, rekaman)
	}
	teks, ok := nilai.(string)
	if !ok {
		t.Fatalf("nilai %q berjenis %T, ingin string", kunci, nilai)
	}
	return teks
}

// TestReplaceAttrMenyensorAtributString memastikan setiap bentuk rahasia pada
// atribut string disensor oleh handler, bukan oleh pemanggil. Catatan di bawah
// sengaja tidak memanggil Redact sama sekali — inilah yang dulu bocor.
func TestReplaceAttrMenyensorAtributString(t *testing.T) {
	kasus := []struct {
		nama       string
		pesan      string
		kunci      string
		nilai      string
		terlarang  []string
		harusAda   []string
		keterangan string
	}{
		{
			nama:      "jwt pada atribut",
			pesan:     "verifikasi id token gagal",
			kunci:     "id_token",
			nilai:     jwtUji,
			terlarang: []string{jwtUji, "eyJhbGciOiJSUzI1NiJ9"},
			harusAda:  []string{"<jwt>"},
		},
		{
			nama:      "pasangan kunci pada atribut",
			pesan:     "panggilan keluar gagal",
			kunci:     "detail",
			nilai:     "memutar sesi gagal: refresh_token=" + tokenAksesUji,
			terlarang: []string{tokenAksesUji},
			harusAda:  []string{"refresh_token", "<nilai>", "memutar sesi"},
		},
		{
			nama:      "tajuk otorisasi pada atribut",
			pesan:     "proxy menolak permintaan",
			kunci:     "tajuk",
			nilai:     "Bearer " + tokenAksesUji,
			terlarang: []string{tokenAksesUji, "Bearer"},
			harusAda:  []string{"<tajuk-otorisasi>"},
		},
		{
			nama:      "surel pada atribut",
			pesan:     "penautan akun ditolak",
			kunci:     "subjek",
			nilai:     "pemilik " + surelUji + " tidak cocok",
			terlarang: []string{surelUji, "budi.santoso"},
			harusAda:  []string{"<surel>", "tidak cocok"},
		},
		{
			nama:      "token opaque 43 karakter pada atribut",
			pesan:     "penyegaran sesi gagal",
			kunci:     "token_mentah",
			nilai:     tokenAksesUji,
			terlarang: []string{tokenAksesUji},
			harusAda:  []string{"<token>"},
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			rekaman := barisLog(t, func(log *slog.Logger) {
				log.Info(k.pesan, k.kunci, k.nilai)
			})

			pesan := teksLog(t, rekaman, "msg")
			if !strings.Contains(pesan, k.pesan) {
				t.Errorf("pesan baik %q hilang dari msg %q", k.pesan, pesan)
			}

			nilai := teksLog(t, rekaman, k.kunci)
			for _, terlarang := range k.terlarang {
				if strings.Contains(nilai, terlarang) {
					t.Errorf("atribut %q masih memuat %q: %q", k.kunci, terlarang, nilai)
				}
				if strings.Contains(pesan, terlarang) {
					t.Errorf("msg masih memuat %q: %q", terlarang, pesan)
				}
			}
			for _, harus := range k.harusAda {
				if !strings.Contains(nilai, harus) {
					t.Errorf("atribut %q = %q, ingin memuat %q", k.kunci, nilai, harus)
				}
			}
		})
	}
}

// TestReplaceAttrMenyensorAtributError memastikan nilai error yang handler
// simpan sebagai KindAny ikut disensor. Bentuknya tetap string pada JSON,
// seperti perilaku handler tanpa ReplaceAttr.
func TestReplaceAttrMenyensorAtributError(t *testing.T) {
	galat := errors.New("panggilan token endpoint gagal: refresh_token=" + tokenAksesUji)

	rekaman := barisLog(t, func(log *slog.Logger) {
		log.Error("pertukaran token gagal", "error", galat)
	})

	nilai := teksLog(t, rekaman, "error")
	if strings.Contains(nilai, tokenAksesUji) {
		t.Errorf("atribut error masih memuat token: %q", nilai)
	}
	if !strings.Contains(nilai, "<nilai>") {
		t.Errorf("atribut error = %q, ingin memuat <nilai>", nilai)
	}
	if !strings.Contains(nilai, "panggilan token endpoint gagal") {
		t.Errorf("bagian diagnostik error hilang: %q", nilai)
	}
	if !strings.Contains(nilai, "refresh_token") {
		t.Errorf("nama kunci diagnostik hilang: %q", nilai)
	}
}

// TestReplaceAttrMembiarkanNilaiAman memastikan nilai yang bukan rahasia tetap
// utuh dan berjenis sama setelah melewati ReplaceAttr, sehingga penautan log
// dan pembacaan mesin tidak rusak.
func TestReplaceAttrMembiarkanNilaiAman(t *testing.T) {
	rekaman := barisLog(t, func(log *slog.Logger) {
		log.Info("permintaan selesai",
			"session_id", idSesiUji,
			"request_id", idPermintaanUji,
			"route", ruteUji,
			"status", 200,
			"aktif", true,
			"latensi", 1250*time.Millisecond,
		)
	})

	if got := teksLog(t, rekaman, "session_id"); got != idSesiUji {
		t.Errorf("session_id = %q, ingin %q utuh", got, idSesiUji)
	}
	if got := teksLog(t, rekaman, "request_id"); got != idPermintaanUji {
		t.Errorf("request_id = %q, ingin %q utuh", got, idPermintaanUji)
	}
	if got := teksLog(t, rekaman, "route"); got != ruteUji {
		t.Errorf("route = %q, ingin %q utuh", got, ruteUji)
	}
	if got, ok := rekaman["status"].(float64); !ok || got != 200 {
		t.Errorf("status = %v (%T), ingin angka 200", rekaman["status"], rekaman["status"])
	}
	if got, ok := rekaman["aktif"].(bool); !ok || !got {
		t.Errorf("aktif = %v (%T), ingin bool true", rekaman["aktif"], rekaman["aktif"])
	}
	// Durasi ditulis handler JSON sebagai jumlah nanodetik, sama seperti
	// encoding/json. Yang penting: nilainya angka, bukan teks tersensor.
	if got, ok := rekaman["latensi"].(float64); !ok || got != float64(1250*time.Millisecond) {
		t.Errorf("latensi = %v (%T), ingin angka %d", rekaman["latensi"], rekaman["latensi"], 1250*time.Millisecond)
	}
}

// TestReplaceAttrMenyensorPesan memastikan `msg` juga melewati Redact; handler
// membangun atribut msg sendiri, jadi jalur inilah yang paling mudah terlewat.
func TestReplaceAttrMenyensorPesan(t *testing.T) {
	rekaman := barisLog(t, func(log *slog.Logger) {
		log.Warn("gagal memverifikasi id token " + jwtUji + " untuk " + surelUji)
	})

	pesan := teksLog(t, rekaman, "msg")
	for _, terlarang := range []string{jwtUji, surelUji, "budi.santoso"} {
		if strings.Contains(pesan, terlarang) {
			t.Errorf("msg masih memuat %q: %q", terlarang, pesan)
		}
	}
	for _, harus := range []string{"<jwt>", "<surel>", "gagal memverifikasi"} {
		if !strings.Contains(pesan, harus) {
			t.Errorf("msg = %q, ingin memuat %q", pesan, harus)
		}
	}
}

// TestReplaceAttrMenyensorAtributDalamGrup memastikan nilai di dalam grup tetap
// tersensor. slog memanggil ReplaceAttr untuk tiap anak grup, bukan untuk
// grupnya, sehingga sensor tidak perlu menelusuri grup sendiri.
func TestReplaceAttrMenyensorAtributDalamGrup(t *testing.T) {
	rekaman := barisLog(t, func(log *slog.Logger) {
		log.Info("panggilan keluar selesai",
			slog.Group("http",
				"route", ruteUji,
				"authorization", "Bearer "+tokenAksesUji,
			),
		)
	})

	grup, ok := rekaman["http"].(map[string]any)
	if !ok {
		t.Fatalf("grup http tidak ada atau bukan objek: %v", rekaman["http"])
	}
	tajuk, _ := grup["authorization"].(string)
	if strings.Contains(tajuk, tokenAksesUji) || strings.Contains(tajuk, "Bearer") {
		t.Errorf("nilai grup masih memuat tajuk otorisasi: %q", tajuk)
	}
	if !strings.Contains(tajuk, "<tajuk-otorisasi>") {
		t.Errorf("nilai grup = %q, ingin memuat <tajuk-otorisasi>", tajuk)
	}
	if grup["route"] != ruteUji {
		t.Errorf("rute di dalam grup = %v, ingin %q utuh", grup["route"], ruteUji)
	}
}

// TestReplaceAttrTidakMenyentuhNilaiNonError memastikan nilai KindAny yang
// bukan error tidak diubah menjadi teks. Mengubahnya akan mengubah bentuk
// keluaran JSON dan menyensor data yang sah.
func TestReplaceAttrTidakMenyentuhNilaiNonError(t *testing.T) {
	rekaman := barisLog(t, func(log *slog.Logger) {
		log.Info("metrik mesin AI", "skor", []float64{0.25, 0.5})
	})

	skor, ok := rekaman["skor"].([]any)
	if !ok {
		t.Fatalf("skor = %v (%T), ingin tetap larik", rekaman["skor"], rekaman["skor"])
	}
	if len(skor) != 2 || skor[0] != 0.25 || skor[1] != 0.5 {
		t.Errorf("skor = %v, ingin [0.25 0.5] utuh", skor)
	}
}

// TestReplaceAttrMenjagaWaktuDanLevel memastikan kolom bawaan handler tidak
// ikut berubah bentuk ketika ReplaceAttr dipasang.
func TestReplaceAttrMenjagaWaktuDanLevel(t *testing.T) {
	rekaman := barisLog(t, func(log *slog.Logger) {
		log.Info("catatan biasa")
	})

	if rekaman["level"] != "INFO" {
		t.Errorf("level = %v, ingin INFO", rekaman["level"])
	}
	waktu, ok := rekaman["time"].(string)
	if !ok || waktu == "" {
		t.Fatalf("time = %v (%T), ingin teks RFC3339", rekaman["time"], rekaman["time"])
	}
	if _, err := time.Parse(time.RFC3339Nano, waktu); err != nil {
		t.Errorf("time = %q bukan RFC3339: %v", waktu, err)
	}
}

// BenchmarkSensor menakar biaya tambahan penyensoran per baris log. Baseline
// adalah handler JSON tanpa ReplaceAttr — persis keadaan sebelum FL-011 — dan
// kedua varian menulis ke io.Discard supaya biaya I/O tidak menutupi selisihnya.
func BenchmarkSensor(b *testing.B) {
	attr := []any{
		"addr", "127.0.0.1:8080",
		"route", ruteUji,
		"request_id", idPermintaanUji,
		"status", 200,
		"latensi", 1250 * time.Millisecond,
	}

	b.Run("tanpa ReplaceAttr", func(b *testing.B) {
		log := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
		b.ReportAllocs()
		for b.Loop() {
			log.Info("permintaan selesai", attr...)
		}
	})

	b.Run("dengan ReplaceAttr", func(b *testing.B) {
		log := New(io.Discard, "info")
		b.ReportAllocs()
		for b.Loop() {
			log.Info("permintaan selesai", attr...)
		}
	})

	b.Run("tanpa ReplaceAttr atribut panjang", func(b *testing.B) {
		log := slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
		b.ReportAllocs()
		for b.Loop() {
			log.Info("panggilan keluar gagal: refresh_token="+tokenAksesUji, attr...)
		}
	})

	b.Run("dengan ReplaceAttr atribut panjang", func(b *testing.B) {
		log := New(io.Discard, "info")
		b.ReportAllocs()
		for b.Loop() {
			log.Info("panggilan keluar gagal: refresh_token="+tokenAksesUji, attr...)
		}
	})
}
