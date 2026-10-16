package httpapi

// Pengujian otorisasi (RBAC dan cakupan) serta kebersihan log/audit FL-011.
//
// Harness bersama ada di authflow_test.go: basis data SQLite sementara
// termigrasi, penyedia Google tiruan (JWKS + pembuat ID token RS256), jam palsu,
// dan router produksi. Setiap skenario menembak HTTP lewat NewRouter dan
// memeriksa kode status sekaligus kode galat pada badan respons.

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// cariNilaiLarik melaporkan apakah ada entri larik objek dengan field bernilai
// nilai yang dicari.
func cariNilaiLarik(larik []any, field, nilai string) bool {
	for _, entri := range larik {
		objek, ok := entri.(map[string]any)
		if !ok {
			continue
		}
		if v, _ := objek[field].(string); v == nilai {
			return true
		}
	}
	return false
}

// aksiAudit mengembalikan seluruh aksi yang tercatat pada tabel audit_events.
func (h *harnessAuth) aksiAudit() []string {
	h.t.Helper()

	baris, err := h.db.QueryContext(context.Background(), `SELECT action FROM audit_events`)
	if err != nil {
		h.t.Fatalf("membaca aksi audit gagal: %v", err)
	}
	defer baris.Close()

	var daftar []string
	for baris.Next() {
		var aksi string
		if err := baris.Scan(&aksi); err != nil {
			h.t.Fatalf("memindai aksi audit gagal: %v", err)
		}
		daftar = append(daftar, aksi)
	}
	if err := baris.Err(); err != nil {
		h.t.Fatalf("penelusuran aksi audit gagal: %v", err)
	}
	return daftar
}

// metadataAudit mengembalikan seluruh metadata_json pada tabel audit_events;
// NULL dibaca sebagai string kosong.
func (h *harnessAuth) metadataAudit() []string {
	h.t.Helper()

	baris, err := h.db.QueryContext(context.Background(),
		`SELECT COALESCE(metadata_json, '') FROM audit_events`)
	if err != nil {
		h.t.Fatalf("membaca metadata audit gagal: %v", err)
	}
	defer baris.Close()

	var daftar []string
	for baris.Next() {
		var isi string
		if err := baris.Scan(&isi); err != nil {
			h.t.Fatalf("memindai metadata audit gagal: %v", err)
		}
		daftar = append(daftar, isi)
	}
	if err := baris.Err(); err != nil {
		h.t.Fatalf("penelusuran metadata audit gagal: %v", err)
	}
	return daftar
}

// ---- skenario C: RBAC dan cakupan -----------------------------------------

// TestDosenDitolakPadaEndpointAdministratif membuktikan endpoint akun tertutup
// bagi dosen dan setiap penolakan tercatat sebagai audit authz.denied
// (docs/AUTHORIZATION.md §7, D-036).
func TestDosenDitolakPadaEndpointAdministratif(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	_, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, "dosen.rbac@kampus.example", "Dosen RBAC", "dosen")

	baca := h.kirim(http.MethodGet, "/api/v1/users", nil, sesiDosen.Access)
	h.harapGalat(baca, http.StatusForbidden, codeForbiddenRole)

	tulis := h.kirim(http.MethodPost, "/api/v1/users", map[string]any{
		"email": "baru@kampus.example", "display_name": "Akun Baru", "role": "mahasiswa",
	}, sesiDosen.Access)
	h.harapGalat(tulis, http.StatusForbidden, codeForbiddenRole)

	if n := h.hitung(`SELECT COUNT(*) FROM users WHERE email = 'baru@kampus.example'`); n != 0 {
		t.Fatalf("akun terbuat dari permintaan yang ditolak = %d, ingin 0", n)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'authz.denied' AND result = 'failure'`); n != 2 {
		t.Fatalf("audit authz.denied = %d, ingin 2", n)
	}

	meta := h.teksKolom(`SELECT metadata_json FROM audit_events WHERE action = 'authz.denied' ORDER BY rowid LIMIT 1`)
	if !strings.Contains(meta, `"reason_code":"forbidden_role"`) {
		t.Fatalf("metadata authz.denied tidak memuat reason_code forbidden_role: %s", meta)
	}
	if !strings.Contains(meta, `"role_before":"dosen"`) {
		t.Fatalf("metadata authz.denied tidak memuat role_before dosen: %s", meta)
	}
	if !strings.Contains(meta, `"path":"/api/v1/users"`) {
		t.Fatalf("metadata authz.denied tidak memuat path permintaan: %s", meta)
	}
}

// TestMahasiswaDitolakMembacaDaftarAkun menegaskan mahasiswa tidak memiliki
// jalur ke data akun lain.
func TestMahasiswaDitolakMembacaDaftarAkun(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	_, sesiMahasiswa := h.undangDanMasuk(sesiAdmin.Access, "mahasiswa.rbac@kampus.example", "Mahasiswa RBAC", "mahasiswa")

	r := h.kirim(http.MethodGet, "/api/v1/users", nil, sesiMahasiswa.Access)
	h.harapGalat(r, http.StatusForbidden, codeForbiddenRole)

	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'authz.denied'`); n != 1 {
		t.Fatalf("audit authz.denied = %d, ingin 1", n)
	}
}

// TestDosenHanyaMelihatKelasYangDitugaskan membuktikan cakupan dosen dibatasi
// penugasan resmi, dan objek di luar cakupan dijawab 404 agar keberadaannya
// tidak terbocor (docs/AUTHORIZATION.md §5).
func TestDosenHanyaMelihatKelasYangDitugaskan(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	dosenID, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, "dosen.kelas@kampus.example", "Dosen Kelas", "dosen")

	kelasDitugaskan := h.buatKelas("IF-101", "Algoritma")
	kelasLain := h.buatKelas("IF-202", "Basis Data")

	tugaskan := h.kirim(http.MethodPut, "/api/v1/classes/"+kelasDitugaskan+"/dosen/"+dosenID, nil, sesiAdmin.Access)
	dataTugas := h.harapSukses(tugaskan, http.StatusCreated)
	if dataTugas["user_id"] != dosenID {
		t.Fatalf("respons penugasan menunjuk %v, ingin %s", dataTugas["user_id"], dosenID)
	}

	ditugaskan := h.kirim(http.MethodGet, "/api/v1/classes/"+kelasDitugaskan+"/dosen", nil, sesiDosen.Access)
	if larik := h.harapSuksesLarik(ditugaskan, http.StatusOK); !cariNilaiLarik(larik, "id", dosenID) {
		t.Fatalf("daftar dosen kelas yang ditugaskan tidak memuat %s: %s", dosenID, ditugaskan.Rec.Body.String())
	}

	diLuarCakupan := h.kirim(http.MethodGet, "/api/v1/classes/"+kelasLain+"/dosen", nil, sesiDosen.Access)
	h.harapGalat(diLuarCakupan, http.StatusNotFound, codeNotFound)

	// Kelas yang sama tetap terbaca admin: 404 di atas memang soal cakupan,
	// bukan kelas yang tidak ada.
	olehAdmin := h.kirim(http.MethodGet, "/api/v1/classes/"+kelasLain+"/dosen", nil, sesiAdmin.Access)
	h.harapSuksesLarik(olehAdmin, http.StatusOK)
}

// TestMeClassesTanpaPengaitanPeserta membuktikan /me/classes menjawab
// ACCOUNT_NOT_LINKED sebelum akun mahasiswa ditautkan ke baris peserta, lalu
// menampilkan kelas setelah admin menautkannya.
func TestMeClassesTanpaPengaitanPeserta(t *testing.T) {
	h := newHarnessAuth(t)
	admin, sesiAdmin := h.adminPertama("admin@kampus.example")
	mahasiswaID, sesiMahasiswa := h.undangDanMasuk(sesiAdmin.Access, "mahasiswa.kelas@kampus.example", "Mahasiswa Kelas", "mahasiswa")

	personID := h.buatPeserta("MHS-001", "Mahasiswa Kelas")
	kelasID := h.buatKelas("IF-303", "Jaringan Komputer")
	h.tambahAnggotaKelas(kelasID, personID, admin.ID)

	sebelum := h.kirim(http.MethodGet, "/api/v1/me/classes", nil, sesiMahasiswa.Access)
	h.harapGalat(sebelum, http.StatusConflict, codeAccountNotLinked)

	tautkan := h.kirim(http.MethodPut, "/api/v1/people/"+personID+"/account-link",
		map[string]any{"user_id": mahasiswaID}, sesiAdmin.Access)
	data := h.harapSukses(tautkan, http.StatusOK)
	if data["person_id"] != personID || data["user_id"] != mahasiswaID {
		t.Fatalf("respons pengaitan = %v, ingin person_id %s dan user_id %s", data, personID, mahasiswaID)
	}

	sesudah := h.kirim(http.MethodGet, "/api/v1/me/classes", nil, sesiMahasiswa.Access)
	if larik := h.harapSuksesLarik(sesudah, http.StatusOK); !cariNilaiLarik(larik, "id", kelasID) {
		t.Fatalf("daftar kelas mahasiswa tidak memuat %s: %s", kelasID, sesudah.Rec.Body.String())
	}
}

// TestMahasiswaDitolakMenautkanAkunPeserta membuktikan pengaitan peserta tetap
// khusus admin (D-034).
func TestMahasiswaDitolakMenautkanAkunPeserta(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	mahasiswaID, sesiMahasiswa := h.undangDanMasuk(sesiAdmin.Access, "mahasiswa.taut@kampus.example", "Mahasiswa Taut", "mahasiswa")
	personID := h.buatPeserta("MHS-002", "Mahasiswa Taut")

	r := h.kirim(http.MethodPut, "/api/v1/people/"+personID+"/account-link",
		map[string]any{"user_id": mahasiswaID}, sesiMahasiswa.Access)
	h.harapGalat(r, http.StatusForbidden, codeForbiddenRole)

	if tertaut := h.teksKolom(`SELECT COALESCE(account_user_id, '') FROM people WHERE id = ?`, personID); tertaut != "" {
		t.Fatalf("akun peserta tertaut menjadi %q, ingin tetap kosong", tertaut)
	}
}

// TestPerubahanAkunSendiriDilindungi membuktikan dua penjaga berbeda pada
// PATCH /users/{id} dan memastikan kode yang dijawab sesuai kontrak §5.2:
//
//   - selama akun itu satu-satunya admin aktif, jawabannya
//     LAST_ADMIN_PROTECTED — termasuk ketika pelakunya akun itu sendiri, karena
//     aturan yang sesungguhnya dilanggar adalah hilangnya admin terakhir
//     (skenario TA-21);
//   - setelah ada admin aktif kedua, jawabannya SELF_ACCOUNT_PROTECTED.
//
// Keadaan akhir selalu diperiksa supaya tidak ada perubahan yang lolos.
func TestPerubahanAkunSendiriDilindungi(t *testing.T) {
	h := newHarnessAuth(t)
	admin, sesiAdmin := h.adminPertama("admin.terakhir@kampus.example")

	turunkan := h.kirim(http.MethodPatch, "/api/v1/users/"+admin.ID,
		map[string]any{"role": "dosen"}, sesiAdmin.Access)
	h.harapGalat(turunkan, http.StatusConflict, codeLastAdminProtected)

	nonaktifkan := h.kirim(http.MethodPatch, "/api/v1/users/"+admin.ID,
		map[string]any{"status": "suspended"}, sesiAdmin.Access)
	h.harapGalat(nonaktifkan, http.StatusConflict, codeLastAdminProtected)

	if role := h.teksKolom(`SELECT role FROM users WHERE id = ?`, admin.ID); role != "admin" {
		t.Fatalf("role akun setelah penolakan = %q, ingin tetap admin", role)
	}
	if status := h.teksKolom(`SELECT status FROM users WHERE id = ?`, admin.ID); status != "active" {
		t.Fatalf("status akun setelah penolakan = %q, ingin tetap active", status)
	}

	// Admin aktif kedua membuat aturan "admin terakhir" tidak lagi berlaku,
	// sehingga yang menolak adalah aturan akun sendiri.
	h.undangDanMasuk(sesiAdmin.Access, "admin.kedua@kampus.example", "Admin Kedua", "admin")

	turunkanSendiri := h.kirim(http.MethodPatch, "/api/v1/users/"+admin.ID,
		map[string]any{"role": "dosen"}, sesiAdmin.Access)
	h.harapGalat(turunkanSendiri, http.StatusConflict, codeSelfAccountProtected)

	nonaktifkanSendiri := h.kirim(http.MethodPatch, "/api/v1/users/"+admin.ID,
		map[string]any{"status": "suspended"}, sesiAdmin.Access)
	h.harapGalat(nonaktifkanSendiri, http.StatusConflict, codeSelfAccountProtected)

	if role := h.teksKolom(`SELECT role FROM users WHERE id = ?`, admin.ID); role != "admin" {
		t.Fatalf("role akun setelah penolakan kedua = %q, ingin tetap admin", role)
	}
	if status := h.teksKolom(`SELECT status FROM users WHERE id = ?`, admin.ID); status != "active" {
		t.Fatalf("status akun setelah penolakan kedua = %q, ingin tetap active", status)
	}
}

// TestPengaitanPesertaUntukAkunDosenDitolak membuktikan pengaitan peserta hanya
// berlaku untuk akun mahasiswa (422, bukan 403: perannya sah, datanya tidak).
func TestPengaitanPesertaUntukAkunDosenDitolak(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	dosenID, _ := h.undangDanMasuk(sesiAdmin.Access, "dosen.taut@kampus.example", "Dosen Taut", "dosen")
	personID := h.buatPeserta("MHS-003", "Peserta Dosen Taut")

	r := h.kirim(http.MethodPut, "/api/v1/people/"+personID+"/account-link",
		map[string]any{"user_id": dosenID}, sesiAdmin.Access)
	h.harapGalat(r, http.StatusUnprocessableEntity, codeValidationFailed)

	if tertaut := h.teksKolom(`SELECT COALESCE(account_user_id, '') FROM people WHERE id = ?`, personID); tertaut != "" {
		t.Fatalf("akun peserta tertaut menjadi %q, ingin tetap kosong", tertaut)
	}
}

// ---- skenario D: kebersihan log dan audit ---------------------------------

// TestLogDanAuditTidakMembocorkanRahasia menjalankan rangkaian alur nyata
// (login, penyegaran, penolakan identitas, penolakan peran, logout) lalu
// memastikan penampung log dan metadata audit tidak memuat token, ID token,
// email, subjek, maupun nonce.
func TestLogDanAuditTidakMembocorkanRahasia(t *testing.T) {
	h := newHarnessAuth(t)

	emailAdmin := "admin.log@kampus.example"
	_, sesiAdmin := h.adminPertama(emailAdmin)

	emailDosen := "dosen.log@kampus.example"
	dosenID, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, emailDosen, "Dosen Log", "dosen")

	segarkan := h.kirim(http.MethodPost, "/api/v1/auth/refresh",
		map[string]any{"refresh_token": sesiDosen.Refresh}, "")
	sesiBaru := h.sesiDari(h.harapSukses(segarkan, http.StatusOK))

	// Login dari email yang tidak diundang: identitas ditolak, bukan dibuat.
	emailAsing := "orang.asing.log@kampus.example"
	nonceAsing := h.nonceBaru()
	tokenAsing := h.tokenID(map[string]any{
		"sub": "sub-asing-log", "email": emailAsing, "nonce": nonceAsing,
	})
	tolakIdentitas := h.kirim(http.MethodPost, "/api/v1/auth/google",
		map[string]any{"id_token": tokenAsing, "nonce": nonceAsing}, "")
	h.harapGalat(tolakIdentitas, http.StatusForbidden, codeIdentityNotLinked)

	// Penolakan peran: dosen menyentuh jalur administratif.
	tolakPeran := h.kirim(http.MethodGet, "/api/v1/users", nil, sesiBaru.Access)
	h.harapGalat(tolakPeran, http.StatusForbidden, codeForbiddenRole)

	logout := h.kirim(http.MethodPost, "/api/v1/auth/logout", nil, sesiBaru.Access)
	if logout.Status != http.StatusNoContent {
		t.Fatalf("logout = %d, ingin 204; badan: %s", logout.Status, logout.Rec.Body.String())
	}

	catatan := h.logs.String()
	if strings.TrimSpace(catatan) == "" {
		t.Fatal("penampung log kosong: pengujian tidak membuktikan apa pun")
	}
	if !strings.Contains(catatan, "permintaan http") {
		t.Fatalf("log akses tidak tercatat: %s", catatan)
	}

	rahasia := map[string]string{
		"access token admin":     sesiAdmin.Access,
		"refresh token admin":    sesiAdmin.Refresh,
		"access token dosen":     sesiDosen.Access,
		"refresh token dosen":    sesiDosen.Refresh,
		"access token tersegar":  sesiBaru.Access,
		"refresh token tersegar": sesiBaru.Refresh,
		"ID token ditolak":       tokenAsing,
		"email admin":            emailAdmin,
		"email dosen":            emailDosen,
		"email tak diundang":     emailAsing,
		"subjek Google":          "sub-asing-log",
		"identitas akun":         dosenID,
	}
	for nama, nilai := range rahasia {
		if nilai == "" {
			t.Fatalf("uji internal: nilai rahasia %q kosong", nama)
		}
		if strings.Contains(catatan, nilai) {
			t.Errorf("log memuat %s (%q)\nlog: %s", nama, nilai, catatan)
		}
	}
	// Nilai nonce tidak boleh muncul di log — sebagai klaim maupun sebagai
	// nilai yang dikirim klien. Nama rute `/auth/nonce` boleh tercatat karena
	// access log memang memuat jalur permintaan; yang dilarang adalah nilainya.
	nonceTerpakai := []string{nonceAsing}
	for _, nonce := range nonceTerpakai {
		if nonce == "" {
			t.Fatal("uji internal: nilai nonce kosong")
		}
		if strings.Contains(catatan, nonce) {
			t.Errorf("log memuat nilai nonce (%q)\nlog: %s", nonce, catatan)
		}
	}
	if strings.Contains(catatan, "Bearer") || strings.Contains(catatan, "Authorization") {
		t.Errorf("log memuat tajuk otorisasi:\n%s", catatan)
	}

	// Metadata audit: setiap aksi tercatat, isinya bebas rahasia.
	aksi := h.aksiAudit()
	diharapkan := []string{
		"auth.google.succeeded", "auth.google.linked", "user.invited",
		"auth.session.refreshed", "auth.login.denied", "authz.denied", "auth.logout",
	}
	for _, ingin := range diharapkan {
		ditemukan := false
		for _, ada := range aksi {
			if ada == ingin {
				ditemukan = true
				break
			}
		}
		if !ditemukan {
			t.Errorf("audit tidak memuat aksi %q; aksi tercatat: %v", ingin, aksi)
		}
	}

	for _, meta := range h.metadataAudit() {
		if strings.TrimSpace(meta) == "" {
			continue
		}
		for nama, nilai := range rahasia {
			if strings.Contains(meta, nilai) {
				t.Errorf("metadata audit memuat %s (%q): %s", nama, nilai, meta)
			}
		}
	}
}

// TestDaftarSesiTidakMengembalikanHashToken membuktikan endpoint administrasi
// daftar sesi hanya mengembalikan metadata sesi, bukan hash atau token.
func TestDaftarSesiTidakMengembalikanHashToken(t *testing.T) {
	h := newHarnessAuth(t)
	_, sesiAdmin := h.adminPertama("admin@kampus.example")
	dosenID, sesiDosen := h.undangDanMasuk(sesiAdmin.Access, "dosen.sesi@kampus.example", "Dosen Sesi", "dosen")

	r := h.kirim(http.MethodGet, "/api/v1/auth/sessions?user_id="+dosenID, nil, sesiAdmin.Access)
	larik := h.harapSuksesLarik(r, http.StatusOK)
	if len(larik) != 1 {
		t.Fatalf("jumlah sesi = %d, ingin 1; badan: %s", len(larik), r.Rec.Body.String())
	}
	sesi, ok := larik[0].(map[string]any)
	if !ok {
		t.Fatalf("entri sesi bukan objek: %v", larik[0])
	}
	if sesi["id"] != sesiDosen.SesiID || sesi["user_id"] != dosenID {
		t.Fatalf("entri sesi = %v, ingin id %s dan user_id %s", sesi, sesiDosen.SesiID, dosenID)
	}
	for _, wajib := range []string{"created_at", "absolute_expires_at"} {
		if nilai, _ := sesi[wajib].(string); nilai == "" {
			t.Fatalf("entri sesi tidak memuat %q: %v", wajib, sesi)
		}
	}
	for kunci := range sesi {
		rendah := strings.ToLower(kunci)
		if strings.Contains(rendah, "hash") || strings.Contains(rendah, "token") {
			t.Fatalf("entri sesi memuat bidang rahasia %q: %v", kunci, sesi)
		}
	}

	badan := r.Rec.Body.String()
	for nama, nilai := range map[string]string{
		"access token korban":  sesiDosen.Access,
		"refresh token korban": sesiDosen.Refresh,
		"access token admin":   sesiAdmin.Access,
	} {
		if strings.Contains(badan, nilai) {
			t.Errorf("respons daftar sesi memuat %s", nama)
		}
	}
	if strings.Contains(strings.ToLower(badan), "hash") {
		t.Errorf("respons daftar sesi menyebut hash: %s", badan)
	}
	if n := h.hitung(`SELECT COUNT(*) FROM audit_events WHERE action = 'auth.session.revoked'`); n != 0 {
		t.Fatalf("membaca daftar sesi mencabut sesi: audit auth.session.revoked = %d, ingin 0", n)
	}
}
