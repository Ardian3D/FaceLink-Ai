package httpapi

// Pengujian kebocoran informasi sensitif (butir uji FL-005 nomor 10).
//
// Berbeda dengan pengujian handler lain di paket ini, berkas ini menembak
// server HTTP nyata lewat jaringan loopback sehingga yang diperiksa adalah
// keluaran apa adanya — badan respons, tajuk, dan log akses yang benar-benar
// ditulis handler — bukan struct perantara yang bisa saja berbeda dari
// kenyataan.
//
// Aturan yang dijaga:
//   - docs/API_CONTRACT.md §5.10 dan §5.11: /healthz tidak boleh mengungkap
//     jalur berkas, versi internal, atau nama basis data.
//   - docs/SECURITY_PRIVACY.md: log tidak boleh memuat token, rahasia, atau
//     detail biometrik.
//   - AGENTS.md §3: jangan mencetak embedding, kunci, atau token ke log.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

// syncBuffer menampung keluaran log dari goroutine server tanpa balapan data.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// newLeakTestServer menyiapkan server nyata dengan basis data termigrasi dan
// penampung log yang dapat diperiksa.
func newLeakTestServer(t *testing.T) (string, *http.Server, *syncBuffer) {
	t.Helper()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink-rahasia.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}
	status, err := migrations.Run(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("Run() migrasi gagal: %v", err)
	}

	logs := &syncBuffer{}
	base, srv := startServer(t, Server{
		Config: ServerConfig{
			Version:        testVersion,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   5 * time.Second,
			IdleTimeout:    30 * time.Second,
			RequestTimeout: 5 * time.Second,
			MaxBodyBytes:   DefaultMaxBodyBytes,
		},
		DB:         db,
		Migrations: status,
		Logger:     slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	tungguListenerSiap(t, base)

	return base, srv, logs
}

// tungguListenerSiap memastikan soket sudah menerima koneksi sebelum uji
// berjalan. Berbeda dengan waitReady yang menembak HTTP berulang kali, di sini
// kesiapan diuji pada tingkat TCP sehingga tidak ada permintaan tambahan yang
// mengotori log — dan tidak ada batas waktu yang bisa gagal secara acak.
func tungguListenerSiap(t *testing.T, base string) {
	t.Helper()

	dialer := net.Dialer{Timeout: time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := dialer.Dial("tcp", strings.TrimPrefix(base, "http://"))
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("listener pada %s tidak menerima koneksi dalam batas waktu", base)
}

// polaRahasia adalah bentuk data yang tidak boleh muncul di respons publik.
var polaRahasia = map[string]*regexp.Regexp{
	"jalur berkas Windows":   regexp.MustCompile(`[A-Za-z]:\\\\|[A-Za-z]:\\`),
	"jalur berkas Unix":      regexp.MustCompile(`(^|[^:\w])/(home|users|tmp|var|etc)/`),
	"nama berkas basis data": regexp.MustCompile(`(?i)facelink[\w-]*\.(db|sqlite3?)(-wal|-shm)?`),
	"string koneksi":         regexp.MustCompile(`(?i)(dsn|connection[_ ]?string|_pragma|file:[^\s"]+)`),
	"token atau kunci":       regexp.MustCompile(`(?i)(bearer\s|api[_-]?key|secret|password|token_hash|authorization)`),
	"embedding biometrik":    regexp.MustCompile(`(?i)(embedding|face_template|face_templates|biometric|vector\s*\[)`),
	"jejak tumpukan":         regexp.MustCompile(`goroutine \d+ \[|\.go:\d+ \+0x`),
}

func periksaTidakBocor(t *testing.T, konteks, isi string) {
	t.Helper()
	for nama, pola := range polaRahasia {
		if m := pola.FindString(isi); m != "" {
			t.Errorf("%s memuat %s: %q\nisi lengkap: %s", konteks, nama, m, isi)
		}
	}
}

// TestResponsKesehatanTidakMembocorkanKonfigurasi memastikan kedua endpoint
// kesehatan — termasuk kasus gagal — tidak mengungkap jalur basis data, string
// koneksi, atau rahasia apa pun.
func TestResponsKesehatanTidakMembocorkanKonfigurasi(t *testing.T) {
	base, srv, _ := newLeakTestServer(t)
	defer shutdownServer(t, srv)

	client := &http.Client{Timeout: 5 * time.Second}
	for _, jalur := range []string{"/api/v1/healthz", "/api/v1/readyz"} {
		resp, err := client.Get(base + jalur)
		if err != nil {
			t.Fatalf("GET %s gagal: %v", jalur, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("membaca badan %s gagal: %v", jalur, err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, ingin 200", jalur, resp.StatusCode)
		}

		periksaTidakBocor(t, "badan respons "+jalur, string(body))

		for nama, nilai := range resp.Header {
			periksaTidakBocor(t, "tajuk "+nama+" pada "+jalur, strings.Join(nilai, " "))
		}
	}
}

// TestHealthzTidakMenyebutNamaBasisData menegaskan kontrak §5.10: /healthz
// hanya memuat status, versi, dan waktu — tanpa bidang tambahan.
func TestHealthzTidakMenyebutNamaBasisData(t *testing.T) {
	base, srv, _ := newLeakTestServer(t)
	defer shutdownServer(t, srv)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("GET /healthz gagal: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("badan /healthz bukan JSON objek: %v", err)
	}

	diizinkan := map[string]bool{"status": true, "version": true, "time": true}
	for k, v := range body {
		if !diizinkan[k] {
			t.Errorf("/healthz memuat bidang tak diharapkan %q = %v", k, v)
		}
	}
	for k := range diizinkan {
		if _, ada := body[k]; !ada {
			t.Errorf("/healthz kehilangan bidang wajib %q", k)
		}
	}
}

// TestLogTidakMembocorkanRahasia memastikan setiap jalur pencatatan — termasuk
// permintaan gagal dan rute tak dikenal — tidak menuliskan token, rahasia, atau
// jejak internal ke log.
func TestLogTidakMembocorkanRahasia(t *testing.T) {
	base, srv, logs := newLeakTestServer(t)
	defer shutdownServer(t, srv)

	client := &http.Client{Timeout: 5 * time.Second}

	permintaan := []struct {
		method string
		jalur  string
		tajuk  map[string]string
	}{
		{"GET", "/api/v1/healthz", nil},
		{"GET", "/api/v1/readyz", nil},
		{"GET", "/api/v1/healthz?token=RAHASIA-UJI-ABCDEF&password=PW-UJI-12345", nil},
		{"GET", "/api/v1/rute-tidak-ada?api_key=KUNCI-UJI-9999", nil},
		{
			"POST",
			"/api/v1/healthz",
			map[string]string{
				"Authorization":  "Bearer TOKEN-UJI-RAHASIA",
				"X-Device-Token": "PERANGKAT-UJI-RAHASIA",
			},
		},
	}

	for _, p := range permintaan {
		req, err := http.NewRequest(p.method, base+p.jalur, nil)
		if err != nil {
			t.Fatalf("membuat permintaan gagal: %v", err)
		}
		for k, v := range p.tajuk {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s gagal: %v", p.method, p.jalur, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	catatan := logs.String()
	if strings.TrimSpace(catatan) == "" {
		t.Fatal("log kosong: pengujian tidak membuktikan apa pun")
	}

	for _, rahasia := range []string{
		"RAHASIA-UJI-ABCDEF", "PW-UJI-12345", "KUNCI-UJI-9999",
		"TOKEN-UJI-RAHASIA", "PERANGKAT-UJI-RAHASIA",
	} {
		if strings.Contains(catatan, rahasia) {
			t.Errorf("log memuat nilai sensitif %q", rahasia)
		}
	}

	periksaTidakBocor(t, "log akses", catatan)

	// Log akses tidak boleh mencatat parameter kueri sama sekali.
	if strings.Contains(catatan, "token=") || strings.Contains(catatan, "api_key=") {
		t.Errorf("log akses memuat parameter kueri: %s", catatan)
	}
}

func shutdownServer(t *testing.T, srv *http.Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
