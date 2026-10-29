package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/smtpemail"
)

func envFrom(pairs map[string]string) EnvLookup {
	return func(key string) (string, bool) {
		v, ok := pairs[key]
		return v, ok
	}
}

// TestGoogleOAuthKonfigurasi memeriksa pembacaan audience Google, nilai bawaan
// JWKS, dan penolakan alamat kunci yang tidak memakai HTTPS (FL-011).
func TestGoogleOAuthKonfigurasi(t *testing.T) {
	bawaan, err := LoadFrom(envFrom(nil))
	if err != nil {
		t.Fatalf("konfigurasi bawaan harus sah: %v", err)
	}
	if len(bawaan.GoogleClientIDs) != 0 {
		t.Errorf("GoogleClientIDs bawaan harus kosong, dapat %v", bawaan.GoogleClientIDs)
	}
	if bawaan.GoogleJWKSURL != DefaultGoogleJWKSURL {
		t.Errorf("GoogleJWKSURL bawaan = %q, ingin %q", bawaan.GoogleJWKSURL, DefaultGoogleJWKSURL)
	}

	cfg, err := LoadFrom(envFrom(map[string]string{
		EnvGoogleClientIDs: " 123-abc.apps.googleusercontent.com , 456-def.apps.googleusercontent.com ,, ",
	}))
	if err != nil {
		t.Fatalf("client ID sah harus diterima: %v", err)
	}
	if len(cfg.GoogleClientIDs) != 2 {
		t.Fatalf("GoogleClientIDs = %v, ingin 2 entri tanpa entri kosong", cfg.GoogleClientIDs)
	}
	if cfg.GoogleClientIDs[1] != "456-def.apps.googleusercontent.com" {
		t.Errorf("entri kedua = %q, ingin tanpa spasi tepi", cfg.GoogleClientIDs[1])
	}

	if _, err := LoadFrom(envFrom(map[string]string{
		EnvGoogleClientIDs: "bukan-client-id",
	})); err == nil {
		t.Error("client ID yang bukan milik Google seharusnya ditolak")
	}

	if _, err := LoadFrom(envFrom(map[string]string{
		EnvGoogleJWKSURL: "http://kunci.contoh.test/certs",
	})); err == nil {
		t.Error("JWKS lewat HTTP terbuka seharusnya ditolak")
	}

	if _, err := LoadFrom(envFrom(map[string]string{
		EnvGoogleJWKSURL: "https://kunci.contoh.test/certs",
	})); err != nil {
		t.Errorf("JWKS HTTPS seharusnya diterima: %v", err)
	}
}

// TestRedactedTidakMembocorkanClientID menegaskan log konfigurasi hanya
// melaporkan bahwa OAuth terkonfigurasi, bukan nilainya.
func TestRedactedTidakMembocorkanClientID(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{
		EnvGoogleClientIDs: "123-abc.apps.googleusercontent.com",
	}))
	if err != nil {
		t.Fatalf("konfigurasi gagal: %v", err)
	}
	isi, err := json.Marshal(cfg.Redacted())
	if err != nil {
		t.Fatalf("marshal gagal: %v", err)
	}
	if strings.Contains(string(isi), "apps.googleusercontent.com") {
		t.Errorf("Redacted memuat client ID: %s", isi)
	}
	if !strings.Contains(string(isi), "configured (1 client id)") {
		t.Errorf("Redacted tidak melaporkan status OAuth: %s", isi)
	}
	if !strings.Contains(string(isi), `"tls":"off"`) {
		t.Errorf("Redacted tidak melaporkan TLS mati secara eksplisit: %s", isi)
	}
}

func TestTLSMengaktifkanHTTPSDanTidakMembocorkanJalurKunci(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.pem")
	keyPath := filepath.Join(dir, "server-key.pem")
	for _, p := range []string{certPath, keyPath} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatalf("menyiapkan berkas uji gagal: %v", err)
		}
	}

	cfg, err := LoadFrom(envFrom(map[string]string{EnvTLSCert: certPath, EnvTLSKey: keyPath}))
	if err != nil {
		t.Fatalf("konfigurasi TLS sah ditolak: %v", err)
	}
	if cfg.TLSCertPath != certPath || cfg.TLSKeyPath != keyPath {
		t.Fatalf("jalur TLS tidak terbaca: %+v", cfg)
	}
	if !strings.HasPrefix(cfg.Addr, "127.0.0.1") {
		t.Errorf("alamat %q tidak loopback meski TLS aktif", cfg.Addr)
	}

	isi, err := json.Marshal(cfg.Redacted())
	if err != nil {
		t.Fatalf("marshal gagal: %v", err)
	}
	if strings.Contains(string(isi), "server-key.pem") || strings.Contains(string(isi), dir) {
		t.Errorf("Redacted memuat jalur sertifikat/kunci: %s", isi)
	}
	if !strings.Contains(string(isi), `"tls":"on"`) {
		t.Errorf("Redacted tidak melaporkan TLS aktif: %s", isi)
	}
}

func TestLoadDefaultsMemakaiNilaiAman(t *testing.T) {
	cfg, err := LoadFrom(envFrom(nil))
	if err != nil {
		t.Fatalf("konfigurasi bawaan harus sah, dapat galat: %v", err)
	}

	if cfg.Addr != DefaultAddr {
		t.Errorf("Addr = %q, ingin %q", cfg.Addr, DefaultAddr)
	}
	if !strings.HasPrefix(cfg.Addr, "127.0.0.1") {
		t.Errorf("alamat bawaan %q tidak dibatasi ke loopback", cfg.Addr)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, ingin %q", cfg.LogLevel, DefaultLogLevel)
	}
	if cfg.AIEngineURL != "" {
		t.Errorf("AIEngineURL bawaan harus kosong (belum dikonfigurasi), dapat %q", cfg.AIEngineURL)
	}
	if cfg.BuildVersion != "dev" {
		t.Errorf("BuildVersion = %q, ingin %q", cfg.BuildVersion, "dev")
	}

	want := map[string]time.Duration{
		"ReadTimeout":     DefaultReadTimeout,
		"WriteTimeout":    DefaultWriteTimeout,
		"IdleTimeout":     DefaultIdleTimeout,
		"ShutdownTimeout": DefaultShutdownTimeout,
	}
	got := map[string]time.Duration{
		"ReadTimeout":     cfg.ReadTimeout,
		"WriteTimeout":    cfg.WriteTimeout,
		"IdleTimeout":     cfg.IdleTimeout,
		"ShutdownTimeout": cfg.ShutdownTimeout,
	}
	for name, wantDur := range want {
		if got[name] != wantDur {
			t.Errorf("%s = %v, ingin %v", name, got[name], wantDur)
		}
	}
}

func TestLoadFromMembacaSeluruhVariabel(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{
		EnvAddr:            "127.0.0.1:9911",
		EnvDatabasePath:    "data/uji.db",
		EnvMigrationsDir:   "migrasi",
		EnvAIEngineURL:     "http://127.0.0.1:8001",
		EnvReadTimeout:     "3s",
		EnvWriteTimeout:    "7s",
		EnvIdleTimeout:     "11s",
		EnvShutdownTimeout: "13s",
		EnvLogLevel:        "DEBUG",
		EnvBuildVersion:    "fl-005-uji",
	}))
	if err != nil {
		t.Fatalf("konfigurasi sah harus diterima, dapat galat: %v", err)
	}

	if cfg.Addr != "127.0.0.1:9911" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.DatabasePath != filepath.Clean("data/uji.db") {
		t.Errorf("DatabasePath = %q", cfg.DatabasePath)
	}
	if cfg.ReadTimeout != 3*time.Second || cfg.WriteTimeout != 7*time.Second {
		t.Errorf("timeout baca/tulis = %v/%v", cfg.ReadTimeout, cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 11*time.Second || cfg.ShutdownTimeout != 13*time.Second {
		t.Errorf("timeout idle/shutdown = %v/%v", cfg.IdleTimeout, cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, ingin dinormalkan menjadi huruf kecil", cfg.LogLevel)
	}
	if cfg.BuildVersion != "fl-005-uji" {
		t.Errorf("BuildVersion = %q", cfg.BuildVersion)
	}
}

func TestLoadFromMenolakKonfigurasiTidakValid(t *testing.T) {
	cases := []struct {
		name    string
		pairs   map[string]string
		wantSub string
	}{
		{"alamat tanpa port", map[string]string{EnvAddr: "127.0.0.1"}, EnvAddr},
		{"alamat terbuka jaringan", map[string]string{EnvAddr: "0.0.0.0:8080"}, "loopback"},
		{"alamat host kosong", map[string]string{EnvAddr: ":8080"}, "semua antarmuka"},
		{"alamat IP LAN", map[string]string{EnvAddr: "192.168.1.10:8080"}, "loopback"},
		{"basis data kosong", map[string]string{EnvDatabasePath: "   "}, EnvDatabasePath},
		{"basis data di OneDrive", map[string]string{EnvDatabasePath: "C:/Users/test-user/OneDrive/data/facelink.db"}, "tersinkronisasi"},
		{"basis data jalur UNC", map[string]string{EnvDatabasePath: "//server/data/facelink.db"}, "lokal"},
		{"migrasi kosong", map[string]string{EnvMigrationsDir: ""}, EnvMigrationsDir},
		{"url AI bukan loopback", map[string]string{EnvAIEngineURL: "http://192.168.1.20:8001"}, "loopback"},
		{"level log tidak dikenal", map[string]string{EnvLogLevel: "verbose"}, EnvLogLevel},
		{"durasi bukan angka", map[string]string{EnvReadTimeout: "cepat"}, EnvReadTimeout},
		{"durasi nol", map[string]string{EnvWriteTimeout: "0"}, EnvWriteTimeout},
		{"durasi negatif", map[string]string{EnvIdleTimeout: "-5s"}, EnvIdleTimeout},
		{"sertifikat tanpa kunci", map[string]string{EnvTLSCert: "server.pem"}, "bersamaan"},
		{"kunci tanpa sertifikat", map[string]string{EnvTLSKey: "server-key.pem"}, "bersamaan"},
		{"sertifikat tidak ada", map[string]string{
			EnvTLSCert: "tidak-ada.pem", EnvTLSKey: "tidak-ada-key.pem",
		}, EnvTLSCert},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadFrom(envFrom(tc.pairs))
			if err == nil {
				t.Fatalf("konfigurasi tidak sah harus ditolak, tetapi diterima: %+v", cfg)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("galat %q tidak memuat %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoadFromMengumpulkanSeluruhMasalahSekaligus(t *testing.T) {
	_, err := LoadFrom(envFrom(map[string]string{
		EnvAddr:        "0.0.0.0:8080",
		EnvLogLevel:    "verbose",
		EnvReadTimeout: "cepat",
	}))
	if err == nil {
		t.Fatal("konfigurasi dengan tiga masalah harus ditolak")
	}
	msg := err.Error()
	for _, want := range []string{EnvAddr, EnvLogLevel, EnvReadTimeout} {
		if !strings.Contains(msg, want) {
			t.Errorf("laporan galat tidak menyebut %s:\n%s", want, msg)
		}
	}
}

func TestLoadFromMenolakPembacaNil(t *testing.T) {
	if _, err := LoadFrom(nil); err == nil {
		t.Fatal("pembaca nil harus ditolak")
	}
}

func TestDurasiMenerimaBentukMilidetik(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{EnvReadTimeout: "2500"}))
	if err != nil {
		t.Fatalf("bentuk milidetik harus diterima: %v", err)
	}
	if cfg.ReadTimeout != 2500*time.Millisecond {
		t.Errorf("ReadTimeout = %v, ingin 2.5s", cfg.ReadTimeout)
	}
}

func TestRedactedTidakMembocorkanNilaiSensitif(t *testing.T) {
	const secretURL = "http://127.0.0.1:8001/mesin/rahasia"

	cfg, err := LoadFrom(envFrom(map[string]string{
		EnvAIEngineURL: secretURL,
		EnvLogLevel:    "debug",
	}))
	if err != nil {
		t.Fatalf("konfigurasi harus sah: %v", err)
	}

	encoded, err := json.Marshal(cfg.Redacted())
	if err != nil {
		t.Fatalf("bentuk aman harus dapat diserialkan: %v", err)
	}
	rendered := string(encoded)

	if strings.Contains(rendered, secretURL) {
		t.Errorf("bentuk aman membocorkan URL mesin AI:\n%s", rendered)
	}
	if strings.Contains(rendered, "/mesin/rahasia") {
		t.Errorf("bentuk aman membocorkan jalur mesin AI:\n%s", rendered)
	}
	if !strings.Contains(rendered, `"ai_engine_url":"set"`) {
		t.Errorf("bentuk aman harus melaporkan ai_engine_url sebagai set:\n%s", rendered)
	}


	// Seluruh nilai yang boleh muncul hanyalah nilai yang memang tidak rahasia.
	for _, forbidden := range []string{"password", "token", "secret", "api_key"} {
		if strings.Contains(strings.ToLower(rendered), forbidden) {
			t.Errorf("bentuk aman memuat nama field sensitif %q:\n%s", forbidden, rendered)
		}
	}
}

func TestRedactedMenandaiMesinAIBelumDikonfigurasi(t *testing.T) {
	cfg, err := LoadFrom(envFrom(nil))
	if err != nil {
		t.Fatalf("konfigurasi bawaan harus sah: %v", err)
	}
	encoded, err := json.Marshal(cfg.Redacted())
	if err != nil {
		t.Fatalf("serialisasi gagal: %v", err)
	}
	if !strings.Contains(string(encoded), `"ai_engine_url":"unset"`) {
		t.Errorf("bentuk aman harus melaporkan ai_engine_url sebagai unset:\n%s", encoded)
	}
}

func TestOSEnvMembacaLingkunganNyata(t *testing.T) {
	const key = "FACELINK_TEST_OS_ENV"
	const value = "nilai-uji-fl-005"

	t.Setenv(key, value)

	got, ok := OSEnv(key)
	if !ok {
		t.Fatalf("OSEnv(%q) melaporkan variabel tidak ada", key)
	}
	if got != value {
		t.Errorf("OSEnv(%q) = %q, ingin %q", key, got, value)
	}

	os.Unsetenv(key)
	if _, ok := OSEnv(key); ok {
		t.Errorf("OSEnv(%q) masih melaporkan variabel ada setelah dihapus", key)
	}
}

func TestLoadMemakaiLingkunganProses(t *testing.T) {
	t.Setenv(EnvBuildVersion, "fl-005-dari-env")
	t.Setenv(EnvLogLevel, "warn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() harus berhasil dengan lingkungan uji: %v", err)
	}
	if cfg.BuildVersion != "fl-005-dari-env" {
		t.Errorf("BuildVersion = %q, ingin nilai dari lingkungan", cfg.BuildVersion)
	}
	if cfg.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, ingin %q", cfg.LogLevel, "warn")
	}
}

func TestEmailOTPConfigFailClosed(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.pem")
	keyPath := filepath.Join(dir, "server-key.pem")
	for _, p := range []string{certPath, keyPath} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tlsEnv := map[string]string{EnvTLSCert: certPath, EnvTLSKey: keyPath}

	// Semua kosong: OTP tidak tersedia, bukan konfigurasi setengah jadi.
	cfg, err := LoadFrom(envFrom(nil))
	if err != nil {
		t.Fatalf("konfigurasi bawaan harus sah: %v", err)
	}
	if len(cfg.OTPHMACKey) != 0 || cfg.SMTP.Host != "" {
		t.Fatal("konfigurasi kosong mengaktifkan Email OTP")
	}

	full := map[string]string{
		"SMTP_HOST":        "smtp.gmail.com",
		"SMTP_PORT":        "587",
		"SMTP_USERNAME":    "smtp-sender@example.test",
		"SMTP_AUTH_SECRET": "rahasia-yang-tidak-boleh-bocor",
		"SMTP_FROM":        "smtp-sender@example.test",
	}
	// SMTP lengkap tanpa HTTPS backend dan konfigurasi parsial harus ditolak.
	if _, err := LoadFrom(envFrom(full)); err == nil {
		t.Fatal("SMTP lengkap tanpa HTTPS backend diterima")
	}
	if _, err := LoadFrom(envFrom(map[string]string{"SMTP_HOST": "smtp.gmail.com"})); err == nil {
		t.Fatal("SMTP parsial diterima")
	}

	// Alias From hanya sah dengan allowlist domain yang eksplisit.
	alias := map[string]string{EnvSMTPAllowedFromDomains: "facelink.online"}
	for k, v := range full {
		alias[k] = v
	}
	alias["SMTP_FROM"] = "support@facelink.online"
	for k, v := range tlsEnv {
		alias[k] = v
	}
	if _, err := LoadFrom(envFrom(alias)); err == nil {
		t.Fatal("alias tanpa key OTP diterima")
	} else if strings.Contains(err.Error(), "SMTP_ALLOWED_FROM_DOMAINS") {
		t.Fatalf("allowlist tidak terbaca oleh loader: %v", err)
	}
	tanpaAllowlist := map[string]string{}
	for k, v := range alias {
		tanpaAllowlist[k] = v
	}
	delete(tanpaAllowlist, EnvSMTPAllowedFromDomains)
	if _, err := LoadFrom(envFrom(tanpaAllowlist)); err == nil || !strings.Contains(err.Error(), "SMTP_ALLOWED_FROM_DOMAINS") {
		t.Fatalf("alias tanpa allowlist tidak ditolak dengan jelas: %v", err)
	}

	// Key plaintext/in-repo tidak boleh lolos sebagai key HMAC.
	keyFile := filepath.Join(dir, "otp.key")
	if err := os.WriteFile(keyFile, []byte("bukan-key-dpapi"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{EnvOTPHMACKeyFile: keyFile}
	for k, v := range full {
		env[k] = v
	}
	for k, v := range tlsEnv {
		env[k] = v
	}
	_, err = LoadFrom(envFrom(env))
	if err == nil {
		t.Fatal("key OTP yang tidak terlindungi diterima")
	}
	if strings.Contains(err.Error(), full["SMTP_AUTH_SECRET"]) {
		t.Fatal("galat konfigurasi membocorkan SMTP secret")
	}

	// Redacted tidak boleh memuat secret, jalur key, atau nama field secret.
	redacted, err := json.Marshal(Config{
		SMTP: smtpemail.Config{
			Host: "smtp.gmail.com", Port: 587,
			Username: "smtp-sender@example.test", AuthSecret: full["SMTP_AUTH_SECRET"],
			From: "smtp-sender@example.test",
		},
		OTPHMACKey: make([]byte, 32),
	}.Redacted())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{full["SMTP_AUTH_SECRET"], keyFile, "auth_secret"} {
		if strings.Contains(string(redacted), forbidden) {
			t.Fatalf("Redacted membocorkan %q: %s", forbidden, redacted)
		}
	}
	if !strings.Contains(string(redacted), `"email_otp":true`) {
		t.Errorf("Redacted tidak menandai OTP aktif: %s", redacted)
	}
}