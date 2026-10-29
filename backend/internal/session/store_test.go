package session

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

// waktuMulai adalah titik awal jam uji: 16 Oktober 2026 09:00 UTC, tanggal yang
// sama dengan berkas keputusan FL-011.
var waktuMulai = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

const agenUji = "Expo/1.0 (Android 14)"

// bukaStore menyiapkan database sementara yang benar-benar memakai migrasi
// proyek, lalu mengembalikan Store dengan jam palsu.
func bukaStore(t *testing.T, cfg Config) (*Store, *database.DB, *clock.Fake) {
	t.Helper()
	ctx := context.Background()

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "facelink.db"))
	if err != nil {
		t.Fatalf("membuka database uji gagal: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("menutup database uji gagal: %v", err)
		}
	})

	if _, err := migrations.Run(ctx, db, dir); err != nil {
		t.Fatalf("menerapkan migrasi proyek gagal: %v", err)
	}

	jam := clock.NewFake(waktuMulai)
	return NewStore(db, jam, cfg), db, jam
}

// siapkanPengguna menulis satu baris users agar foreign key session_tokens
// terpenuhi. Akun dibuat `active` karena sesi tidak pernah terbit untuk akun
// yang belum aktif pada jalur nyata.
func siapkanPengguna(t *testing.T, db *database.DB, id string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO users (id, email, display_name, role, status)
		 VALUES (?, ?, ?, 'mahasiswa', 'active')`,
		id, id+"@kampus.example", "Mahasiswa "+id); err != nil {
		t.Fatalf("menyiapkan pengguna %s gagal: %v", id, err)
	}
}

func terbitkan(t *testing.T, store *Store, userID string) Pair {
	t.Helper()
	pair, err := store.Issue(context.Background(), userID, agenUji)
	if err != nil {
		t.Fatalf("Issue untuk %s gagal: %v", userID, err)
	}
	return pair
}

func harusBerhasil(t *testing.T, err error, konteks string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: galat tak terduga: %v", konteks, err)
	}
}

func harusGalat(t *testing.T, err error, ingin error) {
	t.Helper()
	if !errors.Is(err, ingin) {
		t.Fatalf("galat = %v, ingin errors.Is(galat, %v)", err, ingin)
	}
}

// bacaPencabutan membaca kolom pencabutan langsung dari tabel, bukan lewat Store,
// supaya uji membuktikan keadaan database dan bukan keadaan hasil pemetaan.
func bacaPencabutan(t *testing.T, db *database.DB, sessionID string) (sql.NullString, sql.NullString) {
	t.Helper()
	var revokedAt, reason sql.NullString
	if err := db.QueryRowContext(context.Background(),
		`SELECT revoked_at, revoked_reason FROM session_tokens WHERE id = ?`,
		sessionID).Scan(&revokedAt, &reason); err != nil {
		t.Fatalf("membaca pencabutan sesi %s gagal: %v", sessionID, err)
	}
	return revokedAt, reason
}

func hitungSesi(t *testing.T, db *database.DB, userID string) int {
	t.Helper()
	var jumlah int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM session_tokens WHERE user_id = ?`, userID).Scan(&jumlah); err != nil {
		t.Fatalf("menghitung sesi %s gagal: %v", userID, err)
	}
	return jumlah
}

func hashSHA256(nilai string) string {
	ringkas := sha256.Sum256([]byte(nilai))
	return hex.EncodeToString(ringkas[:])
}

func TestIssueLaluAuthenticate(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")
	if pair.SessionID == "" {
		t.Error("SessionID kosong")
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("token yang diterbitkan kosong")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Error("token akses dan token penyegar identik")
	}

	now := jam.Now()
	if ingin := now.Add(AccessTTLBawaan); !pair.AccessExpiresAt.Equal(ingin) {
		t.Errorf("AccessExpiresAt = %v, ingin %v", pair.AccessExpiresAt, ingin)
	}
	if ingin := now.Add(AbsoluteTTLBawaan); !pair.AbsoluteExpiresAt.Equal(ingin) {
		t.Errorf("AbsoluteExpiresAt = %v, ingin %v", pair.AbsoluteExpiresAt, ingin)
	}
	if !pair.RefreshExpiresAt.Equal(pair.AbsoluteExpiresAt) {
		t.Errorf("RefreshExpiresAt = %v, ingin sama dengan batas absolut %v",
			pair.RefreshExpiresAt, pair.AbsoluteExpiresAt)
	}

	sesi, err := store.Authenticate(ctx, pair.AccessToken)
	harusBerhasil(t, err, "Authenticate token akses yang baru diterbitkan")
	if sesi.ID != pair.SessionID {
		t.Errorf("ID sesi = %q, ingin %q", sesi.ID, pair.SessionID)
	}
	if sesi.UserID != "u-1" {
		t.Errorf("UserID = %q, ingin u-1", sesi.UserID)
	}
	if sesi.UserAgentRingkas != agenUji {
		t.Errorf("UserAgentRingkas = %q, ingin %q", sesi.UserAgentRingkas, agenUji)
	}
	if !sesi.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, ingin %v", sesi.CreatedAt, now)
	}
	if !sesi.AccessExpiresAt.Equal(pair.AccessExpiresAt) {
		t.Errorf("AccessExpiresAt tersimpan = %v, ingin %v", sesi.AccessExpiresAt, pair.AccessExpiresAt)
	}
	if sesi.RevokedAt != nil || sesi.RevokedReason != "" {
		t.Errorf("sesi baru sudah bertanda tercabut: %v / %q", sesi.RevokedAt, sesi.RevokedReason)
	}
	if sesi.LastSeenAt != nil {
		t.Errorf("LastSeenAt sesi baru = %v, ingin kosong", sesi.LastSeenAt)
	}

	// Token asing, token yang diubah satu karakter, dan token penyegar tidak
	// boleh berlaku sebagai token akses.
	for nama, token := range map[string]string{
		"token asing":           "token-asing-yang-tidak-pernah-diterbitkan",
		"token penyegar":        pair.RefreshToken,
		"token akses diubah":    pair.AccessToken + "A",
		"token berakhir kosong": "",
	} {
		if _, err := store.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("Authenticate(%s): galat = %v, ingin ErrUnauthenticated", nama, err)
		}
	}
}

func TestAuthenticateTokenAksesKedaluwarsa(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")

	// Beberapa detik sebelum kedaluwarsa token akses masih berlaku.
	jam.Advance(59*time.Minute + 59*time.Second)
	if _, err := store.Authenticate(ctx, pair.AccessToken); err != nil {
		t.Fatalf("Authenticate sesaat sebelum kedaluwarsa gagal: %v", err)
	}

	// Tepat pada access_expires_at token sudah tidak berlaku lagi.
	jam.Advance(time.Second)
	_, err := store.Authenticate(ctx, pair.AccessToken)
	harusGalat(t, err, ErrAccessExpired)

	// Dua menit kemudian tetap kedaluwarsa, bukan galat lain.
	jam.Advance(2 * time.Minute)
	_, err = store.Authenticate(ctx, pair.AccessToken)
	harusGalat(t, err, ErrAccessExpired)
}

func TestAuthenticateTokenAksesKedaluwarsaSetelahSatuJam(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")
	jam.Advance(61 * time.Minute)

	_, err := store.Authenticate(ctx, pair.AccessToken)
	harusGalat(t, err, ErrAccessExpired)
}

func TestRefreshMemutarKeduaToken(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")
	jam.Advance(5 * time.Minute)

	baru, err := store.Refresh(ctx, pair.RefreshToken)
	harusBerhasil(t, err, "Refresh dengan token penyegar yang sah")

	if baru.SessionID != pair.SessionID {
		t.Errorf("SessionID setelah rotasi = %q, ingin tetap %q", baru.SessionID, pair.SessionID)
	}
	if baru.AccessToken == pair.AccessToken {
		t.Error("token akses tidak berputar")
	}
	if baru.RefreshToken == pair.RefreshToken {
		t.Error("token penyegar tidak berputar")
	}
	if ingin := jam.Now().Add(AccessTTLBawaan); !baru.AccessExpiresAt.Equal(ingin) {
		t.Errorf("AccessExpiresAt = %v, ingin %v", baru.AccessExpiresAt, ingin)
	}
	if !baru.AbsoluteExpiresAt.Equal(pair.AbsoluteExpiresAt) {
		t.Errorf("AbsoluteExpiresAt = %v, ingin tidak berubah dari %v",
			baru.AbsoluteExpiresAt, pair.AbsoluteExpiresAt)
	}
	if !baru.RefreshExpiresAt.Equal(pair.AbsoluteExpiresAt) {
		t.Errorf("RefreshExpiresAt = %v, ingin tetap sama dengan batas absolut %v",
			baru.RefreshExpiresAt, pair.AbsoluteExpiresAt)
	}

	sesi, err := store.Authenticate(ctx, baru.AccessToken)
	harusBerhasil(t, err, "Authenticate dengan token akses hasil rotasi")
	if sesi.ID != pair.SessionID {
		t.Errorf("ID sesi token akses baru = %q, ingin %q", sesi.ID, pair.SessionID)
	}

	// Token akses lama harus mati karena hashnya sudah diganti.
	_, err = store.Authenticate(ctx, pair.AccessToken)
	harusGalat(t, err, ErrUnauthenticated)

	// Hanya satu baris sesi: rotasi tidak menerbitkan sesi kedua.
	if jumlah := hitungSesi(t, db, "u-1"); jumlah != 1 {
		t.Errorf("jumlah baris sesi = %d, ingin 1", jumlah)
	}
}

func TestRefreshPemakaianUlangMencabutSesi(t *testing.T) {
	store, db, _ := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")
	baru, err := store.Refresh(ctx, pair.RefreshToken)
	harusBerhasil(t, err, "Refresh pertama")

	// Token penyegar lama dipakai lagi setelah rotasi.
	_, err = store.Refresh(ctx, pair.RefreshToken)
	harusGalat(t, err, ErrRefreshReused)

	revokedAt, reason := bacaPencabutan(t, db, pair.SessionID)
	if !revokedAt.Valid {
		t.Fatal("sesi tidak dicabut setelah pemakaian ulang penyegar")
	}
	if reason.String != alasanRefreshReuse {
		t.Errorf("revoked_reason = %q, ingin %q", reason.String, alasanRefreshReuse)
	}

	// Sesi yang tercabut tidak dapat dipakai lagi lewat token mana pun.
	_, err = store.Authenticate(ctx, baru.AccessToken)
	harusGalat(t, err, ErrSessionRevoked)
	_, err = store.Refresh(ctx, baru.RefreshToken)
	harusGalat(t, err, ErrRefreshInvalid)

	if jumlah := hitungSesi(t, db, "u-1"); jumlah != 1 {
		t.Errorf("jumlah baris sesi = %d, ingin 1", jumlah)
	}
}

func TestBatasAbsolutTidakDiperpanjang(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")
	batas := pair.AbsoluteExpiresAt

	// Hari ke-29: penyegaran masih boleh dan batas tidak bergerak.
	jam.Advance(29 * 24 * time.Hour)
	kedua, err := store.Refresh(ctx, pair.RefreshToken)
	harusBerhasil(t, err, "Refresh pada hari ke-29")
	if !kedua.AbsoluteExpiresAt.Equal(batas) {
		t.Errorf("batas absolut bergerak menjadi %v, ingin tetap %v", kedua.AbsoluteExpiresAt, batas)
	}
	if !kedua.RefreshExpiresAt.Equal(batas) {
		t.Errorf("refresh_expires_at = %v, ingin tetap %v", kedua.RefreshExpiresAt, batas)
	}

	// Sesi terus dipakai: rotasi lagi 12 jam kemudian.
	jam.Advance(12 * time.Hour)
	ketiga, err := store.Refresh(ctx, kedua.RefreshToken)
	harusBerhasil(t, err, "Refresh pada hari ke-29,5")
	if !ketiga.AbsoluteExpiresAt.Equal(batas) || !ketiga.RefreshExpiresAt.Equal(batas) {
		t.Errorf("batas sesi bergerak setelah dipakai berulang: %v / %v, ingin %v",
			ketiga.AbsoluteExpiresAt, ketiga.RefreshExpiresAt, batas)
	}
	if ketiga.AccessExpiresAt.After(batas) {
		t.Errorf("token akses berlaku sampai %v, melewati batas absolut %v", ketiga.AccessExpiresAt, batas)
	}

	// Melewati 30 hari sejak sesi dibuat: penyegaran gagal walaupun sesi sering
	// dipakai, karena batas absolut tidak pernah diperpanjang.
	jam.Advance(24 * time.Hour)
	_, err = store.Refresh(ctx, ketiga.RefreshToken)
	harusGalat(t, err, ErrRefreshInvalid)

	revokedAt, reason := bacaPencabutan(t, db, pair.SessionID)
	if !revokedAt.Valid {
		t.Fatal("sesi lewat batas absolut tidak ditandai tercabut")
	}
	if reason.String != alasanRefreshExpired {
		t.Errorf("revoked_reason = %q, ingin %q", reason.String, alasanRefreshExpired)
	}

	_, err = store.Authenticate(ctx, ketiga.AccessToken)
	harusGalat(t, err, ErrSessionRevoked)
}

func TestRefreshTokenTidakDikenal(t *testing.T) {
	store, db, _ := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")

	_, err := store.Refresh(ctx, "penyegar-asing-yang-tidak-pernah-diterbitkan")
	harusGalat(t, err, ErrRefreshInvalid)

	// Token asing tidak boleh mencabut sesi mana pun.
	daftar, err := store.ListForUser(ctx, "u-1")
	harusBerhasil(t, err, "ListForUser setelah penyegar asing")
	if len(daftar) != 1 || daftar[0].RevokedAt != nil {
		t.Errorf("sesi tercabut oleh token penyegar asing: %+v", daftar)
	}
	if _, err := store.Authenticate(ctx, pair.AccessToken); err != nil {
		t.Errorf("token akses tidak lagi berlaku setelah penyegar asing: %v", err)
	}
}

func TestRevokeIdempotenDanRevokeAllForUser(t *testing.T) {
	store, db, _ := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	siapkanPengguna(t, db, "u-2")
	ctx := context.Background()

	pertama := terbitkan(t, store, "u-1")
	kedua := terbitkan(t, store, "u-1")
	orangLain := terbitkan(t, store, "u-2")

	harusBerhasil(t, store.Revoke(ctx, pertama.SessionID, "logout"), "Revoke sesi pertama")
	_, err := store.Authenticate(ctx, pertama.AccessToken)
	harusGalat(t, err, ErrSessionRevoked)

	// Kontrak §5.1: mencabut sesi yang sudah tercabut mengembalikan 404, dan
	// catatan pencabutan pertama tidak pernah ditimpa.
	harusGalat(t, store.Revoke(ctx, pertama.SessionID, "alasan-lain"), ErrNotFound)
	revokedAt, reason := bacaPencabutan(t, db, pertama.SessionID)
	if !revokedAt.Valid || reason.String != "logout" {
		t.Errorf("pencabutan ulang menimpa catatan pertama: %v / %q", revokedAt, reason.String)
	}

	// Jalur logout kredensial tetap idempoten setelah pencabutan admin.
	harusBerhasil(t, store.Logout(ctx, pertama.RefreshToken, ""), "Logout sesi yang sudah tercabut")
	if _, err := store.Authenticate(ctx, orangLain.AccessToken); err != nil {
		t.Errorf("sesi pengguna lain ikut tercabut: %v", err)
	}

	// Sesi tak dikenal ditolak dengan jelas.
	harusGalat(t, store.Revoke(ctx, "sesi-tidak-ada", "logout"), ErrNotFound)

	jumlah, err := store.RevokeAllForUser(ctx, "u-1", "account_suspended")
	harusBerhasil(t, err, "RevokeAllForUser")
	if jumlah != 1 {
		t.Errorf("jumlah sesi tercabut = %d, ingin 1 (sesi pertama sudah tercabut)", jumlah)
	}
	jumlah, err = store.RevokeAllForUser(ctx, "u-1", "account_suspended")
	harusBerhasil(t, err, "RevokeAllForUser ulang")
	if jumlah != 0 {
		t.Errorf("jumlah sesi tercabut pada pemanggilan kedua = %d, ingin 0", jumlah)
	}
	jumlah, err = store.RevokeAllForUser(ctx, "u-tidak-ada", "account_suspended")
	harusBerhasil(t, err, "RevokeAllForUser pengguna tanpa sesi")
	if jumlah != 0 {
		t.Errorf("jumlah sesi tercabut pengguna tanpa sesi = %d, ingin 0", jumlah)
	}

	// Seluruh sesi pengguna tercabut; sesi pengguna lain tidak tersentuh.
	if _, err := store.Authenticate(ctx, kedua.AccessToken); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("sesi kedua: galat = %v, ingin ErrSessionRevoked", err)
	}
	if _, err := store.Authenticate(ctx, orangLain.AccessToken); err != nil {
		t.Errorf("sesi pengguna lain ikut tercabut: %v", err)
	}

	daftar, err := store.ListForUser(ctx, "u-1")
	harusBerhasil(t, err, "ListForUser")
	if len(daftar) != 2 {
		t.Fatalf("jumlah sesi u-1 = %d, ingin 2", len(daftar))
	}
	for _, sesi := range daftar {
		if sesi.RevokedAt == nil || sesi.RevokedReason == "" {
			t.Errorf("sesi %s belum bertanda tercabut: %+v", sesi.ID, sesi)
		}
	}
}

func TestRefreshKonkurenHanyaSatuYangBerhasil(t *testing.T) {
	store, db, _ := bukaStore(t, Config{})
	ctx := context.Background()

	for putaran := range 3 {
		userID := "u-konkuren-" + string(rune('a'+putaran))
		siapkanPengguna(t, db, userID)
		pair := terbitkan(t, store, userID)

		const percobaan = 2
		var (
			wg    sync.WaitGroup
			mulai = make(chan struct{})
			hasil = make([]error, percobaan)
			token = make([]Pair, percobaan)
		)
		for i := range percobaan {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-mulai
				token[i], hasil[i] = store.Refresh(ctx, pair.RefreshToken)
			}(i)
		}
		close(mulai)
		wg.Wait()

		sukses, gagal := 0, 0
		var pemenang Pair
		for i, err := range hasil {
			if err == nil {
				sukses++
				pemenang = token[i]
				continue
			}
			gagal++
			if !errors.Is(err, ErrRefreshReused) {
				t.Errorf("putaran %d: galat penyegar bersamaan = %v, ingin ErrRefreshReused", putaran, err)
			}
		}
		if sukses != 1 || gagal != percobaan-1 {
			t.Fatalf("putaran %d: sukses = %d, gagal = %d, ingin tepat satu yang berhasil", putaran, sukses, gagal)
		}

		if jumlah := hitungSesi(t, db, userID); jumlah != 1 {
			t.Fatalf("putaran %d: jumlah sesi = %d, ingin 1 (rotasi tidak boleh melahirkan sesi kedua)", putaran, jumlah)
		}
		revokedAt, reason := bacaPencabutan(t, db, pair.SessionID)
		if !revokedAt.Valid {
			t.Fatalf("putaran %d: sesi tidak tercabut setelah pemakaian ulang bersamaan", putaran)
		}
		if reason.String != alasanRefreshReuse {
			t.Errorf("putaran %d: revoked_reason = %q, ingin %q", putaran, reason.String, alasanRefreshReuse)
		}

		// Respons sukses yang terbit bersamaan bukan sesi yang tetap aman:
		// replay harus mematikan kedua token pemenang juga.
		_, err := store.Authenticate(ctx, pemenang.AccessToken)
		harusGalat(t, err, ErrSessionRevoked)
		_, err = store.Refresh(ctx, pemenang.RefreshToken)
		harusGalat(t, err, ErrRefreshInvalid)
	}
}

func TestTokenHanyaTersimpanSebagaiHash(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")

	akses, penyegar, sebelumnya := bacaHash(t, db, pair.SessionID)
	if akses == pair.AccessToken || penyegar == pair.RefreshToken {
		t.Fatal("token mentah tersimpan di kolom hash")
	}
	if akses != hashSHA256(pair.AccessToken) {
		t.Errorf("access_hash bukan SHA-256 token akses: %q", akses)
	}
	if penyegar != hashSHA256(pair.RefreshToken) {
		t.Errorf("refresh_hash bukan SHA-256 token penyegar: %q", penyegar)
	}
	if len(akses) != 64 || len(penyegar) != 64 {
		t.Errorf("panjang hash = %d/%d, ingin 64 heksadesimal", len(akses), len(penyegar))
	}
	if sebelumnya.Valid {
		t.Errorf("prev_refresh_hash sesi baru = %q, ingin kosong", sebelumnya.String)
	}

	jam.Advance(time.Minute)
	baru, err := store.Refresh(ctx, pair.RefreshToken)
	harusBerhasil(t, err, "Refresh untuk memeriksa jejak rotasi")

	akses, penyegar, sebelumnya = bacaHash(t, db, pair.SessionID)
	if akses != hashSHA256(baru.AccessToken) || penyegar != hashSHA256(baru.RefreshToken) {
		t.Error("hash setelah rotasi tidak cocok dengan token yang dikembalikan")
	}
	if !sebelumnya.Valid || sebelumnya.String != hashSHA256(pair.RefreshToken) {
		t.Errorf("prev_refresh_hash = %v, ingin SHA-256 penyegar sebelumnya", sebelumnya)
	}

	// Token mentah tidak boleh muncul di kolom mana pun, termasuk kolom lama.
	var mentah int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM session_tokens
		 WHERE access_hash IN (?, ?) OR refresh_hash IN (?, ?) OR prev_refresh_hash IN (?, ?)`,
		pair.AccessToken, pair.RefreshToken, pair.AccessToken, pair.RefreshToken,
		pair.AccessToken, pair.RefreshToken).Scan(&mentah); err != nil {
		t.Fatalf("memeriksa token mentah gagal: %v", err)
	}
	if mentah != 0 {
		t.Errorf("ditemukan %d baris yang menyimpan token mentah", mentah)
	}
}

// bacaHash membaca ketiga kolom hash satu sesi langsung dari tabel.
func bacaHash(t *testing.T, db *database.DB, sessionID string) (string, string, sql.NullString) {
	t.Helper()
	var akses, penyegar string
	var sebelumnya sql.NullString
	if err := db.QueryRowContext(context.Background(),
		`SELECT access_hash, refresh_hash, prev_refresh_hash FROM session_tokens WHERE id = ?`,
		sessionID).Scan(&akses, &penyegar, &sebelumnya); err != nil {
		t.Fatalf("membaca kolom hash sesi %s gagal: %v", sessionID, err)
	}
	return akses, penyegar, sebelumnya
}

func TestListForUserTerbaruDahulu(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	siapkanPengguna(t, db, "u-2")
	ctx := context.Background()

	pertama := terbitkan(t, store, "u-1")
	jam.Advance(time.Hour)
	kedua := terbitkan(t, store, "u-1")
	jam.Advance(time.Hour)
	ketiga := terbitkan(t, store, "u-1")
	terbitkan(t, store, "u-2")

	daftar, err := store.ListForUser(ctx, "u-1")
	harusBerhasil(t, err, "ListForUser")

	if len(daftar) != 3 {
		t.Fatalf("jumlah sesi = %d, ingin 3", len(daftar))
	}
	ingin := []string{ketiga.SessionID, kedua.SessionID, pertama.SessionID}
	for i, sesi := range daftar {
		if sesi.ID != ingin[i] {
			t.Errorf("sesi ke-%d = %q, ingin %q (terbaru dahulu)", i, sesi.ID, ingin[i])
		}
		if sesi.UserID != "u-1" {
			t.Errorf("sesi %s milik %q, ingin u-1", sesi.ID, sesi.UserID)
		}
		if sesi.CreatedAt.IsZero() || sesi.AbsoluteExpiresAt.IsZero() {
			t.Errorf("sesi %s tidak memuat waktu yang lengkap: %+v", sesi.ID, sesi)
		}
		if sesi.UserAgentRingkas != agenUji {
			t.Errorf("UserAgentRingkas sesi %s = %q, ingin %q", sesi.ID, sesi.UserAgentRingkas, agenUji)
		}
	}
	for i := range len(daftar) - 1 {
		if !daftar[i].CreatedAt.After(daftar[i+1].CreatedAt) {
			t.Errorf("urutan waktu tidak menurun: %v lalu %v", daftar[i].CreatedAt, daftar[i+1].CreatedAt)
		}
	}

	kosong, err := store.ListForUser(ctx, "u-tidak-ada")
	harusBerhasil(t, err, "ListForUser pengguna tanpa sesi")
	if len(kosong) != 0 {
		t.Errorf("daftar sesi pengguna tanpa sesi = %d baris, ingin 0", len(kosong))
	}
}

func TestTouchMemperbaruiLastSeen(t *testing.T) {
	store, db, jam := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")

	daftar, err := store.ListForUser(ctx, "u-1")
	harusBerhasil(t, err, "ListForUser sebelum Touch")
	if daftar[0].LastSeenAt != nil {
		t.Fatalf("LastSeenAt sebelum Touch = %v, ingin kosong", daftar[0].LastSeenAt)
	}

	jam.Advance(7 * time.Minute)
	harusBerhasil(t, store.Touch(ctx, pair.SessionID), "Touch")

	sesi, err := store.Authenticate(ctx, pair.AccessToken)
	harusBerhasil(t, err, "Authenticate setelah Touch")
	if sesi.LastSeenAt == nil {
		t.Fatal("LastSeenAt belum diperbarui")
	}
	if !sesi.LastSeenAt.Equal(jam.Now()) {
		t.Errorf("LastSeenAt = %v, ingin %v", sesi.LastSeenAt, jam.Now())
	}

	harusGalat(t, store.Touch(ctx, "sesi-tidak-ada"), ErrNotFound)
}

func TestKonfigurasiKhususMemangkasTokenAkses(t *testing.T) {
	store, db, jam := bukaStore(t, Config{AccessTTL: 48 * time.Hour, AbsoluteTTL: 24 * time.Hour})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()

	pair := terbitkan(t, store, "u-1")

	if ingin := jam.Now().Add(24 * time.Hour); !pair.AbsoluteExpiresAt.Equal(ingin) {
		t.Errorf("AbsoluteExpiresAt = %v, ingin %v", pair.AbsoluteExpiresAt, ingin)
	}
	if !pair.AccessExpiresAt.Equal(pair.AbsoluteExpiresAt) {
		t.Errorf("AccessExpiresAt = %v, ingin dipangkas ke batas absolut %v",
			pair.AccessExpiresAt, pair.AbsoluteExpiresAt)
	}

	// Melewati batas absolut: penyegaran gagal dan sesi ditandai tercabut.
	jam.Advance(25 * time.Hour)
	_, err := store.Refresh(ctx, pair.RefreshToken)
	harusGalat(t, err, ErrRefreshInvalid)

	revokedAt, reason := bacaPencabutan(t, db, pair.SessionID)
	if !revokedAt.Valid || reason.String != alasanRefreshExpired {
		t.Errorf("pencabutan setelah batas absolut = %v / %q", revokedAt, reason.String)
	}
}

func TestIssuePenggunaTidakDikenalDitolak(t *testing.T) {
	store, _, _ := bukaStore(t, Config{})

	if _, err := store.Issue(context.Background(), "u-tidak-ada", agenUji); err == nil {
		t.Fatal("Issue untuk pengguna yang tidak ada berhasil, ingin ditolak foreign key")
	}
}

func jumlahAuditLogout(t *testing.T, db *database.DB) int {
	t.Helper()
	var jumlah int
	if err := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.logout'`).Scan(&jumlah); err != nil {
		t.Fatal(err)
	}
	return jumlah
}

func TestLogoutCredentialsAndIsolation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rotations int
		token     string
		valid     bool
	}{
		{"current", 0, "current", true},
		{"previous", 1, "original", true},
		{"current_after_two_rotations", 2, "current", true},
		{"older_history", 2, "original", false},
		{"unknown", 0, "unknown", false},
		{"empty", 0, "", false},
		{"access_is_not_refresh", 0, "access", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, db, _ := bukaStore(t, Config{})
			siapkanPengguna(t, db, "u-1")
			siapkanPengguna(t, db, "u-2")
			ctx := context.Background()
			original := terbitkan(t, store, "u-1")
			current := original
			otherDevice := terbitkan(t, store, "u-1")
			otherUser := terbitkan(t, store, "u-2")
			for range tc.rotations {
				var err error
				current, err = store.Refresh(ctx, current.RefreshToken)
				harusBerhasil(t, err, "rotasi sebelum logout")
			}
			token := tc.token
			switch token {
			case "current":
				token = current.RefreshToken
			case "original":
				token = original.RefreshToken
			case "access":
				token = current.AccessToken
			}
			err := store.Logout(ctx, token, "req-logout")
			if tc.valid {
				harusBerhasil(t, err, "Logout")
				firstAt, firstReason := bacaPencabutan(t, db, original.SessionID)
				harusBerhasil(t, store.Logout(ctx, token, "req-repeat"), "Logout berulang")
				harusBerhasil(t, store.Logout(ctx, current.RefreshToken, ""), "Logout current setelah previous")
				at, reason := bacaPencabutan(t, db, original.SessionID)
				if !at.Valid || reason.String != "logout" || at != firstAt || reason != firstReason {
					t.Fatalf("catatan pencabutan berubah: %v/%v -> %v/%v", firstAt, firstReason, at, reason)
				}
				_, err = store.Authenticate(ctx, current.AccessToken)
				harusGalat(t, err, ErrSessionRevoked)
				_, err = store.Refresh(ctx, original.RefreshToken)
				harusGalat(t, err, ErrRefreshInvalid)
				_, err = store.Refresh(ctx, current.RefreshToken)
				harusGalat(t, err, ErrRefreshInvalid)
				if n := jumlahAuditLogout(t, db); n != 1 {
					t.Fatalf("audit logout = %d, ingin 1", n)
				}
				var actor, entity, requestID string
				harusBerhasil(t, db.QueryRowContext(ctx,
					`SELECT actor_id, entity_id, request_id FROM audit_events WHERE action = 'auth.logout'`).
					Scan(&actor, &entity, &requestID), "baca audit logout")
				if actor != "u-1" || entity != original.SessionID || requestID != "req-logout" {
					t.Fatalf("audit salah: actor=%s entity=%s request=%s", actor, entity, requestID)
				}
			} else {
				harusGalat(t, err, ErrRefreshInvalid)
				_, err = store.Authenticate(ctx, current.AccessToken)
				harusBerhasil(t, err, "sesi tetap aktif setelah kredensial salah")
				if n := jumlahAuditLogout(t, db); n != 0 {
					t.Fatalf("audit kredensial salah = %d", n)
				}
			}
			for _, pair := range []Pair{otherDevice, otherUser} {
				_, err := store.Authenticate(ctx, pair.AccessToken)
				harusBerhasil(t, err, "sesi lain tidak ikut dicabut")
				_, err = store.Refresh(ctx, pair.RefreshToken)
				harusBerhasil(t, err, "penyegar sesi lain masih berlaku")
			}
		})
	}
}

func TestLogoutAbsoluteExpiry(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "before_boundary"
		if expired {
			name = "at_boundary"
		}
		t.Run(name, func(t *testing.T) {
			store, db, jam := bukaStore(t, Config{})
			siapkanPengguna(t, db, "u-1")
			ctx := context.Background()
			pair := terbitkan(t, store, "u-1")
			advance := AbsoluteTTLBawaan
			if !expired {
				advance -= time.Millisecond
			}
			jam.Advance(advance)
			err := store.Logout(ctx, pair.RefreshToken, "")
			if expired {
				harusGalat(t, err, ErrRefreshInvalid)
				if n := jumlahAuditLogout(t, db); n != 0 {
					t.Fatalf("audit logout kedaluwarsa = %d", n)
				}
			} else {
				harusBerhasil(t, err, "Logout tepat sebelum batas")
				jam.Advance(time.Hour)
				harusBerhasil(t, store.Logout(ctx, pair.RefreshToken, ""), "logout tercabut setelah batas")
			}
			var absolute, refreshExpiry string
			harusBerhasil(t, db.QueryRowContext(ctx,
				`SELECT absolute_expires_at, refresh_expires_at FROM session_tokens WHERE id = ?`, pair.SessionID).
				Scan(&absolute, &refreshExpiry), "baca batas sesi")
			if absolute != formatWaktu(pair.AbsoluteExpiresAt) || refreshExpiry != absolute {
				t.Fatalf("batas sesi diperpanjang: absolute=%s refresh=%s", absolute, refreshExpiry)
			}
			_, err = store.Refresh(ctx, pair.RefreshToken)
			harusGalat(t, err, ErrRefreshInvalid)
		})
	}
}

// Jam pertama berhenti setelah BEGIN IMMEDIATE, bukan sebelum mengambil kunci.
type logoutLockClock struct {
	base    clock.Clock
	once    sync.Once
	locked  chan struct{}
	release chan struct{}
}

func (c *logoutLockClock) Now() time.Time {
	c.once.Do(func() {
		close(c.locked)
		<-c.release
	})
	return c.base.Now()
}

func TestLogoutRefreshSerializedBothOrderings(t *testing.T) {
	for _, refreshFirst := range []bool{true, false} {
		name := "logout_first"
		if refreshFirst {
			name = "refresh_first"
		}
		t.Run(name, func(t *testing.T) {
			store, db, jam := bukaStore(t, Config{})
			siapkanPengguna(t, db, "u-1")
			siapkanPengguna(t, db, "u-2")
			original := terbitkan(t, store, "u-1")
			otherDevice := terbitkan(t, store, "u-1")
			otherUser := terbitkan(t, store, "u-2")
			gate := &logoutLockClock{base: jam, locked: make(chan struct{}), release: make(chan struct{})}
			first := NewStore(db, gate, Config{})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			type result struct {
				pair Pair
				err  error
			}
			firstDone := make(chan result, 1)
			secondDone := make(chan result, 1)
			go func() {
				var r result
				if refreshFirst {
					r.pair, r.err = first.Refresh(ctx, original.RefreshToken)
				} else {
					r.err = first.Logout(ctx, original.RefreshToken, "race-first")
				}
				firstDone <- r
			}()
			<-gate.locked
			started := make(chan struct{})
			go func() {
				close(started)
				var r result
				if refreshFirst {
					r.err = store.Logout(ctx, original.RefreshToken, "race-second")
				} else {
					r.pair, r.err = store.Refresh(ctx, original.RefreshToken)
				}
				secondDone <- r
			}()
			<-started
			close(gate.release)
			a, b := <-firstDone, <-secondDone
			harusBerhasil(t, a.err, "operasi pertama")
			if refreshFirst {
				harusBerhasil(t, b.err, "logout previous setelah refresh")
				_, err := store.Refresh(ctx, a.pair.RefreshToken)
				harusGalat(t, err, ErrRefreshInvalid)
				_, err = store.Authenticate(ctx, a.pair.AccessToken)
				harusGalat(t, err, ErrSessionRevoked)
			} else {
				harusGalat(t, b.err, ErrRefreshInvalid)
			}
			_, err := store.Refresh(ctx, original.RefreshToken)
			harusGalat(t, err, ErrRefreshInvalid)
			if n := jumlahAuditLogout(t, db); n != 1 {
				t.Fatalf("audit race = %d, ingin 1", n)
			}
			for _, pair := range []Pair{otherDevice, otherUser} {
				_, err := store.Refresh(ctx, pair.RefreshToken)
				harusBerhasil(t, err, "race/replay tidak mencabut sesi lain")
			}
		})
	}
}

func TestLogoutConcurrentDuplicateAuditsOnce(t *testing.T) {
	store, db, _ := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	pair := terbitkan(t, store, "u-1")
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- store.Logout(context.Background(), pair.RefreshToken, "")
		}()
	}
	close(start)
	for range 2 {
		harusBerhasil(t, <-results, "logout konkuren")
	}
	if n := jumlahAuditLogout(t, db); n != 1 {
		t.Fatalf("audit logout konkuren = %d, ingin 1", n)
	}
}

func TestLogoutPersistsAcrossRestart(t *testing.T) {
	for _, logoutBeforeRestart := range []bool{false, true} {
		name := "previous_survives_restart"
		if logoutBeforeRestart {
			name = "revocation_survives_restart"
		}
		t.Run(name, func(t *testing.T) {
			store, db, jam := bukaStore(t, Config{})
			siapkanPengguna(t, db, "u-1")
			ctx := context.Background()
			pair := terbitkan(t, store, "u-1")
			rotated, err := store.Refresh(ctx, pair.RefreshToken)
			harusBerhasil(t, err, "Refresh sebelum restart")
			if logoutBeforeRestart {
				harusBerhasil(t, store.Logout(ctx, pair.RefreshToken, ""), "Logout sebelum restart")
			}
			path := db.Path()
			harusBerhasil(t, db.Close(), "tutup sebelum restart")
			reopened, err := database.Open(ctx, path)
			harusBerhasil(t, err, "buka ulang database")
			t.Cleanup(func() { _ = reopened.Close() })
			restarted := NewStore(reopened, jam, Config{})
			harusBerhasil(t, restarted.Logout(ctx, pair.RefreshToken, ""), "Logout previous setelah restart")
			_, err = restarted.Refresh(ctx, rotated.RefreshToken)
			harusGalat(t, err, ErrRefreshInvalid)
			if n := jumlahAuditLogout(t, reopened); n != 1 {
				t.Fatalf("audit setelah restart = %d, ingin 1", n)
			}
		})
	}
}

func TestLogoutAuditFailureRollsBackRevocation(t *testing.T) {
	store, db, _ := bukaStore(t, Config{})
	siapkanPengguna(t, db, "u-1")
	ctx := context.Background()
	pair := terbitkan(t, store, "u-1")
	_, err := db.ExecContext(ctx, `CREATE TRIGGER fail_logout_audit BEFORE INSERT ON audit_events
		WHEN NEW.action = 'auth.logout' BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END`)
	harusBerhasil(t, err, "pasang kegagalan audit")
	if err := store.Logout(ctx, pair.RefreshToken, ""); err == nil {
		t.Fatal("Logout berhasil padahal audit gagal")
	}
	at, _ := bacaPencabutan(t, db, pair.SessionID)
	if at.Valid || jumlahAuditLogout(t, db) != 0 {
		t.Fatal("pencabutan/audit parsial tersimpan")
	}
	_, err = store.Authenticate(ctx, pair.AccessToken)
	harusBerhasil(t, err, "sesi tetap aktif setelah rollback")
	_, err = db.ExecContext(ctx, `DROP TRIGGER fail_logout_audit`)
	harusBerhasil(t, err, "hapus kegagalan audit")
	harusBerhasil(t, store.Logout(ctx, pair.RefreshToken, ""), "Logout setelah audit pulih")
	if n := jumlahAuditLogout(t, db); n != 1 {
		t.Fatalf("audit setelah retry = %d, ingin 1", n)
	}
}

func TestLogoutPreviousRacesNextRefreshIsolation(t *testing.T) {
	for _, refreshFirst := range []bool{true, false} {
		name := "previous_logout_first"
		if refreshFirst {
			name = "next_refresh_first"
		}
		t.Run(name, func(t *testing.T) {
			store, db, jam := bukaStore(t, Config{})
			siapkanPengguna(t, db, "u-1")
			siapkanPengguna(t, db, "u-2")
			ctx := context.Background()
			original := terbitkan(t, store, "u-1")
			current, err := store.Refresh(ctx, original.RefreshToken)
			harusBerhasil(t, err, "rotasi awal")
			otherDevice := terbitkan(t, store, "u-1")
			otherUser := terbitkan(t, store, "u-2")
			gate := &logoutLockClock{base: jam, locked: make(chan struct{}), release: make(chan struct{})}
			first := NewStore(db, gate, Config{})
			refreshDone := make(chan error, 1)
			logoutDone := make(chan error, 1)
			refresh := func(s *Store) {
				_, err := s.Refresh(ctx, current.RefreshToken)
				refreshDone <- err
			}
			logout := func(s *Store) {
				logoutDone <- s.Logout(ctx, original.RefreshToken, "")
			}
			if refreshFirst {
				go refresh(first)
			} else {
				go logout(first)
			}
			<-gate.locked
			if refreshFirst {
				go logout(store)
			} else {
				go refresh(store)
			}
			close(gate.release)
			refreshErr, logoutErr := <-refreshDone, <-logoutDone
			if refreshFirst {
				harusBerhasil(t, refreshErr, "refresh mendahului logout previous")
				harusGalat(t, logoutErr, ErrRefreshInvalid)
				at, _ := bacaPencabutan(t, db, original.SessionID)
				if at.Valid || jumlahAuditLogout(t, db) != 0 {
					t.Fatal("logout token dua generasi mencabut sesi atau menulis audit")
				}
			} else {
				harusBerhasil(t, logoutErr, "logout previous mendahului refresh")
				harusGalat(t, refreshErr, ErrRefreshInvalid)
				if n := jumlahAuditLogout(t, db); n != 1 {
					t.Fatalf("audit logout previous = %d, ingin 1", n)
				}
			}
			for _, pair := range []Pair{otherDevice, otherUser} {
				_, err := store.Refresh(ctx, pair.RefreshToken)
				harusBerhasil(t, err, "race previous tidak mencabut sesi lain")
			}
		})
	}
}
