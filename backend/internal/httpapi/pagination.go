package httpapi

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

// Paginasi kursor (docs/API_CONTRACT.md §1.3).
//
// Aturan yang ditegakkan di sini:
//   - `limit` bawaan 50, maksimum 200. Nilai yang melebihi maksimum dipangkas,
//     bukan ditolak: klien yang meminta terlalu banyak tetap harus mendapat
//     jawaban yang dapat dipakai.
//   - `limit` yang bukan bilangan bulat positif ditolak `400 VALIDATION_FAILED`.
//   - Kursor bersifat buram bagi klien dan memuat jenis daftarnya, sehingga
//     kursor dari satu endpoint yang dipakai pada endpoint lain ditolak —
//     bukan diam-diam mengulang halaman pertama.
const (
	limitBawaan = 50
	limitMaks   = 200

	// jenisKursorAkun adalah penanda kursor daftar akun (`GET /users`).
	jenisKursorAkun = "u"
	// jenisKursorKelas adalah penanda kursor daftar kelas (`GET /me/classes`).
	jenisKursorKelas = "k"
)

// permintaanHalaman adalah parameter paginasi yang sudah divalidasi.
type permintaanHalaman struct {
	Limit int
	// Lanjut menandai bahwa permintaan memakai kursor, sehingga daftar harus
	// dimulai sesudah posisi kursor.
	Lanjut bool
	// AfterCreatedAt dan AfterID adalah posisi baris terakhir halaman
	// sebelumnya. Keduanya kosong pada halaman pertama.
	AfterCreatedAt string
	AfterID        string
}

// bacaHalaman membaca `limit` dan `cursor` untuk daftar berjenis `jenis`. Bila
// parameter tidak sah, galat ditulis ke respons dan nilai kedua bernilai false.
func bacaHalaman(w http.ResponseWriter, r *http.Request, jenis string) (permintaanHalaman, bool) {
	h := permintaanHalaman{Limit: limitBawaan}

	if mentah := strings.TrimSpace(r.URL.Query().Get("limit")); mentah != "" {
		nilai, err := strconv.Atoi(mentah)
		if err != nil || nilai <= 0 {
			writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Parameter limit harus bilangan bulat positif.")
			return permintaanHalaman{}, false
		}
		if nilai > limitMaks {
			nilai = limitMaks
		}
		h.Limit = nilai
	}

	posisi, ada, err := bacaKursor(strings.TrimSpace(r.URL.Query().Get("cursor")), jenis)
	if err != nil {
		writeErrorCode(w, r, http.StatusBadRequest, codeValidationFailed, "Parameter cursor tidak dapat dibaca.")
		return permintaanHalaman{}, false
	}
	if ada {
		h.Lanjut = true
		h.AfterCreatedAt, h.AfterID = posisi.kunci, posisi.id
	}
	return h, true
}

// posisiKursor adalah posisi satu baris pada daftar yang sedang dipaginasi.
// `kunci` adalah kolom pengurutan utama (waktu untuk akun, kosong untuk kelas)
// dan `id` adalah pemutus seri.
type posisiKursor struct {
	kunci string
	id    string
}

// kursorAkun membuat kursor buram untuk daftar akun, memakai kunci
// `(created_at, id)`.
func kursorAkun(createdAt, id string) string {
	return bikinKursor(jenisKursorAkun, createdAt, id)
}

// kursorKelas membuat kursor buram untuk daftar kelas, yang urutannya hanya
// ditentukan id kelas.
func kursorKelas(id string) string {
	return bikinKursor(jenisKursorKelas, "", id)
}

func bikinKursor(jenis, kunci, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(jenis + "\x00" + kunci + "\x00" + id))
}

// bacaKursor mengurai kursor buram dan memastikan jenisnya cocok. String kosong
// berarti halaman pertama.
func bacaKursor(mentah, jenis string) (posisiKursor, bool, error) {
	if mentah == "" {
		return posisiKursor{}, false, nil
	}
	isi, err := base64.RawURLEncoding.DecodeString(mentah)
	if err != nil {
		return posisiKursor{}, false, err
	}
	bagian := strings.SplitN(string(isi), "\x00", 3)
	if len(bagian) != 3 {
		return posisiKursor{}, false, errors.New("kursor tidak lengkap")
	}
	if bagian[0] != jenis {
		return posisiKursor{}, false, errors.New("kursor berasal dari daftar lain")
	}
	if bagian[2] == "" {
		return posisiKursor{}, false, errors.New("kursor tanpa id")
	}
	return posisiKursor{kunci: bagian[1], id: bagian[2]}, true, nil
}

// halamanPayload adalah bagian `page` pada amplop daftar (kontrak §1.3).
type halamanPayload struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// daftarEnvelope adalah amplop daftar bernomor: `data`, `page`, `request_id`.
type daftarEnvelope struct {
	Data      any            `json:"data"`
	Page      halamanPayload `json:"page"`
	RequestID string         `json:"request_id"`
}

// writePage menulis daftar dengan amplop halaman. `berikut` kosong berarti ini
// halaman terakhir.
func writePage(w http.ResponseWriter, r *http.Request, data any, berikut string, adaLagi bool, limit int) {
	writeJSON(w, http.StatusOK, daftarEnvelope{
		Data:      data,
		Page:      halamanPayload{NextCursor: berikut, HasMore: adaLagi, Limit: limit},
		RequestID: middleware.GetReqID(r.Context()),
	})
}
