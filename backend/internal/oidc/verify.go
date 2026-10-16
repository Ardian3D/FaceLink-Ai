// Package oidc memverifikasi Google ID token memakai pustaka standar saja.
//
// Paket ini sengaja tidak memegang keadaan selain cache JWKS: ia tidak menulis
// database, tidak menerbitkan sesi, dan tidak menyimpan token. Tanggung jawab
// pemakaian sekali pakai nonce ada di pemanggil (tabel `auth_nonces`); di sini
// hanya disediakan pembandingan nonce dengan waktu tetap.
package oidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Issuer bawaan Google bila Config.Issuers dikosongkan.
const (
	issuerGoogle        = "accounts.google.com"
	issuerGoogleHTTPS   = "https://accounts.google.com"
	algRS256            = "RS256"
	leewayDefault       = 2 * time.Minute
	maxSegmentBytes     = 512 * 1024
	maxSignatureBytes   = 512
	maxClaimsBytes      = 256 * 1024
	jwksFetchTimeout    = 10 * time.Second
	jwksMaxResponseSize = 1 << 20 // 1 MiB
)

// KeySource menyediakan kunci publik berdasarkan `kid` pada header token.
// Implementasi wajib aman dipakai dari beberapa goroutine.
type KeySource interface {
	KeyByID(ctx context.Context, kid string) (crypto.PublicKey, error)
}

// Config menyetel perilaku Verifier. Hanya Audiences dan Keys yang wajib.
type Config struct {
	// Audiences adalah client ID Google yang diizinkan; wajib tidak kosong.
	Audiences []string
	// Issuers adalah issuer yang diterima; kosong berarti issuer Google
	// bawaan.
	Issuers []string
	// Keys adalah sumber kunci publik penanda tangan.
	Keys KeySource
	// Now adalah sumber waktu; kosong berarti time.Now.
	Now func() time.Time
	// Leeway adalah toleransi selisih waktu; nol berarti dua menit.
	Leeway time.Duration
}

// Verifier memverifikasi ID token terhadap konfigurasi yang dibekukan saat
// pembuatan. Verifier aman dipakai dari beberapa goroutine.
type Verifier struct {
	audiences map[string]struct{}
	issuers   map[string]struct{}
	keys      KeySource
	now       func() time.Time
	leeway    time.Duration
}

// Claims adalah klaim ID token yang dipakai alur login. Nilai waktu selalu
// dalam UTC.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Nonce         string
	Audience      string
	Issuer        string
	IssuedAt      time.Time
	ExpiresAt     time.Time
}

// CheckNonce membandingkan nonce yang diharapkan dengan klaim token memakai
// perbandingan waktu tetap. Nonce kosong pada `want` berarti pemanggil tidak
// meminta pemeriksaan. Pencegahan pemakaian ulang nonce bukan tanggung jawab
// paket ini.
func (c Claims) CheckNonce(want string) error {
	if want == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(want), []byte(c.Nonce)) != 1 {
		return ErrNonceMismatch
	}
	return nil
}

// NewVerifier memvalidasi konfigurasi dan mengembalikan verifier. Audience
// kosong, entri audience/issuer kosong, atau Keys nil ditolak.
func NewVerifier(cfg Config) (*Verifier, error) {
	if cfg.Keys == nil {
		return nil, fmt.Errorf("%w: sumber kunci wajib diisi", ErrConfigInvalid)
	}

	audiences, err := daftarUnik(cfg.Audiences, "audience")
	if err != nil {
		return nil, err
	}

	issuers := cfg.Issuers
	if len(issuers) == 0 {
		issuers = []string{issuerGoogle, issuerGoogleHTTPS}
	}
	issuerSet, err := daftarUnik(issuers, "issuer")
	if err != nil {
		return nil, err
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	leeway := cfg.Leeway
	if leeway == 0 {
		leeway = leewayDefault
	}
	if leeway < 0 {
		return nil, fmt.Errorf("%w: leeway tidak boleh negatif", ErrConfigInvalid)
	}

	return &Verifier{
		audiences: audiences,
		issuers:   issuerSet,
		keys:      cfg.Keys,
		now:       now,
		leeway:    leeway,
	}, nil
}

// daftarUnik mengubah daftar menjadi himpunan sambil menolak daftar kosong dan
// entri kosong. Nama dipakai hanya untuk pesan galat.
func daftarUnik(nilai []string, nama string) (map[string]struct{}, error) {
	if len(nilai) == 0 {
		return nil, fmt.Errorf("%w: %s wajib tidak kosong", ErrConfigInvalid, nama)
	}
	himpunan := make(map[string]struct{}, len(nilai))
	for _, n := range nilai {
		if n == "" {
			return nil, fmt.Errorf("%w: entri %s kosong", ErrConfigInvalid, nama)
		}
		himpunan[n] = struct{}{}
	}
	return himpunan, nil
}

// headerToken adalah bagian header JWT yang dipakai verifikasi.
type headerToken struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// klaimToken memetakan klaim ID token. Tipe longgar (json.RawMessage) dipakai
// untuk klaim waktu dan `email_verified` karena penyedia dapat mengirim angka
// maupun teks; penyimpangan bentuk dipetakan ke ErrMalformedToken.
type klaimToken struct {
	Issuer        string          `json:"iss"`
	Subject       string          `json:"sub"`
	Audience      json.RawMessage `json:"aud"`
	ExpiresAt     json.RawMessage `json:"exp"`
	IssuedAt      json.RawMessage `json:"iat"`
	NotBefore     json.RawMessage `json:"nbf"`
	AuthorizedPty string          `json:"azp"`
	Nonce         string          `json:"nonce"`
	Email         string          `json:"email"`
	EmailVerified json.RawMessage `json:"email_verified"`
}

// Verify memeriksa ID token secara menyeluruh: bentuk token, algoritma,
// tanda tangan, issuer, audience, masa berlaku, subjek, dan email. Klaim yang
// dikembalikan hanya setelah seluruh pemeriksaan lolos.
func (v *Verifier) Verify(ctx context.Context, idToken string) (Claims, error) {
	header, payload, signature, signed, err := pisahToken(idToken)
	if err != nil {
		return Claims{}, err
	}

	var hdr headerToken
	if err := unmarshalObjek(header, &hdr); err != nil {
		return Claims{}, fmt.Errorf("%w: header bukan JSON objek", ErrMalformedToken)
	}
	if hdr.Alg != algRS256 {
		return Claims{}, fmt.Errorf("%w: alg %q", ErrUnsupportedAlgorithm, hdr.Alg)
	}
	if hdr.Kid == "" {
		return Claims{}, fmt.Errorf("%w: kid kosong pada header", ErrMalformedToken)
	}

	var klaim klaimToken
	if err := unmarshalObjek(payload, &klaim); err != nil {
		return Claims{}, fmt.Errorf("%w: payload bukan JSON objek", ErrMalformedToken)
	}

	if err := v.periksaTandaTangan(ctx, hdr.Kid, signed, signature); err != nil {
		return Claims{}, err
	}

	hasil, err := v.periksaKlaim(klaim)
	if err != nil {
		return Claims{}, err
	}
	return hasil, nil
}

// periksaTandaTangan mengambil kunci berdasarkan kid dan memverifikasi tanda
// tangan RS256 (PKCS#1 v1.5 atas SHA-256) atas bagian header.payload.
func (v *Verifier) periksaTandaTangan(ctx context.Context, kid string, signed, signature []byte) error {
	if len(signature) == 0 {
		return fmt.Errorf("%w: tanda tangan kosong", ErrSignatureInvalid)
	}

	key, err := v.keys.KeyByID(ctx, kid)
	if err != nil {
		return fmt.Errorf("%w: kid %q: %v", ErrSignatureInvalid, kid, err)
	}
	kunciRSA, ok := key.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("%w: kunci kid %q bukan kunci RSA", ErrSignatureInvalid, kid)
	}

	digest := sha256.Sum256(signed)
	if err := rsa.VerifyPKCS1v15(kunciRSA, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("%w: kid %q", ErrSignatureInvalid, kid)
	}
	return nil
}

// periksaKlaim memvalidasi seluruh klaim wajib dan menyusun Claims.
func (v *Verifier) periksaKlaim(klaim klaimToken) (Claims, error) {
	if _, ok := v.issuers[klaim.Issuer]; !ok {
		return Claims{}, fmt.Errorf("%w: issuer %q", ErrIssuerInvalid, klaim.Issuer)
	}

	audience, err := v.pilihAudience(klaim.Audience, klaim.AuthorizedPty)
	if err != nil {
		return Claims{}, err
	}

	expiresAt, err := waktuKlaim(klaim.ExpiresAt, "exp")
	if err != nil {
		return Claims{}, err
	}
	issuedAt, err := waktuKlaim(klaim.IssuedAt, "iat")
	if err != nil {
		return Claims{}, err
	}
	notBefore, err := waktuKlaimOpsional(klaim.NotBefore, "nbf")
	if err != nil {
		return Claims{}, err
	}

	sekarang := v.now()
	if !sekarang.Before(expiresAt.Add(v.leeway)) {
		return Claims{}, fmt.Errorf("%w: exp %s", ErrTokenExpired, expiresAt.UTC().Format(time.RFC3339))
	}
	if issuedAt.After(sekarang.Add(v.leeway)) {
		return Claims{}, fmt.Errorf("%w: iat %s", ErrIssuedAtInvalid, issuedAt.UTC().Format(time.RFC3339))
	}
	if !notBefore.IsZero() && sekarang.Before(notBefore.Add(-v.leeway)) {
		return Claims{}, fmt.Errorf("%w: nbf %s", ErrNotBeforeInvalid, notBefore.UTC().Format(time.RFC3339))
	}

	if klaim.Subject == "" {
		return Claims{}, ErrSubjectMissing
	}

	verified, err := klaimBool(klaim.EmailVerified)
	if err != nil {
		return Claims{}, err
	}
	if !verified {
		return Claims{}, ErrEmailNotVerified
	}

	email := strings.ToLower(strings.TrimSpace(klaim.Email))
	if email == "" {
		return Claims{}, ErrEmailMissing
	}

	return Claims{
		Subject:       klaim.Subject,
		Email:         email,
		EmailVerified: true,
		Nonce:         klaim.Nonce,
		Audience:      audience,
		Issuer:        klaim.Issuer,
		IssuedAt:      issuedAt.UTC(),
		ExpiresAt:     expiresAt.UTC(),
	}, nil
}

// pilihAudience mengembalikan audience pertama yang diizinkan. Klaim `aud`
// boleh berupa teks tunggal maupun larik teks.
//
// Bila `aud` memuat lebih dari satu nilai, klaim `azp` wajib ada dan wajib
// termasuk audience yang diizinkan (OIDC Core 1.0 §3.1.3.7). Aturan ini
// mencegah token yang diterbitkan untuk aplikasi lain ikut diterima hanya
// karena salah satu audience-nya kebetulan client ID kita. Token ber-audience
// tunggal boleh tanpa `azp`: Google tidak mengirimnya pada kasus itu.
func (v *Verifier) pilihAudience(raw json.RawMessage, azp string) (string, error) {
	kandidat, err := bacaAudience(raw)
	if err != nil {
		return "", err
	}

	// Audience diperiksa lebih dulu agar token yang sama sekali tidak memuat
	// client ID kita tetap dilaporkan sebagai ErrAudienceInvalid, bukan sebagai
	// masalah `azp`.
	var cocok string
	for _, k := range kandidat {
		if _, ok := v.audiences[k]; ok {
			cocok = k
			break
		}
	}
	if cocok == "" {
		return "", ErrAudienceInvalid
	}

	if len(kandidat) > 1 {
		if azp == "" {
			return "", fmt.Errorf("%w: klaim azp tidak ada pada token ber-audience ganda", ErrAuthorizedPartyInvalid)
		}
		if _, ok := v.audiences[azp]; !ok {
			return "", fmt.Errorf("%w: azp %q", ErrAuthorizedPartyInvalid, azp)
		}
	}

	return cocok, nil
}

// pisahToken memecah token menjadi header, payload, tanda tangan, dan bagian
// yang ditandatangani (header.payload). Tanda tangan dikembalikan dalam bentuk
// byte mentah.
func pisahToken(idToken string) (header, payload, signature, signed []byte, err error) {
	bagian := strings.Split(idToken, ".")
	if len(bagian) != 3 {
		return nil, nil, nil, nil, fmt.Errorf("%w: token bukan tiga bagian", ErrMalformedToken)
	}
	for i, b := range bagian {
		if b == "" {
			return nil, nil, nil, nil, fmt.Errorf("%w: bagian ke-%d kosong", ErrMalformedToken, i+1)
		}
	}

	header, err = base64.RawURLEncoding.DecodeString(bagian[0])
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: header bukan base64url", ErrMalformedToken)
	}
	payload, err = base64.RawURLEncoding.DecodeString(bagian[1])
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: payload bukan base64url", ErrMalformedToken)
	}
	signature, err = base64.RawURLEncoding.DecodeString(bagian[2])
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: tanda tangan bukan base64url", ErrMalformedToken)
	}
	if len(header) > maxSegmentBytes || len(payload) > maxClaimsBytes || len(signature) > maxSignatureBytes {
		return nil, nil, nil, nil, fmt.Errorf("%w: ukuran bagian token melebihi batas", ErrMalformedToken)
	}

	signed = []byte(bagian[0] + "." + bagian[1])
	return header, payload, signature, signed, nil
}

// unmarshalObjek menuntut JSON berbentuk objek, bukan larik, angka, atau teks.
func unmarshalObjek(data []byte, dst any) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// waktuKlaim membaca klaim waktu NumericDate. Bentuk yang tidak sah dipetakan
// ke ErrMalformedToken agar tidak ada klaim waktu yang diam-diam dianggap nol.
func waktuKlaim(raw json.RawMessage, nama string) (time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, fmt.Errorf("%w: klaim %s tidak ada", ErrMalformedToken, nama)
	}
	var detik json.Number
	if err := json.Unmarshal(raw, &detik); err != nil {
		return time.Time{}, fmt.Errorf("%w: klaim %s bukan angka", ErrMalformedToken, nama)
	}
	// json.Number hanya menerima literal angka JSON, jadi Int64 gagal untuk
	// nilai pecahan maupun literal yang tidak sah.
	nilai, err := detik.Int64()
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: klaim %s bukan bilangan bulat", ErrMalformedToken, nama)
	}
	if nilai <= 0 {
		return time.Time{}, fmt.Errorf("%w: klaim %s tidak masuk akal", ErrMalformedToken, nama)
	}
	return time.Unix(nilai, 0).UTC(), nil
}

// waktuKlaimOpsional membaca klaim waktu NumericDate yang boleh tidak ada.
// Klaim yang tidak ada atau bernilai null menghasilkan waktu nol tanpa galat;
// klaim yang ada tetapi bentuknya tidak sah tetap ditolak lewat waktuKlaim agar
// tidak ada klaim waktu yang diam-diam diabaikan.
func waktuKlaimOpsional(raw json.RawMessage, nama string) (time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, nil
	}
	return waktuKlaim(raw, nama)
}

// klaimBool membaca klaim boolean yang dapat dikirim sebagai boolean atau teks.
func klaimBool(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return false, fmt.Errorf("%w: klaim email_verified tidak ada", ErrMalformedToken)
	}
	var nilai bool
	if err := json.Unmarshal(raw, &nilai); err == nil {
		return nilai, nil
	}
	var teks string
	if err := json.Unmarshal(raw, &teks); err == nil {
		switch strings.ToLower(teks) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, fmt.Errorf("%w: klaim email_verified bukan boolean", ErrMalformedToken)
}

// bacaAudience menerima klaim `aud` sebagai teks tunggal atau larik teks.
// Bentuk yang salah ditolak sebagai token cacat; nilai yang kosong tetap
// dikembalikan sebagai kandidat kosong agar pemanggil memetakannya ke
// ErrAudienceInvalid, bukan ke galat bentuk.
func bacaAudience(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("%w: klaim aud tidak ada", ErrMalformedToken)
	}
	var tunggal string
	if err := json.Unmarshal(raw, &tunggal); err == nil {
		return []string{tunggal}, nil
	}
	var larik []string
	if err := json.Unmarshal(raw, &larik); err == nil {
		return larik, nil
	}
	return nil, fmt.Errorf("%w: klaim aud bukan teks atau larik teks", ErrMalformedToken)
}
