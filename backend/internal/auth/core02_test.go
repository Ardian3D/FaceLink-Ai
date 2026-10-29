package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
)

type core02Fixture struct {
	s        *Store
	db       *database.DB
	clock    *clock.Fake
	admin    User
	lecturer User
	student  User
	class    Class
	person   Person
}

func newCore02Fixture(t *testing.T) core02Fixture {
	t.Helper()
	s, db, clk := bukaStore(t)
	admin := core01Admin(t, s, db)
	lecturer := core01User(t, s, db, admin.ID, "core02-lecturer", RoleDosen, StatusActive)
	student := core01User(t, s, db, admin.ID, "core02-student", RoleMahasiswa, StatusActive)
	class, err := s.CreateClass(context.Background(), admin.ID, "CORE02-SYN", "Kelas Sintetis")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AssignLecturer(context.Background(), admin.ID, class.ID, lecturer.ID); err != nil {
		t.Fatal(err)
	}
	person := core01Person(t, s, admin.ID, "CORE02-NIM")
	if _, err := s.LinkPersonAccount(context.Background(), admin.ID, person.ID, student.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddClassMember(context.Background(), admin.ID, class.ID, person.ID); err != nil {
		t.Fatal(err)
	}
	return core02Fixture{s, db, clk, admin, lecturer, student, class, person}
}

func (f core02Fixture) draft(t *testing.T, key string) ClassSession {
	t.Helper()
	late := f.clock.Now().Add(10 * time.Minute)
	value, err := f.s.CreateClassSession(context.Background(), f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, Title: "Pertemuan Sintetis", ClosesAt: f.clock.Now().Add(time.Hour), LateAfter: &late}, key)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func (f core02Fixture) open(t *testing.T) ClassSession {
	t.Helper()
	value := f.draft(t, "")
	opened, err := f.s.OpenClassSession(context.Background(), f.lecturer.ID, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

func (f core02Fixture) personMember(t *testing.T, code string) Person {
	t.Helper()
	person := core01Person(t, f.s, f.admin.ID, code)
	if _, err := f.s.AddClassMember(context.Background(), f.admin.ID, f.class.ID, person.ID); err != nil {
		t.Fatal(err)
	}
	return person
}

func (f core02Fixture) auditCount(t *testing.T, action string) int {
	t.Helper()
	return hitung(t, f.db, `SELECT COUNT(*) FROM audit_events WHERE action=?`, action)
}

func TestCore02LifecycleThresholdsAndManualAlternative(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	second := f.personMember(t, "CORE02-WITHDRAWN")
	third := f.personMember(t, "CORE02-NOACCOUNT")
	if _, err := f.db.ExecContext(ctx, `UPDATE people SET consent_state='withdrawn' WHERE id=?`, second.ID); err != nil {
		t.Fatal(err)
	}
	session := f.draft(t, "create-lifecycle")
	if session.Status != "draft" || session.StartedAt != nil || session.Counts != (AttendanceCounts{}) {
		t.Fatalf("draft=%+v", session)
	}
	pending, counts, err := f.s.ListSessionAttendance(ctx, f.lecturer.ID, session.ID, AttendanceQuery{})
	if err != nil || len(pending) != 3 || counts.Absent != 0 {
		t.Fatalf("draft roster=%+v counts=%+v err=%v", pending, counts, err)
	}
	for _, row := range pending {
		if row.Status != "pending" || row.CheckedInAt != nil || row.Source != nil {
			t.Fatalf("draft row=%+v", row)
		}
	}
	manual := ManualAttendanceInput{PersonID: f.person.ID, Reason: "Verifikasi petugas sintetis", Status: "present"}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, manual, ""); !errors.Is(err, ErrSessionNotOpen) {
		t.Fatalf("draft checkin=%v", err)
	}
	if _, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, ""); !errors.Is(err, ErrSessionNotOpen) {
		t.Fatalf("draft close=%v", err)
	}
	session, err = f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID)
	if err != nil || session.Status != "open" || session.StartedAt == nil || !session.StartedAt.Equal(waktuUji) {
		t.Fatalf("open=%+v %v", session, err)
	}
	if _, err := f.s.OpenClassSession(ctx, f.admin.ID, session.ID); !errors.Is(err, ErrSessionAlreadyOpen) {
		t.Fatalf("reopen=%v", err)
	}
	f.clock.Advance(10 * time.Minute)
	first, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, manual, "")
	if err != nil || code != 201 || first.Status != "present" || first.CheckedInAt == nil || !first.CheckedInAt.Equal(*session.LateAfter) {
		t.Fatalf("at late threshold=%+v %d %v", first, code, err)
	}
	f.clock.Advance(time.Millisecond)
	manual.PersonID = second.ID
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, manual, ""); !errors.Is(err, ErrAttendanceValidation) {
		t.Fatalf("wrong derived present=%v", err)
	}
	manual.Status = "late"
	if _, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, manual, ""); err != nil || code != 201 {
		t.Fatalf("manual withdrawn no account=%d %v", code, err)
	}
	manual.PersonID, manual.Status = third.ID, "excused"
	if _, _, err := f.s.RecordManualAttendance(ctx, f.admin.ID, session.ID, manual, ""); err != nil {
		t.Fatalf("excused=%v", err)
	}
	rows, counts, err := f.s.ListSessionAttendance(ctx, f.lecturer.ID, session.ID, AttendanceQuery{})
	if err != nil || len(rows) != 3 || counts != (AttendanceCounts{Present: 1, Late: 1, Excused: 1}) {
		t.Fatalf("attendance=%+v %+v %v", rows, counts, err)
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM consent_records`) > 0 || hitung(t, f.db, `SELECT COUNT(*) FROM face_enrollments`) > 0 {
		t.Fatal("manual path created biometric state")
	}
	if f.auditCount(t, "session.created") != 1 || f.auditCount(t, "session.opened") != 1 || f.auditCount(t, "attendance.manual_corrected") != 3 {
		t.Fatal("required lifecycle audit missing")
	}
}

func TestCore02ClosedExplicitCorrectionAndNoOp(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	missing := f.personMember(t, "CORE02-ABSENT")
	session := f.open(t)
	f.clock.Advance(5 * time.Minute)
	input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Alasan sintetis awal"}
	first, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrAlreadyCheckedIn) {
		t.Fatalf("unkeyed repeat=%v", err)
	}
	f.clock.Advance(time.Minute)
	closed, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, "Penutupan sintetis privat")
	if err != nil || closed.Status != "closed" || closed.ClosedAt == nil || closed.Counts.Absent != 1 {
		t.Fatalf("close=%+v %v", closed, err)
	}
	if _, err := f.s.CloseClassSession(ctx, f.admin.ID, session.ID, ""); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("reclose=%v", err)
	}
	if _, err := f.s.OpenClassSession(ctx, f.admin.ID, session.ID); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("reopen closed=%v", err)
	}
	input.PersonID = missing.ID
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("normal closed=%v", err)
	}
	input.CheckedInAt = first.CheckedInAt
	corrected, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "")
	if err != nil || code != 201 || corrected.CheckedInAt == nil || !corrected.CheckedInAt.Equal(*first.CheckedInAt) {
		t.Fatalf("closed missing correction=%+v %d %v", corrected, code, err)
	}
	before := f.auditCount(t, "attendance.manual_corrected")
	noop, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "")
	if err != nil || code != 200 || !reflect.DeepEqual(noop, corrected) || f.auditCount(t, "attendance.manual_corrected") != before {
		t.Fatalf("explicit noop=%+v %d %v", noop, code, err)
	}
	input.Status, input.Reason = "excused", "Koreksi alasan sintetis baru"
	revised, code, err := f.s.RecordManualAttendance(ctx, f.admin.ID, session.ID, input, "")
	if err != nil || code != 200 || revised.ID != corrected.ID || revised.CreatedBy == nil || *revised.CreatedBy != f.admin.ID || f.auditCount(t, "attendance.manual_corrected") != before+1 {
		t.Fatalf("correction=%+v %d %v", revised, code, err)
	}
	manual, err := f.s.ListManualAttendance(ctx, f.lecturer.ID, session.ID)
	if err != nil || len(manual) != 2 {
		t.Fatalf("manual reads=%+v %v", manual, err)
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records WHERE status='absent'`) != 0 {
		t.Fatal("derived absent persisted")
	}
	var metadata string
	if err := f.db.QueryRowContext(ctx, `SELECT metadata_json FROM audit_events WHERE entity_id=? ORDER BY occurred_at DESC,id DESC LIMIT 1`, revised.ID).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	// Every audit row is inspected because multiple corrections can share a clock tick.
	auditRows, err := f.db.QueryContext(ctx, `SELECT reason,metadata_json FROM audit_events WHERE action LIKE 'session.%' OR action='attendance.manual_corrected'`)
	if err != nil {
		t.Fatal(err)
	}
	defer auditRows.Close()
	for auditRows.Next() {
		var reason string
		var body sql.NullString
		if err := auditRows.Scan(&reason, &body); err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{input.Reason, "Alasan sintetis awal", "Penutupan sintetis privat", f.person.ExternalCode, f.person.IdentityReference, "Peserta Sintetis"} {
			if strings.Contains(body.String, secret) {
				t.Fatalf("audit metadata leaked private text %q", secret)
			}
		}
	}
	if err := auditRows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := auditRows.Close(); err != nil {
		t.Fatal(err)
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM audit_events WHERE action='attendance.manual_corrected' AND entity_id=? AND reason='Alasan sintetis awal'`, revised.ID) != 1 {
		t.Fatal("original human correction reason was lost")
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM audit_events WHERE action='attendance.manual_corrected' AND entity_id=? AND reason=? AND json_extract(metadata_json,'$.status_before')='present' AND json_extract(metadata_json,'$.status_after')='excused' AND json_extract(metadata_json,'$.actor_after')=?`, revised.ID, input.Reason, f.admin.ID) != 1 {
		t.Fatal("correction safe before/after history or human reason missing")
	}
	var meta map[string]string
	if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"status_before", "status_after", "source_before", "source_after", "checked_in_at_before", "checked_in_at_after", "actor_before", "actor_after"} {
		if _, ok := meta[key]; !ok {
			t.Fatalf("missing safe history key %s", key)
		}
	}
}

func TestCore02ManualTimeBoundaries(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	session := f.open(t)
	f.clock.Advance(20 * time.Minute)
	for _, tc := range []struct {
		name   string
		at     time.Time
		status string
		want   error
	}{
		{"before start", waktuUji.Add(-time.Millisecond), "present", ErrAttendanceValidation},
		{"future", waktuUji.Add(21 * time.Minute), "late", ErrAttendanceValidation},
		{"at closes", session.ClosesAt, "late", ErrAttendanceValidation},
		{"threshold disagreement", *session.LateAfter, "late", ErrAttendanceValidation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, ManualAttendanceInput{PersonID: f.person.ID, Reason: "Batas sintetis", Status: tc.status, CheckedInAt: &tc.at}, "")
			if !errors.Is(err, tc.want) {
				t.Fatalf("time validation=%v", err)
			}
		})
	}
	if _, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, ""); err != nil {
		t.Fatal(err)
	}
	after := f.clock.Now().Add(time.Millisecond)
	f.clock.Advance(time.Minute)
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, ManualAttendanceInput{PersonID: f.person.ID, Reason: "Batas penutupan", Status: "late", CheckedInAt: &after}, ""); !errors.Is(err, ErrAttendanceValidation) {
		t.Fatalf("after closed_at=%v", err)
	}
	atStart := waktuUji.In(time.FixedZone("UTC+7", 7*3600))
	record, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, ManualAttendanceInput{PersonID: f.person.ID, Reason: "Batas mulai", Status: "present", CheckedInAt: &atStart}, "")
	if err != nil || code != 201 || record.CheckedInAt == nil || record.CheckedInAt.Location() != time.UTC {
		t.Fatalf("inclusive start and UTC=%+v %d %v", record, code, err)
	}
	atClose := waktuUji.Add(20 * time.Minute)
	closedBoundary, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, ManualAttendanceInput{PersonID: f.person.ID, Reason: "Batas closed_at inklusif", Status: "late", CheckedInAt: &atClose}, "")
	if err != nil || code != 200 || closedBoundary.CheckedInAt == nil || !closedBoundary.CheckedInAt.Equal(atClose) {
		t.Fatalf("inclusive closed_at=%+v %d %v", closedBoundary, code, err)
	}
}

func TestCore02ElapsedSessionBlocksImplicitWithoutAutoClosing(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	session := f.open(t)
	f.clock.Advance(time.Hour)
	input := ManualAttendanceInput{PersonID: f.person.ID, Reason: "Sesi elapsed sintetis", Status: "late"}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("exact closes_at=%v", err)
	}
	current, err := f.s.GetClassSession(ctx, f.lecturer.ID, session.ID)
	if err != nil || current.Status != "open" {
		t.Fatalf("elapsed must not automatically close=%+v %v", current, err)
	}
	at := session.ClosesAt.Add(-time.Millisecond)
	input.CheckedInAt = &at
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); err != nil {
		t.Fatalf("elapsed explicit correction=%v", err)
	}
	if f.auditCount(t, "session.closed") != 0 {
		t.Fatal("implicit automatic close audit")
	}
}

func TestCore02ManualSubjectEligibilityAndValidation(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	session := f.open(t)
	outsider := core01Person(t, f.s, f.admin.ID, "CORE02-OUTSIDER")
	input := ManualAttendanceInput{PersonID: outsider.ID, Reason: "Alasan sintetis", Status: "present"}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrClassMismatch) {
		t.Fatalf("nonmember=%v", err)
	}
	input.PersonID = f.person.ID
	for _, tc := range []struct {
		name  string
		input ManualAttendanceInput
		want  error
	}{
		{"empty reason", ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "  "}, ErrReasonRequired},
		{"long reason", ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: strings.Repeat("é", 201)}, ErrInvalidInput},
		{"control reason", ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "private\ntext"}, ErrInvalidInput},
		{"invalid status", ManualAttendanceInput{PersonID: f.person.ID, Status: "absent", Reason: "Synthetic"}, ErrInvalidInput},
		{"missing person", ManualAttendanceInput{Status: "present", Reason: "Synthetic"}, ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, tc.input, ""); !errors.Is(err, tc.want) {
				t.Fatalf("validation=%v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, query string
		args        []any
		want        error
	}{
		{"inactive person", `UPDATE people SET status='inactive' WHERE id=?`, []any{f.person.ID}, ErrSubjectInactive},
		{"unverified person", `UPDATE people SET identity_verified_at=NULL WHERE id=?`, []any{f.person.ID}, ErrIdentityNotVerified},
		{"suspended binding", `UPDATE users SET status='suspended' WHERE id=?`, []any{f.student.ID}, ErrAccountInactive},
		{"wrong role binding", `UPDATE users SET role='dosen' WHERE id=?`, []any{f.student.ID}, ErrNotMahasiswa},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.db.ExecContext(ctx, tc.query, tc.args...); err != nil {
				t.Fatal(err)
			}
			if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, tc.want) {
				t.Fatalf("eligibility=%v", err)
			}
			if _, err := f.db.ExecContext(ctx, `UPDATE people SET status='active',identity_verified_at=? WHERE id=?`, formatWaktu(waktuUji), f.person.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.ExecContext(ctx, `UPDATE users SET role='mahasiswa',status='active' WHERE id=?`, f.student.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records`) != 0 || f.auditCount(t, "attendance.manual_corrected") != 0 {
		t.Fatal("rejected manual operation mutated attendance")
	}
}

func TestCore02ActorScopeAndReplayReauthorization(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	other, err := f.s.CreateClass(ctx, f.admin.ID, "CORE02-OTHER", "Kelas lain sintetis")
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.s.CreateClassSession(ctx, f.admin.ID, CreateClassSessionInput{ClassID: other.ID, ClosesAt: waktuUji.Add(time.Hour)}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []struct {
		name string
		run  func(string) error
	}{
		{"detail", func(actor string) error { _, err := f.s.GetClassSession(ctx, actor, out.ID); return err }},
		{"open", func(actor string) error { _, err := f.s.OpenClassSession(ctx, actor, out.ID); return err }},
		{"close", func(actor string) error { _, err := f.s.CloseClassSession(ctx, actor, out.ID, ""); return err }},
		{"attendance", func(actor string) error {
			_, _, err := f.s.ListSessionAttendance(ctx, actor, out.ID, AttendanceQuery{})
			return err
		}},
		{"manual list", func(actor string) error { _, err := f.s.ListManualAttendance(ctx, actor, out.ID); return err }},
		{"manual mutation", func(actor string) error {
			_, _, err := f.s.RecordManualAttendance(ctx, actor, out.ID, ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Scope sintetis"}, "")
			return err
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			if err := operation.run(f.lecturer.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("out of scope=%v", err)
			}
			if err := operation.run(f.student.ID); !errors.Is(err, ErrActorUnauthorized) {
				t.Fatalf("wrong actor=%v", err)
			}
		})
	}
	if _, err := f.s.ListClassSessions(ctx, f.lecturer.ID, ClassSessionQuery{ClassID: other.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("filtered outscope=%v", err)
	}
	if _, err := f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: other.ID, ClosesAt: waktuUji.Add(time.Hour)}, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create outscope=%v", err)
	}
	session := f.draft(t, "scope-create-key")
	if _, err := f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID); err != nil {
		t.Fatal(err)
	}
	input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Scope sintetis"}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "scope-manual-key"); err != nil {
		t.Fatal(err)
	}
	if err := f.s.RemoveLecturer(ctx, f.admin.ID, f.class.ID, f.lecturer.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "scope-manual-key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("scope removed replay=%v", err)
	}
	late := waktuUji.Add(10 * time.Minute)
	if _, err := f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, Title: "Pertemuan Sintetis", ClosesAt: waktuUji.Add(time.Hour), LateAfter: &late}, "scope-create-key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create scope removed replay=%v", err)
	}
	if _, err := f.db.ExecContext(ctx, `UPDATE users SET status='suspended' WHERE id=?`, f.lecturer.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "scope-manual-key"); !errors.Is(err, ErrActorUnauthorized) {
		t.Fatalf("suspended replay=%v", err)
	}
	if f.auditCount(t, "attendance.manual_corrected") != 1 {
		t.Fatal("denied replay wrote audit")
	}
}

func TestCore02CanonicalIdempotencySnapshotsActorMethodPathAndTTL(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	late := waktuUji.Add(10 * time.Minute)
	create := CreateClassSessionInput{ClassID: f.class.ID, Title: "Snapshot title sintetis", ClosesAt: waktuUji.Add(time.Hour), LateAfter: &late}
	first, err := f.s.CreateClassSession(ctx, f.lecturer.ID, create, "shared-idempotency")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := f.s.OpenClassSession(ctx, f.lecturer.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	replay, err := f.s.CreateClassSession(ctx, f.lecturer.ID, create, "shared-idempotency")
	if err != nil || !reflect.DeepEqual(first, replay) {
		t.Fatalf("create snapshot=%+v %v", replay, err)
	}
	changed := create
	changed.Title = "Different body"
	if _, err := f.s.CreateClassSession(ctx, f.lecturer.ID, changed, "shared-idempotency"); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("changed create body=%v", err)
	}
	input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Snapshot reason sintetis"}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, opened.ID, input, "shared-idempotency"); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("same actor different path=%v", err)
	}
	record, code, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, opened.ID, input, "manual-snapshot-key")
	if err != nil || code != 201 {
		t.Fatalf("first=%+v %d %v", record, code, err)
	}
	f.clock.Advance(time.Minute)
	correction := input
	correction.CheckedInAt = record.CheckedInAt
	correction.Status = "excused"
	correction.Reason = "Corrected private reason"
	updated, code, err := f.s.RecordManualAttendance(ctx, f.admin.ID, opened.ID, correction, "manual-snapshot-key")
	if err != nil || code != 200 || updated.Status != "excused" {
		t.Fatalf("other actor same key=%+v %d %v", updated, code, err)
	}
	replayed, status, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, opened.ID, input, "manual-snapshot-key")
	if err != nil || status != 201 || !reflect.DeepEqual(record, replayed) {
		t.Fatalf("first body/time/status snapshot=%+v %d %v", replayed, status, err)
	}
	correction.Reason = "Another change"
	if _, _, err := f.s.RecordManualAttendance(ctx, f.admin.ID, opened.ID, correction, "manual-snapshot-key"); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("changed correction body=%v", err)
	}
	correction.Reason = "Corrected private reason"
	replayed, status, err = f.s.RecordManualAttendance(ctx, f.admin.ID, opened.ID, correction, "manual-snapshot-key")
	if err != nil || status != 200 || !reflect.DeepEqual(updated, replayed) {
		t.Fatalf("200 correction snapshot=%+v %d %v", replayed, status, err)
	}
	if record.ExternalCode != "" || record.DisplayName != "" {
		t.Fatal("mutation response needlessly copied roster PII")
	}
	if f.auditCount(t, "attendance.manual_corrected") != 2 || f.auditCount(t, "session.created") != 1 {
		t.Fatal("replay duplicated mutation/audit")
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM idempotency_records WHERE scope=?`, "operator:"+f.lecturer.ID) != 2 {
		t.Fatal("canonical actor namespace not used")
	}
	// Expiry at exactly 24h permits a new operation using only this actor's key.
	f.clock.Advance(24 * time.Hour)
	create.ClosesAt = f.clock.Now().Add(time.Hour)
	create.LateAfter = nil
	if _, err := f.s.CloseClassSession(ctx, f.admin.ID, opened.ID, ""); err != nil {
		t.Fatal(err)
	}
	fresh, err := f.s.CreateClassSession(ctx, f.lecturer.ID, create, "shared-idempotency")
	if err != nil || fresh.ID == first.ID {
		t.Fatalf("expired key replacement=%+v %v", fresh, err)
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM idempotency_records WHERE scope=? AND key='manual-snapshot-key'`, "operator:"+f.admin.ID) != 0 {
		t.Fatal("expired abandoned snapshot was retained")
	}
	for _, key := range []string{"short", strings.Repeat("a", 129), "invalid\nkey", "unicode-é-key"} {
		if _, err := f.s.CreateClassSession(ctx, f.lecturer.ID, create, key); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid key accepted %q: %v", key, err)
		}
	}
}

func TestCore02OwnHistoryAndRecordedRowsSurviveRosterRemoval(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	missing := f.personMember(t, "CORE02-PENDING")
	otherStudent := core01User(t, f.s, f.db, f.admin.ID, "other-student", RoleMahasiswa, StatusActive)
	if _, err := f.s.LinkPersonAccount(ctx, f.admin.ID, missing.ID, otherStudent.ID); err != nil {
		t.Fatal(err)
	}
	session := f.open(t)
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "History private reason"}, ""); err != nil {
		t.Fatal(err)
	}
	own, err := f.s.ListOwnAttendance(ctx, f.student.ID, OwnAttendanceQuery{})
	if err != nil || len(own) != 1 || own[0].Status != "present" {
		t.Fatalf("own=%+v %v", own, err)
	}
	other, err := f.s.ListOwnAttendance(ctx, otherStudent.ID, OwnAttendanceQuery{})
	if err != nil || len(other) != 1 || other[0].Status != "pending" || other[0].Reason != nil {
		t.Fatalf("other ownership=%+v %v", other, err)
	}
	if _, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, ""); err != nil {
		t.Fatal(err)
	}
	other, err = f.s.ListOwnAttendance(ctx, otherStudent.ID, OwnAttendanceQuery{})
	if err != nil || len(other) != 1 || other[0].Status != "absent" {
		t.Fatalf("closed absence=%+v %v", other, err)
	}
	if err := f.s.RemoveClassMember(ctx, f.admin.ID, f.class.ID, f.person.ID); err != nil {
		t.Fatal(err)
	}
	own, err = f.s.ListOwnAttendance(ctx, f.student.ID, OwnAttendanceQuery{ClassID: f.class.ID})
	if err != nil || len(own) != 1 || own[0].Status != "present" {
		t.Fatalf("historical own after removal=%+v %v", own, err)
	}
	rows, counts, err := f.s.ListSessionAttendance(ctx, f.lecturer.ID, session.ID, AttendanceQuery{})
	if err != nil || len(rows) != 2 || counts.Present != 1 || counts.Absent != 1 {
		t.Fatalf("historical roster=%+v %+v %v", rows, counts, err)
	}
	if _, err := f.s.ListOwnAttendance(ctx, f.lecturer.ID, OwnAttendanceQuery{}); !errors.Is(err, ErrActorUnauthorized) {
		t.Fatalf("dosen self attendance=%v", err)
	}
	unlinked := core01User(t, f.s, f.db, f.admin.ID, "unlinked-student", RoleMahasiswa, StatusActive)
	if _, err := f.s.ListOwnAttendance(ctx, unlinked.ID, OwnAttendanceQuery{}); !errors.Is(err, ErrNotLinked) {
		t.Fatalf("unlinked own=%v", err)
	}
	for _, query := range []string{`UPDATE people SET status='inactive' WHERE id=?`, `UPDATE people SET identity_verified_at=NULL WHERE id=?`} {
		if _, err := f.db.ExecContext(ctx, query, f.person.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.s.ListOwnAttendance(ctx, f.student.ID, OwnAttendanceQuery{}); !errors.Is(err, ErrNotLinked) {
			t.Fatalf("invalid binding own=%v", err)
		}
		if _, err := f.db.ExecContext(ctx, `UPDATE people SET status='active',identity_verified_at=? WHERE id=?`, formatWaktu(waktuUji), f.person.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCore02ReadFiltersAndPagination(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	f.personMember(t, "CORE02-PAGE2")
	f.personMember(t, "CORE02-PAGE3")
	one := f.draft(t, "")
	f.clock.Advance(time.Millisecond)
	two := f.draft(t, "")
	first, err := f.s.ListClassSessions(ctx, f.lecturer.ID, ClassSessionQuery{ClassID: f.class.ID, Status: "draft", Limit: 1})
	if err != nil || len(first) != 1 || first[0].ID != one.ID {
		t.Fatalf("first session page=%+v %v", first, err)
	}
	next, err := f.s.ListClassSessions(ctx, f.lecturer.ID, ClassSessionQuery{AfterCreatedAt: formatWaktu(first[0].CreatedAt), AfterID: first[0].ID, Limit: 1})
	if err != nil || len(next) != 1 || next[0].ID != two.ID {
		t.Fatalf("next session page=%+v %v", next, err)
	}
	page, counts, err := f.s.ListSessionAttendance(ctx, f.lecturer.ID, one.ID, AttendanceQuery{Limit: 1})
	if err != nil || len(page) != 1 || counts.Absent != 0 {
		t.Fatalf("first attendance page=%+v %+v %v", page, counts, err)
	}
	following, _, err := f.s.ListSessionAttendance(ctx, f.lecturer.ID, one.ID, AttendanceQuery{AfterID: page[0].PersonID, Limit: 2})
	if err != nil || len(following) != 2 || following[0].PersonID <= page[0].PersonID {
		t.Fatalf("attendance cursor=%+v %v", following, err)
	}
	own, err := f.s.ListOwnAttendance(ctx, f.student.ID, OwnAttendanceQuery{Limit: 1})
	if err != nil || len(own) != 1 || own[0].SessionID != one.ID {
		t.Fatalf("first own page=%+v %v", own, err)
	}
	nextOwn, err := f.s.ListOwnAttendance(ctx, f.student.ID, OwnAttendanceQuery{AfterCreatedAt: formatWaktu(own[0].CreatedAt), AfterID: own[0].SessionID, Limit: 1})
	if err != nil || len(nextOwn) != 1 || nextOwn[0].SessionID != two.ID {
		t.Fatalf("own cursor=%+v %v", nextOwn, err)
	}
	from, to := two.CreatedAt, two.CreatedAt
	filtered, err := f.s.ListClassSessions(ctx, f.lecturer.ID, ClassSessionQuery{From: &from, To: &to})
	if err != nil || len(filtered) != 1 || filtered[0].ID != two.ID {
		t.Fatalf("time filters=%+v %v", filtered, err)
	}
	for _, query := range []ClassSessionQuery{{Status: "cancelled"}, {AfterID: "missing time"}, {AfterCreatedAt: "not-time", AfterID: "id"}, {Limit: 202}} {
		if _, err := f.s.ListClassSessions(ctx, f.lecturer.ID, query); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("bad query=%+v %v", query, err)
		}
	}
}

func TestCore02IndependentHandlesProtectOpenClassAndDevice(t *testing.T) {
	for _, kind := range []string{"class", "device"} {
		t.Run(kind, func(t *testing.T) {
			f := newCore02Fixture(t)
			ctx := context.Background()
			other := core01SecondStore(t, f.db, f.s)
			classID := f.class.ID
			deviceID := ""
			if kind == "device" {
				second, err := f.s.CreateClass(ctx, f.admin.ID, "CORE02-RACE2", "Race device sintetis")
				if err != nil {
					t.Fatal(err)
				}
				classID = second.ID
				deviceID = "core02-device"
				if _, err := f.db.ExecContext(ctx, `INSERT INTO devices(id,device_code) VALUES(?,?)`, deviceID, "SYNTHETIC-DEVICE"); err != nil {
					t.Fatal(err)
				}
			}
			first, err := f.s.CreateClassSession(ctx, f.admin.ID, CreateClassSessionInput{ClassID: f.class.ID, DeviceID: deviceID, ClosesAt: waktuUji.Add(time.Hour)}, "")
			if err != nil {
				t.Fatal(err)
			}
			second, err := f.s.CreateClassSession(ctx, f.admin.ID, CreateClassSessionInput{ClassID: classID, DeviceID: deviceID, ClosesAt: waktuUji.Add(time.Hour)}, "")
			if err != nil {
				t.Fatal(err)
			}
			result := core01Race(func() error { _, err := f.s.OpenClassSession(ctx, f.admin.ID, first.ID); return err }, func() error { _, err := other.OpenClassSession(ctx, f.admin.ID, second.ID); return err })
			want := ErrSessionAlreadyOpen
			if kind == "device" {
				want = ErrDeviceAlreadyBound
			}
			success, conflict := 0, 0
			for _, err := range result {
				if err == nil {
					success++
				} else if errors.Is(err, want) {
					conflict++
				} else {
					t.Fatalf("unexpected race error=%v", err)
				}
			}
			if success != 1 || conflict != 1 || hitung(t, f.db, `SELECT COUNT(*) FROM attendance_sessions WHERE status='open'`) != 1 || f.auditCount(t, "session.opened") != 1 {
				t.Fatalf("open race=%v", result)
			}
		})
	}
}

func TestCore02IndependentHandlesManualAndIdempotencyRaces(t *testing.T) {
	for _, keyed := range []bool{false, true} {
		t.Run(map[bool]string{false: "duplicate", true: "keyed_replay"}[keyed], func(t *testing.T) {
			f := newCore02Fixture(t)
			ctx := context.Background()
			other := core01SecondStore(t, f.db, f.s)
			session := f.open(t)
			key := ""
			if keyed {
				key = "race-manual-key"
			}
			input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Race sintetis"}
			type response struct {
				record AttendanceRecord
				code   int
				err    error
			}
			start := make(chan struct{})
			results := make(chan response, 2)
			for _, store := range []*Store{f.s, other} {
				go func(store *Store) {
					<-start
					r, c, e := store.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, key)
					results <- response{r, c, e}
				}(store)
			}
			close(start)
			a, b := <-results, <-results
			if keyed {
				if a.err != nil || b.err != nil || a.code != 201 || b.code != 201 || !reflect.DeepEqual(a.record, b.record) {
					t.Fatalf("idempotency race=%+v %+v", a, b)
				}
			} else {
				if !((a.err == nil && errors.Is(b.err, ErrAlreadyCheckedIn)) || (b.err == nil && errors.Is(a.err, ErrAlreadyCheckedIn))) {
					t.Fatalf("duplicate race=%v %v", a.err, b.err)
				}
			}
			if hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records`) != 1 || f.auditCount(t, "attendance.manual_corrected") != 1 {
				t.Fatal("race duplicated attendance or audit")
			}
		})
	}
	f := newCore02Fixture(t)
	ctx := context.Background()
	other := core01SecondStore(t, f.db, f.s)
	input := CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji.Add(time.Hour)}
	result := core01Race(func() error {
		_, err := f.s.CreateClassSession(ctx, f.lecturer.ID, input, "race-create-key")
		return err
	}, func() error {
		_, err := other.CreateClassSession(ctx, f.lecturer.ID, input, "race-create-key")
		return err
	})
	if result[0] != nil || result[1] != nil || hitung(t, f.db, `SELECT COUNT(*) FROM attendance_sessions`) != 1 || f.auditCount(t, "session.created") != 1 {
		t.Fatalf("create replay race=%v", result)
	}
}

func TestCore02IndependentCloseAndManualAreAtomic(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	other := core01SecondStore(t, f.db, f.s)
	session := f.open(t)
	var manualErr error
	result := core01Race(func() error { _, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, ""); return err }, func() error {
		_, _, manualErr = other.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, ManualAttendanceInput{PersonID: f.person.ID, Reason: "Close race sintetis", Status: "present"}, "")
		return manualErr
	})
	for _, err := range result {
		if err != nil && !errors.Is(err, ErrSessionClosed) {
			t.Fatalf("close race unexpected=%v", result)
		}
	}
	want := 1
	if errors.Is(manualErr, ErrSessionClosed) {
		want = 0
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records`) != want || f.auditCount(t, "attendance.manual_corrected") != want || f.auditCount(t, "session.closed") != 1 {
		t.Fatalf("close/manual atomic outcome=%v", result)
	}
}

func TestCore02AuditFailureRollsBackEveryMutation(t *testing.T) {
	for _, operation := range []string{"create", "open", "close", "insert", "correction"} {
		t.Run(operation, func(t *testing.T) {
			f := newCore02Fixture(t)
			ctx := context.Background()
			var session ClassSession
			if operation != "create" {
				session = f.draft(t, "")
			}
			if operation == "close" || operation == "insert" || operation == "correction" {
				var err error
				session, err = f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Audit rollback sintetis"}
			if operation == "correction" {
				before, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "")
				if err != nil {
					t.Fatal(err)
				}
				input.CheckedInAt = before.CheckedInAt
				input.Status = "excused"
			}
			if operation == "correction" {
				var err error
				session, err = f.s.GetClassSession(ctx, f.lecturer.ID, session.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			countSessions := hitung(t, f.db, `SELECT COUNT(*) FROM attendance_sessions`)
			countAttendance := hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records`)
			countAudit := hitung(t, f.db, `SELECT COUNT(*) FROM audit_events`)
			if _, err := f.db.ExecContext(ctx, `CREATE TRIGGER core02_reject_audit BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END`); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "create":
				_, err = f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji.Add(time.Hour)}, "rollback-create-key")
			case "open":
				_, err = f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID)
			case "close":
				_, err = f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, "")
			default:
				_, _, err = f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "rollback-manual-key")
			}
			if err == nil {
				t.Fatal("audit failure accepted mutation")
			}
			if hitung(t, f.db, `SELECT COUNT(*) FROM attendance_sessions`) != countSessions || hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records`) != countAttendance || hitung(t, f.db, `SELECT COUNT(*) FROM audit_events`) != countAudit || hitung(t, f.db, `SELECT COUNT(*) FROM idempotency_records`) != 0 {
				t.Fatal("audit failure leaked mutation/snapshot")
			}
			if operation != "create" {
				current, err := f.s.GetClassSession(ctx, f.lecturer.ID, session.ID)
				if err != nil || !reflect.DeepEqual(session, current) {
					t.Fatalf("audit failure changed session=%+v %v", current, err)
				}
			}
			if operation == "correction" {
				var status string
				if err := f.db.QueryRowContext(ctx, `SELECT status FROM attendance_records`).Scan(&status); err != nil || status != "present" {
					t.Fatalf("audit failure changed record=%s %v", status, err)
				}
			}
		})
	}
}

func TestCore02DeviceAndSessionInputValidation(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		input CreateClassSessionInput
		want  error
	}{
		{"missing class", CreateClassSessionInput{ClosesAt: waktuUji.Add(time.Hour)}, ErrInvalidInput},
		{"missing time", CreateClassSessionInput{ClassID: f.class.ID}, ErrInvalidInput},
		{"elapsed", CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji}, ErrAttendanceValidation},
		{"long title", CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji.Add(time.Hour), Title: strings.Repeat("é", 121)}, ErrInvalidInput},
		{"control title", CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji.Add(time.Hour), Title: "hidden\ntext"}, ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.s.CreateClassSession(ctx, f.lecturer.ID, tc.input, ""); !errors.Is(err, tc.want) {
				t.Fatalf("create validation=%v", err)
			}
		})
	}
	for _, status := range []string{"revoked", "maintenance"} {
		if _, err := f.db.ExecContext(ctx, `INSERT INTO devices(id,device_code,status) VALUES(?,?,?)`, status, "SYN-"+status, status); err != nil {
			t.Fatal(err)
		}
		if _, err := f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, DeviceID: status, ClosesAt: waktuUji.Add(time.Hour)}, ""); !errors.Is(err, ErrDeviceUnavailable) {
			t.Fatalf("unavailable device=%v", err)
		}
	}
	if _, err := f.db.ExecContext(ctx, `INSERT INTO devices(id,device_code) VALUES('active-device','SYN-ACTIVE')`); err != nil {
		t.Fatal(err)
	}
	session, err := f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, DeviceID: "active-device", ClosesAt: waktuUji.Add(time.Hour)}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, `UPDATE devices SET status='revoked' WHERE id='active-device'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID); !errors.Is(err, ErrDeviceUnavailable) {
		t.Fatalf("revoked after create=%v", err)
	}
	f.clock.Advance(time.Hour)
	if _, err := f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("open expired draft=%v", err)
	}
}

func TestCore02DuplicateAcrossLateThresholdPreservesFirstTimestamp(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	session := f.open(t)
	input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Retry sintetis lintas threshold"}
	first, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "")
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(11 * time.Minute)
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrAlreadyCheckedIn) {
		t.Fatalf("duplicate after threshold must be409 not422: %v", err)
	}
	rows, err := f.s.ListManualAttendance(ctx, f.lecturer.ID, session.ID)
	if err != nil || len(rows) != 1 || !reflect.DeepEqual(rows[0], first) || f.auditCount(t, "attendance.manual_corrected") != 1 {
		t.Fatalf("duplicate changed initial status/time/audit=%+v %v", rows, err)
	}
	if _, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("implicit duplicate on closed session=%v", err)
	}
}

func TestCore02AllOperationsRereadActiveActor(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	session := f.open(t)
	input := ManualAttendanceInput{PersonID: f.person.ID, Status: "present", Reason: "Otorisasi sintetis"}
	operations := []struct {
		name string
		run  func() error
	}{
		{"create", func() error {
			_, err := f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji.Add(time.Hour)}, "")
			return err
		}},
		{"open", func() error { _, err := f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID); return err }},
		{"close", func() error { _, err := f.s.CloseClassSession(ctx, f.lecturer.ID, session.ID, ""); return err }},
		{"manual", func() error {
			_, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, "")
			return err
		}},
		{"detail", func() error { _, err := f.s.GetClassSession(ctx, f.lecturer.ID, session.ID); return err }},
		{"list", func() error { _, err := f.s.ListClassSessions(ctx, f.lecturer.ID, ClassSessionQuery{}); return err }},
		{"attendance", func() error {
			_, _, err := f.s.ListSessionAttendance(ctx, f.lecturer.ID, session.ID, AttendanceQuery{})
			return err
		}},
		{"manual list", func() error { _, err := f.s.ListManualAttendance(ctx, f.lecturer.ID, session.ID); return err }},
	}
	before := hitung(t, f.db, `SELECT COUNT(*) FROM audit_events`)
	for _, state := range []struct {
		role   Role
		status Status
	}{{RoleDosen, StatusSuspended}, {RoleMahasiswa, StatusActive}} {
		if _, err := f.db.ExecContext(ctx, `UPDATE users SET role=?,status=? WHERE id=?`, state.role, state.status, f.lecturer.ID); err != nil {
			t.Fatal(err)
		}
		for _, operation := range operations {
			t.Run(string(state.role)+"/"+string(state.status)+"/"+operation.name, func(t *testing.T) {
				if err := operation.run(); !errors.Is(err, ErrActorUnauthorized) {
					t.Fatalf("stale actor authorized=%v", err)
				}
			})
		}
	}
	if hitung(t, f.db, `SELECT COUNT(*) FROM attendance_records`) != 0 || hitung(t, f.db, `SELECT COUNT(*) FROM audit_events`) != before {
		t.Fatal("rejected stale actor mutated state")
	}
}

func TestCore02NoLateThresholdAlwaysDerivesPresent(t *testing.T) {
	f := newCore02Fixture(t)
	ctx := context.Background()
	session, err := f.s.CreateClassSession(ctx, f.lecturer.ID, CreateClassSessionInput{ClassID: f.class.ID, ClosesAt: waktuUji.Add(time.Hour)}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.OpenClassSession(ctx, f.lecturer.ID, session.ID); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(30 * time.Minute)
	input := ManualAttendanceInput{PersonID: f.person.ID, Status: "late", Reason: "Tanpa threshold sintetis"}
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); !errors.Is(err, ErrAttendanceValidation) {
		t.Fatalf("client late without threshold=%v", err)
	}
	input.Status = "present"
	if _, _, err := f.s.RecordManualAttendance(ctx, f.lecturer.ID, session.ID, input, ""); err != nil {
		t.Fatalf("no threshold present=%v", err)
	}
}
