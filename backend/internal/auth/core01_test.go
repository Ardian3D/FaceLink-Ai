package auth

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

func core01Admin(t *testing.T, s *Store, db *database.DB) User {
	t.Helper()
	admin := undangAdmin(t, s)
	aktifkanAdmin(t, s, db, admin.ID)
	return admin
}

func core01User(t *testing.T, s *Store, db *database.DB, adminID, label string, role Role, status Status) User {
	t.Helper()
	u, err := s.InviteUser(context.Background(), label+"@example.invalid", label, role, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE users SET status = ? WHERE id = ?`, status, u.ID); err != nil {
		t.Fatal(err)
	}
	u.Status = status
	return u
}

func core01Person(t *testing.T, s *Store, adminID, code string) Person {
	t.Helper()
	p, err := s.CreatePerson(context.Background(), adminID, code, "Peserta Sintetis", "", "campus-roster-"+code)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCore01ExplicitIdentityAttestation(t *testing.T) {
	s, db, jam := bukaStore(t)
	ctx := context.Background()
	admin := core01Admin(t, s, db)
	student := core01User(t, s, db, admin.ID, "synthetic-student", RoleMahasiswa, StatusActive)
	p, err := s.CreatePerson(ctx, admin.ID, "SYN-001", "Peserta Sintetis", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.IdentityVerifiedAt != nil || p.IdentityReference != "" {
		t.Fatalf("pencatatan tanpa referensi dianggap terverifikasi: %+v", p)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); !errors.Is(err, ErrIdentityNotVerified) {
		t.Fatalf("binding tanpa attestation = %v", err)
	}
	name := "Nama Sintetis Baru"
	p, err = s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{DisplayName: &name})
	if err != nil || p.IdentityVerifiedAt != nil {
		t.Fatalf("perubahan nama tidak boleh memverifikasi: %+v, %v", p, err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events WHERE action = 'person.identity_verified'`); got != 0 {
		t.Fatalf("audit verifikasi tanpa referensi = %d", got)
	}
	ref := "opaque-campus-ledger-page-17-row-2"
	jam.Advance(time.Minute)
	p, err = s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{IdentityReference: &ref})
	if err != nil || p.IdentityVerifiedAt == nil || !p.IdentityVerifiedAt.Equal(waktuUji.Add(time.Minute)) {
		t.Fatalf("attestation eksplisit = %+v, %v", p, err)
	}
	before := hitung(t, db, `SELECT COUNT(*) FROM audit_events`)
	if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{IdentityReference: &ref}); err != nil {
		t.Fatal(err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events`); got != before {
		t.Fatalf("attestation sama tidak idempoten: %d -> %d", before, got)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events WHERE action = 'person.account_linked'`); got != 1 {
		t.Fatalf("audit binding idempoten = %d", got)
	}
	for _, replacement := range []string{"different-campus-record", ""} {
		if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{IdentityReference: &replacement}); !errors.Is(err, ErrConflict) {
			t.Fatalf("referensi peserta tertaut dapat diganti %q: %v", replacement, err)
		}
	}
	if id, linked, err := s.PersonForUser(ctx, student.ID); err != nil || !linked || id != p.ID {
		t.Fatalf("self binding = %q, %v, %v", id, linked, err)
	}
	if err := s.UnlinkPersonAccount(ctx, admin.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	empty := ""
	p, err = s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{IdentityReference: &empty})
	if err != nil || p.IdentityVerifiedAt != nil || p.IdentityReference != "" {
		t.Fatalf("pencabutan referensi tidak mengosongkan attestation: %+v, %v", p, err)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); !errors.Is(err, ErrIdentityNotVerified) {
		t.Fatalf("binding setelah referensi dicabut = %v", err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events WHERE action = 'person.identity_verified'`); got != 1 {
		t.Fatalf("pencabutan referensi membuat audit verified: %d", got)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM consent_records`); got != 0 {
		t.Fatalf("roster menulis consent: %d", got)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM face_enrollments`); got != 0 {
		t.Fatalf("roster menulis enrollment: %d", got)
	}
}

func TestCore01PersonValidationAndImmutableNIM(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := core01Admin(t, s, db)
	cases := []struct{ name, code, display, note, reference string }{
		{"empty code", "", "Peserta", "", ""},
		{"long code", strings.Repeat("a", 33), "Peserta", "", ""},
		{"unicode code", "NIM-é", "Peserta", "", ""},
		{"punctuation code", "NIM_1", "Peserta", "", ""},
		{"empty name", "SYN-1", " ", "", ""},
		{"long name", "SYN-1", strings.Repeat("é", 121), "", ""},
		{"control name", "SYN-1", "Nama\x00Peserta", "", ""},
		{"long note", "SYN-1", "Peserta", strings.Repeat("é", 201), ""},
		{"control note", "SYN-1", "Peserta", "catatan\ninternal", ""},
		{"long reference", "SYN-1", "Peserta", "", strings.Repeat("é", 121)},
		{"control reference", "SYN-1", "Peserta", "", "campus\x00reference"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.CreatePerson(ctx, admin.ID, tc.code, tc.display, tc.note, tc.reference); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("CreatePerson = %v", err)
			}
		})
	}
	p, err := s.CreatePerson(ctx, admin.ID, strings.Repeat("A", 32), strings.Repeat("é", 120), strings.Repeat("é", 200), strings.Repeat("é", 120))
	if err != nil {
		t.Fatalf("batas karakter Unicode yang sah ditolak: %v", err)
	}
	if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PATCH kosong = %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE people SET external_code = 'NEW-NIM' WHERE id = ?`, p.ID); err == nil {
		t.Fatal("database mengizinkan perubahan NIM")
	}
	if err := s.DeletePerson(ctx, admin.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePerson(ctx, admin.ID, p.ExternalCode, "Peserta Lain", "", ""); !errors.Is(err, ErrDuplicateExternalCode) {
		t.Fatalf("NIM peserta dihapus dipakai ulang: %v", err)
	}
	if _, err := s.PersonByID(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("detail peserta terhapus = %v", err)
	}
	if err := s.DeletePerson(ctx, admin.ID, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("hapus ulang = %v", err)
	}
}

func TestCore01BindingRequiresActiveVerifiedSubjects(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := core01Admin(t, s, db)
	p := core01Person(t, s, admin.ID, "SYN-BIND")
	for _, status := range []Status{StatusInvited, StatusSuspended} {
		u := core01User(t, s, db, admin.ID, "student-"+string(status), RoleMahasiswa, status)
		if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, u.ID); !errors.Is(err, ErrAccountInactive) {
			t.Fatalf("binding akun %s = %v", status, err)
		}
	}
	student := core01User(t, s, db, admin.ID, "active-student", RoleMahasiswa, StatusActive)
	inactive := "inactive"
	if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{Status: &inactive}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); !errors.Is(err, ErrSubjectInactive) {
		t.Fatalf("binding peserta inactive = %v", err)
	}
	active := "active"
	if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{Status: &active}); err != nil {
		t.Fatal(err)
	}
	// Referensi tanpa timestamp bukan bukti verifikasi dan tidak ditebak dari email.
	if _, err := db.ExecContext(ctx, `UPDATE people SET identity_verified_at = NULL WHERE id = ?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); !errors.Is(err, ErrIdentityNotVerified) {
		t.Fatalf("reference tanpa timestamp = %v", err)
	}
	ref := p.IdentityReference
	if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{IdentityReference: &ref}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	for _, status := range []Status{StatusInvited, StatusSuspended} {
		if _, err := db.ExecContext(ctx, `UPDATE users SET status = ? WHERE id = ?`, status, student.ID); err != nil {
			t.Fatal(err)
		}
		if _, linked, err := s.PersonForUser(ctx, student.ID); err != nil || linked {
			t.Fatalf("self-access akun %s = %v, %v", status, linked, err)
		}
	}
}

func TestCore01MutationsRequireActiveAdmin(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := core01Admin(t, s, db)
	student := core01User(t, s, db, admin.ID, "student", RoleMahasiswa, StatusActive)
	lecturer := core01User(t, s, db, admin.ID, "lecturer", RoleDosen, StatusActive)
	p := core01Person(t, s, admin.ID, "SYN-ACTOR")
	class, err := s.CreateClass(ctx, admin.ID, "SYN-CLASS", "Kelas Sintetis")
	if err != nil {
		t.Fatal(err)
	}
	name := "Nama Baru"
	mutations := []struct {
		name string
		run  func(string) error
	}{
		{"create person", func(a string) error { _, e := s.CreatePerson(ctx, a, "SYN-NEW", "Peserta", "", ""); return e }},
		{"update person", func(a string) error { _, e := s.UpdatePerson(ctx, a, p.ID, PersonUpdate{DisplayName: &name}); return e }},
		{"delete person", func(a string) error { return s.DeletePerson(ctx, a, p.ID) }},
		{"create class", func(a string) error { _, e := s.CreateClass(ctx, a, "SYN-NEW", "Kelas"); return e }},
		{"add member", func(a string) error { _, e := s.AddClassMember(ctx, a, class.ID, p.ID); return e }},
		{"remove member", func(a string) error { return s.RemoveClassMember(ctx, a, class.ID, p.ID) }},
		{"link account", func(a string) error { _, e := s.LinkPersonAccount(ctx, a, p.ID, student.ID); return e }},
		{"unlink account", func(a string) error { return s.UnlinkPersonAccount(ctx, a, p.ID) }},
		{"assign lecturer", func(a string) error { _, e := s.AssignLecturer(ctx, a, class.ID, lecturer.ID); return e }},
		{"remove lecturer", func(a string) error { return s.RemoveLecturer(ctx, a, class.ID, lecturer.ID) }},
	}
	actors := []User{student, lecturer,
		core01User(t, s, db, admin.ID, "invited-admin", RoleAdmin, StatusInvited),
		core01User(t, s, db, admin.ID, "suspended-admin", RoleAdmin, StatusSuspended)}
	before := core01Snapshot(t, db)
	for _, actor := range actors {
		for _, mutation := range mutations {
			t.Run(string(actor.Role)+"/"+string(actor.Status)+"/"+mutation.name, func(t *testing.T) {
				if err := mutation.run(actor.ID); !errors.Is(err, ErrActorUnauthorized) {
					t.Fatalf("mutasi aktor tanpa hak = %v", err)
				}
			})
		}
	}
	// Hak yang berubah setelah pemanggil memuat principal tetap dibaca ulang.
	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'suspended' WHERE id = ?`, admin.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateClass(ctx, admin.ID, "SYN-REVOKED", "Kelas"); !errors.Is(err, ErrActorUnauthorized) {
		t.Fatalf("principal admin kedaluwarsa tetap berwenang: %v", err)
	}
	if after := core01Snapshot(t, db); !reflect.DeepEqual(before, after) {
		t.Fatal("mutasi ditolak mengubah roster/audit")
	}
}

// Dua handle independen menguji kunci SQLite, bukan hanya antrean maxOpenConns.
func core01SecondStore(t *testing.T, db *database.DB, s *Store) *Store {
	t.Helper()
	other, err := database.Open(context.Background(), db.Path())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { other.Close() })
	return NewStore(other, s.clk)
}

func core01Race(a, b func() error) [2]error {
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, fn := range []func() error{a, b} {
		go func(run func() error) { <-start; results <- run() }(fn)
	}
	close(start)
	return [2]error{<-results, <-results}
}

func TestCore01ConcurrentDuplicateNIM(t *testing.T) {
	s, db, _ := bukaStore(t)
	admin := core01Admin(t, s, db)
	other := core01SecondStore(t, db, s)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	create := func(store *Store) func() error {
		return func() error {
			_, err := store.CreatePerson(ctx, admin.ID, "SYN-RACE", "Peserta Sintetis", "", "campus-race")
			return err
		}
	}
	results := core01Race(create(s), create(other))
	if !((results[0] == nil && errors.Is(results[1], ErrDuplicateExternalCode)) ||
		(results[1] == nil && errors.Is(results[0], ErrDuplicateExternalCode))) {
		t.Fatalf("duplicate race = %v", results)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM people WHERE external_code = 'SYN-RACE'`); got != 1 {
		t.Fatalf("duplicate rows = %d", got)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events WHERE action = 'person.created'`); got != 1 {
		t.Fatalf("duplicate audits = %d", got)
	}
}

func TestCore01ConcurrentBindingBothDirections(t *testing.T) {
	for _, samePerson := range []bool{false, true} {
		t.Run(fmt.Sprintf("samePerson=%v", samePerson), func(t *testing.T) {
			s, db, _ := bukaStore(t)
			admin := core01Admin(t, s, db)
			other := core01SecondStore(t, db, s)
			p1 := core01Person(t, s, admin.ID, "SYN-RACE-1")
			p2 := core01Person(t, s, admin.ID, "SYN-RACE-2")
			u1 := core01User(t, s, db, admin.ID, "student-one", RoleMahasiswa, StatusActive)
			u2 := core01User(t, s, db, admin.ID, "student-two", RoleMahasiswa, StatusActive)
			want := ErrConflict
			if samePerson {
				p2 = p1
				want = ErrAlreadyLinked
			} else {
				u2 = u1
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			results := core01Race(
				func() error { _, e := s.LinkPersonAccount(ctx, admin.ID, p1.ID, u1.ID); return e },
				func() error { _, e := other.LinkPersonAccount(ctx, admin.ID, p2.ID, u2.ID); return e })
			if !((results[0] == nil && errors.Is(results[1], want)) || (results[1] == nil && errors.Is(results[0], want))) {
				t.Fatalf("binding race = %v, conflict %v", results, want)
			}
			if got := hitung(t, db, `SELECT COUNT(*) FROM people WHERE account_user_id IS NOT NULL`); got != 1 {
				t.Fatalf("binding race rows = %d", got)
			}
			if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events WHERE action = 'person.account_linked'`); got != 1 {
				t.Fatalf("binding race audits = %d", got)
			}
		})
	}
}

func core01Snapshot(t *testing.T, db *database.DB) map[string]string {
	t.Helper()
	state := make(map[string]string)
	for _, table := range []string{"people", "classes", "class_memberships", "class_lecturers", "audit_events"} {
		rows, err := db.QueryContext(context.Background(), `SELECT * FROM `+table+` ORDER BY 1, 2`)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatal(err)
		}
		var snapshot strings.Builder
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			fmt.Fprintf(&snapshot, "%#v\n", values)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		state[table] = snapshot.String()
	}
	return state
}

func TestCore01AuditFailureRollsBackEveryMutation(t *testing.T) {
	actions := []string{"person.created", "person.identity_verified", "person.identity_verified/update", "person.updated", "person.deleted",
		"class.created", "class.member.added", "class.member.removed", "person.account_linked",
		"person.account_unlinked", "class.dosen_assigned", "class.dosen_removed"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			s, db, _ := bukaStore(t)
			ctx := context.Background()
			admin := core01Admin(t, s, db)
			p := core01Person(t, s, admin.ID, "SYN-ROLLBACK")
			student := core01User(t, s, db, admin.ID, "student", RoleMahasiswa, StatusActive)
			lecturer := core01User(t, s, db, admin.ID, "lecturer", RoleDosen, StatusActive)
			class, err := s.CreateClass(ctx, admin.ID, "SYN-ROLLBACK", "Kelas Sintetis")
			if err != nil {
				t.Fatal(err)
			}
			if action == "class.member.removed" || action == "person.deleted" {
				if _, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); err != nil {
					t.Fatal(err)
				}
			}
			if action == "person.account_unlinked" || action == "person.deleted" {
				if _, err := s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID); err != nil {
					t.Fatal(err)
				}
			}
			if action == "class.dosen_removed" {
				if _, err := s.AssignLecturer(ctx, admin.ID, class.ID, lecturer.ID); err != nil {
					t.Fatal(err)
				}
			}
			// Action berasal dari daftar tetap pengujian, bukan masukan produk.
			auditAction, _, _ := strings.Cut(action, "/")
			if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_core01_audit BEFORE INSERT ON audit_events WHEN NEW.action = '`+auditAction+`' BEGIN SELECT RAISE(ABORT, 'synthetic audit failure'); END`); err != nil {
				t.Fatal(err)
			}
			before := core01Snapshot(t, db)
			switch action {
			case "person.created", "person.identity_verified":
				_, err = s.CreatePerson(ctx, admin.ID, "SYN-NEW", "Peserta Baru", "", "campus-new")
			case "person.identity_verified/update":
				ref := "new-campus-reference"
				_, err = s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{IdentityReference: &ref})
			case "person.updated":
				name := "Nama Berubah"
				_, err = s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{DisplayName: &name})
			case "person.deleted":
				err = s.DeletePerson(ctx, admin.ID, p.ID)
			case "class.created":
				_, err = s.CreateClass(ctx, admin.ID, "SYN-NEW", "Kelas Baru")
			case "class.member.added":
				_, err = s.AddClassMember(ctx, admin.ID, class.ID, p.ID)
			case "class.member.removed":
				err = s.RemoveClassMember(ctx, admin.ID, class.ID, p.ID)
			case "person.account_linked":
				_, err = s.LinkPersonAccount(ctx, admin.ID, p.ID, student.ID)
			case "person.account_unlinked":
				err = s.UnlinkPersonAccount(ctx, admin.ID, p.ID)
			case "class.dosen_assigned":
				_, err = s.AssignLecturer(ctx, admin.ID, class.ID, lecturer.ID)
			case "class.dosen_removed":
				err = s.RemoveLecturer(ctx, admin.ID, class.ID, lecturer.ID)
			}
			if err == nil || !strings.Contains(err.Error(), "synthetic audit failure") {
				t.Fatalf("kegagalan audit tidak diteruskan: %v", err)
			}
			if after := core01Snapshot(t, db); !reflect.DeepEqual(before, after) {
				t.Fatal("audit gagal tetapi roster/audit berubah")
			}
		})
	}
}

func TestCore01MembershipLifecycleAndScopedRoster(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := core01Admin(t, s, db)
	lecturer := core01User(t, s, db, admin.ID, "lecturer", RoleDosen, StatusActive)
	p := core01Person(t, s, admin.ID, "SYN-MEMBER")
	class, err := s.CreateClass(ctx, admin.ID, "SYN-CLASS", "Kelas Sintetis")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateClass(ctx, admin.ID, "SYN-OTHER", "Kelas Lain")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignLecturer(ctx, admin.ID, class.ID, lecturer.ID); err != nil {
		t.Fatal(err)
	}
	if created, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); err != nil || !created {
		t.Fatalf("add = %v, %v", created, err)
	}
	if created, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); err != nil || created {
		t.Fatalf("repeat = %v, %v", created, err)
	}
	if got := hitung(t, db, `SELECT COUNT(*) FROM audit_events WHERE action = 'class.member.added'`); got != 1 {
		t.Fatalf("audit repeat = %d", got)
	}
	if _, err := db.ExecContext(ctx, `UPDATE class_memberships SET left_at = ? WHERE class_id = ? AND person_id = ?`, formatWaktu(waktuUji), class.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if members, err := s.MembersForClass(ctx, class.ID); err != nil || len(members) != 0 {
		t.Fatalf("left member visible = %+v, %v", members, err)
	}
	if created, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); err != nil || !created {
		t.Fatalf("rejoin = %v, %v", created, err)
	}
	if _, err := s.AddClassMember(ctx, admin.ID, other.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListPeople(ctx, PersonQuery{ScopeUserID: lecturer.ID, ClassID: other.ID, Limit: 10}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign class filter = %v", err)
	}
	people, err := s.ListPeople(ctx, PersonQuery{ScopeUserID: lecturer.ID, Limit: 10})
	if err != nil || len(people) != 1 || people[0].ID != p.ID {
		t.Fatalf("scoped people = %+v, %v", people, err)
	}
	if err := s.RemoveClassMember(ctx, admin.ID, class.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveClassMember(ctx, admin.ID, class.ID, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove repeat = %v", err)
	}
	people, err = s.ListPeople(ctx, PersonQuery{ScopeUserID: lecturer.ID, Limit: 10})
	if err != nil || len(people) != 0 {
		t.Fatalf("removed participant visible = %+v, %v", people, err)
	}
}

func TestCore01RetentionGuardsAndNonbiometricSchema(t *testing.T) {
	for _, dependency := range []string{"attendance", "enrollment", "future-template"} {
		t.Run(dependency, func(t *testing.T) {
			s, db, _ := bukaStore(t)
			ctx := context.Background()
			admin := core01Admin(t, s, db)
			p := core01Person(t, s, admin.ID, "SYN-RETAIN")
			if got := hitung(t, db, `SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name IN ('face_templates','face_enrollment_samples')`); got != 0 {
				t.Fatalf("schema nonbiometrik membuat tabel payload: %d", got)
			}
			if _, err := db.ExecContext(ctx, `UPDATE people SET identity_reference = NULL WHERE id = ?`, p.ID); err == nil {
				t.Fatal("schema mengizinkan timestamp verified tanpa referensi")
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO people (id, external_code, display_name, identity_verified_at) VALUES ('invalid-proof', 'SYN-INVALID', 'Sintetis', ?)`, formatWaktu(waktuUji)); err == nil {
				t.Fatal("schema mengizinkan insert verified tanpa referensi")
			}
			switch dependency {
			case "attendance":
				class, err := s.CreateClass(ctx, admin.ID, "SYN-RETAIN", "Kelas Sintetis")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO attendance_sessions (id, class_id, status, closes_at, created_by) VALUES ('synthetic-session', ?, 'open', ?, ?)`, class.ID, formatWaktu(waktuUji.Add(time.Hour)), admin.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO attendance_records (id, session_id, person_id, status, source, checked_in_at, created_by, reason) VALUES ('synthetic-attendance', 'synthetic-session', ?, 'present', 'manual', ?, ?, 'synthetic fixture')`, p.ID, formatWaktu(waktuUji), admin.ID); err != nil {
					t.Fatal(err)
				}
				if err := s.RemoveClassMember(ctx, admin.ID, class.ID, p.ID); !errors.Is(err, ErrPolicyConflict) {
					t.Fatalf("remove with open attendance = %v", err)
				}
				if _, err := db.ExecContext(ctx, `UPDATE attendance_sessions SET status = 'closed' WHERE id = 'synthetic-session'`); err != nil {
					t.Fatal(err)
				}
				if err := s.RemoveClassMember(ctx, admin.ID, class.ID, p.ID); err != nil {
					t.Fatal(err)
				}
			case "enrollment":
				if _, err := db.ExecContext(ctx, `INSERT INTO face_enrollments (id, person_id, created_by) VALUES ('synthetic-enrollment', ?, ?)`, p.ID, admin.ID); err != nil {
					t.Fatal(err)
				}
			case "future-template":
				// Hanya sentinel struktural sintetis: tidak ada kolom/payload biometrik.
				if _, err := db.ExecContext(ctx, `CREATE TABLE face_templates (person_id TEXT NOT NULL REFERENCES people(id))`); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO face_templates (person_id) VALUES (?)`, p.ID); err != nil {
					t.Fatal(err)
				}
			}
			before := core01Snapshot(t, db)
			if err := s.DeletePerson(ctx, admin.ID, p.ID); !errors.Is(err, ErrPolicyConflict) {
				t.Fatalf("delete with %s = %v", dependency, err)
			}
			if after := core01Snapshot(t, db); !reflect.DeepEqual(before, after) {
				t.Fatal("retention denial changed roster/audit")
			}
		})
	}
}

func TestCore01ClassValidationAndInactiveMembership(t *testing.T) {
	s, db, _ := bukaStore(t)
	ctx := context.Background()
	admin := core01Admin(t, s, db)
	for _, tc := range []struct{ code, title string }{
		{"", "Kelas"}, {"SYN_1", "Kelas"}, {strings.Repeat("A", 33), "Kelas"},
		{"SYN-1", ""}, {"SYN-1", strings.Repeat("é", 121)}, {"SYN-1", "Kelas\x00Sintetis"},
	} {
		if _, err := s.CreateClass(ctx, admin.ID, tc.code, tc.title); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("CreateClass(%q, %q) = %v", tc.code, tc.title, err)
		}
	}
	class, err := s.CreateClass(ctx, admin.ID, strings.Repeat("A", 32), strings.Repeat("é", 120))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateClass(ctx, admin.ID, class.Code, "Kelas Lain"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate class = %v", err)
	}
	p := core01Person(t, s, admin.ID, "SYN-INACTIVE")
	if _, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	inactive := "inactive"
	if _, err := s.UpdatePerson(ctx, admin.ID, p.ID, PersonUpdate{Status: &inactive}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddClassMember(ctx, admin.ID, class.ID, p.ID); !errors.Is(err, ErrSubjectInactive) {
		t.Fatalf("inactive participant accepted = %v", err)
	}
	if members, err := s.MembersForClass(ctx, class.ID); err != nil || len(members) != 0 {
		t.Fatalf("inactive member listed = %+v, %v", members, err)
	}
	if detail, err := s.ClassByID(ctx, class.ID); err != nil || detail.MemberCount != 0 {
		t.Fatalf("inactive member counted = %+v, %v", detail, err)
	}
	if _, err := s.MembersForClass(ctx, "missing-class"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing class roster = %v", err)
	}
}
