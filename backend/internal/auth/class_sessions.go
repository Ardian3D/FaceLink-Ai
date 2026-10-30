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

const classSessionColumns = `s.id,s.class_id,s.device_id,COALESCE(s.title,''),s.status,
	s.started_at,s.closes_at,s.late_after,s.closed_at,COALESCE(s.close_reason,''),s.created_by,s.created_at`

// Tidak menciptakan absent di database. Catatan lama tetap dihitung meskipun
// peserta kemudian keluar dari roster; anggota tanpa catatan hanya absent closed.
const classSessionCountsColumns = `,
	(SELECT COUNT(*) FROM attendance_records r WHERE r.session_id=s.id AND r.status='present'),
	(SELECT COUNT(*) FROM attendance_records r WHERE r.session_id=s.id AND r.status='late'),
	(SELECT COUNT(*) FROM attendance_records r WHERE r.session_id=s.id AND r.status='excused'),
	(SELECT COUNT(*) FROM attendance_records r WHERE r.session_id=s.id AND r.status='absent') +
	CASE WHEN s.status='closed' THEN (SELECT COUNT(*) FROM class_memberships m JOIN people p ON p.id=m.person_id
	 WHERE m.class_id=s.class_id AND m.left_at IS NULL AND p.deleted_at IS NULL AND p.status='active'
	 AND NOT EXISTS(SELECT 1 FROM attendance_records r WHERE r.session_id=s.id AND r.person_id=p.id)) ELSE 0 END`

func scanClassSession(row rowScanner) (ClassSession, error) {
	var result ClassSession
	var device, start, late, closed sql.NullString
	var closeAt, createdAt string
	if err := row.Scan(&result.ID, &result.ClassID, &device, &result.Title, &result.Status,
		&start, &closeAt, &late, &closed, &result.CloseReason, &result.CreatedBy, &createdAt,
		&result.Counts.Present, &result.Counts.Late, &result.Counts.Excused, &result.Counts.Absent); err != nil {
		return ClassSession{}, err
	}
	var err error
	result.DeviceID = optionalText(device)
	if result.StartedAt, err = optionalTime(start); err != nil {
		return ClassSession{}, err
	}
	if result.LateAfter, err = optionalTime(late); err != nil {
		return ClassSession{}, err
	}
	if result.ClosedAt, err = optionalTime(closed); err != nil {
		return ClassSession{}, err
	}
	if result.ClosesAt, err = parseWaktu(closeAt); err != nil {
		return ClassSession{}, err
	}
	if result.CreatedAt, err = parseWaktu(createdAt); err != nil {
		return ClassSession{}, err
	}
	return result, nil
}

func optionalText(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func optionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseWaktu(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func core02Time(value time.Time) time.Time { return value.UTC().Truncate(time.Millisecond) }

func activeSessionActorTx(ctx context.Context, conn *sql.Conn, actorID string) (Role, error) {
	actor, err := userByIDTx(ctx, conn, actorID)
	if errors.Is(err, ErrNotFound) {
		return "", ErrActorUnauthorized
	}
	if err != nil {
		return "", err
	}
	if actor.Status != StatusActive || (actor.Role != RoleAdmin && actor.Role != RoleDosen) {
		return "", ErrActorUnauthorized
	}
	return actor.Role, nil
}

func sessionClassScopeTx(ctx context.Context, conn *sql.Conn, actorID string, role Role, classID string) error {
	var allowed int
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM classes c WHERE c.id=? AND c.deleted_at IS NULL
		AND (?='admin' OR EXISTS(SELECT 1 FROM class_lecturers l WHERE l.class_id=c.id AND l.user_id=?)))`,
		classID, role, actorID).Scan(&allowed); err != nil {
		return err
	}
	if allowed == 0 {
		return ErrNotFound
	}
	return nil
}

func scopedClassSessionTx(ctx context.Context, conn *sql.Conn, actorID, sessionID string) (ClassSession, error) {
	role, err := activeSessionActorTx(ctx, conn, actorID)
	if err != nil {
		return ClassSession{}, err
	}
	result, err := scanClassSession(conn.QueryRowContext(ctx, `SELECT `+classSessionColumns+classSessionCountsColumns+`
		FROM attendance_sessions s JOIN classes c ON c.id=s.class_id
		WHERE s.id=? AND c.deleted_at IS NULL AND (?='admin' OR EXISTS(
		SELECT 1 FROM class_lecturers l WHERE l.class_id=s.class_id AND l.user_id=?))`, sessionID, role, actorID))
	if errors.Is(err, sql.ErrNoRows) {
		return ClassSession{}, ErrNotFound
	}
	if err != nil {
		return ClassSession{}, fmt.Errorf("auth: gagal membaca sesi absensi: %w", err)
	}
	return result, nil
}

// Satu snapshot SQLite meliputi role, cakupan, dan hasil seluruh pembacaan.
func (s *Store) core02ReadTx(ctx context.Context, fn func(context.Context, *sql.Conn) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "ROLLBACK")
	if err := fn(ctx, conn); err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return err
}

func validateSessionDeviceTx(ctx context.Context, conn *sql.Conn, deviceID, exceptSession string) error {
	if deviceID == "" {
		return nil
	}
	var status string
	var revoked sql.NullString
	err := conn.QueryRowContext(ctx, `SELECT status,revoked_at FROM devices WHERE id=?`, deviceID).Scan(&status, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "active" || revoked.Valid {
		return ErrDeviceUnavailable
	}
	var bound int
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM attendance_sessions WHERE device_id=? AND status='open' AND id<>?)`, deviceID, exceptSession).Scan(&bound); err != nil {
		return err
	}
	if bound != 0 {
		return ErrDeviceAlreadyBound
	}
	return nil
}

func ensureNoOpenClassTx(ctx context.Context, conn *sql.Conn, classID, exceptSession string) error {
	var open int
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM attendance_sessions WHERE class_id=? AND status='open' AND id<>?)`, classID, exceptSession).Scan(&open); err != nil {
		return err
	}
	if open != 0 {
		return ErrSessionAlreadyOpen
	}
	return nil
}

func (s *Store) CreateClassSession(ctx context.Context, actorID string, input CreateClassSessionInput, key string) (ClassSession, error) {
	actorID = teks(actorID)
	input.ClassID, input.DeviceID, input.Title = teks(input.ClassID), teks(input.DeviceID), teks(input.Title)
	if actorID == "" || input.ClassID == "" || input.ClosesAt.IsZero() || utf8.RuneCountInString(input.Title) > 120 || memuatKontrol(input.Title) {
		return ClassSession{}, ErrInvalidInput
	}
	input.ClosesAt = core02Time(input.ClosesAt)
	if input.LateAfter != nil {
		value := core02Time(*input.LateAfter)
		input.LateAfter = &value
	}
	request, err := newAttendanceIdempotency(key, "/api/v1/sessions", input)
	if err != nil {
		return ClassSession{}, err
	}
	var result ClassSession
	err = s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		role, err := activeSessionActorTx(ctx, conn, actorID)
		if err != nil {
			return err
		}
		if err := sessionClassScopeTx(ctx, conn, actorID, role, input.ClassID); err != nil {
			return err
		}
		now := core02Time(s.waktuSekarang())
		if _, replayed, err := request.replay(ctx, conn, actorID, now, &result); err != nil || replayed {
			return err
		}
		if !input.ClosesAt.After(now) || (input.LateAfter != nil && !input.LateAfter.Before(input.ClosesAt)) {
			return ErrAttendanceValidation
		}
		if err := ensureNoOpenClassTx(ctx, conn, input.ClassID, ""); err != nil {
			return err
		}
		if err := validateSessionDeviceTx(ctx, conn, input.DeviceID, ""); err != nil {
			return err
		}
		id := ids.New()
		var late any
		if input.LateAfter != nil {
			late = formatWaktu(*input.LateAfter)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO attendance_sessions
			(id,class_id,device_id,title,status,closes_at,late_after,created_by,created_at,updated_at)
			VALUES(?,?,?,?,'draft',?,?,?,?,?)`, id, input.ClassID, nullable(input.DeviceID), nullable(input.Title), formatWaktu(input.ClosesAt), late, actorID, formatWaktu(now), formatWaktu(now)); err != nil {
			return fmt.Errorf("auth: gagal membuat sesi absensi: %w", err)
		}
		if err := s.auditTx(ctx, conn, actorID, "session.created", "attendance_session", id, "session_created", map[string]string{"status_after": "draft"}); err != nil {
			return err
		}
		result, err = scopedClassSessionTx(ctx, conn, actorID, id)
		if err != nil {
			return err
		}
		return request.save(ctx, conn, actorID, now, 201, result)
	})
	if err != nil {
		return ClassSession{}, err
	}
	return result, nil
}

func (s *Store) OpenClassSession(ctx context.Context, actorID, sessionID string) (ClassSession, error) {
	actorID, sessionID = teks(actorID), teks(sessionID)
	if actorID == "" || sessionID == "" {
		return ClassSession{}, ErrInvalidInput
	}
	var result ClassSession
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		current, err := scopedClassSessionTx(ctx, conn, actorID, sessionID)
		if err != nil {
			return err
		}
		if current.Status == "open" {
			return ErrSessionAlreadyOpen
		}
		if current.Status != "draft" {
			return ErrSessionClosed
		}
		now := core02Time(s.waktuSekarang())
		if err := purgeExpiredAttendanceIdempotency(ctx, conn, now); err != nil {
			return err
		}
		if !now.Before(current.ClosesAt) {
			return ErrSessionClosed
		}
		if err := ensureNoOpenClassTx(ctx, conn, current.ClassID, sessionID); err != nil {
			return err
		}
		if current.DeviceID != nil {
			if err := validateSessionDeviceTx(ctx, conn, *current.DeviceID, sessionID); err != nil {
				return err
			}
		}
		if _, err := conn.ExecContext(ctx, `UPDATE attendance_sessions SET status='open',opens_at=?,started_at=?,updated_at=? WHERE id=?`, formatWaktu(now), formatWaktu(now), formatWaktu(now), sessionID); err != nil {
			if isUniqueViolation(err) {
				return ErrSessionAlreadyOpen
			}
			return err
		}
		if err := s.auditTx(ctx, conn, actorID, "session.opened", "attendance_session", sessionID, "session_opened", map[string]string{"status_before": "draft", "status_after": "open"}); err != nil {
			return err
		}
		result, err = scopedClassSessionTx(ctx, conn, actorID, sessionID)
		return err
	})
	if err != nil {
		return ClassSession{}, err
	}
	return result, nil
}

func (s *Store) CloseClassSession(ctx context.Context, actorID, sessionID, reason string) (ClassSession, error) {
	actorID, sessionID, reason = teks(actorID), teks(sessionID), teks(reason)
	if actorID == "" || sessionID == "" || utf8.RuneCountInString(reason) > 200 || memuatKontrol(reason) {
		return ClassSession{}, ErrInvalidInput
	}
	var result ClassSession
	err := s.immediateTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		current, err := scopedClassSessionTx(ctx, conn, actorID, sessionID)
		if err != nil {
			return err
		}
		if current.Status == "draft" {
			return ErrSessionNotOpen
		}
		if current.Status != "open" {
			return ErrSessionClosed
		}
		now := core02Time(s.waktuSekarang())
		if err := purgeExpiredAttendanceIdempotency(ctx, conn, now); err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, `UPDATE attendance_sessions SET status='closed',closed_at=?,closed_by=?,close_reason=?,updated_at=? WHERE id=?`, formatWaktu(now), actorID, nullable(reason), formatWaktu(now), sessionID); err != nil {
			return err
		}
		auditReason := reason
		if auditReason == "" {
			auditReason = "session_closed"
		}
		if err := s.auditTx(ctx, conn, actorID, "session.closed", "attendance_session", sessionID, auditReason, map[string]string{"status_before": "open", "status_after": "closed"}); err != nil {
			return err
		}
		result, err = scopedClassSessionTx(ctx, conn, actorID, sessionID)
		return err
	})
	if err != nil {
		return ClassSession{}, err
	}
	return result, nil
}

func (s *Store) GetClassSession(ctx context.Context, actorID, sessionID string) (ClassSession, error) {
	if teks(actorID) == "" || teks(sessionID) == "" {
		return ClassSession{}, ErrInvalidInput
	}
	var result ClassSession
	err := s.core02ReadTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		var err error
		result, err = scopedClassSessionTx(ctx, conn, teks(actorID), teks(sessionID))
		return err
	})
	return result, err
}

func validCore02Range(from, to *time.Time) bool { return from == nil || to == nil || !from.After(*to) }

func (s *Store) ListClassSessions(ctx context.Context, actorID string, q ClassSessionQuery) ([]ClassSession, error) {
	actorID, q.ClassID, q.Status = teks(actorID), teks(q.ClassID), teks(q.Status)
	if actorID == "" || (q.Status != "" && q.Status != "draft" && q.Status != "open" && q.Status != "closed") || !validCore02Range(q.From, q.To) || (q.AfterID == "") != (q.AfterCreatedAt == "") || q.Limit < 0 || q.Limit > 201 {
		return nil, ErrInvalidInput
	}
	if q.AfterCreatedAt != "" {
		if _, err := parseWaktu(q.AfterCreatedAt); err != nil {
			return nil, ErrInvalidInput
		}
	}
	result := make([]ClassSession, 0)
	err := s.core02ReadTx(ctx, func(ctx context.Context, conn *sql.Conn) error {
		role, err := activeSessionActorTx(ctx, conn, actorID)
		if err != nil {
			return err
		}
		if q.ClassID != "" {
			if err := sessionClassScopeTx(ctx, conn, actorID, role, q.ClassID); err != nil {
				return err
			}
		}
		where := []string{"c.deleted_at IS NULL", "(?='admin' OR EXISTS(SELECT 1 FROM class_lecturers l WHERE l.class_id=s.class_id AND l.user_id=?))"}
		args := []any{role, actorID}
		if q.ClassID != "" {
			where = append(where, "s.class_id=?")
			args = append(args, q.ClassID)
		}
		if q.Status != "" {
			where = append(where, "s.status=?")
			args = append(args, q.Status)
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
		query := `SELECT ` + classSessionColumns + classSessionCountsColumns + ` FROM attendance_sessions s JOIN classes c ON c.id=s.class_id WHERE ` + strings.Join(where, " AND ") + ` ORDER BY s.created_at,s.id`
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
			value, err := scanClassSession(rows)
			if err != nil {
				return err
			}
			result = append(result, value)
		}
		return rows.Err()
	})
	return result, err
}
