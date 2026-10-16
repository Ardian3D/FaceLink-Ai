package oidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// ttlDefault adalah masa pakai cache JWKS bila pemanggil tidak menentukan TTL.
const ttlDefault = time.Hour

// intervalMuatMinimum adalah jarak minimum antar percobaan pengambilan JWKS,
// termasuk percobaan yang gagal. `kid` pada header token dikendalikan
// penyerang: tanpa batas ini setiap kid acak memaksa satu permintaan HTTPS ke
// penyedia identitas, sehingga lonjakan permintaan login berubah menjadi
// lonjakan trafik ke Google. Dengan batas ini jumlah permintaan keluar paling
// banyak satu per interval, berapa pun volume permintaan masuk.
const intervalMuatMinimum = 60 * time.Second

// JWKSCache mengambil JWKS dari satu URL dan menyimpan kunci publik per `kid`.
// Cache hanya memakai http.Client yang diberikan, sehingga pemanggil menguasai
// proxy, timeout, dan kebijakan jaringan lainnya. Isi respons maupun token
// tidak pernah dicatat.
//
// JWKSCache aman dipakai dari beberapa goroutine.
type JWKSCache struct {
	url    string
	ttl    time.Duration
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex
	kunci   map[string]crypto.PublicKey
	diambil time.Time
	// percobaan adalah waktu percobaan pengambilan terakhir, berhasil maupun
	// gagal, dan galatMuat menyimpan kegagalan terakhir agar pemanggil
	// berikutnya menerima galat asal tanpa percobaan jaringan baru.
	percobaan time.Time
	galatMuat error
	// terbang menandai pengambilan yang sedang berjalan agar pemanggil
	// bersamaan berbagi satu permintaan (single-flight).
	terbang *muatTerbang
}

// muatTerbang adalah satu pengambilan JWKS yang sedang berjalan. Goroutine
// pertama yang membutuhkannya menjalankannya di luar kunci cache; goroutine
// lain menunggu saluran selesai lalu membaca ulang cache.
type muatTerbang struct {
	selesai chan struct{}
	galat   error
}

// NewJWKSCache membuat cache JWKS. TTL nol atau negatif memakai bawaan satu
// jam; client nil memakai http.Client dengan batas waktu wajar; now nil memakai
// time.Now.
func NewJWKSCache(url string, ttl time.Duration, client *http.Client, now func() time.Time) *JWKSCache {
	if ttl <= 0 {
		ttl = ttlDefault
	}
	if client == nil {
		client = &http.Client{Timeout: jwksFetchTimeout}
	}
	if now == nil {
		now = time.Now
	}
	return &JWKSCache{
		url:    url,
		ttl:    ttl,
		client: client,
		now:    now,
		kunci:  make(map[string]crypto.PublicKey),
	}
}

// KeyByID mengembalikan kunci publik untuk kid. Kunci disajikan dari cache
// selama TTL belum habis; TTL habis atau kid belum dikenal memicu pengambilan
// ulang agar rotasi kunci penyedia terikuti tanpa menunggu TTL, tetapi tidak
// lebih sering daripada intervalMuatMinimum.
//
// Permintaan jaringan tidak pernah berjalan sambil memegang kunci cache, dan
// pemanggil bersamaan yang membutuhkan pengambilan ulang berbagi satu
// permintaan yang sama (single-flight). Setelah pengambilan selesai, cache
// dibaca ulang.
func (c *JWKSCache) KeyByID(ctx context.Context, kid string) (crypto.PublicKey, error) {
	if kid == "" {
		return nil, fmt.Errorf("%w: kid kosong", ErrKeyNotFound)
	}

	// Jalur cepat: kunci sudah ada dan cache masih dalam TTL.
	c.mu.Lock()
	if kunci, ok := c.kunci[kid]; ok && c.segar() {
		c.mu.Unlock()
		return kunci, nil
	}

	terbang, jalankan := c.mulaiMuat()
	if terbang == nil {
		// Pembatasan interval aktif: pengambilan ulang belum boleh dijalankan,
		// jadi tidak ada permintaan jaringan sama sekali di jalur ini.
		galatMuat := c.galatMuat
		kunci, ada := c.kunci[kid]
		c.mu.Unlock()

		if ada {
			// Kunci tersimpan tetap disajikan meski TTL-nya sudah lewat:
			// menolak token sah hanya karena pengambilan ulang belum boleh
			// dijalankan akan mematikan login saat penyedia identitas
			// terganggu.
			return kunci, nil
		}
		if galatMuat != nil {
			// Tidak ada kunci tersimpan dan percobaan terakhir gagal: galat
			// asal diteruskan agar pemanggil melihat penyebabnya, bukan
			// ErrKeyNotFound yang menyesatkan, dan tetap tanpa percobaan
			// jaringan baru.
			return nil, galatMuat
		}
		return nil, fmt.Errorf("%w: kid %q", ErrKeyNotFound, kid)
	}
	c.mu.Unlock()

	var err error
	if jalankan {
		err = c.jalankanMuat(ctx, terbang)
	} else {
		err = c.tungguMuat(ctx, terbang)
	}
	if err != nil {
		return nil, err
	}

	// Cache dibaca ulang setelah pengambilan selesai, termasuk oleh goroutine
	// yang hanya menunggu pengambilan goroutine lain.
	c.mu.Lock()
	kunci, ok := c.kunci[kid]
	c.mu.Unlock()
	if ok {
		return kunci, nil
	}
	return nil, fmt.Errorf("%w: kid %q", ErrKeyNotFound, kid)
}

// mulaiMuat menentukan siapa yang menjalankan pengambilan JWKS berikutnya.
// Kembaliannya: pengambilan yang harus ditunggu atau dijalankan, dan penanda
// bahwa pemanggil yang menjalankannya. `terbang` nil berarti pengambilan tidak
// boleh dilakukan sekarang. Pemanggil wajib memegang kunci cache.
//
// Tidak ada pengambilan baru selama satu pengambilan masih berjalan, dan tidak
// ada pengambilan baru sebelum intervalMuatMinimum lewat sejak percobaan
// terakhir, berhasil maupun gagal. Karena itu lonjakan kid acak dari penyerang
// hanya menghasilkan paling banyak satu permintaan keluar per interval.
func (c *JWKSCache) mulaiMuat() (*muatTerbang, bool) {
	if c.terbang != nil {
		return c.terbang, false
	}

	if !c.percobaan.IsZero() && c.now().Before(c.percobaan.Add(intervalMuatMinimum)) {
		return nil, false
	}

	c.terbang = &muatTerbang{selesai: make(chan struct{})}
	return c.terbang, true
}

// jalankanMuat menjalankan pengambilan JWKS di luar kunci cache, menyimpan
// hasilnya bila berhasil, lalu mengumumkan hasilnya kepada penunggu.
// Pemanggil tidak boleh memegang kunci.
func (c *JWKSCache) jalankanMuat(ctx context.Context, terbang *muatTerbang) error {
	kunci, err := c.ambil(ctx)

	c.mu.Lock()
	c.percobaan = c.now()
	c.galatMuat = err
	if err == nil {
		// Hanya pengambilan yang berhasil mengganti isi cache. Kegagalan
		// membiarkan kunci lama tetap terpakai sampai pengambilan berikutnya
		// berhasil.
		c.kunci = kunci
		c.diambil = c.percobaan
	}
	c.terbang = nil
	c.mu.Unlock()

	terbang.galat = err
	close(terbang.selesai)
	return err
}

// tungguMuat menunggu pengambilan yang dijalankan goroutine lain dan memakai
// hasilnya, sehingga satu permintaan jaringan melayani seluruh pemanggil.
// Pemanggil tidak boleh memegang kunci.
func (c *JWKSCache) tungguMuat(ctx context.Context, terbang *muatTerbang) error {
	select {
	case <-terbang.selesai:
		return terbang.galat
	case <-ctx.Done():
		return fmt.Errorf("%w: menunggu pengambilan JWKS: %v", ErrJWKSUnavailable, ctx.Err())
	}
}

// segar melaporkan apakah isi cache masih dalam TTL. Pemanggil wajib memegang
// kunci.
func (c *JWKSCache) segar() bool {
	if c.diambil.IsZero() {
		return false
	}
	return c.now().Before(c.diambil.Add(c.ttl))
}

// ambil mengambil JWKS dan mengembalikan seluruh isi cache yang baru. Fungsi
// ini tidak menyentuh keadaan cache sama sekali sehingga aman dipanggil tanpa
// memegang kunci: permintaan jaringan berjalan paralel, bukan berbaris di
// belakang kunci.
//
// Kunci lama dibuang oleh pemanggil agar kid yang sudah dicabut penyedia tidak
// terus dipakai; kid yang hilang dari dokumen akan memicu pengambilan ulang
// berikutnya dan berakhir ErrKeyNotFound.
func (c *JWKSCache) ambil(ctx context.Context) (map[string]crypto.PublicKey, error) {
	if c.url == "" {
		return nil, fmt.Errorf("%w: URL JWKS kosong", ErrJWKSUnavailable)
	}

	ctx, batal := context.WithTimeout(ctx, jwksFetchTimeout)
	defer batal()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: menyusun permintaan gagal", ErrJWKSUnavailable)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: permintaan gagal: %v", ErrJWKSUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Badan respons tidak dibaca: isi galat penyedia tidak pernah masuk log.
		return nil, fmt.Errorf("%w: status %d", ErrJWKSUnavailable, resp.StatusCode)
	}

	isi, err := io.ReadAll(io.LimitReader(resp.Body, jwksMaxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: membaca respons gagal: %v", ErrJWKSUnavailable, err)
	}
	if len(isi) > jwksMaxResponseSize {
		return nil, fmt.Errorf("%w: respons melebihi %d byte", ErrJWKSInvalid, jwksMaxResponseSize)
	}

	return uraikanJWKS(isi)
}

// dokumenJWKS adalah bentuk dokumen JWKS yang dipakai. Field di luar `keys`
// diabaikan.
type dokumenJWKS struct {
	Keys []jwkRSA `json:"keys"`
}

// jwkRSA adalah JWK bertipe RSA. Field `use` dan `alg` opsional tetapi bila ada
// wajib menandakan kunci tanda tangan RS256.
type jwkRSA struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// uraikanJWKS mengubah isi JWKS menjadi peta kunci publik. Kunci yang jelas
// bukan kunci tanda tangan RSA dilewati; kunci RSA dengan n/e tidak sah membuat
// seluruh respons ditolak agar penyedia yang menyajikan data rusak tidak
// menghasilkan kunci setengah benar.
func uraikanJWKS(isi []byte) (map[string]crypto.PublicKey, error) {
	var dok dokumenJWKS
	if err := json.Unmarshal(isi, &dok); err != nil {
		return nil, fmt.Errorf("%w: JSON tidak dapat dibaca", ErrJWKSInvalid)
	}
	if len(dok.Keys) == 0 {
		return nil, fmt.Errorf("%w: tidak ada kunci", ErrJWKSInvalid)
	}

	hasil := make(map[string]crypto.PublicKey, len(dok.Keys))
	for _, k := range dok.Keys {
		if k.Kty != "RSA" {
			continue
		}
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if k.Alg != "" && k.Alg != algRS256 {
			continue
		}
		if k.Kid == "" {
			continue
		}
		if _, ada := hasil[k.Kid]; ada {
			continue
		}

		kunci, err := kunciRSADariJWK(k)
		if err != nil {
			return nil, err
		}
		hasil[k.Kid] = kunci
	}

	if len(hasil) == 0 {
		return nil, fmt.Errorf("%w: tidak ada kunci RSA yang dapat dipakai", ErrJWKSInvalid)
	}
	return hasil, nil
}

// kunciRSADariJWK menyusun *rsa.PublicKey dari modulus dan eksponen base64url.
// Nilai yang tidak masuk akal ditolak sebelum kunci dipakai verifikasi.
func kunciRSADariJWK(k jwkRSA) (*rsa.PublicKey, error) {
	if k.N == "" || k.E == "" {
		return nil, fmt.Errorf("%w: kid %q tanpa n atau e", ErrJWKSInvalid, k.Kid)
	}

	modulus, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil || len(modulus) == 0 || modulus[0] == 0 {
		return nil, fmt.Errorf("%w: kid %q modulus tidak sah", ErrJWKSInvalid, k.Kid)
	}

	eksponen, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil || len(eksponen) == 0 || len(eksponen) > 8 || eksponen[0] == 0 {
		return nil, fmt.Errorf("%w: kid %q eksponen tidak sah", ErrJWKSInvalid, k.Kid)
	}

	n := new(big.Int).SetBytes(modulus)
	e := 0
	for _, b := range eksponen {
		e = e<<8 | int(b)
	}
	if n.BitLen() < 512 || e < 3 || e%2 == 0 || int64(e) > int64(^uint32(0)>>1) {
		return nil, fmt.Errorf("%w: kid %q parameter RSA tidak sah", ErrJWKSInvalid, k.Kid)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}
