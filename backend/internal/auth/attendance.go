package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

const attendanceRecordColumns = `r.id,r.session_id,r.person_id,r.status,r.source,r.checked_in_at,r.created_by,r.reason`

func scanAttendanceRecord(row rowScanner) (AttendanceRecord, error) {
	var result AttendanceRecord
	var source, checked, actor, reason sql.NullString
	if err := row.Scan(&result.ID, &result.SessionID, &result.PersonID, &result.Status, &source, &checked, &actor, &reason); err != nil {
		return AttendanceRecord{}, err
	}
	result.Source, result.CreatedBy, result.Reason = optionalText(source), optionalText(actor), optionalText(reason)
	var err error
	result.CheckedInAt, err = optionalTime(checked)
	return result, err
}

func manualSubjectTx(ctx context.Context, conn *sql.Conn, classID, personID string) error {
	var status string
	var reference, verified, account sql.NullString
	err := conn.QueryRowContext(ctx, `SELECT status,identity_reference,identity_verified_at,account_user_id FROM people WHERE id=? AND deleted_at IS NULL`, personID).Scan(&status, &reference, &verified, &account)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return ErrSubjectInactive
	}
	if !reference.Valid || teks(reference.String) == "" || !verified.Valid || teks(verified.String) == "" {
		return ErrIdentityNotVerified
	}
	if account.Valid {
		user, err := userByIDTx(ctx, conn, account.String)
		if err != nil {
			return err
		}
		if user.Role != RoleMahasiswa {
			return ErrNotMahasiswa
		}
		if user.Status != StatusActive {
			return ErrAccountInactive
		}
	}
	var member int
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM class_memberships WHERE class_id=? AND person_id=? AND left_at IS NULL)`, classID, personID).Scan(&member); err != nil {
		return err
	}
	if member == 0 {
		return ErrClassMismatch
	}
	return nil
}

func validateManualTime(session ClassSession, status string, explicit *time.Time, now time.Time) (time.Time, error) {
	if session.Status == "draft" || session.StartedAt == nil {
		return time.Time{}, ErrSessionNotOpen
	}
	checked := now
	if explicit == nil {
		if session.Status != "open" || !now.Before(session.ClosesAt) {
			return time.Time{}, ErrSessionClosed
		}
	} else {
		if session.Status != "open" && session.Status != "closed" {
			return time.Time{}, ErrSessionClosed
		}
		checked = *explicit
	}
	if checked.Before(*session.StartedAt) || !checked.Before(session.ClosesAt) || checked.After(now) || (session.ClosedAt != nil && checked.After(*session.ClosedAt)) {
		return time.Time{}, ErrAttendanceValidation
	}
	derived := "present"
	if session.LateAfter != nil && checked.After(*session.LateAfter) {
		derived = "late"
	}
	if status != "excused" && status != derived {
		return time.Time{}, ErrAttendanceValidation
	}
	return checked, nil
}

func (s *Store) RecordManualAttendance(ctx context.Context, actorID, sessionID string, input ManualAttendanceInput, key string) (AttendanceRecord, int, error) {
	actorID, sessionID, input.PersonID, input.Reason, input.Status = teks(actorID), teks(sessionID), teks(input.PersonID), teks(input.Reason), teks(input.Status)
	if actorID == "" || sessionID == "" || input.PersonID == "" {
		return AttendanceRecord{}, 0, ErrInvalidInput
	}
	if input.Reason == "" {
		return AttendanceRecord{}, 0, ErrReasonRequired
	}
	if utf8.RuneCountInString(input.Reason) > 200 || memuatKontrol(input.Reason) || (input.Status != "present" && input.Status != "late" && input.Status != "excused") {
		return AttendanceRecord{}, 0, ErrInvalidInput
	}
	if input.CheckedInAt != nil {
		value := core02Time(*input.CheckedInAt)
		input.CheckedInAt = &value
	}
	request, err := newAttendanceIdempotency(key, "/api/v1/sessions/"+sessionID+"/manual-checkins", input)
	if err != nil {
		return AttendanceRecord{}, 0, err
	}
	var result AttendanceRecord
	code := 201
	err = s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		session, err := scopedClassSessionTx(ctx, conn, actorID, sessionID)
		if err != nil {
			return err
		}
		if err := manualSubjectTx(ctx, conn, session.ClassID, input.PersonID); err != nil {
			return err
		}
		now := core02Time(s.waktuSekarang())
		if saved, replayed, err := request.replay(ctx, conn, actorID, now, &result); err != nil || replayed {
			if replayed {
				code = saved
			}
			return err
		}
		before, err := scanAttendanceRecord(conn.QueryRowContext(ctx, `SELECT `+attendanceRecordColumns+` FROM attendance_records r WHERE r.session_id=? AND r.person_id=?`, sessionID, input.PersonID))
		exists := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if exists && input.CheckedInAt == nil {
			if session.Status == "draft" || session.StartedAt == nil {
				return ErrSessionNotOpen
			}
			if session.Status != "open" || !now.Before(session.ClosesAt) {
				return ErrSessionClosed
			}
			return ErrAlreadyCheckedIn
		}
		checked, err := validateManualTime(session, input.Status, input.CheckedInAt, now)
		if err != nil {
			return err
		}
		if exists {
			code = 200
			if before.Status == input.Status && before.Source != nil && *before.Source == "manual" && before.CheckedInAt != nil && before.CheckedInAt.Equal(checked) && before.CreatedBy != nil && *before.CreatedBy == actorID && before.Reason != nil && *before.Reason == input.Reason {
				result = before
				return request.save(ctx, conn, actorID, now, code, result)
			}
			if _, err := conn.ExecContext(ctx, `UPDATE attendance_records SET status=?,source='manual',confidence=NULL,device_id=NULL,checked_in_at=?,created_by=?,reason=?,updated_at=? WHERE id=?`, input.Status, formatWaktu(checked), actorID, input.Reason, formatWaktu(now), before.ID); err != nil {
				return err
			}
		} else {
			if _, err := conn.ExecContext(ctx, `INSERT INTO attendance_records
				(id,session_id,person_id,status,source,checked_in_at,created_by,reason,created_at,updated_at)
				VALUES(?,?,?,?,'manual',?,?,?,?,?)`, ids.New(), sessionID, input.PersonID, input.Status, formatWaktu(checked), actorID, input.Reason, formatWaktu(now), formatWaktu(now)); err != nil {
				if isUniqueViolation(err) {
					return ErrAlreadyCheckedIn
				}
				return err
			}
		}
		result, err = scanAttendanceRecord(conn.QueryRowContext(ctx, `SELECT `+attendanceRecordColumns+` FROM attendance_records r WHERE r.session_id=? AND r.person_id=?`, sessionID, input.PersonID))
		if err != nil {
			return err
		}
		metadata := map[string]string{"session_id": sessionID, "status_before": before.Status, "status_after": result.Status, "source_before": attendanceText(before.Source), "source_after": "manual", "checked_in_at_before": attendanceTime(before.CheckedInAt), "checked_in_at_after": formatWaktu(checked), "actor_before": attendanceText(before.CreatedBy), "actor_after": actorID}
		if err := s.auditTx(ctx, conn, actorID, "attendance.manual_corrected", "attendance_record", result.ID, input.Reason, metadata); err != nil {
			return err
		}
		return request.save(ctx, conn, actorID, now, code, result)
	})
	if err != nil {
		return AttendanceRecord{}, 0, err
	}
	return result, code, nil
}

func attendanceText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func attendanceTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatWaktu(*value)
}

func (s *Store) ListManualAttendance(ctx context.Context, actorID, sessionID string) ([]AttendanceRecord, error) {
	if teks(actorID) == "" || teks(sessionID) == "" {
		return nil, ErrInvalidInput
	}
	result := make([]AttendanceRecord, 0)
	err := s.core02ReadTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		if _, err := scopedClassSessionTx(ctx, conn, teks(actorID), teks(sessionID)); err != nil {
			return err
		}
		rows, err := conn.QueryContext(ctx, `SELECT `+attendanceRecordColumns+` FROM attendance_records r WHERE r.session_id=? AND r.source='manual' ORDER BY r.person_id`, sessionID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			value, err := scanAttendanceRecord(rows)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return rows.Err()
	})
	return result, err
}

// UNION mempertahankan catatan historis peserta yang sudah keluar dari roster.
const sessionAttendanceCTE = `WITH participants AS (
	SELECT p.id FROM class_memberships m JOIN people p ON p.id=m.person_id
	WHERE m.class_id=? AND m.left_at IS NULL AND p.status='active' AND p.deleted_at IS NULL
	UNION SELECT r.person_id FROM attendance_records r WHERE r.session_id=?
), attendance AS (
	SELECT COALESCE(r.id,'') AS id,? AS session_id,p.id AS person_id,p.external_code,p.display_name,
	COALESCE(r.status,CASE WHEN ?='closed' THEN 'absent' ELSE 'pending' END) AS status,
	r.source,r.checked_in_at,r.created_by,r.reason
	FROM participants x JOIN people p ON p.id=x.id LEFT JOIN attendance_records r ON r.person_id=p.id AND r.session_id=?
) `

func (s *Store) ListSessionAttendance(ctx context.Context, actorID, sessionID string, q AttendanceQuery) ([]AttendanceRecord, AttendanceCounts, error) {
	if teks(actorID) == "" || teks(sessionID) == "" || q.Limit < 0 || q.Limit > 201 {
		return nil, AttendanceCounts{}, ErrInvalidInput
	}
	result := make([]AttendanceRecord, 0)
	var counts AttendanceCounts
	err := s.core02ReadTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		session, err := scopedClassSessionTx(ctx, conn, teks(actorID), teks(sessionID))
		if err != nil {
			return err
		}
		counts = session.Counts
		query := sessionAttendanceCTE + `SELECT id,session_id,person_id,status,source,checked_in_at,created_by,reason,external_code,display_name FROM attendance WHERE person_id>? ORDER BY person_id`
		args := []any{session.ClassID, sessionID, sessionID, session.Status, sessionID, q.AfterID}
		if q.Limit > 0 {
			query += " LIMIT ?"
			args = append(args, q.Limit)
		}
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var value AttendanceRecord
			var source, checked, actor, reason sql.NullString
			if err := rows.Scan(&value.ID, &value.SessionID, &value.PersonID, &value.Status, &source, &checked, &actor, &reason, &value.ExternalCode, &value.DisplayName); err != nil {
				return err
			}
			value.Source, value.CreatedBy, value.Reason = optionalText(source), optionalText(actor), optionalText(reason)
			value.CheckedInAt, err = optionalTime(checked)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return rows.Err()
	})
	return result, counts, err
}

func (s *Store) ListOwnAttendance(ctx context.Context, actorID string, q OwnAttendanceQuery) ([]OwnAttendanceRecord, error) {
	actorID, q.ClassID = teks(actorID), teks(q.ClassID)
	if actorID == "" || !validCore02Range(q.From, q.To) || (q.AfterID == "") != (q.AfterCreatedAt == "") || q.Limit < 0 || q.Limit > 201 {
		return nil, ErrInvalidInput
	}
	if q.AfterCreatedAt != "" {
		if _, err := parseWaktu(q.AfterCreatedAt); err != nil {
			return nil, ErrInvalidInput
		}
	}
	result := make([]OwnAttendanceRecord, 0)
	err := s.core02ReadTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		actor, err := userByIDTx(ctx, conn, actorID)
		if errors.Is(err, ErrNotFound) {
			return ErrActorUnauthorized
		}
		if err != nil {
			return err
		}
		if actor.Role != RoleMahasiswa || actor.Status != StatusActive {
			return ErrActorUnauthorized
		}
		var personID string
		err = conn.QueryRowContext(ctx, `SELECT p.id FROM people p WHERE p.account_user_id=? AND p.status='active' AND p.deleted_at IS NULL
			AND p.identity_reference IS NOT NULL AND trim(p.identity_reference)<>'' AND p.identity_verified_at IS NOT NULL AND trim(p.identity_verified_at)<>''`, actorID).Scan(&personID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotLinked
		}
		if err != nil {
			return err
		}
		where := []string{`(r.id IS NOT NULL OR (c.deleted_at IS NULL AND s.status IN ('draft','open','closed') AND EXISTS(
			SELECT 1 FROM class_memberships m WHERE m.person_id=? AND m.class_id=s.class_id AND m.left_at IS NULL)))`}
		args := []any{personID, personID}
		if q.ClassID != "" {
			where = append(where, "s.class_id=?")
			args = append(args, q.ClassID)
		}
		if q.From != nil {
			where = append(where, "s.created_at>=?")
			args = append(args, formatWaktu(*q.From))
		}
		if q.To != nil {
			where = append(where, "s.created_at<=?")
			args = append(args, formatWaktu(*q.To))
		}
		if q.AfterID != "" {
			where = append(where, "(s.created_at>? OR (s.created_at=? AND s.id>?))")
			args = append(args, q.AfterCreatedAt, q.AfterCreatedAt, q.AfterID)
		}
		query := `SELECT s.id,s.class_id,c.code,COALESCE(s.title,''),COALESCE(r.status,CASE WHEN s.status='closed' THEN 'absent' ELSE 'pending' END),
			r.source,r.checked_in_at,r.reason,s.created_at FROM attendance_sessions s JOIN classes c ON c.id=s.class_id
			LEFT JOIN attendance_records r ON r.session_id=s.id AND r.person_id=? WHERE ` + strings.Join(where, " AND ") + ` ORDER BY s.created_at,s.id`
		if q.Limit > 0 {
			query += " LIMIT ?"
			args = append(args, q.Limit)
		}
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("auth: gagal membaca riwayat sendiri: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var value OwnAttendanceRecord
			var source, checked, reason sql.NullString
			var created string
			if err := rows.Scan(&value.SessionID, &value.ClassID, &value.ClassCode, &value.SessionTitle, &value.Status, &source, &checked, &reason, &created); err != nil {
				return err
			}
			value.Source, value.Reason = optionalText(source), optionalText(reason)
			value.CheckedInAt, err = optionalTime(checked)
			if err != nil {
				return err
			}
			value.CreatedAt, err = parseWaktu(created)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return rows.Err()
	})
	return result, err
}
