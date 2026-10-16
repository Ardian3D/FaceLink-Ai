package authz

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/auth"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

// direktoriUji adalah Directory buatan pengujian: kebijakan diuji tanpa
// database sehingga penolakan dapat dibuktikan secara tepat.
type direktoriUji struct {
	dosenKelas   map[string]bool // "userID|classID"
	anggotaKelas map[string]bool // "personID|classID"
	lingkupDosen map[string]bool // "userID|personID"
	personAkun   map[string]string
}

func newDirektoriUji() *direktoriUji {
	return &direktoriUji{
		dosenKelas:   map[string]bool{},
		anggotaKelas: map[string]bool{},
		lingkupDosen: map[string]bool{},
		personAkun:   map[string]string{},
	}
}

func (d *direktoriUji) IsLecturerAssigned(_ context.Context, userID, classID string) (bool, error) {
	return d.dosenKelas[userID+"|"+classID], nil
}

func (d *direktoriUji) IsClassMember(_ context.Context, personID, classID string) (bool, error) {
	return d.anggotaKelas[personID+"|"+classID], nil
}

func (d *direktoriUji) IsPersonInLecturerScope(_ context.Context, userID, personID string) (bool, error) {
	return d.lingkupDosen[userID+"|"+personID], nil
}

func (d *direktoriUji) PersonForUser(_ context.Context, userID string) (string, bool, error) {
	personID, ok := d.personAkun[userID]
	return personID, ok, nil
}

func TestKebijakanHanyaAdmin(t *testing.T) {
	admin := Principal{UserID: "a1", Role: auth.RoleAdmin}
	dosen := Principal{UserID: "d1", Role: auth.RoleDosen}
	mahasiswa := Principal{UserID: "m1", Role: auth.RoleMahasiswa}

	for nama, izin := range map[string]func(Principal) error{
		"kelola akun":      CanManageAccounts,
		"kelola kelas":     CanManageClasses,
		"kelola perangkat": CanManageDevices,
		"kelola biometrik": CanManageBiometrics,
		"baca audit":       CanViewAuditEvents,
		"ekspor laporan":   CanExportAttendance,
	} {
		if err := izin(admin); err != nil {
			t.Errorf("%s: admin ditolak: %v", nama, err)
		}
		if err := izin(dosen); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: dosen seharusnya 403, dapat %v", nama, err)
		}
		if err := izin(mahasiswa); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: mahasiswa seharusnya 403, dapat %v", nama, err)
		}
	}
}

func TestKebijakanCakupanKelas(t *testing.T) {
	dir := newDirektoriUji()
	dir.dosenKelas["d1|c1"] = true
	ctx := context.Background()

	admin := Principal{UserID: "a1", Role: auth.RoleAdmin}
	dosenDitugaskan := Principal{UserID: "d1", Role: auth.RoleDosen}
	dosenLain := Principal{UserID: "d2", Role: auth.RoleDosen}
	mahasiswa := Principal{UserID: "m1", Role: auth.RoleMahasiswa}

	if err := CanOperateClass(ctx, dir, admin, "c1"); err != nil {
		t.Errorf("admin harus dapat mengoperasikan kelas apa pun: %v", err)
	}
	if err := CanOperateClass(ctx, dir, dosenDitugaskan, "c1"); err != nil {
		t.Errorf("dosen yang ditugaskan harus diizinkan: %v", err)
	}
	if err := CanOperateClass(ctx, dir, dosenLain, "c1"); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("dosen lain harus 404 (bukan 403), dapat %v", err)
	}
	if err := CanOperateClass(ctx, dir, mahasiswa, "c1"); !errors.Is(err, ErrForbidden) {
		t.Errorf("mahasiswa tidak boleh mengoperasikan kelas, dapat %v", err)
	}
}

func TestKebijakanMelihatPeserta(t *testing.T) {
	dir := newDirektoriUji()
	dir.lingkupDosen["d1|p1"] = true
	dir.personAkun["m1"] = "p1"
	ctx := context.Background()

	admin := Principal{UserID: "a1", Role: auth.RoleAdmin}
	dosenDitugaskan := Principal{UserID: "d1", Role: auth.RoleDosen}
	dosenLain := Principal{UserID: "d2", Role: auth.RoleDosen}
	mahasiswa := Principal{UserID: "m1", Role: auth.RoleMahasiswa}
	mahasiswaLain := Principal{UserID: "m2", Role: auth.RoleMahasiswa}

	if err := CanViewPerson(ctx, dir, admin, "p1"); err != nil {
		t.Errorf("admin harus dapat melihat peserta mana pun: %v", err)
	}
	if err := CanViewPerson(ctx, dir, dosenDitugaskan, "p1"); err != nil {
		t.Errorf("dosen pada kelas yang ditugaskan harus diizinkan: %v", err)
	}
	if err := CanViewPerson(ctx, dir, dosenLain, "p1"); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("dosen di luar penugasan harus 404, dapat %v", err)
	}
	if err := CanViewPerson(ctx, dir, mahasiswa, "p1"); err != nil {
		t.Errorf("mahasiswa harus dapat melihat datanya sendiri: %v", err)
	}
	if err := CanViewPerson(ctx, dir, mahasiswa, "p2"); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("mahasiswa melihat peserta lain harus 404, dapat %v", err)
	}
	if err := CanViewPerson(ctx, dir, mahasiswaLain, "p2"); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("akun tanpa pengaitan harus 404 di lapisan kebijakan, dapat %v", err)
	}
}

func TestKebijakanMahasiswaHanyaKelasYangDiikuti(t *testing.T) {
	dir := newDirektoriUji()
	dir.personAkun["m1"] = "p1"
	dir.anggotaKelas["p1|c1"] = true
	ctx := context.Background()

	mahasiswa := Principal{UserID: "m1", Role: auth.RoleMahasiswa}
	dosen := Principal{UserID: "d1", Role: auth.RoleDosen}

	if err := CanViewClassAsStudent(ctx, dir, mahasiswa, "c1"); err != nil {
		t.Errorf("kelas yang diikuti harus terlihat: %v", err)
	}
	if err := CanViewClassAsStudent(ctx, dir, mahasiswa, "c2"); !errors.Is(err, ErrOutOfScope) {
		t.Errorf("kelas lain harus 404, dapat %v", err)
	}
	if err := CanViewClassAsStudent(ctx, dir, dosen, "c1"); !errors.Is(err, ErrForbidden) {
		t.Errorf("dosen tidak memakai jalur mahasiswa, dapat %v", err)
	}
}

// TestSQLDirectoryMembacaCakupanNyata memastikan adapter SQL membaca tabel
// cakupan dengan benar, termasuk baris yang sudah tidak aktif.
func TestSQLDirectoryMembacaCakupanNyata(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "facelink-authz.db"))
	if err != nil {
		t.Fatalf("Open() gagal: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	dir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolusi jalur migrasi gagal: %v", err)
	}
	if _, err := migrations.Run(ctx, db, dir); err != nil {
		t.Fatalf("migrasi gagal: %v", err)
	}

	seed := []string{
		`INSERT INTO users (id, email, display_name, role, status) VALUES ('d1','dosen@contoh.test','Dosen','dosen','active')`,
		`INSERT INTO users (id, email, display_name, role, status) VALUES ('a1','admin@contoh.test','Admin','admin','active')`,
		`INSERT INTO users (id, email, display_name, role, status) VALUES ('m1','mhs@contoh.test','Mahasiswa','mahasiswa','active')`,
		`INSERT INTO classes (id, code, title) VALUES ('c1','KLS1','Kelas Satu'), ('c2','KLS2','Kelas Dua')`,
		`INSERT INTO people (id, external_code, display_name) VALUES ('p1','P001','Peserta Satu'), ('p2','P002','Peserta Dua')`,
		`INSERT INTO class_lecturers (class_id, user_id, assigned_by) VALUES ('c1','d1','a1')`,
		`INSERT INTO class_memberships (class_id, person_id, created_by) VALUES ('c1','p1','a1')`,
		`INSERT INTO people (id, external_code, display_name, account_user_id) VALUES ('p3','P003','Peserta Tiga','m1')`,
		`UPDATE class_memberships SET left_at = '2026-10-16T00:00:00.000Z' WHERE class_id = 'c1' AND person_id = 'p1'`,
	}
	for _, stmt := range seed {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("penyiapan data gagal pada %q: %v", stmt, err)
		}
	}

	sqlDir := NewSQLDirectory(db)

	if ok, err := sqlDir.IsLecturerAssigned(ctx, "d1", "c1"); err != nil || !ok {
		t.Errorf("dosen d1 seharusnya ditugaskan pada c1 (ok=%v err=%v)", ok, err)
	}
	if ok, _ := sqlDir.IsLecturerAssigned(ctx, "d1", "c2"); ok {
		t.Error("d1 tidak ditugaskan pada c2")
	}
	if ok, _ := sqlDir.IsClassMember(ctx, "p1", "c1"); ok {
		t.Error("keanggotaan p1 sudah diakhiri (left_at terisi) sehingga harus false")
	}
	if ok, _ := sqlDir.IsPersonInLecturerScope(ctx, "d1", "p1"); ok {
		t.Error("p1 bukan lagi anggota aktif kelas yang ditugaskan ke d1")
	}
	if personID, ok, err := sqlDir.PersonForUser(ctx, "m1"); err != nil || !ok || personID != "p3" {
		t.Errorf("pengaitan akun m1 = (%q,%v,%v), ingin p3", personID, ok, err)
	}
	if _, ok, _ := sqlDir.PersonForUser(ctx, "d1"); ok {
		t.Error("akun dosen tidak boleh memiliki pengaitan peserta")
	}
}
