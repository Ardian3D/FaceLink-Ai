// Package clock menyediakan satu sumber waktu untuk seluruh komponen
// autentikasi. Dengan menyuntikkan jam, masa berlaku token akses, rotasi
// penyegar, batas absolut sesi, dan kedaluwarsa nonce dapat diuji tanpa tidur
// panjang dan tanpa hasil yang bergantung pada kecepatan mesin uji.
package clock

import (
	"sync"
	"time"
)

// Clock adalah sumber waktu tunggal. Implementasi selalu mengembalikan waktu
// dalam UTC agar perbandingan dengan nilai ISO 8601 yang disimpan di database
// tidak bergantung zona waktu proses.
type Clock interface {
	Now() time.Time
}

// Real memakai waktu sistem.
type Real struct{}

// Now mengembalikan waktu sistem dalam UTC.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fake adalah jam yang dikendalikan pengujian. Aman dipakai dari beberapa
// goroutine karena pengujian konkurensi sesi menjalankan beberapa permintaan
// terhadap jam yang sama.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake membuat jam uji yang dimulai pada waktu yang diberikan.
func NewFake(mulai time.Time) *Fake {
	return &Fake{now: mulai.UTC()}
}

// Now mengembalikan waktu jam uji saat ini.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance memajukan jam uji.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
