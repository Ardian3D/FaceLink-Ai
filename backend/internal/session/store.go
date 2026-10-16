package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// kolomSession adalah kolom yang dibaca untuk membentuk Session. Kolom hash
// token sengaja tidak pernah ada di daftar ini, sehingga tidak mungkin ikut
// terbaca lalu terserialisasi ke respons API.
const kolomSession = `id, user_id, created_at, access_expires_at, absolute_expires_at, ` +
	`revoked_at, revoked_reason, last_seen_at, user_agent_ringkas`

const (
	sqlSisipSesi = `INSERT INTO session_tokens (
		id, user_id, access_hash, access_expires_at, refresh_hash, refresh_expires_at,
		absolute_expires_at, prev_refresh_hash, revoked_at, revoked_reason,
		last_seen_at, user_agent_ringkas, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, ?, ?)`

	sqlCariLewatAkses = `SELECT ` + kolomSession + ` FROM session_tokens WHERE access_hash = ?`

	sqlCariLewatPenyegar = `SELECT ` + kolomSession + `, refresh_hash FROM session_tokens WHERE refresh_hash = ?`

	sqlCariLewatPenyegarLama = `SELECT ` + kolomSession + `, refresh_hash FROM session_tokens ` +
		`WHERE prev_refresh_hash = ? ORDER BY created_at DESC, rowid DESC LIMIT 1`

	// Pencarian riwayat pemakaian ulang. `prev_refresh_hash` hanya menautkan
	// hash satu generasi ke belakang, sehingga token yang sudah diputar dua
	// kali atau lebih tidak lagi ditemukan di `session_tokens`. Tabel riwayat
	// menutup celah itu (D-033, kontrak §5.1).
	sqlCariRiwayatPenyegar = `SELECT session_id FROM session_token_history WHERE refresh_hash = ? LIMIT 1`

	// Riwayat ditulis pada setiap rotasi. Nilai hash mentah penyegar, bukan
	// tokennya: tabel ini hanya memuat bentuk yang sudah boleh disimpan.
	sqlSisipRiwayatPenyegar = `INSERT INTO session_token_history (session_id, refresh_hash, superseded_at)
		VALUES (?, ?, ?) ON CONFLICT (refresh_hash) DO NOTHING`

	// Sesi yang sudah dicabut dikenali lewat hash apa pun yang masih tersimpan
	// di barisnya, supaya percobaan penyegaran setelah pencabutan dijawab
	// "sesi telah dicabut", bukan "token tidak dikenali".
	sqlCariPenyegarTercabut = `SELECT ` + kolomSession + `, refresh_hash FROM session_tokens
		WHERE (refresh_hash = ? OR prev_refresh_hash = ?) AND revoked_at IS NOT NULL LIMIT 1`

	// Cakupan kepemilikan baris sesi, dipakai pencabutan administratif.
	sqlCariSesiRingkas = `SELECT user_id, revoked_at FROM session_tokens WHERE id = ?`

	sqlCariSesiLewatID = `SELECT ` + kolomSession + ` FROM session_tokens WHERE id = ?`

	// Rotasi mengunci baris pada hash penyegar yang dibaca di dalam transaksi
	// yang sama, sehingga token yang sudah diputar tidak dapat diputar dua kali.
	sqlPutarPenyegar = `UPDATE session_tokens
		SET access_hash = ?, access_expires_at = ?, refresh_hash = ?, refresh_expires_at = ?, prev_refresh_hash = ?
		WHERE id = ? AND refresh_hash = ?`

	sqlCabutSesi = `UPDATE session_tokens SET revoked_at = ?, revoked_reason = ?
		WHERE id = ? AND revoked_at IS NULL`

	sqlCabutSemuaSesiPengguna = `UPDATE session_tokens SET revoked_at = ?, revoked_reason = ?
		WHERE user_id = ? AND revoked_at IS NULL`

	sqlDaftarSesiPengguna = `SELECT ` + kolomSession + ` FROM session_tokens
		WHERE user_id = ? ORDER BY created_at DESC, rowid DESC`

	sqlDaftarSesiAktifPengguna = `SELECT ` + kolomSession + ` FROM session_tokens
		WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at DESC, rowid DESC`

	sqlPerbaruiTerakhirDilihat = `UPDATE session_tokens SET last_seen_at = ? WHERE id = ?`
)

// Issue menerbitkan sesi baru untuk pengguna: token akses dan token penyegar
// acak yang berbeda, dengan batas absolut `now + AbsoluteTTL` dan token akses
// dipangkas agar tidak pernah melewati batas absolut itu.
func (s *Store) Issue(ctx context.Context, userID, userAgentRingkas string) (Pair, error) {
	now := s.clk.Now().UTC()
	absolute := now.Add(s.cfg.AbsoluteTTL)
	accessExpires := now.Add(s.cfg.AccessTTL)
	if accessExpires.After(absolute) {
		accessExpires = absolute
	}

	accessToken, err := tokenBaru()
	if err != nil {
		return Pair{}, err
	}
	refreshToken, err := tokenBaru()
	if err != nil {
		return Pair{}, err
	}

	sessionID := ids.New()
	if _, err := s.db.ExecContext(ctx, sqlSisipSesi,
		sessionID, userID,
		hashToken(accessToken), formatWaktu(accessExpires),
		hashToken(refreshToken), formatWaktu(absolute),
		formatWaktu(absolute),
		userAgentRingkas, formatWaktu(now),
	); err != nil {
		return Pair{}, fmt.Errorf("sesi: menerbitkan sesi untuk pengguna %s: %w", userID, err)
	}

	return Pair{
		SessionID:         sessionID,
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		AccessExpiresAt:   accessExpires,
		RefreshExpiresAt:  absolute,
		AbsoluteExpiresAt: absolute,
	}, nil
}

// Authenticate memetakan token akses ke sesinya. Token yang tidak dikenali,
// sesi yang sudah dicabut, dan token akses yang kedaluwarsa masing-masing
// dibedakan oleh sentinel galat agar pemanggil dapat memutuskan pencatatan audit
// maupun percobaan penyegaran.
func (s *Store) Authenticate(ctx context.Context, accessToken string) (Session, error) {
	now := s.clk.Now().UTC()

	sesi, err := s.bacaSatu(ctx, s.db, sqlCariLewatAkses, hashToken(accessToken))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, fmt.Errorf("sesi: token akses tidak dikenali: %w", ErrUnauthenticated)
	}
	if err != nil {
		return Session{}, fmt.Errorf("sesi: membaca sesi dari token akses: %w", err)
	}

	if sesi.RevokedAt != nil {
		return Session{}, fmt.Errorf("sesi: sesi %s telah dicabut: %w", sesi.ID, ErrSessionRevoked)
	}
	if !now.Before(sesi.AccessExpiresAt) {
		return Session{}, fmt.Errorf("sesi: token akses sesi %s kedaluwarsa: %w", sesi.ID, ErrAccessExpired)
	}
	return sesi, nil
}

// Refresh memutar token akses dan token penyegar dalam satu transaksi
// `BEGIN IMMEDIATE`.
//
// Batas absolut tidak pernah berpindah: `absolute_expires_at` dan
// `refresh_expires_at` tetap seperti saat sesi diterbitkan, dan token akses baru
// dipangkas pada batas itu. Token penyegar yang sudah diputar tetapi dikirim
// kembali dianggap pemakaian ulang: sesinya dicabut lalu ErrRefreshReused
// dikembalikan, sehingga pencuri token tidak mendapat sesi yang dapat dipakai.
func (s *Store) Refresh(ctx context.Context, refreshToken string) (Pair, error) {
	now := s.clk.Now().UTC()
	hashDiminta := hashToken(refreshToken)

	var (
		pair    Pair
		ditolak error // galat yang baru dikembalikan setelah transaksi di-commit
	)

	err := s.dalamTransaksiImmediate(ctx, func(conn *sql.Conn) error {
		// 1. Token penyegar yang masih berlaku.
		baris, err := s.bacaPenyegar(ctx, conn, sqlCariLewatPenyegar, hashDiminta)
		if err != nil {
			return err
		}
		if baris != nil {
			if baris.sesi.RevokedAt != nil {
				// Sesi sudah mati; pencabutan tidak diulang dan alasan aslinya
				// dipertahankan supaya jejaknya tidak ditimpa.
				ditolak = fmt.Errorf("sesi: sesi %s telah dicabut: %w", baris.sesi.ID, ErrRefreshInvalid)
				return nil
			}
			if !now.Before(baris.sesi.AbsoluteExpiresAt) {
				ditolak = fmt.Errorf("sesi: sesi %s melewati batas absolut: %w", baris.sesi.ID, ErrRefreshInvalid)
				return s.cabutDiTransaksi(ctx, conn, baris.sesi.ID, now, alasanRefreshExpired)
			}

			pair, err = s.putarDiTransaksi(ctx, conn, baris, now)
			return err
		}

		// 2. Token penyegar yang sudah diputar. Inilah jejak pemakaian ulang.
		lama, err := s.bacaPenyegar(ctx, conn, sqlCariLewatPenyegarLama, hashDiminta)
		if err != nil {
			return err
		}
		if lama == nil {
			// 3. Riwayat rotasi. Token generasi kedua ke belakang tidak lagi
			// tersimpan di `session_tokens`, sehingga tanpa langkah ini
			// pemakaian ulangnya lolos tanpa pencabutan dan tanpa audit.
			sesiID, err := s.cariRiwayatPenyegar(ctx, conn, hashDiminta)
			if err != nil {
				return err
			}
			if sesiID == "" {
				// 4. Sesi yang sudah dicabut dikenali lewat hash apa pun yang
				// masih tersimpan, supaya jawabannya "sesi telah dicabut"
				// (kontrak §5.1) dan bukan "token tidak dikenali".
				tercabut, err := s.bacaPenyegar(ctx, conn, sqlCariPenyegarTercabut, hashDiminta, hashDiminta)
				if err != nil {
					return err
				}
				if tercabut != nil {
					ditolak = fmt.Errorf("sesi: sesi %s telah dicabut: %w", tercabut.sesi.ID, ErrRefreshInvalid)
					return nil
				}
				ditolak = fmt.Errorf("sesi: token penyegar tidak dikenali: %w", ErrRefreshInvalid)
				return nil
			}
			sesi, err := bacaSesiLewatID(ctx, conn, sesiID)
			if err != nil {
				return err
			}
			if sesi == nil {
				ditolak = fmt.Errorf("sesi: token penyegar tidak dikenali: %w", ErrRefreshInvalid)
				return nil
			}
			if sesi.RevokedAt != nil {
				ditolak = fmt.Errorf("sesi: sesi %s telah dicabut: %w", sesi.ID, ErrRefreshInvalid)
				return nil
			}
			if !now.Before(sesi.AbsoluteExpiresAt) {
				ditolak = fmt.Errorf("sesi: sesi %s melewati batas absolut: %w", sesi.ID, ErrRefreshInvalid)
				return s.cabutDiTransaksi(ctx, conn, sesi.ID, now, alasanRefreshExpired)
			}
			ditolak = fmt.Errorf("sesi: penyegar sesi %s dipakai ulang: %w", sesi.ID, ErrRefreshReused)
			return s.cabutSemuaDiTransaksi(ctx, conn, sesi.UserID, now, alasanRefreshReuse)
		}
		if lama.sesi.RevokedAt != nil {
			ditolak = fmt.Errorf("sesi: sesi %s telah dicabut: %w", lama.sesi.ID, ErrRefreshInvalid)
			return nil
		}
		if !now.Before(lama.sesi.AbsoluteExpiresAt) {
			ditolak = fmt.Errorf("sesi: sesi %s melewati batas absolut: %w", lama.sesi.ID, ErrRefreshInvalid)
			return s.cabutDiTransaksi(ctx, conn, lama.sesi.ID, now, alasanRefreshExpired)
		}

		// Sesi masih hidup sementara penyegarnya sudah lama diputar: token ini
		// pasti salinan yang bocor atau dipakai dua perangkat. Seluruh sesi
		// pengguna itu dicabut, bukan hanya sesi yang terkena (D-033, kontrak
		// §5.1: "penyegar yang sudah dirotasi dipakai ulang → seluruh sesi
		// pengguna itu dicabut").
		ditolak = fmt.Errorf("sesi: penyegar sesi %s dipakai ulang: %w", lama.sesi.ID, ErrRefreshReused)
		return s.cabutSemuaDiTransaksi(ctx, conn, lama.sesi.UserID, now, alasanRefreshReuse)
	})
	if err != nil {
		return Pair{}, err
	}
	if ditolak != nil {
		return Pair{}, ditolak
	}
	return pair, nil
}

// Revoke mencabut satu sesi. Pencabutan bersifat idempoten: memanggilnya pada
// sesi yang sudah tercabut berhasil tanpa mengubah alasan dan waktu pencabutan
// yang pertama. Sesi yang tidak ada menghasilkan ErrNotFound.
//
// Batas waktu pencabutan diperiksa lebih dahulu: mencabut sesi yang sudah
// kedaluwarsa tidak mengubah apa pun, sehingga jawabannya `404` sesuai kontrak
// §5.1 ("mencabut sesi yang sudah tercabut mengembalikan 404") dan keadaan itu
// tetap dapat dibedakan dari pencabutan yang benar-benar terjadi.
func (s *Store) Revoke(ctx context.Context, sessionID, reason string) error {
	now := s.clk.Now().UTC()
	tercabut, err := s.RevokeOwned(ctx, sessionID, "", now, reason)
	if err != nil {
		return err
	}
	if !tercabut {
		return fmt.Errorf("sesi: sesi %s tidak ditemukan: %w", sessionID, ErrNotFound)
	}
	return nil
}

// RevokeOwned mencabut satu sesi dengan pemeriksaan cakupan kepemilikan.
//
// ownerID kosong berarti pemanggil berwenang atas sesi mana pun (admin pada
// `DELETE /auth/sessions/{session_id}`, kontrak §5.1). ownerID terisi berarti
// sesi hanya boleh dicabut bila memang milik pengguna itu — inilah yang menjaga
// sesi yang dibatalkan pengguna lain tidak dapat dicabut lewat jalur ini, dan
// membuat percobaan seperti itu tidak dapat membedakan "bukan milik saya" dari
// "tidak ada" (keduanya `404`).
//
// Nilai balik kedua false berarti tidak ada yang dicabut: sesi tidak ada,
// bukan milik ownerID, atau sudah tercabut.
func (s *Store) RevokeOwned(ctx context.Context, sessionID, ownerID string, now time.Time, reason string) (bool, error) {
	// 1. Jalur cepat: pencabutan langsung oleh pemilik sesi atau admin. Sesi
	// yang sudah kedaluwarsa tidak disentuh agar jawabannya tetap 404.
	hasil, err := s.db.ExecContext(ctx,
		`UPDATE session_tokens SET revoked_at = ?, revoked_reason = ?
		  WHERE id = ? AND revoked_at IS NULL
		    AND (access_expires_at > ? OR refresh_expires_at > ?)
		    AND (? = '' OR user_id = ?)`,
		formatWaktu(now), reason, sessionID, formatWaktu(now), formatWaktu(now), ownerID, ownerID)
	if err != nil {
		return false, fmt.Errorf("sesi: mencabut sesi %s: %w", sessionID, err)
	}
	jumlah, err := hasil.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("sesi: menghitung baris tercabut untuk sesi %s: %w", sessionID, err)
	}
	if jumlah == 1 {
		return true, nil
	}

	// 2. Nol baris: sesinya sudah tercabut, sudah kedaluwarsa, tidak ada, atau
	// bukan milik ownerID. Keadaan baris dibaca untuk membedakannya.
	var (
		userID     string
		revokedAt  sql.NullString
		accessExp  string
		refreshExp string
	)
	err = s.db.QueryRowContext(ctx,
		`SELECT user_id, revoked_at, access_expires_at, refresh_expires_at FROM session_tokens WHERE id = ?`,
		sessionID).Scan(&userID, &revokedAt, &accessExp, &refreshExp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("sesi: memeriksa keberadaan sesi %s: %w", sessionID, err)
	}

	if revokedAt.Valid {
		return false, nil
	}
	if ownerID != "" && userID != ownerID {
		return false, nil
	}
	batas, err := parseWaktu(refreshExp)
	if err != nil {
		return false, fmt.Errorf("sesi: membaca batas sesi %s: %w", sessionID, err)
	}
	if !now.Before(batas) {
		return false, nil
	}
	return true, nil
}

// ListActiveForUser mengembalikan sesi aktif satu pengguna. Sesi yang sudah
// dicabut tidak ikut karena endpoint administrasi hanya menyajikan sesi aktif
// (kontrak §5.1).
func (s *Store) ListActiveForUser(ctx context.Context, userID string) ([]Session, error) {
	return s.daftarSesi(ctx, sqlDaftarSesiAktifPengguna, userID)
}

// RevokeAllForUser mencabut seluruh sesi aktif milik satu pengguna dan
// mengembalikan jumlah baris yang benar-benar tercabut. Dipakai saat akun
// dinonaktifkan (D-032) maupun saat pemakaian ulang penyegar terdeteksi.
func (s *Store) RevokeAllForUser(ctx context.Context, userID, reason string) (int64, error) {
	now := s.clk.Now().UTC()

	hasil, err := s.db.ExecContext(ctx, sqlCabutSemuaSesiPengguna, formatWaktu(now), reason, userID)
	if err != nil {
		return 0, fmt.Errorf("sesi: mencabut seluruh sesi pengguna %s: %w", userID, err)
	}
	jumlah, err := hasil.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sesi: menghitung sesi tercabut pengguna %s: %w", userID, err)
	}
	return jumlah, nil
}

// ListForUser mengembalikan seluruh sesi pengguna, terbaru dahulu, tanpa hash
// token. Sesi yang sudah dicabut tetap disertakan supaya pemilik akun dapat
// melihat riwayatnya.
func (s *Store) ListForUser(ctx context.Context, userID string) ([]Session, error) {
	return s.daftarSesi(ctx, sqlDaftarSesiPengguna, userID)
}

// daftarSesi menjalankan satu kueri daftar sesi per pengguna dan memindai
// seluruh barisnya.
func (s *Store) daftarSesi(ctx context.Context, kueri, userID string) ([]Session, error) {
	baris, err := s.db.QueryContext(ctx, kueri, userID)
	if err != nil {
		return nil, fmt.Errorf("sesi: membaca daftar sesi pengguna %s: %w", userID, err)
	}
	defer baris.Close()

	daftar := make([]Session, 0)
	for baris.Next() {
		sesi, err := pindaiSession(baris)
		if err != nil {
			return nil, fmt.Errorf("sesi: membaca daftar sesi pengguna %s: %w", userID, err)
		}
		daftar = append(daftar, sesi)
	}
	if err := baris.Err(); err != nil {
		return nil, fmt.Errorf("sesi: membaca daftar sesi pengguna %s: %w", userID, err)
	}
	return daftar, nil
}

// Touch memperbarui `last_seen_at` sehingga sesi yang lama tidak dipakai dapat
// dikenali. Sesi yang tidak ada menghasilkan ErrNotFound; sesi yang sudah
// dicabut tetap boleh diperbarui karena barisnya masih ada.
func (s *Store) Touch(ctx context.Context, sessionID string) error {
	hasil, err := s.db.ExecContext(ctx, sqlPerbaruiTerakhirDilihat, formatWaktu(s.clk.Now().UTC()), sessionID)
	if err != nil {
		return fmt.Errorf("sesi: memperbarui last_seen_at sesi %s: %w", sessionID, err)
	}
	jumlah, err := hasil.RowsAffected()
	if err != nil {
		return fmt.Errorf("sesi: menghitung baris last_seen_at sesi %s: %w", sessionID, err)
	}
	if jumlah == 0 {
		return fmt.Errorf("sesi: sesi %s tidak ditemukan: %w", sessionID, ErrNotFound)
	}
	return nil
}

// barisPenyegar adalah satu baris sesi beserta hash penyegarnya, yang hanya
// dibutuhkan di dalam transaksi rotasi.
type barisPenyegar struct {
	sesi        Session
	refreshHash string
}

// pengambil adalah bagian koneksi yang dibutuhkan pembacaan satu baris; dipenuhi
// oleh *database.DB maupun *sql.Conn di dalam transaksi.
type pengambil interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// bacaSatu membaca tepat satu baris Session.
func (s *Store) bacaSatu(ctx context.Context, q pengambil, kueri string, args ...any) (Session, error) {
	sesi, err := pindaiSession(q.QueryRowContext(ctx, kueri, args...))
	if err != nil {
		return Session{}, err
	}
	return sesi, nil
}

// bacaPenyegar membaca satu baris sesi beserta hash penyegarnya. Baris yang tidak
// ada dikembalikan sebagai (nil, nil) supaya pemanggil dapat membedakan "tidak
// ada" dari kesalahan database.
func (s *Store) bacaPenyegar(ctx context.Context, q pengambil, kueri string, args ...any) (*barisPenyegar, error) {
	var refreshHash sql.NullString
	sesi, err := pindaiSessionDengan(q.QueryRowContext(ctx, kueri, args...), &refreshHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sesi: membaca baris penyegar: %w", err)
	}
	return &barisPenyegar{sesi: sesi, refreshHash: refreshHash.String}, nil
}

// putarDiTransaksi menerbitkan pasangan token baru untuk baris yang sudah dibaca
// di dalam transaksi, lalu mengganti hash lamanya. Penggantian dikunci pada
// `id` dan hash penyegar lama: bila baris sudah berubah oleh permintaan lain,
// nol baris terpengaruh dan rotasi ditolak tanpa mengubah apa pun.
func (s *Store) putarDiTransaksi(ctx context.Context, conn *sql.Conn, baris *barisPenyegar, now time.Time) (Pair, error) {
	accessToken, err := tokenBaru()
	if err != nil {
		return Pair{}, err
	}
	refreshToken, err := tokenBaru()
	if err != nil {
		return Pair{}, err
	}

	absolute := baris.sesi.AbsoluteExpiresAt
	accessExpires := now.Add(s.cfg.AccessTTL)
	if accessExpires.After(absolute) {
		accessExpires = absolute
	}

	hasil, err := conn.ExecContext(ctx, sqlPutarPenyegar,
		hashToken(accessToken), formatWaktu(accessExpires),
		hashToken(refreshToken), formatWaktu(absolute),
		baris.refreshHash,
		baris.sesi.ID, baris.refreshHash,
	)
	if err != nil {
		return Pair{}, fmt.Errorf("sesi: memutar penyegar sesi %s: %w", baris.sesi.ID, err)
	}
	jumlah, err := hasil.RowsAffected()
	if err != nil {
		return Pair{}, fmt.Errorf("sesi: menghitung baris rotasi sesi %s: %w", baris.sesi.ID, err)
	}
	if jumlah != 1 {
		return Pair{}, fmt.Errorf("sesi: rotasi penyegar sesi %s ditolak: %w", baris.sesi.ID, ErrRefreshInvalid)
	}

	// Hash penyegar yang baru saja digantikan disimpan sebagai riwayat supaya
	// pemakaian ulangnya masih dapat dikenali setelah generasi berikutnya
	// berputar (D-033). Kegagalan menulis riwayat tidak menggagalkan rotasi
	// yang sudah terjadi; efeknya hanya deteksi yang lebih sempit.
	if _, err := conn.ExecContext(ctx, sqlSisipRiwayatPenyegar,
		baris.sesi.ID, baris.refreshHash, formatWaktu(now)); err != nil {
		return Pair{}, fmt.Errorf("sesi: mencatat riwayat penyegar sesi %s: %w", baris.sesi.ID, err)
	}

	return Pair{
		SessionID:         baris.sesi.ID,
		AccessToken:       accessToken,
		RefreshToken:      refreshToken,
		AccessExpiresAt:   accessExpires,
		RefreshExpiresAt:  absolute,
		AbsoluteExpiresAt: absolute,
	}, nil
}

// cabutDiTransaksi mencabut satu sesi di dalam transaksi yang sedang berjalan.
func (s *Store) cabutDiTransaksi(ctx context.Context, conn *sql.Conn, sessionID string, now time.Time, reason string) error {
	if _, err := conn.ExecContext(ctx, sqlCabutSesi, formatWaktu(now), reason, sessionID); err != nil {
		return fmt.Errorf("sesi: mencabut sesi %s: %w", sessionID, err)
	}
	return nil
}

// cabutSemuaDiTransaksi mencabut seluruh sesi aktif satu pengguna di dalam
// transaksi yang sedang berjalan. Inilah cakupan yang dituntut kontrak §5.1
// ketika pemakaian ulang penyegar terdeteksi: seluruh sesi pengguna itu, bukan
// hanya sesi yang tokennya diputar.
func (s *Store) cabutSemuaDiTransaksi(ctx context.Context, conn *sql.Conn, userID string, now time.Time, reason string) error {
	if _, err := conn.ExecContext(ctx, sqlCabutSemuaSesiPengguna, formatWaktu(now), reason, userID); err != nil {
		return fmt.Errorf("sesi: mencabut seluruh sesi pengguna %s: %w", userID, err)
	}
	return nil
}

// cariRiwayatPenyegar mengembalikan id sesi yang pernah memakai hash penyegar
// tersebut, atau string kosong bila hash tidak ada di riwayat.
func (s *Store) cariRiwayatPenyegar(ctx context.Context, conn *sql.Conn, refreshHash string) (string, error) {
	var sesiID string
	err := conn.QueryRowContext(ctx, sqlCariRiwayatPenyegar, refreshHash).Scan(&sesiID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("sesi: membaca riwayat penyegar: %w", err)
	}
	return sesiID, nil
}

// bacaSesiLewatID membaca satu sesi tanpa hash penyegarnya. Nilai (nil, nil)
// berarti sesi tidak ada.
func bacaSesiLewatID(ctx context.Context, conn *sql.Conn, sessionID string) (*Session, error) {
	sesi, err := pindaiSession(conn.QueryRowContext(ctx, sqlCariSesiLewatID, sessionID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sesi: membaca sesi %s: %w", sessionID, err)
	}
	return &sesi, nil
}

// dalamTransaksiImmediate menjalankan fn di dalam satu transaksi
// `BEGIN IMMEDIATE`.
//
// Transaksi eksplisit dipakai — bukan database/sql Tx — karena kunci tulis harus
// diambil di awal transaksi: dengan begitu dua penyegaran bersamaan tidak dapat
// sama-sama membaca baris yang sama lalu sama-sama menulis. Koneksi selalu
// dikembalikan ke pool dalam keadaan bersih: ROLLBACK dijalankan bila COMMIT
// tidak pernah terjadi.
func (s *Store) dalamTransaksiImmediate(ctx context.Context, fn func(conn *sql.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sesi: mengambil koneksi: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("sesi: memulai transaksi: %w", err)
	}

	selesai := false
	defer func() {
		if selesai {
			return
		}
		// Konteks pemanggil mungkin sudah dibatalkan; pembersihan tetap
		// dijalankan agar kunci tulis tidak tertinggal pada koneksi berikutnya.
		// Bila ROLLBACK sendiri gagal, tidak ada lagi yang dapat dilakukan:
		// COMMIT-nya memang tidak pernah terjadi.
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
	}()

	if err := fn(conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("sesi: menyimpan transaksi: %w", err)
	}
	selesai = true
	return nil
}

// pindaiSession membaca satu baris kolomSession.
func pindaiSession(baris interface{ Scan(...any) error }) (Session, error) {
	return pindaiSessionDengan(baris, nil)
}

// pindaiSessionDengan membaca satu baris kolomSession dan, bila tambahan tidak
// nil, satu kolom hash penyegar sesudahnya.
func pindaiSessionDengan(baris interface{ Scan(...any) error }, tambahan *sql.NullString) (Session, error) {
	var (
		id, userID, createdAt, accessExpires, absoluteExpires string
		revokedAt, revokedReason, lastSeenAt, userAgent       sql.NullString
	)

	tujuan := []any{
		&id, &userID, &createdAt, &accessExpires, &absoluteExpires,
		&revokedAt, &revokedReason, &lastSeenAt, &userAgent,
	}
	if tambahan != nil {
		tujuan = append(tujuan, tambahan)
	}
	if err := baris.Scan(tujuan...); err != nil {
		return Session{}, err
	}

	sesi := Session{
		ID:               id,
		UserID:           userID,
		RevokedReason:    revokedReason.String,
		UserAgentRingkas: userAgent.String,
	}

	var err error
	if sesi.CreatedAt, err = parseWaktu(createdAt); err != nil {
		return Session{}, err
	}
	if sesi.AccessExpiresAt, err = parseWaktu(accessExpires); err != nil {
		return Session{}, err
	}
	if sesi.AbsoluteExpiresAt, err = parseWaktu(absoluteExpires); err != nil {
		return Session{}, err
	}
	if revokedAt.Valid {
		waktu, err := parseWaktu(revokedAt.String)
		if err != nil {
			return Session{}, err
		}
		sesi.RevokedAt = &waktu
	}
	if lastSeenAt.Valid {
		waktu, err := parseWaktu(lastSeenAt.String)
		if err != nil {
			return Session{}, err
		}
		sesi.LastSeenAt = &waktu
	}
	return sesi, nil
}
