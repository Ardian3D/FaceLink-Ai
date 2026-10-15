package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

func startServer(t *testing.T, s Server) (string, *http.Server) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gagal membuka listener: %v", err)
	}

	srv := &http.Server{
		Handler:           NewRouter(s),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()

	return "http://" + ln.Addr().String(), srv
}

func waitReady(t *testing.T, base string) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(base + "/api/v1/healthz")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server tidak siap pada %s dalam batas waktu", base)
}

func TestShutdownMenghentikanLayananDenganRapi(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	defer db.Close()

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}
	status, err := migrations.Run(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("Run() migrasi gagal: %v", err)
	}

	base, srv := startServer(t, Server{
		Config:     ServerConfig{Version: testVersion, RequestTimeout: 5 * time.Second},
		DB:         db,
		Migrations: status,
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})
	waitReady(t, base)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() gagal: %v", err)
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	if _, err := client.Get(base + "/api/v1/healthz"); err == nil {
		t.Error("server masih menerima permintaan setelah Shutdown()")
	}
}

func TestShutdownTidakMenghentikanPermintaanYangSedangBerjalan(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	defer db.Close()

	started := make(chan struct{})
	finish := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/lambat", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"selesai":true}`))
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("gagal membuka listener: %v", err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()

	base := "http://" + ln.Addr().String()
	shutdownDone := make(chan error, 1)

	go func() {
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + "/lambat")
		if err == nil {
			resp.Body.Close()
		}
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("permintaan lambat tidak pernah mulai")
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone <- srv.Shutdown(ctx)
	}()

	// Shutdown harus menunggu permintaan yang sedang berjalan selesai, bukan
	// memutusnya di tengah jalan.
	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown() selesai sebelum permintaan berjalan dihentikan: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(finish)

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown() melaporkan galat: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown() tidak selesai setelah permintaan berjalan selesai")
	}
}

func TestServerNyataMelayaniHealthDanReady(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	defer db.Close()

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}
	status, err := migrations.Run(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("Run() migrasi gagal: %v", err)
	}

	base, srv := startServer(t, Server{
		Config:     ServerConfig{Version: testVersion, RequestTimeout: 5 * time.Second},
		DB:         db,
		Migrations: status,
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	client := &http.Client{Timeout: 3 * time.Second}

	resp, err := client.Get(base + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("permintaan /healthz gagal: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/healthz menjawab %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("respons nyata tidak memuat X-Request-Id")
	}

	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("badan /healthz bukan JSON: %v", err)
	}
	if health["status"] != "ok" || health["version"] != testVersion {
		t.Errorf("badan /healthz tidak sesuai kontrak: %v", health)
	}

	readyResp, err := client.Get(base + "/api/v1/readyz")
	if err != nil {
		t.Fatalf("permintaan /readyz gagal: %v", err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("/readyz menjawab %d", readyResp.StatusCode)
	}

	var ready map[string]any
	if err := json.NewDecoder(readyResp.Body).Decode(&ready); err != nil {
		t.Fatalf("badan /readyz bukan JSON: %v", err)
	}
	if ready["database"] != "ok" || ready["migrations"] != "applied" {
		t.Errorf("badan /readyz tidak sesuai kontrak: %v", ready)
	}
	wal, ok := ready["wal"].(map[string]any)
	if !ok || !strings.EqualFold(wal["journal_mode"].(string), "wal") {
		t.Errorf("wal /readyz tidak melaporkan mode WAL: %v", ready["wal"])
	}
}

func TestBasisDataMenggunakanWALPadaServerNyata(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	defer db.Close()

	mode, err := db.JournalMode(context.Background())
	if err != nil {
		t.Fatalf("JournalMode() gagal: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, ingin wal", mode)
	}
}
