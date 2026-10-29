package httpapi

// Pengujian Email OTP lewat router produksi. Yang diperiksa adalah perilaku
// yang terlihat klien dan efek database: anti-enumeration, role dari database,
// expiry/reuse/brute-force, konsumsi konkuren, rate limit, kegagalan SMTP,
// lifecycle sesi, serta tidak bocornya kode/email/token ke log atau audit.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/emailotp"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

type senderUji struct {
	mu    sync.Mutex
	email []string
	kode  []string
	gagal bool
}

func (s *senderUji) Send(_ context.Context, email, kode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gagal {
		return errors.New("smtp uji gagal")
	}
	s.email = append(s.email, email)
	s.kode = append(s.kode, kode)
	return nil
}

func (s *senderUji) terakhir(t *testing.T) (string, string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.kode) == 0 {
		t.Fatal("tidak ada email terkirim")
	}
	return s.email[len(s.email)-1], s.kode[len(s.kode)-1]
}

func (s *senderUji) jumlah() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.kode)
}

// pasangEmailOTP memakai database dan jam yang sama dengan harness FL-011.
func pasangEmailOTP(t *testing.T, h *harnessAuth, sender *senderUji) {
	t.Helper()
	svc, err := emailotp.New(h.db, h.clk, bytes.Repeat([]byte("k"), 32), sender)
	if err != nil {
		t.Fatalf("emailotp.New() gagal: %v", err)
	}
	h.srv.EmailOTP = svc
	h.router = NewRouter(h.srv)
	t.Cleanup(svc.Close)
}

// kirimTLS mengirim permintaan seolah melewati TLS yang sudah diterminasi
// server. Tidak memakai header forwarded: handler hanya memeriksa r.TLS.
func (h *harnessAuth) kirimTLS(method, jalur string, badan any) responsUji {
	h.t.Helper()
	var isi []byte
	if badan != nil {
		var err error
		isi, err = json.Marshal(badan)
		if err != nil {
			h.t.Fatalf("marshal badan gagal: %v", err)
		}
	}
	var req *http.Request
	if isi == nil {
		req = httptest.NewRequest(method, jalur, nil)
	} else {
		req = httptest.NewRequest(method, jalur, bytes.NewReader(isi))
		req.Header.Set("Content-Type", "application/json")
	}
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	hasil := responsUji{Status: rec.Code, Rec: rec}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &hasil.Body)
	}
	return hasil
}

func (h *harnessAuth) buatAkun(email string, role auth.Role, status auth.Status) string {
	h.t.Helper()
	id := ids.New()
	if _, err := h.db.Exec(
		`INSERT INTO users (id, email, display_name, role, status) VALUES (?, ?, ?, ?, ?)`,
		id, email, "Akun Uji", string(role), string(status),
	); err != nil {
		h.t.Fatalf("menyiapkan akun uji gagal: %v", err)
	}
	return id
}

func (h *harnessAuth) tungguDelivery(id, ingin string) {
	h.t.Helper()
	batas := time.Now().Add(3 * time.Second)
	for time.Now().Before(batas) {
		var status string
		if err := h.db.QueryRow(`SELECT delivery FROM email_otp_challenges WHERE id = ?`, id).Scan(&status); err == nil && status == ingin {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.t.Fatalf("challenge %s tidak mencapai status %q", id, ingin)
}

// mintaOTP mengembalikan challenge dan kode yang benar-benar diterima sender.
func (h *harnessAuth) mintaOTP(sender *senderUji, email string) (string, string) {
	h.t.Helper()
	r := h.kirimTLS(http.MethodPost, "/api/v1/auth/email/request", map[string]any{"email": email})
	data := h.harapSukses(r, http.StatusAccepted)
	id, _ := data["challenge_id"].(string)
	if id == "" {
		h.t.Fatalf("challenge_id kosong: %s", r.Rec.Body.String())
	}
	if data["expires_in"] != float64(300) || data["retry_after"] != float64(60) {
		h.t.Fatalf("metadata challenge tidak sesuai kontrak: %s", r.Rec.Body.String())
	}
	h.tungguDelivery(id, "delivered")
	_, kode := sender.terakhir(h.t)
	return id, kode
}

func (h *harnessAuth) verifikasiOTP(id, kode string) responsUji {
	h.t.Helper()
	return h.kirimTLS(http.MethodPost, "/api/v1/auth/email/verify",
		map[string]any{"challenge_id": id, "code": kode})
}

func TestEmailOTPMenolakKoneksiTanpaTLS(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)

	r := h.kirim(http.MethodPost, "/api/v1/auth/email/request",
		map[string]any{"email": "admin@facelink.online"}, "")
	h.harapGalat(r, http.StatusForbidden, "HTTPS_REQUIRED")

	var jumlah int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM email_otp_challenges`).Scan(&jumlah); err != nil {
		t.Fatal(err)
	}
	if jumlah != 0 {
		t.Fatalf("HTTP membuat %d challenge; seharusnya ditolak sebelum penyimpanan", jumlah)
	}
	if sender.jumlah() != 0 {
		t.Fatal("HTTP memicu pengiriman email")
	}
}

func TestEmailOTPRequestAntiEnumerationTanpaProvisioning(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	h.buatAkun("aktif@example.test", auth.RoleDosen, auth.StatusActive)
	h.buatAkun("ditangguhkan@example.test", auth.RoleAdmin, auth.StatusSuspended)

	var publik []map[string]any
	var decoy []string
	for _, email := range []string{"aktif@example.test", "tidak-ada@example.test", "ditangguhkan@example.test"} {
		r := h.kirimTLS(http.MethodPost, "/api/v1/auth/email/request", map[string]any{"email": email})
		data := h.harapSukses(r, http.StatusAccepted)
		if email != "aktif@example.test" {
			// Challenge decoy tidak boleh dapat dipakai, bahkan bila kode ditebak.
			id, _ := data["challenge_id"].(string)
			decoy = append(decoy, id)
		}
		delete(data, "challenge_id")
		publik = append(publik, data)
	}
	for i := 1; i < len(publik); i++ {
		if !samaPeta(publik[0], publik[i]) {
			t.Fatalf("respons request membedakan akun: %v vs %v", publik[0], publik[i])
		}
	}
	if sender.jumlah() != 1 {
		t.Fatalf("email terkirim = %d, ingin hanya untuk akun aktif", sender.jumlah())
	}
	if terkirim, _ := sender.terakhir(t); terkirim != "aktif@example.test" {
		t.Fatalf("penerima = %q", terkirim)
	}
	var users int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if users != 2 {
		t.Fatalf("jumlah akun = %d, ingin tetap 2 (tidak ada provisioning)", users)
	}
	for _, id := range decoy {
		h.harapGalat(h.verifikasiOTP(id, "00000000"), http.StatusUnauthorized, "OTP_INVALID")
	}
	var sesi int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM session_tokens`).Scan(&sesi); err != nil {
		t.Fatal(err)
	}
	if sesi != 0 {
		t.Fatalf("challenge decoy menerbitkan %d sesi", sesi)
	}
}

func TestEmailOTPRoleSelaluDariDatabase(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	// Alamat resmi tidak boleh menentukan role: database yang menentukan.
	adminID := h.buatAkun("admin@facelink.online", auth.RoleMahasiswa, auth.StatusInvited)
	dosenID := h.buatAkun("dosen@facelink.online", auth.RoleAdmin, auth.StatusInvited)

	for _, kasus := range []struct {
		id, email string
		role      auth.Role
	}{
		{adminID, "admin@facelink.online", auth.RoleMahasiswa},
		{dosenID, "dosen@facelink.online", auth.RoleAdmin},
	} {
		id, kode := h.mintaOTP(sender, kasus.email)
		r := h.verifikasiOTP(id, kode)
		data := h.harapSukses(r, http.StatusOK)
		user, _ := data["user"].(map[string]any)
		if user["id"] != kasus.id || user["role"] != string(kasus.role) {
			t.Fatalf("%s: identitas/role salah: %v", kasus.email, user)
		}
	}
}

func TestEmailOTPExpiredReusedDanBruteForce(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	h.buatAkun("dosen@example.test", auth.RoleDosen, auth.StatusActive)

	id, kode := h.mintaOTP(sender, "dosen@example.test")
	salah := "00000000"
	if kode == salah {
		salah = "11111111"
	}
	for range 5 {
		h.harapGalat(h.verifikasiOTP(id, salah), http.StatusUnauthorized, "OTP_INVALID")
	}
	h.harapGalat(h.verifikasiOTP(id, kode), http.StatusUnauthorized, "OTP_INVALID")

	// Challenge baru, lalu lewati batas 5 menit: kode benar pun ditolak.
	h.clk.Advance(time.Minute)
	id2, kode2 := h.mintaOTP(sender, "dosen@example.test")
	h.clk.Advance(emailotp.TTL)
	h.harapGalat(h.verifikasiOTP(id2, kode2), http.StatusUnauthorized, "OTP_INVALID")

	var sesi int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM session_tokens`).Scan(&sesi); err != nil {
		t.Fatal(err)
	}
	if sesi != 0 {
		t.Fatalf("percobaan gagal menerbitkan %d sesi", sesi)
	}
}

func TestEmailOTPConcurrentVerifySatuSesi(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	userID := h.buatAkun("mahasiswa@example.test", auth.RoleMahasiswa, auth.StatusInvited)
	id, kode := h.mintaOTP(sender, "mahasiswa@example.test")

	var sukses sync.Map
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := h.verifikasiOTP(id, kode)
			if r.Status == http.StatusOK {
				sukses.Store(struct{}{}, true)
			} else if r.Status != http.StatusUnauthorized {
				t.Errorf("status verifikasi konkuren = %d", r.Status)
			}
		}()
	}
	wg.Wait()
	var jumlah int
	sukses.Range(func(_, _ any) bool { jumlah++; return true })
	if jumlah != 1 {
		t.Fatalf("verifikasi sukses = %d, ingin tepat 1", jumlah)
	}
	var sesi int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM session_tokens WHERE user_id = ?`, userID).Scan(&sesi); err != nil {
		t.Fatal(err)
	}
	if sesi != 1 {
		t.Fatalf("sesi terbit = %d, ingin 1", sesi)
	}
}

func TestEmailOTPSMTPFailureTidakMengaktifkanAkun(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{gagal: true}
	pasangEmailOTP(t, h, sender)
	userID := h.buatAkun("dosen@example.test", auth.RoleDosen, auth.StatusInvited)

	r := h.kirimTLS(http.MethodPost, "/api/v1/auth/email/request",
		map[string]any{"email": "dosen@example.test"})
	data := h.harapSukses(r, http.StatusAccepted)
	id, _ := data["challenge_id"].(string)
	h.tungguDelivery(id, "failed")

	// Worker menyimpan kode hanya di memori; gunakan kode yang pasti salah dan
	// buktikan akun tetap invited tanpa sesi meski percobaan dilakukan.
	sender.mu.Lock()
	kode := "00000000"
	sender.mu.Unlock()
	h.harapGalat(h.verifikasiOTP(id, kode), http.StatusUnauthorized, "OTP_INVALID")

	var status string
	if err := h.db.QueryRow(`SELECT status FROM users WHERE id = ?`, userID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(auth.StatusInvited) {
		t.Fatalf("status akun = %q, ingin invited", status)
	}
	var sesi int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM session_tokens`).Scan(&sesi); err != nil {
		t.Fatal(err)
	}
	if sesi != 0 {
		t.Fatalf("SMTP gagal menerbitkan %d sesi", sesi)
	}
}

func TestEmailOTPRateLimitIPDanRetryAfter(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	for i := range 30 {
		r := h.kirimTLS(http.MethodPost, "/api/v1/auth/email/request",
			map[string]any{"email": "tidak-ada-" + itoa(i) + "@example.test"})
		if r.Status != http.StatusAccepted {
			t.Fatalf("permintaan ke-%d status %d: %s", i+1, r.Status, r.Rec.Body.String())
		}
	}
	r := h.kirimTLS(http.MethodPost, "/api/v1/auth/email/request",
		map[string]any{"email": "lain@example.test"})
	h.harapGalat(r, http.StatusTooManyRequests, "RATE_LIMITED")
	if r.Rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 tanpa header Retry-After")
	}
}

func TestEmailOTPSessionLifecycleDanSuspended(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	userID := h.buatAkun("dosen@facelink.online", auth.RoleDosen, auth.StatusInvited)
	id, kode := h.mintaOTP(sender, "dosen@facelink.online")

	data := h.harapSukses(h.verifikasiOTP(id, kode), http.StatusOK)
	akses, _ := data["access_token"].(string)
	segarkan, _ := data["refresh_token"].(string)
	if akses == "" || segarkan == "" {
		t.Fatal("respons verifikasi tidak memuat token sesi")
	}
	me := h.harapSukses(h.kirim(http.MethodGet, "/api/v1/me", nil, akses), http.StatusOK)
	if me["role"] != string(auth.RoleDosen) {
		t.Fatalf("/me role = %v", me["role"])
	}
	rotasi := h.harapSukses(h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": segarkan}, ""), http.StatusOK)
	aksesBaru, _ := rotasi["access_token"].(string)
	segarkanBaru, _ := rotasi["refresh_token"].(string)
	if aksesBaru == "" || segarkanBaru == "" || aksesBaru == akses {
		t.Fatal("rotasi tidak menerbitkan pasangan token baru")
	}
	// Access lama langsung tidak berlaku; logout memakai penyegar hasil rotasi.
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me", nil, akses), http.StatusUnauthorized, codeUnauthenticated)
	if r := h.kirim(http.MethodPost, "/api/v1/auth/logout", map[string]any{"refresh_token": segarkanBaru}, ""); r.Status != http.StatusNoContent {
		t.Fatalf("logout status = %d, ingin 204: %s", r.Status, r.Rec.Body.String())
	}
	h.harapGalat(h.kirim(http.MethodGet, "/api/v1/me", nil, aksesBaru), http.StatusUnauthorized, codeSessionRevoked)
	h.harapGalat(h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": segarkanBaru}, ""), http.StatusUnauthorized, codeUnauthenticated)

	// Kode yang sudah dipakai tidak dapat menghidupkan sesi baru.
	h.harapGalat(h.verifikasiOTP(id, kode), http.StatusUnauthorized, "OTP_INVALID")

	// Challenge baru lalu akun ditangguhkan sebelum verifikasi.
	h.clk.Advance(time.Minute)
	id2, kode2 := h.mintaOTP(sender, "dosen@facelink.online")
	if _, err := h.db.Exec(`UPDATE users SET status = 'suspended' WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	h.harapGalat(h.verifikasiOTP(id2, kode2), http.StatusUnauthorized, "OTP_INVALID")
}

func TestEmailOTPTidakMembocorkanRahasia(t *testing.T) {
	h := newHarnessAuth(t)
	sender := &senderUji{}
	pasangEmailOTP(t, h, sender)
	h.buatAkun("admin@facelink.online", auth.RoleAdmin, auth.StatusActive)
	id, kode := h.mintaOTP(sender, "admin@facelink.online")
	data := h.harapSukses(h.verifikasiOTP(id, kode), http.StatusOK)
	akses, _ := data["access_token"].(string)

	var verifier []byte
	if err := h.db.QueryRow(`SELECT verifier FROM email_otp_challenges WHERE id = ?`, id).Scan(&verifier); err != nil {
		t.Fatal(err)
	}
	if len(verifier) != 32 || strings.Contains(hex.EncodeToString(verifier), kode) {
		t.Fatal("verifier bukan HMAC 32 byte atau memuat kode mentah")
	}
	for _, rahasia := range []string{kode, "admin@facelink.online", akses} {
		if strings.Contains(h.logs.String(), rahasia) {
			t.Fatalf("log backend memuat rahasia/PII: %q", rahasia)
		}
	}
	var audit string
	if err := h.db.QueryRow(
		`SELECT COALESCE(group_concat(action || ':' || COALESCE(metadata_json,''), ' '), '') FROM audit_events`,
	).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	for _, rahasia := range []string{kode, "admin@facelink.online", akses} {
		if strings.Contains(audit, rahasia) {
			t.Fatalf("audit memuat rahasia/PII: %q", rahasia)
		}
	}
}

// Pembantu kecil agar berkas uji tidak bergantung pada detail enkoding respons.
func samaPeta(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}