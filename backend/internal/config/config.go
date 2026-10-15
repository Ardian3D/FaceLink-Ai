// Package config memuat konfigurasi backend FaceLink AI dari variabel
// lingkungan.
//
// Aturan yang mengikat paket ini (docs/SECURITY_PRIVACY.md §6, AGENTS.md §3):
//   - Tidak ada alamat, kredensial, atau jalur yang ditanam permanen di kode.
//   - Nilai sensitif tidak pernah dicetak; Config.Redacted() hanya mengembalikan
//     bentuk aman untuk log.
//   - Konfigurasi tidak valid menggagalkan startup, bukan diam-diam memakai
//     nilai bawaan yang berbahaya.
package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Nama variabel lingkungan yang dikenali. Dikumpulkan di satu tempat agar
// dokumentasi operasional dan pengujian tidak pernah menyimpang dari kode.
const (
	EnvAddr            = "FACELINK_ADDR"
	EnvDatabasePath    = "FACELINK_DB_PATH"
	EnvMigrationsDir   = "FACELINK_MIGRATIONS_DIR"
	EnvAIEngineURL     = "FACELINK_AI_ENGINE_URL"
	EnvReadTimeout     = "FACELINK_READ_TIMEOUT"
	EnvWriteTimeout    = "FACELINK_WRITE_TIMEOUT"
	EnvIdleTimeout     = "FACELINK_IDLE_TIMEOUT"
	EnvShutdownTimeout = "FACELINK_SHUTDOWN_TIMEOUT"
	EnvLogLevel        = "FACELINK_LOG_LEVEL"
	EnvBuildVersion    = "FACELINK_BUILD_VERSION"
)

// Nilai bawaan. Alamat bawaan sengaja loopback: mesin AI lokal dan backend tidak
// boleh terbuka bebas pada jaringan kelas (docs/ARCHITECTURE.md §2,
// docs/SECURITY_PRIVACY.md §6).
const (
	DefaultAddr            = "127.0.0.1:8080"
	DefaultDatabasePath    = "data/facelink.db"
	DefaultMigrationsDir   = "migrations"
	DefaultReadTimeout     = 10 * time.Second
	DefaultWriteTimeout    = 20 * time.Second
	DefaultIdleTimeout     = 60 * time.Second
	DefaultShutdownTimeout = 15 * time.Second
	DefaultLogLevel        = "info"
)

var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Config adalah konfigurasi backend yang sudah tervalidasi.
type Config struct {
	Addr string
	// DatabasePath adalah jalur berkas SQLite. Direktori induknya harus berada
	// di penyimpanan lokal, bukan folder yang disinkronkan ke awan
	// (docs/DATA_MODEL.md §7.4).
	DatabasePath string
	// MigrationsDir adalah direktori berkas migrasi SQL berversi.
	MigrationsDir string
	// AIEngineURL adalah alamat mesin AI lokal. Kosong berarti belum
	// dikonfigurasi; kesiapan dilaporkan sebagai "unavailable", bukan gagal
	// seluruh backend (docs/SECURITY_PRIVACY.md §5e).
	AIEngineURL string
	// Timeout server HTTP.
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	// LogLevel adalah salah satu dari debug, info, warn, error.
	LogLevel string
	// BuildVersion dipakai oleh /healthz.
	BuildVersion string
}

// Redacted mengembalikan konfigurasi dalam bentuk yang aman dicatat pada log.
// Nilai yang dapat memuat rahasia tidak pernah dikembalikan apa adanya; hanya
// fakta apakah variabelnya terisi yang dilaporkan.
func (c Config) Redacted() map[string]any {
	aiState := "unset"
	if c.AIEngineURL != "" {
		aiState = "set"
	}
	return map[string]any{
		"addr":             c.Addr,
		"database_path":    c.DatabasePath,
		"migrations_dir":   c.MigrationsDir,
		"ai_engine_url":    aiState,
		"read_timeout":     c.ReadTimeout.String(),
		"write_timeout":    c.WriteTimeout.String(),
		"idle_timeout":     c.IdleTimeout.String(),
		"shutdown_timeout": c.ShutdownTimeout.String(),
		"log_level":        c.LogLevel,
		"build_version":    c.BuildVersion,
	}
}

// EnvLookup adalah fungsi pembaca variabel lingkungan; disuntikkan agar
// pengujian tidak perlu menyentuh lingkungan proses nyata.
type EnvLookup func(string) (string, bool)

// OSEnv membaca dari lingkungan proses nyata.
func OSEnv(key string) (string, bool) { return os.LookupEnv(key) }

// Load membaca konfigurasi dari lingkungan proses nyata.
func Load() (Config, error) { return LoadFrom(OSEnv) }

// LoadFrom membaca konfigurasi memakai pembaca lingkungan yang diberikan,
// memvalidasi seluruh nilai, dan menolak nilai yang tidak masuk akal alih-alih
// memakai tebakan.
func LoadFrom(lookup EnvLookup) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("config: pembaca variabel lingkungan tidak boleh nil")
	}

	var problems []string

	addr := strings.TrimSpace(getString(lookup, EnvAddr, DefaultAddr))
	if err := validateAddr(addr); err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", EnvAddr, err))
	}

	dbPath := strings.TrimSpace(getString(lookup, EnvDatabasePath, DefaultDatabasePath))
	if dbPath == "" {
		problems = append(problems, EnvDatabasePath+": tidak boleh kosong")
	} else if err := validateLocalPath(dbPath); err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", EnvDatabasePath, err))
	}

	migrationsDir := strings.TrimSpace(getString(lookup, EnvMigrationsDir, DefaultMigrationsDir))
	if migrationsDir == "" {
		problems = append(problems, EnvMigrationsDir+": tidak boleh kosong")
	}

	aiURL := strings.TrimSpace(getString(lookup, EnvAIEngineURL, ""))
	if aiURL != "" {
		if err := validateLoopbackURL(aiURL); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", EnvAIEngineURL, err))
		}
	}

	readTimeout, err := getDuration(lookup, EnvReadTimeout, DefaultReadTimeout)
	if err != nil {
		problems = append(problems, err.Error())
	}
	writeTimeout, err := getDuration(lookup, EnvWriteTimeout, DefaultWriteTimeout)
	if err != nil {
		problems = append(problems, err.Error())
	}
	idleTimeout, err := getDuration(lookup, EnvIdleTimeout, DefaultIdleTimeout)
	if err != nil {
		problems = append(problems, err.Error())
	}
	shutdownTimeout, err := getDuration(lookup, EnvShutdownTimeout, DefaultShutdownTimeout)
	if err != nil {
		problems = append(problems, err.Error())
	}

	logLevel := strings.ToLower(strings.TrimSpace(getString(lookup, EnvLogLevel, DefaultLogLevel)))
	if !validLogLevels[logLevel] {
		problems = append(problems, fmt.Sprintf("%s: %q tidak dikenal (pilih debug, info, warn, atau error)", EnvLogLevel, logLevel))
	}

	buildVersion := strings.TrimSpace(getString(lookup, EnvBuildVersion, "dev"))
	if buildVersion == "" {
		buildVersion = "dev"
	}

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("konfigurasi tidak valid:\n  - %s", strings.Join(problems, "\n  - "))
	}

	return Config{
		Addr:            addr,
		DatabasePath:    filepath.Clean(dbPath),
		MigrationsDir:   filepath.Clean(migrationsDir),
		AIEngineURL:     aiURL,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		IdleTimeout:     idleTimeout,
		ShutdownTimeout: shutdownTimeout,
		LogLevel:        logLevel,
		BuildVersion:    buildVersion,
	}, nil
}

func getString(lookup EnvLookup, key, fallback string) string {
	if v, ok := lookup(key); ok {
		return v
	}
	return fallback
}

func getDuration(lookup EnvLookup, key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	raw = strings.TrimSpace(raw)

	// Terima bentuk durasi Go ("15s") dan bentuk milidetik bulat ("15000").
	if ms, err := strconv.Atoi(raw); err == nil {
		if ms <= 0 {
			return 0, fmt.Errorf("%s: %d harus lebih besar dari nol", key, ms)
		}
		return time.Duration(ms) * time.Millisecond, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %q bukan durasi yang sah (contoh: 15s atau 15000)", key, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: %q harus lebih besar dari nol", key, raw)
	}
	return d, nil
}

func validateAddr(addr string) error {
	if addr == "" {
		return errors.New("alamat listen tidak boleh kosong")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%q bukan alamat host:port yang sah", addr)
	}
	if port == "" {
		return fmt.Errorf("%q tidak memuat port", addr)
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		return fmt.Errorf("port %q tidak sah", port)
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return nil
	case "":
		return errors.New("host kosong berarti mendengarkan semua antarmuka; tulis 127.0.0.1 secara eksplisit")
	default:
		return fmt.Errorf("alamat %q berada di luar loopback; MVP hanya boleh mendengarkan 127.0.0.1 atau localhost (docs/SECURITY_PRIVACY.md §6)", addr)
	}
}

// validateLoopbackURL menegakkan aturan bahwa mesin AI lokal tidak boleh terbuka
// bebas pada jaringan kelas (AGENTS.md §2).
func validateLoopbackURL(raw string) error {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(raw, "http://"), "https://")
	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		// Tanpa port tetap boleh selama host-nya loopback.
		host = trimmed
		if strings.Contains(raw, "://") && strings.Contains(trimmed, "/") {
			host = strings.SplitN(strings.SplitN(trimmed, "/", 2)[0], ":", 2)[0]
		}
	}
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return nil
	default:
		return fmt.Errorf("host %q bukan loopback; mesin AI lokal harus dibatasi ke 127.0.0.1/IPC", host)
	}
}

// validateLocalPath menolak jalur berkas database pada folder yang
// disinkronkan ke awan (docs/DATA_MODEL.md §7.4). Berkas SQLite pada
// OneDrive/Dropbox/Google Drive menghasilkan penguncian yang tidak dapat
// diandalkan dan menyebarkan data biometrik ke luar mesin.
func validateLocalPath(path string) error {
	lower := strings.ToLower(filepath.ToSlash(path))
	synced := []string{
		"onedrive", "dropbox", "google drive", "googledrive", "icloud",
		"sharepoint", "megasync", "yandexdisk", "\\\\?\\unc", "//",
	}
	for _, marker := range synced {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("jalur %q tampak berada di folder tersinkronisasi awan atau lokasi jaringan; SQLite harus berada di penyimpanan lokal (docs/DATA_MODEL.md §7.4)", path)
		}
	}
	if strings.HasPrefix(path, "\\\\") {
		return fmt.Errorf("jalur %q adalah jalur UNC; SQLite harus berada di penyimpanan lokal (docs/DATA_MODEL.md §7.4)", path)
	}
	return nil
}
