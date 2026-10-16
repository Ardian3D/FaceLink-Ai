package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// serverJWKS adalah JWKS simulasi. Tidak ada panggilan jaringan keluar: seluruh
// uji memakai httptest. Server juga menghitung permintaan agar uji dapat
// membuktikan cache tidak mengambil ulang dalam TTL.
type serverJWKS struct {
	*httptest.Server

	mu     sync.Mutex
	doc    []byte
	status int
	hit    int
	tunda  time.Duration
}

func newServerJWKS(t *testing.T) *serverJWKS {
	t.Helper()
	s := &serverJWKS{status: http.StatusOK}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		doc, status, tunda := s.doc, s.status, s.tunda
		s.hit++
		s.mu.Unlock()

		// Jeda dipakai uji single-flight agar pemanggil bersamaan benar-benar
		// menumpuk pada satu pengambilan yang sedang berjalan.
		if tunda > 0 {
			time.Sleep(tunda)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(doc)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *serverJWKS) setTunda(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tunda = d
}

func (s *serverJWKS) setDoc(doc []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc = doc
}

func (s *serverJWKS) setStatus(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *serverJWKS) hits() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hit
}

// jamUji adalah jam palsu aman-goroutine untuk uji cache JWKS. Uji single-flight
// menjalankan banyak goroutine terhadap jam yang sama, sehingga variabel jam
// biasa akan memicu keluhan race detector.
type jamUji struct {
	mu  sync.Mutex
	now time.Time
}

func jamBaru(mulai time.Time) *jamUji { return &jamUji{now: mulai} }

func (j *jamUji) Now() time.Time {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.now
}

func (j *jamUji) maju(d time.Duration) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.now = j.now.Add(d)
}

// jwkDariKunci menyusun JWK RSA dari kunci publik uji.
func jwkDariKunci(t *testing.T, kid string, pub *rsa.PublicKey) map[string]any {
	t.Helper()
	return map[string]any{
		"kty": "RSA",
		"use": "sig",
		"alg": algRS256,
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func dokumenJWKSUji(t *testing.T, kunci ...map[string]any) []byte {
	t.Helper()
	doc, err := json.Marshal(map[string]any{"keys": kunci})
	if err != nil {
		t.Fatalf("marshal JWKS uji gagal: %v", err)
	}
	return doc
}

// TestJWKSCacheAmbilDanSimpan membuktikan kunci diambil dari httptest dan tidak
// diambil ulang selama TTL belum habis.
func TestJWKSCacheAmbilDanSimpan(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })

	kunci, err := cache.KeyByID(context.Background(), "kid-uji")
	if err != nil {
		t.Fatalf("KeyByID gagal: %v", err)
	}
	pub, ok := kunci.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("kunci bertipe %T, ingin *rsa.PublicKey", kunci)
	}
	if pub.N.Cmp(utama.PublicKey.N) != 0 || pub.E != utama.PublicKey.E {
		t.Error("kunci yang dikembalikan tidak sama dengan kunci pada JWKS")
	}

	for i := range 3 {
		if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
			t.Fatalf("KeyByID ke-%d gagal: %v", i+2, err)
		}
	}
	if srv.hits() != 1 {
		t.Errorf("JWKS diambil %d kali dalam TTL, ingin 1", srv.hits())
	}
}

// TestJWKSCacheKidBaru memicu pengambilan ulang saat kid belum dikenal (rotasi
// kunci) meski TTL belum habis, dan berakhir ErrKeyNotFound bila tetap tidak
// ada.
//
// Catatan FL-011 (K-06): pengambilan ulang kini dibatasi intervalMuatMinimum,
// jadi jam uji dimajukan sebelum kid baru diminta. Tanpa pemajuan itu permintaan
// ditolak tanpa jaringan — perilaku yang diuji tersendiri oleh
// TestJWKSCacheKidAsingDibatasiInterval.
func TestJWKSCacheKidBaru(t *testing.T) {
	utama, kedua := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	jam := waktuUji
	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return jam })

	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Fatalf("KeyByID kid lama gagal: %v", err)
	}

	// Penyedia memutar kunci: kid lama hilang, kid baru muncul.
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-baru", &kedua.PublicKey)))
	jam = waktuUji.Add(intervalMuatMinimum)

	kunci, err := cache.KeyByID(context.Background(), "kid-baru")
	if err != nil {
		t.Fatalf("KeyByID kid baru gagal: %v", err)
	}
	pub, ok := kunci.(*rsa.PublicKey)
	if !ok || pub.N.Cmp(kedua.PublicKey.N) != 0 {
		t.Error("kunci kid baru tidak sesuai dokumen terbaru")
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali, ingin 2 setelah kid baru", srv.hits())
	}

	// kid lama sudah dicabut penyedia dan tidak dikembalikan dari cache lama.
	if _, err := cache.KeyByID(context.Background(), "kid-uji"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("KeyByID kid lama = %v, ingin ErrKeyNotFound", err)
	}
	if _, err := cache.KeyByID(context.Background(), "kid-hantu"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("KeyByID kid tidak ada = %v, ingin ErrKeyNotFound", err)
	}
}

// TestJWKSCacheTTLHabis membuktikan TTL habis memicu pengambilan ulang.
func TestJWKSCacheTTLHabis(t *testing.T) {
	utama, kedua := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	jam := waktuUji
	cache := NewJWKSCache(srv.URL, time.Minute, srv.Client(), func() time.Time { return jam })

	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Fatalf("KeyByID pertama gagal: %v", err)
	}

	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &kedua.PublicKey)))
	jam = waktuUji.Add(2 * time.Minute)

	kunci, err := cache.KeyByID(context.Background(), "kid-uji")
	if err != nil {
		t.Fatalf("KeyByID setelah TTL gagal: %v", err)
	}
	pub, ok := kunci.(*rsa.PublicKey)
	if !ok || pub.N.Cmp(kedua.PublicKey.N) != 0 {
		t.Error("kunci lama masih dipakai setelah TTL habis")
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali, ingin 2 setelah TTL habis", srv.hits())
	}
}

// TestJWKSCacheResponsDitolak mengumpulkan respons JWKS yang harus ditolak.
func TestJWKSCacheResponsDitolak(t *testing.T) {
	utama, _ := kunciUji(t)

	rusak := jwkDariKunci(t, "kid-uji", &utama.PublicKey)
	rusak["n"] = "bukan base64!"
	modulusNol := jwkDariKunci(t, "kid-uji", &utama.PublicKey)
	modulusNol["n"] = base64.RawURLEncoding.EncodeToString([]byte{0x00})
	eksponenRusak := jwkDariKunci(t, "kid-uji", &utama.PublicKey)
	eksponenRusak["e"] = base64.RawURLEncoding.EncodeToString([]byte{0x02})
	tanpaN := jwkDariKunci(t, "kid-uji", &utama.PublicKey)
	delete(tanpaN, "n")

	kunciEC, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("membuat kunci EC uji gagal: %v", err)
	}
	ecJWK := map[string]any{
		"kty": "EC",
		"kid": "kid-uji",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(kunciEC.X.Bytes()),
		"y":   base64.RawURLEncoding.EncodeToString(kunciEC.Y.Bytes()),
	}

	terlaluBesar := append([]byte(`{"keys":[],"pad":"`), []byte(strings.Repeat("a", jwksMaxResponseSize+1))...)
	terlaluBesar = append(terlaluBesar, []byte(`"}`)...)

	kasus := []struct {
		nama  string
		doc   []byte
		ingin error
	}{
		{nama: "bukan JSON", doc: []byte("bukan json"), ingin: ErrJWKSInvalid},
		{nama: "JSON kosong", doc: []byte(`{}`), ingin: ErrJWKSInvalid},
		{nama: "daftar kunci kosong", doc: dokumenJWKSUji(t), ingin: ErrJWKSInvalid},
		{nama: "hanya kunci EC", doc: dokumenJWKSUji(t, ecJWK), ingin: ErrJWKSInvalid},
		{nama: "modulus bukan base64", doc: dokumenJWKSUji(t, rusak), ingin: ErrJWKSInvalid},
		{nama: "modulus nol", doc: dokumenJWKSUji(t, modulusNol), ingin: ErrJWKSInvalid},
		{nama: "eksponen genap", doc: dokumenJWKSUji(t, eksponenRusak), ingin: ErrJWKSInvalid},
		{nama: "tanpa modulus", doc: dokumenJWKSUji(t, tanpaN), ingin: ErrJWKSInvalid},
		{nama: "respons melebihi 1 MiB", doc: terlaluBesar, ingin: ErrJWKSInvalid},
		{nama: "penanda rahasia tidak bocor", doc: dokumenJWKSUji(t, map[string]any{
			"kty": "RSA", "kid": "kid-uji", "n": "RAHASIA-JWKS", "e": "AQAB",
		}), ingin: ErrJWKSInvalid},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			srv := newServerJWKS(t)
			srv.setDoc(k.doc)

			cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })
			_, err := cache.KeyByID(context.Background(), "kid-uji")
			if !errors.Is(err, k.ingin) {
				t.Fatalf("KeyByID galat = %v, ingin errors.Is %v", err, k.ingin)
			}
			if strings.Contains(err.Error(), "RAHASIA-JWKS") {
				t.Errorf("pesan galat memuat isi JWKS: %v", err)
			}
		})
	}
}

// TestJWKSCacheResponsTidakTersedia membuktikan kegagalan jaringan dan status
// bukan 200 dipetakan ke ErrJWKSUnavailable tanpa membocorkan isi respons.
func TestJWKSCacheResponsTidakTersedia(t *testing.T) {
	srv := newServerJWKS(t)
	srv.setDoc([]byte(`{"keys":[],"rahasia":"JANGAN-DICATAT"}`))
	srv.setStatus(http.StatusInternalServerError)

	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })
	_, err := cache.KeyByID(context.Background(), "kid-uji")
	if !errors.Is(err, ErrJWKSUnavailable) {
		t.Fatalf("KeyByID galat = %v, ingin ErrJWKSUnavailable", err)
	}
	if strings.Contains(err.Error(), "JANGAN-DICATAT") {
		t.Errorf("pesan galat memuat isi respons: %v", err)
	}

	mati := httptest.NewServer(http.NotFoundHandler())
	url := mati.URL
	client := mati.Client()
	mati.Close()

	cacheMati := NewJWKSCache(url, time.Hour, client, func() time.Time { return waktuUji })
	if _, err := cacheMati.KeyByID(context.Background(), "kid-uji"); !errors.Is(err, ErrJWKSUnavailable) {
		t.Errorf("KeyByID server mati = %v, ingin ErrJWKSUnavailable", err)
	}
}

// TestJWKSCacheKunciTidakRelevanDilewati menegaskan kunci dengan use/alg yang
// bukan tanda tangan RS256 tidak dipakai.
func TestJWKSCacheKunciTidakRelevanDilewati(t *testing.T) {
	utama, _ := kunciUji(t)

	enkripsi := jwkDariKunci(t, "kid-enkripsi", &utama.PublicKey)
	enkripsi["use"] = "enc"
	algoritmaLain := jwkDariKunci(t, "kid-rs512", &utama.PublicKey)
	algoritmaLain["alg"] = "RS512"

	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, enkripsi, algoritmaLain,
		jwkDariKunci(t, "kid-sah", &utama.PublicKey)))

	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })

	if _, err := cache.KeyByID(context.Background(), "kid-sah"); err != nil {
		t.Fatalf("KeyByID kunci sah gagal: %v", err)
	}
	for _, kid := range []string{"kid-enkripsi", "kid-rs512"} {
		if _, err := cache.KeyByID(context.Background(), kid); !errors.Is(err, ErrKeyNotFound) {
			t.Errorf("KeyByID %q = %v, ingin ErrKeyNotFound", kid, err)
		}
	}
}

// TestJWKSCacheKidKosong memastikan kid kosong ditolak tanpa memanggil jaringan.
func TestJWKSCacheKidKosong(t *testing.T) {
	srv := newServerJWKS(t)
	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })

	if _, err := cache.KeyByID(context.Background(), ""); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("KeyByID kid kosong = %v, ingin ErrKeyNotFound", err)
	}
	if srv.hits() != 0 {
		t.Errorf("JWKS diambil %d kali untuk kid kosong, ingin 0", srv.hits())
	}
}

// TestJWKSCacheTanpaURL menegaskan URL kosong tidak membuat permintaan apa pun.
func TestJWKSCacheTanpaURL(t *testing.T) {
	cache := NewJWKSCache("", time.Hour, nil, func() time.Time { return waktuUji })
	if _, err := cache.KeyByID(context.Background(), "kid-uji"); !errors.Is(err, ErrJWKSUnavailable) {
		t.Errorf("KeyByID tanpa URL = %v, ingin ErrJWKSUnavailable", err)
	}
}

// TestJWKSCacheBawaan menegaskan nilai bawaan TTL, client, dan jam terisi.
func TestJWKSCacheBawaan(t *testing.T) {
	cache := NewJWKSCache("https://contoh.invalid/jwks", 0, nil, nil)
	if cache.ttl != ttlDefault {
		t.Errorf("ttl = %s, ingin %s", cache.ttl, ttlDefault)
	}
	if cache.client == nil || cache.now == nil {
		t.Fatal("client atau jam bawaan tidak terisi")
	}
	if !cache.diambil.IsZero() {
		t.Error("cache baru sudah dianggap terisi")
	}
}

// TestVerifierDenganJWKSCache menjalankan alur utuh: token ditandatangani kunci
// uji, kunci publik disajikan httptest, dan Verifier memakai JWKSCache sebagai
// KeySource.
func TestVerifierDenganJWKSCache(t *testing.T) {
	utama, asing := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })
	v, err := NewVerifier(Config{
		Audiences: []string{klienIDUji},
		Keys:      cache,
		Now:       func() time.Time { return waktuUji },
	})
	if err != nil {
		t.Fatalf("NewVerifier gagal: %v", err)
	}

	claims, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaimSah()))
	if err != nil {
		t.Fatalf("Verify dengan JWKS httptest gagal: %v", err)
	}
	if claims.Email != "mahasiswa@kampus.example" {
		t.Errorf("Email = %q, ingin %q", claims.Email, "mahasiswa@kampus.example")
	}

	_, err = v.Verify(context.Background(), buatToken(t, asing, headerSah(), klaimSah()))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("Verify token kunci asing = %v, ingin ErrSignatureInvalid", err)
	}
}

// TestJWKSCacheKonkuren menjalankan cache dari beberapa goroutine sekaligus
// agar `go test -race` dapat memeriksa penguncian.
func TestJWKSCacheKonkuren(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), func() time.Time { return waktuUji })

	var wg sync.WaitGroup
	galat := make([]error, 8)
	for i := range galat {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, galat[i] = cache.KeyByID(context.Background(), "kid-uji")
		}(i)
	}
	wg.Wait()

	for i, err := range galat {
		if err != nil {
			t.Errorf("KeyByID goroutine %d gagal: %v", i, err)
		}
	}
	if srv.hits() < 1 {
		t.Errorf("JWKS diambil %d kali, ingin minimal 1", srv.hits())
	}
}

// TestJWKSCacheKidAsingDibatasiInterval membuktikan kid yang dikendalikan
// penyerang tidak dapat memaksa pengambilan JWKS berulang: N permintaan
// bersamaan dengan kid acak yang tidak dikenal hanya menghasilkan SATU
// permintaan ke penyedia, dan permintaan berikutnya dalam interval yang sama
// ditolak tanpa jaringan sama sekali (K-06).
func TestJWKSCacheKidAsingDibatasiInterval(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))
	// Jeda membuat seluruh goroutine menumpuk pada satu pengambilan berjalan,
	// sehingga single-flight benar-benar teruji, bukan sekadar kebetulan waktu.
	srv.setTunda(20 * time.Millisecond)

	jam := jamBaru(waktuUji)
	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), jam.Now)

	// Pemanasan: cache terisi dan kid-uji tersedia.
	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Fatalf("KeyByID pemanasan gagal: %v", err)
	}
	if srv.hits() != 1 {
		t.Fatalf("JWKS diambil %d kali setelah pemanasan, ingin 1", srv.hits())
	}

	// Lonjakan kid acak dalam interval yang sama: seluruh permintaan ditolak
	// tanpa satu pun permintaan jaringan.
	const jumlah = 32
	var wg sync.WaitGroup
	galat := make([]error, jumlah)
	for i := range galat {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, galat[i] = cache.KeyByID(context.Background(), "kid-asing-"+strconv.Itoa(i))
		}(i)
	}
	wg.Wait()

	for i, err := range galat {
		if !errors.Is(err, ErrKeyNotFound) {
			t.Errorf("KeyByID kid asing ke-%d = %v, ingin ErrKeyNotFound", i, err)
		}
	}
	if srv.hits() != 1 {
		t.Errorf("JWKS diambil %d kali setelah lonjakan kid asing, ingin tetap 1", srv.hits())
	}

	// Kunci lama tetap dapat disajikan dari cache selama interval berjalan.
	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Errorf("KeyByID kid-uji setelah lonjakan gagal: %v", err)
	}
	if srv.hits() != 1 {
		t.Errorf("JWKS diambil %d kali untuk kunci yang sudah ada, ingin 1", srv.hits())
	}

	// Setelah interval lewat, kid yang sudah ada di dokumen baru boleh dipicu.
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey),
		jwkDariKunci(t, "kid-baru", &utama.PublicKey)))
	jam.maju(intervalMuatMinimum)

	if _, err := cache.KeyByID(context.Background(), "kid-baru"); err != nil {
		t.Fatalf("KeyByID kid baru setelah interval gagal: %v", err)
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali setelah interval lewat, ingin 2", srv.hits())
	}

	// Dan kembali dibatasi untuk interval berikutnya.
	if _, err := cache.KeyByID(context.Background(), "kid-asing-lagi"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("KeyByID kid asing setelah interval = %v, ingin ErrKeyNotFound", err)
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali dalam interval kedua, ingin tetap 2", srv.hits())
	}
}

// TestJWKSCacheSingleFlight membuktikan banyak pemanggil yang bersamaan
// membutuhkan pengambilan ulang berbagi SATU permintaan, dan permintaan itu
// tidak berjalan sambil memegang kunci cache (K-06).
func TestJWKSCacheSingleFlight(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))
	srv.setTunda(20 * time.Millisecond)

	jam := jamBaru(waktuUji)
	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), jam.Now)

	const jumlah = 16
	var wg sync.WaitGroup
	galat := make([]error, jumlah)
	for i := range galat {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, galat[i] = cache.KeyByID(context.Background(), "kid-uji")
		}(i)
	}
	wg.Wait()

	for i, err := range galat {
		if err != nil {
			t.Errorf("KeyByID goroutine %d gagal: %v", i, err)
		}
	}
	if srv.hits() != 1 {
		t.Errorf("JWKS diambil %d kali oleh %d pemanggil bersamaan, ingin 1 (single-flight)", srv.hits(), jumlah)
	}
}

// TestJWKSCacheKidAcakBersamaanPadaCacheKosong membuktikan lonjakan kid acak
// yang dikendalikan penyerang terhadap cache yang masih kosong hanya memicu
// SATU pengambilan, bukan satu per permintaan: seluruh pemanggil bersamaan
// berbagi pengambilan yang sama lalu menerima ErrKeyNotFound (K-06).
func TestJWKSCacheKidAcakBersamaanPadaCacheKosong(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))
	srv.setTunda(20 * time.Millisecond)

	jam := jamBaru(waktuUji)
	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), jam.Now)

	const jumlah = 24
	var wg sync.WaitGroup
	galat := make([]error, jumlah)
	for i := range galat {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, galat[i] = cache.KeyByID(context.Background(), "kid-acak-"+strconv.Itoa(i))
		}(i)
	}
	wg.Wait()

	for i, err := range galat {
		if !errors.Is(err, ErrKeyNotFound) {
			t.Errorf("KeyByID kid acak ke-%d = %v, ingin ErrKeyNotFound", i, err)
		}
	}
	if srv.hits() != 1 {
		t.Errorf("JWKS diambil %d kali untuk %d kid acak bersamaan, ingin 1", srv.hits(), jumlah)
	}
}

// TestJWKSCacheGagalTidakMenghapusKunci membuktikan kegagalan pengambilan tidak
// menghapus kunci yang sudah tersimpan dan tidak diulang bertubi-tubi: satu
// percobaan gagal, lalu percobaan berikutnya dalam interval yang sama ditolak
// tanpa jaringan sementara kunci lama tetap dapat disajikan (K-06).
func TestJWKSCacheGagalTidakMenghapusKunci(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	jam := jamBaru(waktuUji)
	cache := NewJWKSCache(srv.URL, time.Minute, srv.Client(), jam.Now)

	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Fatalf("KeyByID awal gagal: %v", err)
	}

	// TTL habis dan penyedia sedang rusak.
	jam.maju(2 * time.Minute)
	srv.setStatus(http.StatusInternalServerError)

	// Kid yang tidak ada di cache memaksa satu percobaan; kegagalannya
	// dilaporkan apa adanya, bukan sebagai ErrKeyNotFound.
	if _, err := cache.KeyByID(context.Background(), "kid-asing"); !errors.Is(err, ErrJWKSUnavailable) {
		t.Fatalf("KeyByID saat penyedia rusak = %v, ingin ErrJWKSUnavailable", err)
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali, ingin 2 setelah satu percobaan gagal", srv.hits())
	}

	// Interval belum lewat: tidak ada percobaan jaringan baru.
	if _, err := cache.KeyByID(context.Background(), "kid-asing"); !errors.Is(err, ErrJWKSUnavailable) {
		t.Errorf("KeyByID dalam interval setelah gagal = %v, ingin ErrJWKSUnavailable", err)
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali dalam interval setelah gagal, ingin tetap 2", srv.hits())
	}

	// Kunci lama tidak terhapus: meski TTL-nya sudah lewat dan penyegaran
	// tertahan pembatasan interval, kunci yang tersimpan tetap disajikan.
	kunci, err := cache.KeyByID(context.Background(), "kid-uji")
	if err != nil {
		t.Fatalf("KeyByID kunci lama setelah kegagalan gagal: %v", err)
	}
	pub, ok := kunci.(*rsa.PublicKey)
	if !ok || pub.N.Cmp(utama.PublicKey.N) != 0 {
		t.Error("kunci lama tidak disajikan setelah pengambilan gagal")
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali untuk kunci lama, ingin tetap 2", srv.hits())
	}

	// Setelah penyedia pulih dan interval lewat, cache disegarkan kembali.
	srv.setStatus(http.StatusOK)
	jam.maju(intervalMuatMinimum)
	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Fatalf("KeyByID setelah penyedia pulih gagal: %v", err)
	}
	if srv.hits() != 3 {
		t.Errorf("JWKS diambil %d kali, ingin 3 setelah penyedia pulih", srv.hits())
	}
}

// TestJWKSCacheKegagalanDiingat membuktikan kegagalan pengambilan pada cache
// kosong diingat dan tidak diulang dalam interval yang sama, dengan galat asal
// yang tetap dapat dicocokkan pemanggil (K-06).
func TestJWKSCacheKegagalanDiingat(t *testing.T) {
	utama, _ := kunciUji(t)
	srv := newServerJWKS(t)
	srv.setDoc([]byte(`{"keys":[]}`))
	srv.setStatus(http.StatusBadGateway)

	jam := jamBaru(waktuUji)
	cache := NewJWKSCache(srv.URL, time.Hour, srv.Client(), jam.Now)

	if _, err := cache.KeyByID(context.Background(), "kid-uji"); !errors.Is(err, ErrJWKSUnavailable) {
		t.Fatalf("KeyByID pertama = %v, ingin ErrJWKSUnavailable", err)
	}
	if srv.hits() != 1 {
		t.Fatalf("JWKS diambil %d kali, ingin 1", srv.hits())
	}

	for i := range 5 {
		if _, err := cache.KeyByID(context.Background(), "kid-uji"); !errors.Is(err, ErrJWKSUnavailable) {
			t.Fatalf("KeyByID ke-%d dalam interval = %v, ingin ErrJWKSUnavailable", i+2, err)
		}
	}
	if srv.hits() != 1 {
		t.Errorf("JWKS diambil %d kali dalam interval, ingin tetap 1", srv.hits())
	}

	jam.maju(intervalMuatMinimum)
	srv.setStatus(http.StatusOK)
	srv.setDoc(dokumenJWKSUji(t, jwkDariKunci(t, "kid-uji", &utama.PublicKey)))

	if _, err := cache.KeyByID(context.Background(), "kid-uji"); err != nil {
		t.Fatalf("KeyByID setelah interval gagal: %v", err)
	}
	if srv.hits() != 2 {
		t.Errorf("JWKS diambil %d kali, ingin 2 setelah interval lewat", srv.hits())
	}
}
