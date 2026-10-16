package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrNonceNotFound dikembalikan bila nonce tidak terdaftar atau sudah
// kedaluwarsa. Keduanya diperlakukan sama: login tidak boleh dilanjutkan, dan
// pemanggil tidak diberi tahu mana yang terjadi.
var ErrNonceNotFound = errors.New("auth: nonce tidak terdaftar atau kedaluwarsa")

// TTL nonce dan lama hidup barisnya. Kontrak §5.1: nonce sekali pakai dengan
// TTL 5 menit. Barisnya dipertahankan sedikit lebih lama daripada masa
// berlakunya supaya nonce yang sudah terpakai masih dapat dikenali sebagai
// "sudah dipakai" (bukan sebagai "tidak pernah ada") — perbedaan itu penting
// agar pemakaian ulang tetap tercatat di audit.
const (
	TTLNonce        = 5 * time.Minute
	masaSimpanNonce = TTLNonce + 5*time.Minute
)

// NonceHash adalah bentuk yang boleh disimpan: SHA-256 heksadesimal. Nilai
// nonce mentah tidak pernah menyentuh database, log, atau pesan galat.
func NonceHash(nonce string) string {
	ringkas := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(ringkas[:])
}

// NonceBaru membuat nilai nonce acak 256 bit dalam base64url tanpa padding.
// Sumbernya kriptografis; nilai yang dihasilkan hanya pernah ada di respons
// endpoint nonce dan pada ID token yang ditandatangani Google.
func NonceBaru() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("auth: sumber acak nonce tidak tersedia: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// StoreNonce mendaftarkan nonce yang baru dibuat aplikasi klien. Baris yang
// sudah ada — terpakai maupun kedaluwarsa — tidak dihidupkan kembali: nonce
// bersifat sekali pakai untuk selama masa simpannya (D-033, kontrak §5.1).
// Pemanggilan yang aman adalah saat aplikasi meminta nonce, bukan saat login.
func (s *Store) StoreNonce(ctx context.Context, nonce string) error {
	if teks(nonce) == "" {
		return fmt.Errorf("%w: nonce wajib diisi", ErrInvalidInput)
	}

	now := s.waktuSekarang()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO auth_nonces (nonce_hash, created_at, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT (nonce_hash) DO NOTHING`,
		NonceHash(nonce), formatWaktu(now), formatWaktu(now.Add(masaSimpanNonce)),
	); err != nil {
		return fmt.Errorf("auth: gagal menyimpan nonce: %w", err)
	}

	// Baris yang masa simpannya sudah lewat dibuang pada kesempatan yang sama.
	// Endpoint penerbit nonce bersifat publik dan belum dibatasi lajunya, jadi
	// tanpa pembersihan di jalur ini tabel `auth_nonces` tumbuh tanpa batas.
	// Kegagalan pembersihan tidak menggagalkan penerbitan: nonce yang baru
	// tersimpan tetap sah, dan sisa baris lama akan dibersihkan lain kali.
	_, _ = s.CleanupExpiredNonces(ctx)
	return nil
}

// ConsumeNonce memakai nonce tepat sekali. Penghapusan bersyarat dilakukan dalam
// satu pernyataan DELETE, sehingga pemeriksaan kedaluwarsa dan pemakaian tidak
// dapat berlomba: dua permintaan dengan nonce yang sama hanya satu yang
// mendapatkan jumlah baris terhapus satu.
//
// Nonce yang tidak terdaftar, kedaluwarsa, atau sudah dipakai sama-sama
// menghasilkan ErrNonceNotFound tanpa galat database — pemanggil hanya perlu
// tahu bahwa login tidak boleh dilanjutkan.
func (s *Store) ConsumeNonce(ctx context.Context, nonce string) error {
	if teks(nonce) == "" {
		return fmt.Errorf("%w: nonce wajib diisi", ErrInvalidInput)
	}

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM auth_nonces WHERE nonce_hash = ? AND expires_at > ?`,
		NonceHash(nonce), formatWaktu(s.waktuSekarang().Add(TTLNonce)))
	if err != nil {
		return fmt.Errorf("auth: gagal memakai nonce: %w", err)
	}
	terhapus, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: jumlah nonce terpakai tidak dapat dibaca: %w", err)
	}
	if terhapus != 1 {
		return ErrNonceNotFound
	}
	return nil
}

// CleanupExpiredNonces menghapus baris nonce yang masa simpannya lewat dan
// mengembalikan jumlah baris yang dibuang. Dipanggil berkala oleh server supaya
// tabel tidak tumbuh tanpa batas.
func (s *Store) CleanupExpiredNonces(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM auth_nonces WHERE expires_at <= ?`, formatWaktu(s.waktuSekarang()))
	if err != nil {
		return 0, fmt.Errorf("auth: gagal membersihkan nonce kedaluwarsa: %w", err)
	}
	jumlah, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("auth: jumlah nonce terhapus tidak dapat dibaca: %w", err)
	}
	return jumlah, nil
}
