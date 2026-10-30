package httpapi

// Pengujian alur autentikasi FL-011 lewat HTTP.
//
// Seluruh skenario menembak NewRouter dengan httptest.NewRecorder, memakai
// basis data SQLite termigrasi yang sementara, jam palsu (tanpa tidur panjang),
// dan penyedia identitas Google tiruan: server JWKS lokal plus pembuat ID token
// RS256. Yang diperiksa adalah perilaku yang benar-benar terlihat klien — kode
// status, kode galat pada badan respons, keadaan tabel, dan isi penampung log.
//
// Berkas ini memuat harness bersama; uji otorisasi ada di rbac_test.go.

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/audit"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/authz"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/oidc"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/session"
)

// waktuMulaiUji adalah titik acuan jam palsu: 16 Oktober 2026 09:00 UTC, tanggal
// yang sama dengan berkas keputusan FL-011.
var waktuMulaiUji = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

const (
	// kidUjiOIDC adalah kid pada header ID token uji sekaligus kid pada JWKS.
	kidUjiOIDC = "k1"
	// klienIDUjiOIDC adalah audience yang diterima verifier uji.
	klienIDUjiOIDC = "client-uji.apps.googleusercontent.com"
	// issuerUjiOIDC adalah issuer yang diterima verifier uji.
	issuerUjiOIDC = "https://accounts.google.com"
)

var (
	kunciUjiSekali sync.Once
	kunciUjiUtama  *rsa.PrivateKey
	kunciUjiAsing  *rsa.PrivateKey
	galatKunciUji  error
)

// kunciOIDCUji menyiapkan dua pasang kunci RSA sekali saja untuk seluruh uji:
// kunci pertama menandatangani token yang sah, kunci kedua membuktikan tanda
// tangan dari kunci asing ditolak.
func kunciOIDCUji(t *testing.T) (*rsa.PrivateKey, *rsa.PrivateKey) {
	t.Helper()
	kunciUjiSekali.Do(func() {
		kunciUjiUtama, galatKunciUji = rsa.GenerateKey(rand.Reader, 2048)
		if galatKunciUji != nil {
			return
		}
		kunciUjiAsing, galatKunciUji = rsa.GenerateKey(rand.Reader, 2048)
	})
	if galatKunciUji != nil {
		t.Fatalf("membuat kunci RSA uji gagal: %v", galatKunciUji)
	}
	return kunciUjiUtama, kunciUjiAsing
}

// jwksUji menyajikan dokumen JWKS dari kunci publik uji pada jalur /jwks.
func jwksUji(kunci *rsa.PrivateKey) http.Handler {
	dokumen := map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"kid": kidUjiOIDC,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(kunci.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(kunci.PublicKey.E)).Bytes()),
	}}}
	isi, err := json.Marshal(dokumen)
	if err != nil {
		// Tidak mungkin: dokumen dibentuk dari nilai yang pasti dapat
		// diserialisasi. Panik di sini akan terlihat sebagai kegagalan harness.
		panic("menyusun dokumen JWKS uji gagal: " + err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(isi)
	})
	return mux
}

// harnessAuth adalah server lengkap beserta komponen FL-011 dan bantuannya.
type harnessAuth struct {
	t          *testing.T
	srv        Server
	router     http.Handler
	db         *database.DB
	clk        *clock.Fake
	logs       *syncBuffer
	kunci      *rsa.PrivateKey
	kunciAsing *rsa.PrivateKey
	clientID   string
	penghitung int
}

// newHarnessAuth menyiapkan basis data sementara termigrasi, penyedia identitas
// Google tiruan, dan router produksi.
func newHarnessAuth(t *testing.T) *harnessAuth {
	t.Helper()
	ctx := context.Background()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "facelink-uji.db"))
	if err != nil {
		t.Fatalf("database.Open() gagal: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}
	status, err := migrations.Run(ctx, db, dir)
	if err != nil {
		t.Fatalf("migrations.Run() gagal: %v", err)
	}

	clk := clock.NewFake(waktuMulaiUji)
	utama, asing := kunciOIDCUji(t)

	penerbit := httptest.NewServer(jwksUji(utama))
	t.Cleanup(penerbit.Close)

	verifier, err := oidc.NewVerifier(oidc.Config{
		Audiences: []string{klienIDUjiOIDC},
		Keys:      oidc.NewJWKSCache(penerbit.URL+"/jwks", 5*time.Minute, http.DefaultClient, clk.Now),
		Now:       clk.Now,
	})
	if err != nil {
		t.Fatalf("oidc.NewVerifier() gagal: %v", err)
	}

	logs := &syncBuffer{}
	s := Server{
		Config: ServerConfig{
			Version:        testVersion,
			ReadTimeout:    5 * time.Second,
			WriteTimeout:   5 * time.Second,
			IdleTimeout:    30 * time.Second,
			RequestTimeout: 10 * time.Second,
			MaxBodyBytes:   DefaultMaxBodyBytes,
		},
		DB:         db,
		Migrations: status,
		Logger:     slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Accounts:   auth.NewStore(db, clk),
		Sessions:   session.NewStore(db, clk, session.Config{}),
		Google:     verifier,
		Audit:      audit.NewWriter(db, clk),
		Scope:      authz.NewSQLDirectory(db),
		Clock:      clk,
	}

	return &harnessAuth{
		t:          t,
		srv:        s,
		router:     NewRouter(s),
		db:         db,
		clk:        clk,
		logs:       logs,
		kunci:      utama,
		kunciAsing: asing,
		clientID:   klienIDUjiOIDC,
	}
}

// responsUji adalah hasil satu permintaan HTTP beserta badan yang sudah diurai.
type responsUji struct {
	Status int
	Rec    *httptest.ResponseRecorder
	Body   map[string]any
}

// kirim menjalankan satu permintaan lewat router produksi. `badan` diserialisasi
// sebagai JSON bila tidak nil; `token` dipasang sebagai Bearer bila tidak kosong.
func (h *harnessAuth) kirim(method, jalur string, badan any, token string) responsUji {
	h.t.Helper()

	var isi []byte
	if badan != nil {
		var err error
		isi, err = json.Marshal(badan)
		if err != nil {
			h.t.Fatalf("menyusun badan permintaan gagal: %v", err)
		}
	}

	var pembaca *bytes.Reader
	if isi != nil {
		pembaca = bytes.NewReader(isi)
	}
	var req *http.Request
	if pembaca == nil {
		req = httptest.NewRequest(method, jalur, nil)
	} else {
		req = httptest.NewRequest(method, jalur, pembaca)
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	hasil := responsUji{Status: rec.Code, Rec: rec}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &hasil.Body)
	}
	return hasil
}

// galatKode mengambil kode galat stabil dari amplop galat kontrak §1.4.
func (h *harnessAuth) galatKode(r responsUji) string {
	h.t.Helper()
	amplop, ok := r.Body["error"].(map[string]any)
	if !ok {
		h.t.Fatalf("respons tidak memuat amplop galat: status %d badan %s", r.Status, r.Rec.Body.String())
	}
	kode, _ := amplop["code"].(string)
	if kode == "" {
		h.t.Fatalf("amplop galat tanpa kode: %s", r.Rec.Body.String())
	}
	return kode
}

// harapGalat memastikan status HTTP dan kode galat badan sesuai harapan.
func (h *harnessAuth) harapGalat(r responsUji, status int, kode string) {
	h.t.Helper()
	if r.Status != status {
		h.t.Fatalf("status = %d, ingin %d; badan: %s", r.Status, status, r.Rec.Body.String())
	}
	if got := h.galatKode(r); got != kode {
		h.t.Fatalf("kode galat = %q, ingin %q; badan: %s", got, kode, r.Rec.Body.String())
	}
}

// harapSukses memastikan status HTTP sesuai harapan lalu mengembalikan `data`.
// Respons sukses yang memuat amplop galat ditolak agar tidak lolos diam-diam.
func (h *harnessAuth) harapSukses(r responsUji, status int) map[string]any {
	h.t.Helper()
	if r.Status != status {
		h.t.Fatalf("status = %d, ingin %d; badan: %s", r.Status, status, r.Rec.Body.String())
	}
	if _, ada := r.Body["error"]; ada {
		h.t.Fatalf("respons sukses (%d) memuat amplop galat: %s", r.Status, r.Rec.Body.String())
	}
	data, ok := r.Body["data"].(map[string]any)
	if !ok {
		h.t.Fatalf("respons sukses tidak memuat objek data: %s", r.Rec.Body.String())
	}
	return data
}

// harapSuksesLarik sama dengan harapSukses tetapi untuk respons berdata larik.
func (h *harnessAuth) harapSuksesLarik(r responsUji, status int) []any {
	h.t.Helper()
	if r.Status != status {
		h.t.Fatalf("status = %d, ingin %d; badan: %s", r.Status, status, r.Rec.Body.String())
	}
	data, ok := r.Body["data"].([]any)
	if !ok {
		h.t.Fatalf("respons sukses tidak memuat larik data: %s", r.Rec.Body.String())
	}
	return data
}

// nonceBaru meminta nonce sekali pakai ke endpoint nonce, persis seperti yang
// dilakukan aplikasi klien sebelum memulai alur Google Sign-In. Nilai nonce
// hanya sah bila terdaftar di backend, sehingga harness tidak boleh mengarang
// nilainya sendiri (kontrak §5.1).
func (h *harnessAuth) nonceBaru() string {
	h.t.Helper()

	r := h.kirim(http.MethodPost, "/api/v1/auth/nonce", nil, "")
	data := h.harapSukses(r, http.StatusOK)
	nonce, _ := data["nonce"].(string)
	if nonce == "" {
		h.t.Fatalf("endpoint nonce tidak mengembalikan nilai nonce: %s", r.Rec.Body.String())
	}
	return nonce
}

// noncePalsu mengembalikan nonce yang tidak pernah didaftarkan ke backend;
// dipakai untuk menguji penolakan nonce asing.
func (h *harnessAuth) noncePalsu() string {
	h.penghitung++
	return "nonce-palsu-" + ids.New() + "-" + itoa(h.penghitung)
}

// tokenID menyusun ID token uji yang ditandatangani kunci sah.
func (h *harnessAuth) tokenID(klaim map[string]any) string {
	h.t.Helper()
	return h.tokenIDKunci(klaim, h.kunci)
}

// tokenIDKunci menyusun ID token uji dengan kunci penanda tangan tertentu.
// Klaim bernilai nil dihapus dari token supaya uji dapat memalsukan klaim yang
// hilang.
func (h *harnessAuth) tokenIDKunci(klaim map[string]any, kunci *rsa.PrivateKey) string {
	h.t.Helper()

	sekarang := h.clk.Now()
	penuh := map[string]any{
		"iss":            issuerUjiOIDC,
		"aud":            h.clientID,
		"sub":            "sub-uji",
		"email":          "orang@kampus.example",
		"email_verified": true,
		"nonce":          "nonce-uji",
		"iat":            sekarang.Add(-time.Minute).Unix(),
		"exp":            sekarang.Add(time.Hour).Unix(),
	}
	for k, v := range klaim {
		if v == nil {
			delete(penuh, k)
			continue
		}
		penuh[k] = v
	}

	header, err := json.Marshal(map[string]any{"alg": "RS256", "kid": kidUjiOIDC, "typ": "JWT"})
	if err != nil {
		h.t.Fatalf("menyusun header token gagal: %v", err)
	}
	payload, err := json.Marshal(penuh)
	if err != nil {
		h.t.Fatalf("menyusun klaim token gagal: %v", err)
	}

	tertanda := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	ringkas := sha256.Sum256([]byte(tertanda))
	tanda, err := rsa.SignPKCS1v15(rand.Reader, kunci, crypto.SHA256, ringkas[:])
	if err != nil {
		h.t.Fatalf("menandatangani token uji gagal: %v", err)
	}
	return tertanda + "." + base64.RawURLEncoding.EncodeToString(tanda)
}

// sesiUji adalah hasil login/penyegaran yang dipakai uji berikutnya.
type sesiUji struct {
	Access  string
	Refresh string
	SesiID  string
	UserID  string
	Role    string
	Status  string
}

// sesiDariSegarkan membaca respons `POST /auth/refresh`.
//
// Kontrak §5.1 hanya mengizinkan pasangan token pada respons itu — tidak ada
// objek `user` maupun `session`. Identitas sesi tidak berubah oleh rotasi, jadi
// nilai yang sudah diketahui dari sesi sebelumnya dipakai kembali; inilah yang
// dilihat aplikasi klien, yang memang sudah memegang identitas itu.
func (h *harnessAuth) sesiDariSegarkan(data map[string]any, sebelumnya sesiUji) sesiUji {
	h.t.Helper()

	akses, _ := data["access_token"].(string)
	segarkan, _ := data["refresh_token"].(string)
	if akses == "" || segarkan == "" {
		h.t.Fatalf("respons penyegaran tidak memuat access_token/refresh_token: %v", data)
	}
	if _, ada := data["user"]; ada {
		h.t.Errorf("respons penyegaran memuat objek user: %v", data)
	}
	if _, ada := data["session"]; ada {
		h.t.Errorf("respons penyegaran memuat objek session: %v", data)
	}

	hasil := sesiUji{Access: akses, Refresh: segarkan, SesiID: sebelumnya.SesiID}
	// Identitas pemanggil diverifikasi lewat endpoint yang memang menyediakannya.
	me := h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, akses), http.StatusOK)
	hasil.UserID, _ = me["id"].(string)
	hasil.Role, _ = me["role"].(string)
	hasil.Status, _ = me["status"].(string)
	return hasil
}

// hashUji menghitung SHA-256 heksadesimal sebuah token, bentuk yang sama
// dengan kolom hash pada `session_tokens`. Dipakai pengujian untuk membuktikan
// token mentah tidak pernah tersimpan.
func hashUji(token string) string {
	ringkas := sha256.Sum256([]byte(token))
	return hex.EncodeToString(ringkas[:])
}

// sesiDari membaca pasangan token dari respons login atau respons penyegaran.
//
// Respons penyegaran tidak lagi memuat objek `user` (kontrak §5.1: bentuk
// `user` tidak dikirim ulang), sehingga identitas pemanggil diambil dari
// `GET /me` memakai token akses yang baru — persis seperti yang dilakukan
// aplikasi klien.
func (h *harnessAuth) sesiDari(data map[string]any) sesiUji {
	h.t.Helper()

	akses, _ := data["access_token"].(string)
	segarkan, _ := data["refresh_token"].(string)
	if akses == "" || segarkan == "" {
		h.t.Fatalf("respons sesi tidak memuat access_token/refresh_token: %v", data)
	}
	sesi, ok := data["session"].(map[string]any)
	if !ok {
		h.t.Fatalf("respons sesi tidak memuat objek session: %v", data)
	}

	hasil := sesiUji{Access: akses, Refresh: segarkan}
	hasil.SesiID, _ = sesi["id"].(string)

	if pengguna, ada := data["user"].(map[string]any); ada {
		hasil.UserID, _ = pengguna["id"].(string)
		hasil.Role, _ = pengguna["role"].(string)
		hasil.Status, _ = pengguna["status"].(string)
	} else {
		me := h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, akses), http.StatusOK)
		hasil.UserID, _ = me["id"].(string)
		hasil.Role, _ = me["role"].(string)
		hasil.Status, _ = me["status"].(string)
	}
	if hasil.SesiID == "" || hasil.UserID == "" {
		h.t.Fatalf("respons sesi tidak memuat id sesi/pengguna: %v", data)
	}
	return hasil
}

// loginGoogle menjalankan alur login lengkap dengan nonce baru dan mengharapkan
// 200.
func (h *harnessAuth) loginGoogle(email, subjek string) sesiUji {
	h.t.Helper()
	return h.loginGoogleNonce(email, subjek, h.nonceBaru())
}

// loginGoogleNonce menjalankan alur login memakai nonce yang ditentukan uji.
func (h *harnessAuth) loginGoogleNonce(email, subjek, nonce string) sesiUji {
	h.t.Helper()
	token := h.tokenID(map[string]any{"sub": subjek, "email": email, "nonce": nonce})
	r := h.kirim(http.MethodPost, "/api/v1/auth/google",
		map[string]any{"id_token": token, "nonce": nonce}, "")
	return h.sesiDari(h.harapSukses(r, http.StatusOK))
}

// adminPertama membuat admin bootstrap lalu memasukkannya lewat alur Google
// sehingga akunnya aktif dan memiliki sesi yang dapat dipakai.
func (h *harnessAuth) adminPertama(email string) (auth.User, sesiUji) {
	h.t.Helper()

	user, err := h.srv.Accounts.CreateFirstAdmin(context.Background(), email, "Admin Pertama")
	if err != nil {
		h.t.Fatalf("CreateFirstAdmin() gagal: %v", err)
	}
	if user.Role != auth.RoleAdmin || user.Status != auth.StatusInvited {
		h.t.Fatalf("admin bootstrap = role %q status %q, ingin admin/invited", user.Role, user.Status)
	}

	sesi := h.loginGoogle(email, "sub-admin-"+user.ID)
	if sesi.Role != string(auth.RoleAdmin) || sesi.Status != "active" {
		h.t.Fatalf("sesi admin = role %q status %q, ingin admin/active", sesi.Role, sesi.Status)
	}
	if sesi.UserID != user.ID {
		h.t.Fatalf("sesi admin menunjuk pengguna %s, ingin %s", sesi.UserID, user.ID)
	}
	return user, sesi
}

// undangAkun mengundang akun lewat POST /users memakai token admin.
func (h *harnessAuth) undangAkun(token, email, nama, role string) map[string]any {
	h.t.Helper()
	r := h.kirim(http.MethodPost, "/api/v1/users",
		map[string]any{"email": email, "display_name": nama, "role": role}, token)
	return h.harapSukses(r, http.StatusCreated)
}

// undangDanMasuk mengundang satu akun lalu memasukkannya lewat Google.
func (h *harnessAuth) undangDanMasuk(tokenAdmin, email, nama, role string) (string, sesiUji) {
	h.t.Helper()

	data := h.undangAkun(tokenAdmin, email, nama, role)
	userID, _ := data["id"].(string)
	if userID == "" {
		h.t.Fatalf("respons undangan tanpa id: %v", data)
	}
	sesi := h.loginGoogle(email, "sub-"+userID)
	if sesi.UserID != userID {
		h.t.Fatalf("sesi menunjuk pengguna %s, ingin %s", sesi.UserID, userID)
	}
	return userID, sesi
}

// ---- akses basis data untuk menyiapkan keadaan yang belum punya endpoint ----

// execSQL menjalankan satu pernyataan tulis di luar jalur HTTP.
func (h *harnessAuth) execSQL(kueri string, args ...any) {
	h.t.Helper()
	if _, err := h.db.ExecContext(context.Background(), kueri, args...); err != nil {
		h.t.Fatalf("menjalankan SQL gagal (%s): %v", kueri, err)
	}
}

// hitung mengembalikan hasil COUNT(*) dari kueri.
func (h *harnessAuth) hitung(kueri string, args ...any) int {
	h.t.Helper()
	var jumlah int
	if err := h.db.QueryRowContext(context.Background(), kueri, args...).Scan(&jumlah); err != nil {
		h.t.Fatalf("menghitung baris gagal (%s): %v", kueri, err)
	}
	return jumlah
}

// teksKolom membaca satu kolom teks; NULL dibaca sebagai string kosong.
func (h *harnessAuth) teksKolom(kueri string, args ...any) string {
	h.t.Helper()
	var nilai sql.NullString
	if err := h.db.QueryRowContext(context.Background(), kueri, args...).Scan(&nilai); err != nil {
		h.t.Fatalf("membaca kolom gagal (%s): %v", kueri, err)
	}
	return nilai.String
}

// buatKelas menyisipkan baris kelas (belum ada endpoint pembuatnya).
func (h *harnessAuth) buatKelas(kode, judul string) string {
	h.t.Helper()
	id := ids.New()
	h.execSQL(`INSERT INTO classes (id, code, title) VALUES (?, ?, ?)`, id, kode, judul)
	return id
}

// buatPeserta menyisipkan identitas sintetis dengan bukti roster nonbiometrik.
func (h *harnessAuth) buatPeserta(kodeEksternal, nama string) string {
	h.t.Helper()
	id := ids.New()
	h.execSQL(`INSERT INTO people (id, external_code, display_name, identity_reference, identity_verified_at) VALUES (?, ?, ?, ?, ?)`,
		id, kodeEksternal, nama, "fixture-roster-"+kodeEksternal, audit.FormatWaktu(h.clk.Now()))
	return id
}

// tambahAnggotaKelas menautkan peserta ke kelas (dibuat oleh adminID).
func (h *harnessAuth) tambahAnggotaKelas(classID, personID, adminID string) {
	h.t.Helper()
	h.execSQL(`INSERT INTO class_memberships (class_id, person_id, created_by) VALUES (?, ?, ?)`,
		classID, personID, adminID)
}

// ---- skenario A: Google OIDC dan lifecycle akun ---------------------------

// TestLoginGoogleAkunUndanganMenjadiAktif membuktikan alur penuh: admin
// mengundang, akun undangan masuk lewat Google, identitasnya tertaut, dan
// statusnya berubah menjadi active (D-032).
func TestLoginGoogleAkunUndanganMenjadiAktif(t *testing.T) {
	h := newHarnessAuth(t)
	admin, sesiAdmin := h.adminPertama("admin@kampus.example")

	emailDosen := "dosen@kampus.example"
	data := h.undangAkun(sesiAdmin.Access, emailDosen, "Dosen Uji", "dosen")
	if data["role"] != "dosen" {
		t.Fatalf("role akun undangan = %v, ingin dosen", data["role"])
	}
	if data["status"] != "invited" {
		t.Fatalf("status akun undangan = %v, ingin invited", data["status"])
	}
	dosenID, _ := data["id"].(string)

	sesiDosen := h.loginGoogle(emailDosen, "sub-dosen-1")
	if sesiDosen.UserID != dosenID {
		t.Fatalf("sesi menunjuk pengguna %s, ingin %s", sesiDosen.UserID, dosenID)
	}
	if sesiDosen.Role != "dosen" {
		t.Fatalf("user.role pada respons login = %q, ingin dosen", sesiDosen.Role)
	}
	if sesiDosen.Status != "active" {
		t.Fatalf("user.status pada respons login = %q, ingin active", sesiDosen.Status)
	}

	if n := h.hitung(`SELECT COUNT(*) FROM user_identities WHERE user_id = ? AND provider = 'google' AND subject = ?`,
		dosenID, "sub-dosen-1"); n != 1 {
		t.Fatalf("baris user_identities tertaut = %d, ingin 1", n)
	}
	if status := h.teksKolom(`SELECT status FROM users WHERE id = ?`, dosenID); status != "active" {
		t.Fatalf("users.status di database = %q, ingin active", status)
	}
	if login := h.teksKolom(`SELECT last_login_at FROM users WHERE id = ?`, dosenID); login == "" {
		t.Fatal("last_login_at tidak tercatat setelah login pertama")
	}
	if admin.Role != auth.RoleAdmin {
		t.Fatalf("admin bootstrap = role %q, ingin admin", admin.Role)
	}
}

// TestLoginGoogleMenolakBidangWajibYangKosong menegaskan id_token dan nonce
// sama-sama wajib (kontrak §5.1).
func TestLoginGoogleMenolakBidangWajibYangKosong(t *testing.T) {
	h := newHarnessAuth(t)
	nonce := h.nonceBaru()
	token := h.tokenID(map[string]any{"email": "siapa@kampus.example", "nonce": nonce})

	kosong := []struct {
		nama  string
		badan map[string]any
	}{
		{"tanpa id_token", map[string]any{"nonce": nonce}},
		{"id_token kosong", map[string]any{"id_token": "   ", "nonce": nonce}},
		{"tanpa nonce", map[string]any{"id_token": token}},
		{"nonce kosong", map[string]any{"id_token": token, "nonce": "  "}},
	}
	for _, kasus := range kosong {
		t.Run(kasus.nama, func(t *testing.T) {
			r := h.kirim(http.MethodPost, "/api/v1/auth/google", kasus.badan, "")
			h.harapGalat(r, http.StatusBadRequest, codeMissingField)
		})
	}
}

// TestLoginGoogleMenolakBuktiIdentitasTidakSah menyapu setiap jalur penolakan
// verifikasi OIDC: tanda tangan, issuer, audience, masa berlaku, dan
// email_verified (kontrak §5.1: 401 IDENTITY_TOKEN_INVALID).
func TestLoginGoogleMenolakBuktiIdentitasTidakSah(t *testing.T) {
	kasus := []struct {
		nama    string
		email   string
		siapkan func(h *harnessAuth) (token, nonce string)
	}{
		{
			nama:  "tanda tangan dari kunci asing",
			email: "kunci.asing@kampus.example",
			siapkan: func(h *harnessAuth) (string, string) {
				nonce := h.nonceBaru()
				token := h.tokenIDKunci(map[string]any{
					"sub": "sub-asal", "email": "kunci.asing@kampus.example", "nonce": nonce,
				}, h.kunciAsing)
				return token, nonce
			},
		},
		{
			nama:  "issuer tidak dikenal",
			email: "issuer.salah@kampus.example",
			siapkan: func(h *harnessAuth) (string, string) {
				nonce := h.nonceBaru()
				token := h.tokenID(map[string]any{
					"iss": "https://accounts.evil.example",
					"sub": "sub-asal", "email": "issuer.salah@kampus.example", "nonce": nonce,
				})
				return token, nonce
			},
		},
		{
			nama:  "audience tidak dikenal",
			email: "audience.salah@kampus.example",
			siapkan: func(h *harnessAuth) (string, string) {
				nonce := h.nonceBaru()
				token := h.tokenID(map[string]any{
					"aud": "client-lain.apps.googleusercontent.com",
					"sub": "sub-asal", "email": "audience.salah@kampus.example", "nonce": nonce,
				})
				return token, nonce
			},
		},
		{
			nama:  "token kedaluwarsa setelah jam dimajukan",
			email: "kedaluwarsa@kampus.example",
			siapkan: func(h *harnessAuth) (string, string) {
				nonce := h.nonceBaru()
				sekarang := h.clk.Now()
				token := h.tokenID(map[string]any{
					"sub": "sub-asal", "email": "kedaluwarsa@kampus.example", "nonce": nonce,
					"iat": sekarang.Add(-time.Minute).Unix(),
					"exp": sekarang.Add(30 * time.Minute).Unix(),
				})
				h.clk.Advance(2 * time.Hour)
				return token, nonce
			},
		},
		{
			nama:  "email belum diverifikasi",
			email: "belum.verifikasi@kampus.example",
			siapkan: func(h *harnessAuth) (string, string) {
				nonce := h.nonceBaru()
				token := h.tokenID(map[string]any{
					"sub": "sub-asal", "email": "belum.verifikasi@kampus.example",
					"email_verified": false, "nonce": nonce,
				})
				return token, nonce
			},
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			h := newHarnessAuth(t)

			// Akun undangan disiapkan lebih dahulu supaya satu-satunya penyebab
			// penolakan adalah verifikasi ID token, bukan undangan yang hilang.
			_, sesiAdmin := h.adminPertama("admin@kampus.example")
			data := h.undangAkun(sesiAdmin.Access, k.email, "Akun Bukti Identitas", "dosen")
			akunID, _ := data["id"].(string)

			token, nonce := k.siapkan(h)
			r := h.kirim(http.MethodPost, "/api/v1/auth/google",
				map[string]any{"id_token": token, "nonce": nonce}, "")
			h.harapGalat(r, http.StatusUnauthorized, codeIdentityTokenInvalid)

			if n := h.hitung(`SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, akunID); n != 0 {
				t.Fatalf("identitas akun undangan = %d, ingin 0 setelah verifikasi gagal", n)
			}
			if status := h.teksKolom(`SELECT status FROM users WHERE id = ?`, akunID); status != "invited" {
				t.Fatalf("status akun = %q, ingin tetap invited", status)
			}
		})
	}
}

// TestLoginGoogleNonceTidakCocokTidakMenautkanAkun membuktikan klaim nonce
// benar-benar dibandingkan dan penautan tidak terjadi saat nonce berbeda
// (D-033).
func TestLoginGoogleNonceTidakCocokTidakMenautkanAkun(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	email := "dosen.nonce@kampus.example"
	data := h.undangAkun(sesiAdmin.Access, email, "Dosen Nonce", "dosen")
	dosenID, _ := data["id"].(string)

	// Nonce yang diminta berbeda dari klaim pada token; keduanya sah menurut
	// bentuk, tetapi tidak saling terikat.
	nonceKlaim := h.nonceBaru()
	nonceDiminta := h.nonceBaru()
	token := h.tokenID(map[string]any{"sub": "sub-nonce", "email": email, "nonce": nonceKlaim})
	r := h.kirim(http.MethodPost, "/api/v1/auth/google",
		map[string]any{"id_token": token, "nonce": nonceDiminta}, "")
	h.harapGalat(r, http.StatusUnauthorized, codeIdentityTokenInvalid)

	if n := h.hitung(`SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, dosenID); n != 0 {
		t.Fatalf("baris user_identities akun undangan = %d, ingin 0 saat nonce tidak cocok", n)
	}
	if status := h.teksKolom(`SELECT status FROM users WHERE id = ?`, dosenID); status != "invited" {
		t.Fatalf("status akun = %q, ingin tetap invited", status)
	}
}

// TestLoginGoogleNonceSekaliPakaiMenolakPengulangan membuktikan nonce yang sama
// tidak dapat dipakai dua kali dan sesi kedua tidak pernah terbit.
func TestLoginGoogleNonceSekaliPakaiMenolakPengulangan(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")

	email := "dosen.ulang@kampus.example"
	data := h.undangAkun(sesiAdmin.Access, email, "Dosen Ulang", "dosen")
	dosenID, _ := data["id"].(string)

	subjek := "sub-" + dosenID
	nonce := h.nonceBaru()
	token := h.tokenID(map[string]any{"sub": subjek, "email": email, "nonce": nonce})

	pertama := h.kirim(http.MethodPost, "/api/v1/auth/google",
		map[string]any{"id_token": token, "nonce": nonce}, "")
	if sesi := h.sesiDari(h.harapSukses(pertama, http.StatusOK)); sesi.UserID != dosenID {
		t.Fatalf("sesi pertama menunjuk %s, ingin %s", sesi.UserID, dosenID)
	}

	// Permintaan kedua memakai ID token dan nonce yang sama persis.
	kedua := h.kirim(http.MethodPost, "/api/v1/auth/google",
		map[string]any{"id_token": token, "nonce": nonce}, "")
	h.harapGalat(kedua, http.StatusUnauthorized, codeIdentityTokenInvalid)

	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE user_id = ?`, dosenID); n != 1 {
		t.Fatalf("sesi pengguna = %d, ingin 1 (tidak ada sesi kedua)", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, dosenID); n != 1 {
		t.Fatalf("identitas pengguna = %d, ingin 1", n)
	}
}

// TestLoginGoogleEmailTidakDiundangTidakMembuatAkun menegaskan tidak ada
// auto-provision: login dari email yang tidak diundang ditolak dan tidak
// menghasilkan baris users baru (D-032).
func TestLoginGoogleEmailTidakDiundangTidakMembuatAkun(t *testing.T) {
	h := newHarnessAuth(t)
	h.adminPertama("admin@kampus.example")
	sebelum := h.hitung(`SELECT COUNT(*) FROM users`)

	email := "orang.asing@kampus.example"
	nonce := h.nonceBaru()
	r := h.kirim(http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": h.tokenID(map[string]any{"sub": "sub-asing", "email": email, "nonce": nonce}),
		"nonce":    nonce,
	}, "")
	h.harapGalat(r, http.StatusForbidden, codeIdentityNotLinked)

	if n := h.hitung(`SELECT COUNT(*) FROM users WHERE email = ?`, email); n != 0 {
		t.Fatalf("baris users untuk email tak diundang = %d, ingin 0", n)
	}
	if sesudah := h.hitung(`SELECT COUNT(*) FROM users`); sesudah != sebelum {
		t.Fatalf("jumlah users berubah dari %d menjadi %d setelah login ditolak", sebelum, sesudah)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens`); n != 1 {
		t.Fatalf("sesi = %d, ingin hanya sesi admin (1)", n)
	}
}

// TestLoginGoogleAkunSuspendedDitolak membuktikan akun yang dinonaktifkan tidak
// dapat masuk walaupun identitasnya sudah tertaut.
func TestLoginGoogleAkunSuspendedDitolak(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	email := "dosen.suspend@kampus.example"
	dosenID, _ := h.undangDanMasuk(sesiAdmin.Access, email, "Dosen Suspend", "dosen")

	r := h.kirim(http.MethodPatch, "/api/v1/users/"+dosenID,
		map[string]any{"status": "suspended"}, sesiAdmin.Access)
	data := h.harapSukses(r, http.StatusOK)
	if data["status"] != "suspended" {
		t.Fatalf("status akun setelah PATCH = %v, ingin suspended", data["status"])
	}

	nonce := h.nonceBaru()
	rLogin := h.kirim(http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": h.tokenID(map[string]any{"sub": "sub-" + dosenID, "email": email, "nonce": nonce}),
		"nonce":    nonce,
	}, "")
	h.harapGalat(rLogin, http.StatusForbidden, codeIdentityNotLinked)
}

// TestBootstrapAdminHanyaSekali membuktikan admin pertama hanya dapat dibuat
// sekali dan tidak ada admin kedua yang terbuat otomatis.
func TestBootstrapAdminHanyaSekali(t *testing.T) {
	h := newHarnessAuth(t)

	if _, err := h.srv.Accounts.CreateFirstAdmin(context.Background(), "admin@kampus.example", "Admin Pertama"); err != nil {
		t.Fatalf("CreateFirstAdmin() pertama gagal: %v", err)
	}
	if _, err := h.srv.Accounts.CreateFirstAdmin(context.Background(), "admin.kedua@kampus.example", "Admin Kedua"); !errors.Is(err, auth.ErrConflict) {
		t.Fatalf("CreateFirstAdmin() kedua = %v, ingin ErrConflict", err)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM users WHERE role = 'admin'`); n != 1 {
		t.Fatalf("jumlah akun admin = %d, ingin 1", n)
	}

	// Login Google dari email lain tidak boleh menghasilkan admin otomatis.
	sebelum := h.hitung(`SELECT COUNT(*) FROM users`)
	nonce := h.nonceBaru()
	r := h.kirim(http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": h.tokenID(map[string]any{
			"sub": "sub-asing", "email": "orang.lain@kampus.example", "nonce": nonce,
		}),
		"nonce": nonce,
	}, "")
	h.harapGalat(r, http.StatusForbidden, codeIdentityNotLinked)

	if n := h.hitung(`SELECT COUNT(*) FROM users WHERE role = 'admin'`); n != 1 {
		t.Fatalf("jumlah akun admin setelah login ditolak = %d, ingin 1", n)
	}
	if sesudah := h.hitung(`SELECT COUNT(*) FROM users`); sesudah != sebelum {
		t.Fatalf("jumlah users berubah dari %d menjadi %d", sebelum, sesudah)
	}
}

// TestPerubahanRoleBerlakuPadaTokenLama membuktikan role dibaca dari database
// pada setiap permintaan, bukan dari token (D-037).
func TestPerubahanRoleBerlakuPadaTokenLama(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	email := "dosen.naik@kampus.example"
	dosenID, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, email, "Dosen Naik", "dosen")

	// Menaikkan role menjadi admin: token lama harus langsung berrole admin.
	naik := h.kirim(http.MethodPatch, "/api/v1/users/"+dosenID,
		map[string]any{"role": "admin"}, sesiAdmin.Access)
	if data := h.harapSukses(naik, http.StatusOK); data["role"] != "admin" {
		t.Fatalf("role setelah PATCH = %v, ingin admin", data["role"])
	}
	me := h.kirim(http.MethodGet, "/api/v1/me", nil, sesiDosen.Access)
	if data := h.harapSukses(me, http.StatusOK); data["role"] != "admin" {
		t.Fatalf("role pada GET /me dengan token lama = %v, ingin admin", data["role"])
	}

	// Menurunkan kembali menjadi mahasiswa: token yang sama ikut berubah lagi.
	turun := h.kirim(http.MethodPatch, "/api/v1/users/"+dosenID,
		map[string]any{"role": "mahasiswa"}, sesiAdmin.Access)
	if data := h.harapSukses(turun, http.StatusOK); data["role"] != "mahasiswa" {
		t.Fatalf("role setelah PATCH kedua = %v, ingin mahasiswa", data["role"])
	}
	me2 := h.kirim(http.MethodGet, "/api/v1/me", nil, sesiDosen.Access)
	if data := h.harapSukses(me2, http.StatusOK); data["role"] != "mahasiswa" {
		t.Fatalf("role pada GET /me dengan token lama = %v, ingin mahasiswa", data["role"])
	}
}

// TestPenonaktifanAkunMencabutSesiAktif membuktikan penonaktifan akun mencabut
// seluruh sesinya seketika (D-033).
func TestPenonaktifanAkunMencabutSesiAktif(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	email := "dosen.nonaktif@kampus.example"
	dosenID, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, email, "Dosen Nonaktif", "dosen")

	r := h.kirim(http.MethodPatch, "/api/v1/users/"+dosenID,
		map[string]any{"status": "suspended"}, sesiAdmin.Access)
	h.harapSukses(r, http.StatusOK)

	me := h.kirim(http.MethodGet, "/api/v1/me", nil, sesiDosen.Access)
	h.harapGalat(me, http.StatusUnauthorized, codeSessionRevoked)

	if alasan := h.teksKolom(`SELECT revoked_reason FROM session_tokens WHERE user_id = ?`, dosenID); alasan != "akun_tidak_aktif" {
		t.Fatalf("revoked_reason = %q, ingin akun_tidak_aktif", alasan)
	}
}

// ---- skenario B: sesi ------------------------------------------------------

// TestMeTanpaTokenDanTokenAsingDitolak menegaskan /me selalu menuntut token
// akses yang dikenali.
func TestMeTanpaTokenDanTokenAsingDitolak(t *testing.T) {
	h := newHarnessAuth(t)

	tanpaToken := h.kirim(http.MethodGet, "/api/v1/me", nil, "")
	h.harapGalat(tanpaToken, http.StatusUnauthorized, codeUnauthenticated)

	asing := h.kirim(http.MethodGet, "/api/v1/me", nil, "token-asing-tidak-dikenal")
	h.harapGalat(asing, http.StatusUnauthorized, codeUnauthenticated)
}

// TestTokenAksesKedaluwarsaDitolak membuktikan masa berlaku token akses 60
// menit ditegakkan dan dibedakan dari token yang tidak dikenali.
func TestTokenAksesKedaluwarsaDitolak(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi := h.adminPertama("admin@kampus.example")

	// Sebelum batas: token masih bekerja.
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access), http.StatusOK)

	// Batas kontrak D-033: token akses berlaku 60 menit. Nilai ditulis eksplisit
	// supaya perubahan kebijakan produksi membuat uji ini gagal, bukan ikut
	// menyesuaikan diri.
	h.clk.Advance(61 * time.Minute)
	r := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access)
	h.harapGalat(r, http.StatusUnauthorized, codeTokenExpired)
}

// TestPenyegaranMemutarToken membuktikan token akses yang ditolak karena
// kedaluwarsa dapat dipulihkan lewat rotasi, tanpa login Google ulang.
func TestPenyegaranMemutarToken(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi1 := h.adminPertama("admin@kampus.example")

	h.clk.Advance(61 * time.Minute)
	kedaluwarsa := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi1.Access)
	h.harapGalat(kedaluwarsa, http.StatusUnauthorized, codeTokenExpired)

	r := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	sesi2 := h.sesiDariSegarkan(h.harapSukses(r, http.StatusOK), sesi1)

	if sesi2.Access == sesi1.Access {
		t.Fatal("token akses baru sama dengan token akses lama")
	}
	if sesi2.Refresh == sesi1.Refresh {
		t.Fatal("token penyegar baru sama dengan token penyegar lama")
	}
	if sesi2.SesiID != sesi1.SesiID {
		t.Fatalf("id sesi berubah dari %s menjadi %s", sesi1.SesiID, sesi2.SesiID)
	}
	if sesi2.UserID != sesi1.UserID || sesi2.Role != sesi1.Role {
		t.Fatal("identitas pengguna berubah setelah penyegaran token kedaluwarsa")
	}

	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, sesi2.Access), http.StatusOK)
	lama := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi1.Access)
	h.harapGalat(lama, http.StatusUnauthorized, codeUnauthenticated)
}

// TestPemakaianUlangPenyegarMencabutSesi membuktikan replay token penyegar yang
// sudah diputar mencabut sesi dan tercatat di audit (D-033).
func TestPemakaianUlangPenyegarMencabutSesi(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi1 := h.adminPertama("admin@kampus.example")

	r := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	sesi2 := h.sesiDariSegarkan(h.harapSukses(r, http.StatusOK), sesi1)

	ulang := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	h.harapGalat(ulang, http.StatusUnauthorized, codeSessionRevoked)

	if alasan := h.teksKolom(`SELECT revoked_reason FROM session_tokens WHERE id = ?`, sesi1.SesiID); alasan != "refresh_reuse" {
		t.Fatalf("revoked_reason = %q, ingin refresh_reuse", alasan)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE id = ? AND revoked_at IS NOT NULL`, sesi1.SesiID); n != 1 {
		t.Fatalf("sesi tercabut = %d, ingin 1", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.session.reuse_detected'`); n != 1 {
		t.Fatalf("audit auth.session.reuse_detected = %d, ingin 1", n)
	}

	// Token akses hasil rotasi ikut mati karena sesinya sudah dicabut.
	mati := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi2.Access)
	h.harapGalat(mati, http.StatusUnauthorized, codeSessionRevoked)
}

// TestBatasAbsolutTigaPuluhHariMenghentikanPenyegaran membuktikan batas absolut
// sesi tidak pernah diperpanjang oleh rotasi.
func TestBatasAbsolutTigaPuluhHariMenghentikanPenyegaran(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi1 := h.adminPertama("admin@kampus.example")

	// Hari ke-29: sesi masih hidup dan token akses diperbarui.
	h.clk.Advance(29 * 24 * time.Hour)
	r := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	sesi2 := h.sesiDariSegarkan(h.harapSukses(r, http.StatusOK), sesi1)
	if sesi2.Access == sesi1.Access {
		t.Fatal("token akses hari ke-29 tidak diperbarui")
	}
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, sesi2.Access), http.StatusOK)

	// Melewati 30 hari sejak sesi dibuat: penyegaran berhenti walaupun token
	// penyegarnya belum pernah diputar.
	h.clk.Advance(2 * 24 * time.Hour)
	lewat := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi2.Refresh}, "")
	h.harapGalat(lewat, http.StatusUnauthorized, codeUnauthenticated)

	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE id = ? AND revoked_at IS NOT NULL`, sesi1.SesiID); n != 1 {
		t.Fatalf("sesi lewat batas absolut tercabut = %d, ingin 1", n)
	}
}

// TestPerubahanEmailGoogleTidakMengalihkanAkun membuktikan pengenal utama
// setelah penautan pertama adalah `sub`, bukan email (D-032 butir 6, GA-14).
// Email hanya dipakai sekali pada penautan pertama; setelah itu perubahan
// alamat di penyedia identitas tidak boleh mengalihkan akun maupun menautkan
// akun kedua.
func TestPerubahanEmailGoogleTidakMengalihkanAkun(t *testing.T) {
	h := newHarnessAuth(t)
	admin, sesiAdmin := h.adminPertama("admin@kampus.example")
	email := "dosen.email.berubah@kampus.example"
	dosenID, _ := h.undangDanMasuk(sesiAdmin.Access, email, "Dosen Email", "dosen")

	// Identitas yang tertaut sekarang memakai email baru, `sub` tetap sama.
	subjek := h.teksKolom(`SELECT subject FROM user_identities WHERE user_id = ?`, dosenID)
	if subjek == "" {
		t.Fatal("identitas dosen tidak ditemukan setelah login pertama")
	}
	nonce := h.nonceBaru()
	login := h.kirim(http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": h.tokenID(map[string]any{
			"sub": subjek, "email": "dosen.baru@kampus.example", "nonce": nonce,
		}),
		"nonce": nonce,
	}, "")
	sesi := h.sesiDari(h.harapSukses(login, http.StatusOK))

	if sesi.UserID != dosenID {
		t.Fatalf("akun berpindah ke %s, ingin tetap %s", sesi.UserID, dosenID)
	}
	if emailTersimpan := h.teksKolom(`SELECT email FROM users WHERE id = ?`, dosenID); emailTersimpan != email {
		t.Fatalf("email akun berubah menjadi %q, ingin tetap %q", emailTersimpan, email)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM user_identities WHERE user_id = ?`, dosenID); n != 1 {
		t.Fatalf("identitas akun = %d, ingin tetap 1", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM users`); n != 2 {
		t.Fatalf("jumlah akun = %d, ingin tetap 2 (admin + dosen)", n)
	}

	// Email baru itu belum diundang: login dengan `sub` lain dan email baru
	// tidak boleh menemukan akun mana pun.
	nonce2 := h.nonceBaru()
	asing := h.kirim(http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": h.tokenID(map[string]any{
			"sub": "sub-lain-email-baru", "email": "dosen.baru@kampus.example", "nonce": nonce2,
		}),
		"nonce": nonce2,
	}, "")
	h.harapGalat(asing, http.StatusForbidden, codeIdentityNotLinked)
	if n := h.hitung(`SELECT COUNT(*) FROM users`); n != 2 {
		t.Fatalf("jumlah akun setelah login email baru = %d, ingin tetap 2", n)
	}

	// Role akun asli tetap dibaca dari database, bukan dari klaim Google.
	if role := h.teksKolom(`SELECT role FROM users WHERE id = ?`, admin.ID); role != "admin" {
		t.Fatalf("role admin berubah menjadi %q", role)
	}
}

// TestPerubahanEmailGoogleTidakMengalihkanAkunSaatSudahTertaut menutup sisi
// lain GA-14: email yang sama tidak dapat dipakai untuk menautkan akun kedua
// setelah penautan pertama selesai.
func TestPerubahanEmailGoogleTidakMengalihkanAkunSaatSudahTertaut(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	email := "dosen.ganda.email@kampus.example"
	dosenID, _ := h.undangDanMasuk(sesiAdmin.Access, email, "Dosen Ganda", "dosen")

	// Akun kedua dengan email sama tidak dapat dibuat: batasan unik menolaknya.
	ulang := h.kirim(http.MethodPost, "/api/v1/users", map[string]any{
		"email": email, "display_name": "Dosen Ganda Dua", "role": "dosen",
	}, sesiAdmin.Access)
	h.harapGalat(ulang, http.StatusConflict, codeConflict)

	if n := h.hitung(`SELECT COUNT(*) FROM users WHERE email = ?`, email); n != 1 {
		t.Fatalf("jumlah akun dengan email sama = %d, ingin 1", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM user_identities`); n != 2 {
		t.Fatalf("jumlah identitas = %d, ingin 2 (admin + dosen)", n)
	}
	if dosenID == "" {
		t.Fatal("id dosen kosong")
	}
}

func TestLogoutExpiredAccessWithRefreshRevokesSession(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi := h.adminPertama("admin@kampus.example")
	h.clk.Advance(time.Hour)
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access),
		http.StatusUnauthorized, codeTokenExpired)

	logout := h.kirim(http.MethodPost, "/api/v1/auth/logout",
		map[string]any{"refresh_token": sesi.Refresh}, "")
	if logout.Status != http.StatusNoContent {
		t.Fatalf("logout dengan access kedaluwarsa = %d, ingin 204; badan: %s", logout.Status, logout.Rec.Body.String())
	}
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi.Refresh}, ""), http.StatusUnauthorized, codeUnauthenticated)
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE id = ? AND revoked_at IS NOT NULL AND revoked_reason = 'logout'`, sesi.SesiID); n != 1 {
		t.Fatalf("sesi benar-benar dicabut = %d, ingin 1", n)
	}
}

// TestLogoutIdempoten membuktikan logout mencabut sesi dan panggilan kedua
// dengan token yang sama tetap menjawab 204 (kontrak §5.1).
func TestLogoutIdempoten(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi := h.adminPertama("admin@kampus.example")

	pertama := h.kirim(http.MethodPost, "/api/v1/auth/logout", map[string]any{"refresh_token": sesi.Refresh}, "")
	if pertama.Status != http.StatusNoContent {
		t.Fatalf("logout pertama = %d, ingin 204; badan: %s", pertama.Status, pertama.Rec.Body.String())
	}

	ditolak := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access)
	h.harapGalat(ditolak, http.StatusUnauthorized, codeSessionRevoked)

	segarkan := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi.Refresh}, "")
	h.harapGalat(segarkan, http.StatusUnauthorized, codeUnauthenticated)

	kedua := h.kirim(http.MethodPost, "/api/v1/auth/logout", map[string]any{"refresh_token": sesi.Refresh}, "")
	if kedua.Status != http.StatusNoContent {
		t.Fatalf("logout kedua = %d, ingin 204; badan: %s", kedua.Status, kedua.Rec.Body.String())
	}
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.logout'`); n != 1 {
		t.Fatalf("audit auth.logout = %d, ingin 1 (pencabutan ulang tidak diaudit lagi)", n)
	}
}

// TestAdminMencabutSesiPenggunaLain membuktikan admin dapat mencabut sesi mana
// pun dan token korban langsung ditolak.
func TestAdminMencabutSesiPenggunaLain(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	_, sesiKorban := h.undangDanMasuk(sesiAdmin.Access, "dosen.korban@kampus.example", "Dosen Korban", "dosen")

	r := h.kirim(http.MethodDelete, "/api/v1/auth/sessions/"+sesiKorban.SesiID, nil, sesiAdmin.Access)
	if r.Status != http.StatusNoContent {
		t.Fatalf("DELETE sesi = %d, ingin 204; badan: %s", r.Status, r.Rec.Body.String())
	}

	ditolak := h.kirim(http.MethodGet, "/api/v1/me", nil, sesiKorban.Access)
	h.harapGalat(ditolak, http.StatusUnauthorized, codeSessionRevoked)
}

// TestPemakaianUlangPenyegarMencabutSeluruhSesi membuktikan cakupan pencabutan
// pada deteksi pemakaian ulang sesuai kontrak §5.1: SELURUH sesi pengguna itu
// dicabut, bukan hanya sesi yang tokennya diputar. Tanpa cakupan ini, sesi
// kedua di perangkat lain tetap hidup walaupun token penyegarnya bocor.
func TestPemakaianUlangPenyegarMencabutSeluruhSesi(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi1 := h.adminPertama("admin@kampus.example")

	// Sesi kedua untuk pengguna yang sama: login ulang lewat Google dengan
	// identitas yang sudah tertaut, seperti login di perangkat lain.
	subjek := h.teksKolom(`SELECT subject FROM user_identities WHERE user_id = ?`, sesi1.UserID)
	if subjek == "" {
		t.Fatal("identitas Google admin tidak ditemukan")
	}
	nonce2 := h.nonceBaru()
	login2 := h.kirim(http.MethodPost, "/api/v1/auth/google", map[string]any{
		"id_token": h.tokenID(map[string]any{
			"sub": subjek, "email": "admin@kampus.example", "nonce": nonce2,
		}),
		"nonce": nonce2,
	}, "")
	sesi2 := h.sesiDari(h.harapSukses(login2, http.StatusOK))
	if sesi2.SesiID == sesi1.SesiID {
		t.Fatal("login kedua memakai sesi yang sama, ingin sesi terpisah")
	}

	// Rotasi sesi pertama, lalu pakai token penyegar lamanya.
	putar := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	sesi1b := h.sesiDariSegarkan(h.harapSukses(putar, http.StatusOK), sesi1)

	ulang := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	h.harapGalat(ulang, http.StatusUnauthorized, codeSessionRevoked)

	// Kedua sesi harus mati, bukan hanya sesi yang tokennya diputar. Token
	// akses sesi pertama yang dipakai di sini adalah hasil rotasi, karena token
	// akses lamanya memang sudah tidak berlaku setelah penyegaran.
	for nama, sesi := range map[string]sesiUji{"sesi pertama": sesi1b, "sesi kedua": sesi2} {
		mati := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access)
		h.harapGalat(mati, http.StatusUnauthorized, codeSessionRevoked)
		if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE id = ? AND revoked_at IS NOT NULL`, sesi.SesiID); n != 1 {
			t.Errorf("%s belum tercabut (%d baris)", nama, n)
		}
	}
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE revoked_at IS NULL`); n != 0 {
		t.Errorf("sesi aktif tersisa = %d, ingin 0", n)
	}
}

// TestPemakaianUlangPenyegarGenerasiLamaTerdeteksi membuktikan token penyegar
// yang sudah diputar lebih dari sekali tetap dikenali sebagai pemakaian ulang.
// `prev_refresh_hash` hanya menautkan satu generasi ke belakang; tabel riwayat
// rotasi (migrasi 00003) yang menutup celah generasi lebih lama.
func TestPemakaianUlangPenyegarGenerasiLamaTerdeteksi(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi1 := h.adminPertama("admin@kampus.example")

	putar1 := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	sesi2 := h.sesiDariSegarkan(h.harapSukses(putar1, http.StatusOK), sesi1)

	putar2 := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi2.Refresh}, "")
	sesi3 := h.sesiDariSegarkan(h.harapSukses(putar2, http.StatusOK), sesi2)

	// Token generasi pertama kini dua langkah di belakang: tidak ada lagi di
	// `refresh_hash` maupun `prev_refresh_hash`.
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE refresh_hash = ? OR prev_refresh_hash = ?`,
		hashUji(sesi1.Refresh), hashUji(sesi1.Refresh)); n != 0 {
		t.Fatalf("token generasi lama masih tersimpan di session_tokens = %d, ingin 0", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM session_token_history`); n != 2 {
		t.Fatalf("riwayat rotasi = %d, ingin 2", n)
	}

	ulang := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesi1.Refresh}, "")
	h.harapGalat(ulang, http.StatusUnauthorized, codeSessionRevoked)

	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.session.reuse_detected'`); n != 1 {
		t.Fatalf("audit auth.session.reuse_detected = %d, ingin 1", n)
	}
	// Sesi terbaru ikut dicabut karena cakupannya seluruh pengguna.
	mati := h.kirim(http.MethodGet, "/api/v1/me", nil, sesi3.Access)
	h.harapGalat(mati, http.StatusUnauthorized, codeSessionRevoked)
}

// TestNonceKedaluwarsaDitolak membuktikan nonce yang melewati TTL-nya tidak
// dapat dipakai lagi, walaupun ID tokennya masih berlaku (temuan audit K-01).
func TestNonceKedaluwarsaDitolak(t *testing.T) {
	h := newHarnessAuth(t)
	h.adminPertama("admin@kampus.example")

	nonce := h.nonceBaru()
	// ID token tetap sah: hanya jam nonce yang lewat.
	h.clk.Advance(6 * time.Minute)
	token := h.tokenID(map[string]any{
		"sub": "sub-admin", "email": "admin@kampus.example", "nonce": nonce,
	})
	r := h.kirim(http.MethodPost, "/api/v1/auth/google",
		map[string]any{"id_token": token, "nonce": nonce}, "")
	h.harapGalat(r, http.StatusUnauthorized, codeIdentityTokenInvalid)

	if n := h.hitung(`SELECT COUNT(*) FROM auth_nonces WHERE expires_at > ?`, h.teksKolom(`SELECT strftime('%Y-%m-%dT%H:%M:%fZ','now')`)); n != 0 {
		t.Fatalf("nonce masih dianggap berlaku = %d, ingin 0", n)
	}
}

// TestNonceParalelHanyaSatuYangLolos membuktikan pemakaian nonce bersifat
// atomik: dua permintaan bersamaan dengan nonce dan ID token yang sama tidak
// dapat sama-sama menghasilkan sesi (temuan audit K-02).
func TestNonceParalelHanyaSatuYangLolos(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi := h.adminPertama("admin@kampus.example")

	// Identitas yang sudah tertaut dipakai supaya hasilnya hanya ditentukan
	// oleh konsumsi nonce, bukan oleh pencarian undangan.
	subjek := h.teksKolom(`SELECT subject FROM user_identities WHERE user_id = ?`, sesi.UserID)
	nonce := h.nonceBaru()
	token := h.tokenID(map[string]any{
		"sub": subjek, "email": "admin@kampus.example", "nonce": nonce,
	})
	badan, err := json.Marshal(map[string]any{"id_token": token, "nonce": nonce})
	if err != nil {
		t.Fatalf("menyusun badan permintaan gagal: %v", err)
	}

	const jumlah = 8
	var (
		tunggu  sync.WaitGroup
		mu      sync.Mutex
		sukses  int
		ditolak int
	)
	for range jumlah {
		tunggu.Add(1)
		go func() {
			defer tunggu.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", bytes.NewReader(badan))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.router.ServeHTTP(rec, req)

			mu.Lock()
			defer mu.Unlock()
			switch rec.Code {
			case http.StatusOK:
				sukses++
			case http.StatusUnauthorized:
				ditolak++
			default:
				t.Errorf("status tak terduga pada permintaan paralel: %d", rec.Code)
			}
		}()
	}
	tunggu.Wait()

	if sukses != 1 {
		t.Fatalf("permintaan paralel yang lolos = %d, ingin tepat 1", sukses)
	}
	if ditolak != jumlah-1 {
		t.Fatalf("permintaan paralel yang ditolak = %d, ingin %d", ditolak, jumlah-1)
	}
	// Sesi yang sudah ada sebelum uji ditambah satu sesi hasil login paralel.
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE user_id = ?`, sesi.UserID); n != 2 {
		t.Fatalf("sesi terbit = %d, ingin 2 (satu sesi awal + satu login paralel)", n)
	}
}

// TestPencabutanSesiKedaluwarsaMenjawab404 menutup kontrak §5.1: mencabut sesi
// yang sudah tercabut atau sudah kedaluwarsa menjawab 404, bukan 204.
func TestPencabutanSesiKedaluwarsaMenjawab404(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	_, sesiKorban := h.undangDanMasuk(sesiAdmin.Access, "dosen.404@kampus.example", "Dosen 404", "dosen")

	pertama := h.kirim(http.MethodDelete, "/api/v1/auth/sessions/"+sesiKorban.SesiID, nil, sesiAdmin.Access)
	if pertama.Status != http.StatusNoContent {
		t.Fatalf("pencabutan pertama = %d, ingin 204", pertama.Status)
	}
	ulang := h.kirim(http.MethodDelete, "/api/v1/auth/sessions/"+sesiKorban.SesiID, nil, sesiAdmin.Access)
	h.harapGalat(ulang, http.StatusNotFound, codeNotFound)

	// Sesi yang sudah lewat batas absolutnya juga tidak lagi dapat dicabut.
	_, sesiLama := h.undangDanMasuk(sesiAdmin.Access, "dosen.lama@kampus.example", "Dosen Lama", "dosen")
	// Lewatkan batas absolut sesi itu. Token admin pemanggil ikut kedaluwarsa
	// pada titik yang sama, sehingga sesi admin baru diterbitkan langsung lewat
	// store pada jam yang sudah dimajukan — sama seperti admin yang masuk ulang
	// lewat Google pada waktu itu.
	h.clk.Advance(31 * 24 * time.Hour)
	pair, err := h.srv.Sessions.Issue(context.Background(), sesiAdmin.UserID, "uji")
	if err != nil {
		t.Fatalf("Issue() sesi admin baru gagal: %v", err)
	}
	lewat := h.kirim(http.MethodDelete, "/api/v1/auth/sessions/"+sesiLama.SesiID, nil, pair.AccessToken)
	h.harapGalat(lewat, http.StatusNotFound, codeNotFound)
}

// TestDaftarSesiHanyaSesiAktifDanBermuatanRole memastikan endpoint administrasi
// menyajikan sesi aktif beserta role akunnya sesuai kontrak §5.1.
func TestDaftarSesiHanyaSesiAktifDanBermuatanRole(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	dosenID, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, "dosen.daftar@kampus.example", "Dosen Daftar", "dosen")

	// Satu permintaan terautentikasi mencatat `last_seen_at` sesi itu.
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, sesiDosen.Access), http.StatusOK)

	aktif := h.harapSuksesLarik(
		h.kirim(http.MethodGet, "/api/v1/auth/sessions?user_id="+dosenID, nil, sesiAdmin.Access), http.StatusOK)
	if len(aktif) != 1 {
		t.Fatalf("jumlah sesi aktif = %d, ingin 1", len(aktif))
	}
	entri, _ := aktif[0].(map[string]any)
	if role, _ := entri["role"].(string); role != "dosen" {
		t.Fatalf("role pada entri sesi = %q, ingin dosen", role)
	}
	for _, wajib := range []string{"created_at", "absolute_expires_at"} {
		if nilai, _ := entri[wajib].(string); nilai == "" {
			t.Errorf("entri sesi tidak memuat %q: %v", wajib, entri)
		}
	}
	// `last_seen_at` terisi setelah ada permintaan terautentikasi; nilai itu
	// yang membuat sesi terlantar dapat dikenali (kontrak §5.1).
	if h.teksKolom(`SELECT COALESCE(last_seen_at,'') FROM session_tokens WHERE id = ?`, sesiDosen.SesiID) == "" {
		t.Fatalf("last_seen_at sesi dosen belum tercatat setelah login")
	}
	if nilai, _ := entri["last_seen_at"].(string); nilai == "" {
		t.Errorf("entri sesi tidak memuat last_seen_at: %v", entri)
	}
	for kunci := range entri {
		rendah := strings.ToLower(kunci)
		if strings.Contains(rendah, "hash") || strings.Contains(rendah, "token") || strings.Contains(rendah, "revoked") {
			t.Errorf("entri sesi memuat bidang yang tidak dijanjikan kontrak: %q", kunci)
		}
	}

	// Setelah dicabut, sesinya tidak lagi muncul pada daftar sesi aktif.
	if r := h.kirim(http.MethodDelete, "/api/v1/auth/sessions/"+sesiDosen.SesiID, nil, sesiAdmin.Access); r.Status != http.StatusNoContent {
		t.Fatalf("pencabutan = %d, ingin 204", r.Status)
	}
	kosong := h.harapSuksesLarik(
		h.kirim(http.MethodGet, "/api/v1/auth/sessions?user_id="+dosenID, nil, sesiAdmin.Access), http.StatusOK)
	if len(kosong) != 0 {
		t.Fatalf("sesi tercabut masih terdaftar: %v", kosong)
	}
}

func TestLogoutRejectsInvalidRefreshCredential(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi := h.adminPertama("admin@kampus.example")
	for _, tc := range []struct {
		name   string
		body   any
		status int
		code   string
	}{
		{"access_only_legacy", nil, http.StatusBadRequest, codeValidationFailed},
		{"missing", map[string]any{}, http.StatusBadRequest, codeMissingField},
		{"blank", map[string]any{"refresh_token": " "}, http.StatusBadRequest, codeMissingField},
		{"wrong_type", map[string]any{"refresh_token": 7}, http.StatusBadRequest, codeValidationFailed},
		{"unknown", map[string]any{"refresh_token": "unknown"}, http.StatusUnauthorized, codeUnauthenticated},
		{"access_as_refresh", map[string]any{"refresh_token": sesi.Access}, http.StatusUnauthorized, codeUnauthenticated},
		{"session_id", map[string]any{"refresh_token": sesi.Refresh, "session_id": sesi.SesiID}, http.StatusBadRequest, codeValidationFailed},
		{"user_id", map[string]any{"refresh_token": sesi.Refresh, "user_id": sesi.UserID}, http.StatusBadRequest, codeValidationFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h.harapGalat(h.kirim(http.MethodPost, "/api/v1/auth/logout", tc.body, sesi.Access), tc.status, tc.code)
		})
	}
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access), http.StatusOK)
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.logout'`); n != 0 {
		t.Fatalf("audit kredensial ditolak = %d", n)
	}
	h.clk.Advance(30 * 24 * time.Hour)
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/auth/logout",
		map[string]any{"refresh_token": sesi.Refresh}, ""), http.StatusUnauthorized, codeUnauthenticated)
}

func TestLogoutPreviousCredentialScopesSessionNotBearer(t *testing.T) {
	h := newHarnessAuth(t)
	_, admin := h.adminPertama("admin@kampus.example")
	userID, owner := h.undangDanMasuk(admin.Access, "dosen.logout@kampus.example", "Dosen Logout", "dosen")
	otherDevice, err := h.srv.Sessions.Issue(context.Background(), userID, "perangkat-lain")
	if err != nil {
		t.Fatal(err)
	}
	refreshed := h.sesiDariSegarkan(h.harapSukses(h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": owner.Refresh}, ""), http.StatusOK), owner)
	for _, token := range []string{owner.Refresh, owner.Refresh, refreshed.Refresh} {
		// Bearer akun lain tidak menentukan sesi yang dicabut.
		r := h.kirim(http.MethodPost, "/api/v1/auth/logout", map[string]any{"refresh_token": token}, admin.Access)
		if r.Status != http.StatusNoContent || r.Rec.Body.Len() != 0 {
			t.Fatalf("logout previous/current = %d %s, ingin 204 kosong", r.Status, r.Rec.Body.String())
		}
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me", nil, refreshed.Access), http.StatusUnauthorized, codeSessionRevoked)
	for _, token := range []string{owner.Refresh, refreshed.Refresh} {
		h.harapGalat(h.kirim(http.MethodPost, "/api/v1/auth/refresh",
			map[string]any{"refresh_token": token}, ""), http.StatusUnauthorized, codeUnauthenticated)
	}
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, otherDevice.AccessToken), http.StatusOK)
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, admin.Access), http.StatusOK)
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.logout' AND actor_id = ? AND entity_id = ?`,
		userID, owner.SesiID); n != 1 {
		t.Fatalf("audit pemilik sesi = %d, ingin 1", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.session.reuse_detected'`); n != 0 {
		t.Fatalf("logout previous menjalankan deteksi reuse: %d", n)
	}
}

func TestLogoutAuditFailureReturns500WithoutPartialRevocation(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesi := h.adminPertama("admin@kampus.example")
	h.execSQL(`CREATE TRIGGER fail_logout_audit BEFORE INSERT ON audit_events
		WHEN NEW.action = 'auth.logout' BEGIN SELECT RAISE(ABORT, 'injected audit failure'); END`)
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/auth/logout",
		map[string]any{"refresh_token": sesi.Refresh}, ""), http.StatusInternalServerError, codeInternalError)
	h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, sesi.Access), http.StatusOK)
	if n := h.hitung(`SELECT COUNT(*) FROM session_tokens WHERE id = ? AND revoked_at IS NOT NULL`, sesi.SesiID); n != 0 {
		t.Fatalf("pencabutan parsial = %d", n)
	}
	h.execSQL(`DROP TRIGGER fail_logout_audit`)
	if r := h.kirim(http.MethodPost, "/api/v1/auth/logout",
		map[string]any{"refresh_token": sesi.Refresh}, ""); r.Status != http.StatusNoContent {
		t.Fatalf("retry logout = %d %s", r.Status, r.Rec.Body.String())
	}
}
