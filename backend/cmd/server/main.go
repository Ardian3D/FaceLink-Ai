// Command server menjalankan backend FaceLink AI.
//
// Backend Go adalah satu-satunya penulis basis data dan satu-satunya pengambil
// keputusan domain absensi (D-027). Pada tahap FL-005 perintah ini hanya
// menyajikan endpoint kesehatan; endpoint domain ditambahkan pada tugas
// berikutnya.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/config"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/httpapi"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/logging"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/oidc"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "facelink backend gagal dijalankan:", err)
		os.Exit(1)
	}
}

func run() error {
	checkConfig := flag.Bool("check-config", false, "memuat dan memvalidasi konfigurasi lalu keluar")
	migrateOnly := flag.Bool("migrate", false, "menerapkan migrasi lalu keluar")
	bootstrapAdminFlag := flag.Bool("bootstrap-admin", false, "membuat akun admin pertama (sekali, lokal, tanpa kata sandi)")
	emailFlag := flag.String("email", "", "alamat Google untuk akun admin pertama (wajib dengan -bootstrap-admin)")
	namaFlag := flag.String("display-name", "", "nama tampilan akun admin pertama (opsional)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.LogLevel)
	logger.Info("konfigurasi dimuat", "config", cfg.Redacted())

	if *checkConfig {
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("gagal menutup basis data", "error", logging.Redact(err.Error()))
		}
	}()

	status, err := migrations.Run(ctx, db, cfg.MigrationsDir)
	if err != nil {
		return err
	}
	logger.Info("migrasi selesai",
		"current_version", status.CurrentVersion,
		"applied", status.Applied,
		"available", status.Available,
	)
	if *migrateOnly {
		return nil
	}

	// Bootstrap admin pertama: perintah lokal yang eksplisit, tanpa kata sandi
	// dan tanpa kredensial bawaan (D-032). Perintah ini menolak berjalan bila
	// sudah ada admin aktif atau menunggu.
	if *bootstrapAdminFlag {
		return bootstrapAdmin(ctx, db, logger, *emailFlag, *namaFlag)
	}

	if err := serve(ctx, cfg, logger, db, status); err != nil {
		return err
	}
	return nil
}

// bootstrapAdmin membuat akun admin pertama dari konsol lokal. Identitas Google
// yang dipakai adalah alamat yang diberikan operator; identitas itu belum
// tertaut sampai orangnya benar-benar masuk lewat Google Sign-In.
func bootstrapAdmin(ctx context.Context, db *database.DB, logger *slog.Logger, email, nama string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("bootstrap admin memerlukan parameter -email berisi alamat Google admin pertama")
	}
	if strings.TrimSpace(nama) == "" {
		nama = "Administrator"
	}

	clk := clock.Real{}
	akun := auth.NewStore(db, clk)
	user, err := akun.CreateFirstAdmin(ctx, email, nama)
	if err != nil {
		return fmt.Errorf("bootstrap admin gagal: %w", err)
	}

	penulisAudit := audit.NewWriter(db, clk)
	if err := penulisAudit.Record(ctx, audit.Event{
		ActorType:  audit.ActorSystem,
		Action:     "user.bootstrapped",
		EntityType: "user",
		EntityID:   user.ID,
		Result:     audit.ResultSuccess,
		Reason:     "Admin pertama dibuat melalui perintah lokal",
		Metadata:   map[string]string{"role_after": string(user.Role), "status_after": string(user.Status)},
	}); err != nil {
		return fmt.Errorf("bootstrap admin: gagal menulis jejak audit: %w", err)
	}

	logger.Info("akun admin pertama dibuat", "user_id", user.ID, "status", string(user.Status))
	fmt.Fprintf(os.Stdout,
		"Akun admin pertama dibuat dengan status %q.\n"+
			"Langkah berikutnya:\n"+
			"  1. Setel FACELINK_GOOGLE_CLIENT_IDS agar endpoint autentikasi melayani.\n"+
			"  2. Masuk aplikasi memakai Google dengan alamat yang baru didaftarkan.\n"+
			"Akun menjadi aktif setelah login pertama menautkan identitas Google.\n", user.Status)
	return nil
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, db *database.DB, status migrations.Status) error {
	clk := clock.Real{}
	akun := auth.NewStore(db, clk)
	sesi := session.NewStore(db, clk, session.Config{})
	penulisAudit := audit.NewWriter(db, clk)
	lingkup := authz.NewSQLDirectory(db)

	// Verifier OIDC hanya disiapkan bila audience dikonfigurasi. Tanpa client
	// ID, endpoint autentikasi menolak melayani dengan pesan jelas, sementara
	// operasi lokal lain tetap berjalan (SECURITY_PRIVACY §5e).
	var google *oidc.Verifier
	if len(cfg.GoogleClientIDs) > 0 {
		kunci := oidc.NewJWKSCache(cfg.GoogleJWKSURL, 15*time.Minute, &http.Client{Timeout: 5 * time.Second}, clk.Now)
		verifier, err := oidc.NewVerifier(oidc.Config{
			Audiences: cfg.GoogleClientIDs,
			Keys:      kunci,
			Now:       clk.Now,
		})
		if err != nil {
			return fmt.Errorf("verifikasi Google OIDC tidak dapat disiapkan: %w", err)
		}
		google = verifier
	} else {
		logger.Warn("login Google belum dikonfigurasi; setel FACELINK_GOOGLE_CLIENT_IDS agar /api/v1/auth/google melayani")
	}

	api := httpapi.Server{
		Config: httpapi.ServerConfig{
			Version:      cfg.BuildVersion,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
			AIEngineURL:  cfg.AIEngineURL,
			// Batas waktu proses per permintaan dipisahkan dari WriteTimeout
			// layanan agar satu permintaan lambat tidak bergantung pada konfigurasi
			// socket.
			RequestTimeout: httpapi.DefaultRequestTimeout,
		},
		DB:         db,
		Migrations: status,
		Logger:     logger,
		Accounts:   akun,
		Sessions:   sesi,
		Google:     google,
		Audit:      penulisAudit,
		Scope:      lingkup,
		Clock:      clk,
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(api),
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("gagal mendengarkan pada %s: %w", cfg.Addr, err)
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server mendengarkan", "addr", listener.Addr().String())
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err

	case <-ctx.Done():
		logger.Info("sinyal berhenti diterima; menutup server dengan rapi",
			"timeout_s", cfg.ShutdownTimeout.Seconds())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("penutupan server melebihi batas waktu", "error", logging.Redact(err.Error()))
		_ = srv.Close()
	}

	select {
	case err := <-serveErr:
		if err != nil {
			return err
		}
	case <-time.After(2 * time.Second):
		logger.Warn("goroutine server belum berhenti; lanjut menutup basis data")
	}

	logger.Info("server berhenti dengan rapi")
	return nil
}
