package ids

import (
	"regexp"
	"sync"
	"testing"
)

// polaUUID memastikan bentuk identifier sesuai UUID v4 seperti yang diharapkan
// docs/DATA_MODEL.md §3 (teks UUID, bukan bilangan yang bocor ke API).
var polaUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewMembentukUUIDV4(t *testing.T) {
	id := New()
	if !polaUUID.MatchString(id) {
		t.Fatalf("New() = %q, bukan UUID v4", id)
	}
}

// TestNewTidakMenghasilkanDuplikat memeriksa identifier yang dibuat bersamaan
// dari beberapa goroutine tetap unik — tabrakan identifier akan menimpa baris
// audit atau sesi tanpa gejala lain.
func TestNewTidakMenghasilkanDuplikat(t *testing.T) {
	const jumlah = 500

	hasil := make(chan string, jumlah)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jumlah / 4 {
				hasil <- New()
			}
		}()
	}
	wg.Wait()
	close(hasil)

	terlihat := make(map[string]bool, jumlah)
	for id := range hasil {
		if terlihat[id] {
			t.Fatalf("identifier berulang: %q", id)
		}
		terlihat[id] = true
	}
	if len(terlihat) != jumlah/4*4 {
		t.Fatalf("jumlah identifier = %d, ingin %d", len(terlihat), jumlah/4*4)
	}
}
