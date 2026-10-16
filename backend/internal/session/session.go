// Package session mengelola sesi pengguna FaceLink AI: token opak acak 256 bit
// yang di database hanya disimpan sebagai hash SHA-256, rotasi token penyegar
// pada setiap penyegaran dengan deteksi pemakaian ulang, dan batas absolut sesi
// yang tidak pernah diperpanjang (D-033, docs/AUTHORIZATION.md §3.1).
//
// Paket ini tidak mengenal HTTP maupun Google: ia hanya berbicara ke tabel
// `session_tokens`. Token mentah keluar dari paket ini tepat sekali — saat
// diterbitkan — dan tidak pernah ditulis ke kolom hash, log, atau pesan galat;
// pesan galat hanya memuat ID sesi.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

// Masa berlaku bawaan (D-033): token akses 60 menit, batas absolut sesi 30 hari.
const (
	AccessTTLBawaan   = 60 * time.Minute
	AbsoluteTTLBawaan = 30 * 24 * time.Hour
)

// panjangToken adalah jumlah byte acak per token: 32 byte = 256 bit
// (docs/AUTHORIZATION.md §3.1 "token opak acak 256-bit").
const panjangToken = 32

// layoutWaktu adalah bentuk teks kolom waktu pada skema FaceLink AI
// (docs/DATA_MODEL.md §3): ISO 8601 UTC dengan milidetik dan akhiran Z. Bentuknya
// sama dengan audit.FormatWaktu, sehingga seluruh kolom waktu dapat dibandingkan
// secara leksikografis maupun numerik tanpa penafsiran zona waktu.
const layoutWaktu = "2006-01-02T15:04:05.000Z"

// Config mengatur masa berlaku token. Nilai nol atau negatif diganti dengan
// bawaan D-033, sehingga pemanggil yang tidak mengisi apa pun tetap mendapat
// kebijakan sesi yang disetujui.
type Config struct {
	AccessTTL   time.Duration
	AbsoluteTTL time.Duration
}

// denganBawaan mengisi bidang yang tidak diisi pemanggil.
func (c Config) denganBawaan() Config {
	if c.AccessTTL <= 0 {
		c.AccessTTL = AccessTTLBawaan
	}
	if c.AbsoluteTTL <= 0 {
		c.AbsoluteTTL = AbsoluteTTLBawaan
	}
	return c
}

// Store mengelola baris `session_tokens`.
type Store struct {
	db  *database.DB
	clk clock.Clock
	cfg Config
}

// NewStore membuat pengelola sesi. Jam disuntikkan agar masa berlaku, batas
// absolut, dan rotasi penyegar dapat diuji tanpa menunggu waktu nyata.
func NewStore(db *database.DB, clk clock.Clock, cfg Config) *Store {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Store{db: db, clk: clk, cfg: cfg.denganBawaan()}
}

// Pair adalah pasangan token yang diberikan kepada klien. Kedua token mentah
// hanya ada di nilai ini dan pada respons; database tidak pernah menyimpannya.
type Pair struct {
	SessionID         string
	AccessToken       string
	RefreshToken      string
	AccessExpiresAt   time.Time
	RefreshExpiresAt  time.Time
	AbsoluteExpiresAt time.Time
}

// Session adalah potret satu sesi tanpa hash token: nilai inilah yang aman
// dikembalikan ke pemanggil atau diserialisasi ke API.
type Session struct {
	ID                string
	UserID            string
	CreatedAt         time.Time
	AccessExpiresAt   time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	RevokedReason     string
	LastSeenAt        *time.Time
	UserAgentRingkas  string
}

// tokenBaru membuat token opak acak 256 bit yang dikodekan base64url tanpa
// padding. Tanpa padding karena token dipakai pada header dan badan JSON, bukan
// pada segmen URL yang menuntut padding.
func tokenBaru() (string, error) {
	var b [panjangToken]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("sesi: sumber acak tidak tersedia: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// hashToken menghitung SHA-256 heksadesimal token. Inilah satu-satunya bentuk
// token yang boleh menyentuh database.
func hashToken(token string) string {
	ringkas := sha256.Sum256([]byte(token))
	return hex.EncodeToString(ringkas[:])
}

// formatWaktu menulis waktu ke bentuk kolom teks skema.
func formatWaktu(t time.Time) string { return t.UTC().Format(layoutWaktu) }

// parseWaktu membaca kembali waktu dari kolom teks skema.
func parseWaktu(nilai string) (time.Time, error) {
	t, err := time.Parse(layoutWaktu, nilai)
	if err != nil {
		return time.Time{}, fmt.Errorf("sesi: waktu %q tidak dikenali: %w", nilai, err)
	}
	return t.UTC(), nil
}
