package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Kunci adalah namespace penyimpanan kanonik lama, bukan role pengguna.
// Snapshot hanya disimpan lokal dan tidak pernah diteruskan ke log/audit.
type attendanceIdempotency struct {
	key  string
	hash string
}

func newAttendanceIdempotency(key, path string, body any) (attendanceIdempotency, error) {
	if key == "" {
		return attendanceIdempotency{}, nil
	}
	if len(key) < 8 || len(key) > 128 {
		return attendanceIdempotency{}, ErrInvalidInput
	}
	for _, c := range []byte(key) {
		if c < 0x20 || c > 0x7e {
			return attendanceIdempotency{}, ErrInvalidInput
		}
	}
	canonical, err := json.Marshal(struct {
		Method string `json:"method"`
		Path   string `json:"path"`
		Body   any    `json:"body"`
	}{"POST", path, body})
	if err != nil {
		return attendanceIdempotency{}, fmt.Errorf("auth: gagal menyusun hash permintaan: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return attendanceIdempotency{key: key, hash: hex.EncodeToString(sum[:])}, nil
}

// Pemanggil wajib memeriksa aktor dan cakupan target SEBELUM membaca replay.
func (k attendanceIdempotency) replay(ctx context.Context, conn *sql.Conn, actorID string, now time.Time, target any) (int, bool, error) {
	// Pembersihan oportunistik mencakup kunci terbengkalai. Perubahan ikut
	// rollback bersama mutasi, dan tidak menulis snapshot ke log/audit.
	if err := purgeExpiredAttendanceIdempotency(ctx, conn, now); err != nil {
		return 0, false, err
	}
	if k.key == "" {
		return 0, false, nil
	}
	var hash, body, expires string
	var code int
	err := conn.QueryRowContext(ctx, `SELECT request_hash,response_code,response_body,expires_at
		FROM idempotency_records WHERE scope=? AND key=?`, "operator:"+actorID, k.key).Scan(&hash, &code, &body, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("auth: gagal membaca idempotensi: %w", err)
	}
	until, err := parseWaktu(expires)
	if err != nil {
		return 0, false, fmt.Errorf("auth: waktu idempotensi tidak terbaca: %w", err)
	}
	if !now.Before(until) {
		_, err := conn.ExecContext(ctx, `DELETE FROM idempotency_records WHERE scope=? AND key=?`, "operator:"+actorID, k.key)
		return 0, false, err
	}
	if hash != k.hash {
		return 0, false, ErrIdempotencyKeyReused
	}
	if err := json.Unmarshal([]byte(body), target); err != nil {
		return 0, false, fmt.Errorf("auth: snapshot idempotensi tidak terbaca: %w", err)
	}
	return code, true, nil
}

func (k attendanceIdempotency) save(ctx context.Context, conn *sql.Conn, actorID string, now time.Time, code int, response any) error {
	if k.key == "" {
		return nil
	}
	body, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("auth: gagal menyusun snapshot idempotensi: %w", err)
	}
	_, err = conn.ExecContext(ctx, `INSERT INTO idempotency_records
		(scope,key,request_hash,response_code,response_body,created_at,expires_at) VALUES(?,?,?,?,?,?,?)`,
		"operator:"+actorID, k.key, k.hash, code, string(body), formatWaktu(now), formatWaktu(now.Add(24*time.Hour)))
	if err != nil {
		return fmt.Errorf("auth: gagal menyimpan idempotensi: %w", err)
	}
	return nil
}

func purgeExpiredAttendanceIdempotency(ctx context.Context, conn *sql.Conn, now time.Time) error {
	_, err := conn.ExecContext(ctx, `DELETE FROM idempotency_records WHERE expires_at<=?`, formatWaktu(now))
	if err != nil {
		return fmt.Errorf("auth: gagal membersihkan idempotensi kedaluwarsa: %w", err)
	}
	return nil
}
