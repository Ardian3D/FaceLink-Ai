package auth

import "time"

// ClassSession adalah sesi absensi, bukan sesi autentikasi.
type ClassSession struct {
	ID          string           `json:"id"`
	ClassID     string           `json:"class_id"`
	DeviceID    *string          `json:"device_id"`
	Title       string           `json:"title"`
	Status      string           `json:"status"`
	StartedAt   *time.Time       `json:"started_at"`
	ClosesAt    time.Time        `json:"closes_at"`
	LateAfter   *time.Time       `json:"late_after"`
	ClosedAt    *time.Time       `json:"closed_at"`
	CloseReason string           `json:"close_reason,omitempty"`
	CreatedBy   string           `json:"created_by"`
	CreatedAt   time.Time        `json:"created_at"`
	Counts      AttendanceCounts `json:"counts"`
}

type AttendanceCounts struct {
	Present       int `json:"present"`
	Late          int `json:"late"`
	Excused       int `json:"excused"`
	Absent        int `json:"absent"`
	PendingReview int `json:"pending_review"`
}

type CreateClassSessionInput struct {
	ClassID   string     `json:"class_id"`
	DeviceID  string     `json:"device_id"`
	Title     string     `json:"title"`
	ClosesAt  time.Time  `json:"closes_at"`
	LateAfter *time.Time `json:"late_after"`
}

type ClassSessionQuery struct {
	ClassID        string
	Status         string
	From           *time.Time
	To             *time.Time
	AfterCreatedAt string
	AfterID        string
	Limit          int
}

type ManualAttendanceInput struct {
	PersonID    string     `json:"person_id"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	CheckedInAt *time.Time `json:"checked_in_at"`
}

type AttendanceRecord struct {
	ID           string     `json:"id,omitempty"`
	SessionID    string     `json:"session_id"`
	PersonID     string     `json:"person_id"`
	ExternalCode string     `json:"external_code,omitempty"`
	DisplayName  string     `json:"display_name,omitempty"`
	Status       string     `json:"status"`
	Source       *string    `json:"source"`
	CheckedInAt  *time.Time `json:"checked_in_at"`
	CreatedBy    *string    `json:"created_by,omitempty"`
	Reason       *string    `json:"reason"`
}

type AttendanceQuery struct {
	AfterID string
	Limit   int
}

type OwnAttendanceQuery struct {
	ClassID        string
	From           *time.Time
	To             *time.Time
	AfterCreatedAt string
	AfterID        string
	Limit          int
}

type OwnAttendanceRecord struct {
	SessionID    string     `json:"session_id"`
	ClassID      string     `json:"class_id"`
	ClassCode    string     `json:"class_code"`
	SessionTitle string     `json:"session_title"`
	Status       string     `json:"status"`
	Source       *string    `json:"source"`
	CheckedInAt  *time.Time `json:"checked_in_at"`
	Reason       *string    `json:"reason"`
	CreatedAt    time.Time  `json:"-"`
}
