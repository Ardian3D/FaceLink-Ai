package audit

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ardian3D/FaceLink-Ai/backend/internal/clock"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/database"
	"github.com/Ardian3D/FaceLink-Ai/backend/internal/migrations"
)

func siapkanDB(t *testing.T) *database.DB {
	t.Helper()
	ctx := context.Background()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "facelink-audit.db"))
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
	return db
}

func TestRecordMenulisPeristiwaValid(t *testing.T) {
	ctx := context.Background()
	db := siapkanDB(t)
	waktu := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	w := NewWriter(db, clock.NewFake(waktu))

	err := w.Record(ctx, Event{
		ActorType:  ActorUser,
		ActorID:    "u1",
		Action:     "user.role_changed",
		EntityType: "user",
		EntityID:   "u2",
		Result:     ResultSuccess,
		Reason:     "Perubahan role oleh admin",
		RequestID:  "req-1",
		Metadata:   map[string]string{"role_before": "dosen", "role_after": "admin"},
	})
	if err != nil {
		t.Fatalf("Record() gagal: %v", err)
	}

	var (
		aktor    string
		aksi     string
		hasil    string
		metadata string
	)
	if err := db.QueryRowContext(ctx,
		`SELECT actor_type, action, result, metadata_json FROM audit_events WHERE action = 'user.role_changed'`).
		Scan(&aktor, &aksi, &hasil, &metadata); err != nil {
		t.Fatalf("pembacaan audit gagal: %v", err)
	}
	if aktor != "user" || hasil != "success" {
		t.Errorf("baris audit = (%q,%q), ingin (user,success)", aktor, hasil)
	}
	if !strings.Contains(metadata, "role_after") {
		t.Errorf("metadata tidak memuat perubahan role: %s", metadata)
	}
}

// TestRecordMenolakMetadataTerlarang adalah inti pertahanan paket ini: kunci di
// luar daftar tertutup ditolak sehingga email, token, atau data biometrik tidak
// dapat masuk karena salah tulis di pemanggil.
func TestRecordMenolakMetadataTerlarang(t *testing.T) {
	ctx := context.Background()
	db := siapkanDB(t)
	w := NewWriter(db, clock.Real{})

	kasus := map[string]map[string]string{
		"email":           {"email": "peserta@contoh.test"},
		"token":           {"access_token": "rahasia"},
		"id token":        {"id_token": "rahasia"},
		"embedding":       {"embedding": "0.1,0.2"},
		"kunci tak cocok": {"reason_code": strings.Repeat("x", batasNilaiMetadata+1)},
		"kontrol":         {"reason_code": "baris\nbaru"},
	}
	for nama, metadata := range kasus {
		err := w.Record(ctx, Event{
			ActorType: ActorSystem,
			Action:    "auth.login.denied",
			Result:    ResultFailure,
			Metadata:  metadata,
		})
		if err == nil {
			t.Errorf("metadata %s seharusnya ditolak", nama)
		}
	}

	var jumlah int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events`).Scan(&jumlah); err != nil {
		t.Fatalf("penghitungan audit gagal: %v", err)
	}
	if jumlah != 0 {
		t.Errorf("jumlah baris audit = %d, ingin 0 (peristiwa tidak sah tidak boleh ditulis)", jumlah)
	}
}

func TestRecordMenolakPeristiwaTidakLengkap(t *testing.T) {
	ctx := context.Background()
	db := siapkanDB(t)
	w := NewWriter(db, clock.Real{})

	kasus := map[string]Event{
		"aksi kosong":       {ActorType: ActorSystem, Result: ResultSuccess},
		"aktor tak dikenal": {ActorType: "robot", Action: "auth.logout", Result: ResultSuccess},
		"hasil tak dikenal": {ActorType: ActorSystem, Action: "auth.logout", Result: "mungkin"},
	}
	for nama, e := range kasus {
		if err := w.Record(ctx, e); err == nil {
			t.Errorf("peristiwa %s seharusnya ditolak", nama)
		}
	}
}

func TestWriterTanpaDatabaseMenolak(t *testing.T) {
	w := NewWriter(nil, clock.Real{})
	if err := w.Record(context.Background(), Event{ActorType: ActorSystem, Action: "x", Result: ResultSuccess}); err == nil {
		t.Error("penulis tanpa koneksi seharusnya gagal, bukan diam-diam berhasil")
	}
}
