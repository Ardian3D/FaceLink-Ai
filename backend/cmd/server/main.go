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
	"syscall"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/config"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/httpapi"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/logging"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
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
			logger.Error("gagal menutup basis data", "error", err)
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

	if err := serve(ctx, cfg, logger, db, status); err != nil {
		return err
	}
	return nil
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, db *database.DB, status migrations.Status) error {
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
		logger.Error("penutupan server melebihi batas waktu", "error", err)
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
