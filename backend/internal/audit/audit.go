// Package audit menulis jejak aktivitas keamanan ke tabel `audit_events`
// (D-036). Nilai yang masuk divalidasi sebelum ditulis: hanya kunci metadata
// yang terdaftar yang diterima, sehingga email, token, ID token Google,
// embedding, dan kredensial perangkat tidak dapat ikut tersimpan karena salah
// tulis di pemanggil.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/ids"
)

// ActorType mengikuti CHECK pada tabel audit_events.
type ActorType string

// Jenis aktor yang diizinkan skema.
const (
	ActorUser   ActorType = "user"
	ActorDevice ActorType = "device"
	ActorSystem ActorType = "system"
)

// Result mengikuti CHECK pada tabel audit_events.
type Result string

// Hasil peristiwa yang diizinkan skema.
const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

// Execer adalah bagian *sql.DB yang dibutuhkan penulis audit.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Event adalah satu peristiwa audit.
type Event struct {
	ActorType  ActorType
	ActorID    string
	Action     string
	EntityType string
	EntityID   string
	Result     Result
	Reason     string
	RequestID  string
	// Metadata hanya boleh memakai kunci pada kunciMetadata. Nilai dibatasi
	// panjangnya dan tidak boleh mengandung karakter kontrol.
	Metadata map[string]string
}

// kunciMetadata adalah daftar tertutup kunci metadata yang boleh disimpan.
// Daftar ini sengaja sempit: apa pun yang dapat memuat identitas, token, atau
// data biometrik tidak termasuk di dalamnya.
var kunciMetadata = map[string]bool{
	"reason_code":       true,
	"role_before":       true,
	"role_after":        true,
	"status_before":     true,
	"status_after":      true,
	"provider":          true,
	"session_id":        true,
	"method":            true,
	"path":              true,
	"count":             true,
	"migration":         true,
	"identifier_prefix": true,
}

// batasNilaiMetadata membatasi panjang setiap nilai metadata.
const batasNilaiMetadata = 200

// Writer menulis peristiwa audit.
type Writer struct {
	exec Execer
	now  clock.Clock
}

// NewWriter membuat penulis audit. Jam disuntikkan agar waktu peristiwa dapat
// diperiksa pengujian.
func NewWriter(exec Execer, clk clock.Clock) *Writer {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Writer{exec: exec, now: clk}
}

// Record memvalidasi peristiwa lalu menulisnya. Peristiwa yang tidak sah
// ditolak sebelum menyentuh database: lebih baik kehilangan satu baris audit
// yang salah bentuk daripada menyimpan data sensitif ke dalamnya.
func (w *Writer) Record(ctx context.Context, e Event) error {
	if w == nil || w.exec == nil {
		return errors.New("audit: penulis tidak siap")
	}
	if strings.TrimSpace(e.Action) == "" {
		return errors.New("audit: aksi wajib diisi")
	}
	if !aktorSah(e.ActorType) {
		return fmt.Errorf("audit: actor_type %q tidak dikenal", e.ActorType)
	}
	if e.Result != ResultSuccess && e.Result != ResultFailure {
		return fmt.Errorf("audit: result %q tidak dikenal", e.Result)
	}

	safe, err := bersihkanMetadata(e.Metadata)
	if err != nil {
		return err
	}

	_, err = w.exec.ExecContext(ctx,
		`INSERT INTO audit_events
		   (id, occurred_at, actor_type, actor_id, action, entity_type, entity_id, result, reason, request_id, metadata_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ids.New(),
		w.now.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		string(e.ActorType),
		nullable(e.ActorID),
		e.Action,
		nullable(e.EntityType),
		nullable(e.EntityID),
		string(e.Result),
		nullable(e.Reason),
		nullable(e.RequestID),
		safe,
	)
	if err != nil {
		return fmt.Errorf("audit: gagal menulis peristiwa %s: %w", e.Action, err)
	}
	return nil
}

// bersihkanMetadata memvalidasi dan menyusun JSON metadata yang aman.
func bersihkanMetadata(metadata map[string]string) (any, error) {
	if len(metadata) == 0 {
		return nil, nil
	}
	bersih := make(map[string]string, len(metadata))
	for kunci, nilai := range metadata {
		if !kunciMetadata[kunci] {
			return nil, fmt.Errorf("audit: kunci metadata %q tidak diizinkan", kunci)
		}
		if len(nilai) > batasNilaiMetadata {
			return nil, fmt.Errorf("audit: nilai metadata %q melebihi %d karakter", kunci, batasNilaiMetadata)
		}
		if memuatKarakterKontrol(nilai) {
			return nil, fmt.Errorf("audit: nilai metadata %q memuat karakter kontrol", kunci)
		}
		bersih[kunci] = nilai
	}
	isi, err := json.Marshal(bersih)
	if err != nil {
		return nil, fmt.Errorf("audit: metadata tidak dapat disusun: %w", err)
	}
	return string(isi), nil
}

func memuatKarakterKontrol(nilai string) bool {
	for _, r := range nilai {
		if r < 0x20 && r != '\t' {
			return true
		}
	}
	return false
}

func aktorSah(a ActorType) bool {
	return a == ActorUser || a == ActorDevice || a == ActorSystem
}

func nullable(nilai string) any {
	if strings.TrimSpace(nilai) == "" {
		return nil
	}
	return nilai
}

// FormatWaktu mengembalikan format waktu yang dipakai kolom teks waktu.
func FormatWaktu(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
