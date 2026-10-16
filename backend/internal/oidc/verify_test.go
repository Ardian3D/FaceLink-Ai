package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// waktuUji adalah titik acuan jam uji: 16 Oktober 2026 09:00 UTC, tanggal yang
// sama dengan berkas keputusan FL-011.
var waktuUji = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

// klienIDUji adalah audience yang diizinkan pada seluruh uji paket ini.
const klienIDUji = "client-uji.apps.googleusercontent.com"

var (
	kunciSekali   sync.Once
	kunciUtama    *rsa.PrivateKey
	kunciLain     *rsa.PrivateKey
	galatKunciUji error
)

// kunciUji membuat dua pasang kunci RSA 2048 sekali saja untuk seluruh uji:
// kunci pertama untuk tanda tangan yang sah, kunci kedua untuk membuktikan
// tanda tangan dari kunci asing ditolak.
func kunciUji(t *testing.T) (*rsa.PrivateKey, *rsa.PrivateKey) {
	t.Helper()
	kunciSekali.Do(func() {
		kunciUtama, galatKunciUji = rsa.GenerateKey(rand.Reader, 2048)
		if galatKunciUji != nil {
			return
		}
		kunciLain, galatKunciUji = rsa.GenerateKey(rand.Reader, 2048)
	})
	if galatKunciUji != nil {
		t.Fatalf("membuat kunci RSA uji gagal: %v", galatKunciUji)
	}
	return kunciUtama, kunciLain
}

// sumberKunciUji adalah KeySource in-memory untuk uji verifikasi. Ia juga
// menghitung pemanggilan agar uji dapat membuktikan kunci tidak diambil dua
// kali untuk token yang sama.
type sumberKunciUji struct {
	kunci   map[string]crypto.PublicKey
	galat   error
	panggil int
}

func (s *sumberKunciUji) KeyByID(_ context.Context, kid string) (crypto.PublicKey, error) {
	s.panggil++
	if s.galat != nil {
		return nil, s.galat
	}
	kunci, ok := s.kunci[kid]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return kunci, nil
}

// verifierUji menyiapkan Verifier dengan jam palsu dan sumber kunci memuat
// kunci uji pada kid "kid-uji".
func verifierUji(t *testing.T, ubah func(*Config)) (*Verifier, *sumberKunciUji, *rsa.PrivateKey) {
	t.Helper()
	utama, _ := kunciUji(t)

	sumber := &sumberKunciUji{kunci: map[string]crypto.PublicKey{"kid-uji": &utama.PublicKey}}
	cfg := Config{
		Audiences: []string{klienIDUji},
		Keys:      sumber,
		Now:       func() time.Time { return waktuUji },
	}
	if ubah != nil {
		ubah(&cfg)
	}

	v, err := NewVerifier(cfg)
	if err != nil {
		t.Fatalf("NewVerifier gagal: %v", err)
	}
	return v, sumber, utama
}

// klaimSah adalah klaim dasar yang seluruh ujinya lolos. Setiap uji penolakan
// mengubah satu klaim saja sehingga penyebab kegagalan tetap tunggal.
func klaimSah() map[string]any {
	return map[string]any{
		"iss":            issuerGoogleHTTPS,
		"aud":            klienIDUji,
		"sub":            "subjek-123",
		"exp":            waktuUji.Add(time.Hour).Unix(),
		"iat":            waktuUji.Add(-time.Minute).Unix(),
		"nonce":          "nonce-uji",
		"email":          "  Mahasiswa@Kampus.Example ",
		"email_verified": true,
	}
}

func b64(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }

// buatToken menyusun JWT uji. Header dan klaim ditulis apa adanya agar uji
// dapat memalsukan alg, kid, maupun bentuk klaim.
func buatToken(t *testing.T, key *rsa.PrivateKey, header, klaim map[string]any) string {
	t.Helper()
	h, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header uji gagal: %v", err)
	}
	p, err := json.Marshal(klaim)
	if err != nil {
		t.Fatalf("marshal klaim uji gagal: %v", err)
	}
	signed := b64(h) + "." + b64(p)
	return signed + "." + tandaTangan(t, key, signed)
}

// tandaTangan menandatangani bagian header.payload dengan RS256.
func tandaTangan(t *testing.T, key *rsa.PrivateKey, signed string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("menandatangani token uji gagal: %v", err)
	}
	return b64(sig)
}

func headerSah() map[string]any {
	return map[string]any{"alg": algRS256, "kid": "kid-uji", "typ": "JWT"}
}

// TestVerifyTokenSah membuktikan jalur sukses: klaim yang dikembalikan lengkap,
// email dinormalkan, dan waktu dalam UTC.
func TestVerifyTokenSah(t *testing.T) {
	v, sumber, utama := verifierUji(t, nil)
	token := buatToken(t, utama, headerSah(), klaimSah())

	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify menolak token sah: %v", err)
	}

	if claims.Subject != "subjek-123" {
		t.Errorf("Subject = %q, ingin %q", claims.Subject, "subjek-123")
	}
	if claims.Email != "mahasiswa@kampus.example" {
		t.Errorf("Email = %q, ingin dinormalkan menjadi %q", claims.Email, "mahasiswa@kampus.example")
	}
	if !claims.EmailVerified {
		t.Error("EmailVerified = false, ingin true")
	}
	if claims.Nonce != "nonce-uji" {
		t.Errorf("Nonce = %q, ingin %q", claims.Nonce, "nonce-uji")
	}
	if claims.Audience != klienIDUji {
		t.Errorf("Audience = %q, ingin %q", claims.Audience, klienIDUji)
	}
	if claims.Issuer != issuerGoogleHTTPS {
		t.Errorf("Issuer = %q, ingin %q", claims.Issuer, issuerGoogleHTTPS)
	}
	if !claims.ExpiresAt.Equal(waktuUji.Add(time.Hour).UTC()) {
		t.Errorf("ExpiresAt = %s, ingin %s", claims.ExpiresAt, waktuUji.Add(time.Hour).UTC())
	}
	if !claims.IssuedAt.Equal(waktuUji.Add(-time.Minute).UTC()) {
		t.Errorf("IssuedAt = %s, ingin %s", claims.IssuedAt, waktuUji.Add(-time.Minute).UTC())
	}
	if claims.ExpiresAt.Location() != time.UTC || claims.IssuedAt.Location() != time.UTC {
		t.Error("waktu klaim tidak dalam UTC")
	}
	if err := claims.CheckNonce("nonce-uji"); err != nil {
		t.Errorf("CheckNonce menolak nonce yang cocok: %v", err)
	}
	if sumber.panggil != 1 {
		t.Errorf("sumber kunci dipanggil %d kali, ingin 1", sumber.panggil)
	}
}

// TestVerifyPenolakan mengumpulkan seluruh jalur penolakan yang diwajibkan
// kontrak: tanda tangan, algoritma, issuer, audience, masa berlaku, subjek,
// email, nonce, dan bentuk token.
func TestVerifyPenolakan(t *testing.T) {
	v, _, utama := verifierUji(t, nil)
	_, asing := kunciUji(t)

	kasus := []struct {
		nama  string
		ingin error
		token func(t *testing.T) string
	}{
		{
			nama:  "tanda tangan dari kunci lain",
			ingin: ErrSignatureInvalid,
			token: func(t *testing.T) string {
				return buatToken(t, asing, headerSah(), klaimSah())
			},
		},
		{
			nama:  "tanda tangan dirusak",
			ingin: ErrSignatureInvalid,
			token: func(t *testing.T) string {
				token := buatToken(t, utama, headerSah(), klaimSah())
				bagian := strings.Split(token, ".")
				rusak, err := base64.RawURLEncoding.DecodeString(bagian[2])
				if err != nil {
					t.Fatalf("membaca tanda tangan uji gagal: %v", err)
				}
				rusak[len(rusak)-1] ^= 0xff
				return bagian[0] + "." + bagian[1] + "." + b64(rusak)
			},
		},
		{
			nama:  "alg none",
			ingin: ErrUnsupportedAlgorithm,
			token: func(t *testing.T) string {
				return buatToken(t, utama, map[string]any{"alg": "none", "kid": "kid-uji"}, klaimSah())
			},
		},
		{
			nama:  "alg HS256",
			ingin: ErrUnsupportedAlgorithm,
			token: func(t *testing.T) string {
				return buatToken(t, utama, map[string]any{"alg": "HS256", "kid": "kid-uji"}, klaimSah())
			},
		},
		{
			nama:  "alg ES256",
			ingin: ErrUnsupportedAlgorithm,
			token: func(t *testing.T) string {
				return buatToken(t, utama, map[string]any{"alg": "ES256", "kid": "kid-uji"}, klaimSah())
			},
		},
		{
			nama:  "issuer lain",
			ingin: ErrIssuerInvalid,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["iss"] = "https://penyerang.example"
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "issuer kosong",
			ingin: ErrIssuerInvalid,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				delete(klaim, "iss")
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "audience lain",
			ingin: ErrAudienceInvalid,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["aud"] = "klien-lain.apps.googleusercontent.com"
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "audience kosong",
			ingin: ErrAudienceInvalid,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["aud"] = []string{}
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "kedaluwarsa",
			ingin: ErrTokenExpired,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["exp"] = waktuUji.Add(-time.Hour).Unix()
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "iat di masa depan",
			ingin: ErrIssuedAtInvalid,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["iat"] = waktuUji.Add(time.Hour).Unix()
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "sub kosong",
			ingin: ErrSubjectMissing,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["sub"] = ""
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "email_verified false",
			ingin: ErrEmailNotVerified,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["email_verified"] = false
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "email_verified tidak ada",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				delete(klaim, "email_verified")
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "email kosong",
			ingin: ErrEmailMissing,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["email"] = "   "
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "kid tidak dikenal",
			ingin: ErrSignatureInvalid,
			token: func(t *testing.T) string {
				return buatToken(t, utama, map[string]any{"alg": algRS256, "kid": "kid-asing"}, klaimSah())
			},
		},
		{
			nama:  "kid kosong",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				return buatToken(t, utama, map[string]any{"alg": algRS256}, klaimSah())
			},
		},
		{
			nama:  "hanya dua bagian",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				token := buatToken(t, utama, headerSah(), klaimSah())
				return strings.Join(strings.Split(token, ".")[:2], ".")
			},
		},
		{
			nama:  "bagian kosong",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string { return ".." },
		},
		{
			nama:  "header bukan base64url",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				return "***." + b64([]byte(`{}`)) + "." + b64([]byte("tanda"))
			},
		},
		{
			nama:  "payload bukan base64url",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				return b64([]byte(`{"alg":"RS256","kid":"kid-uji"}`)) + ".***." + b64([]byte("tanda"))
			},
		},
		{
			nama:  "header bukan objek JSON",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				return b64([]byte(`["RS256"]`)) + "." + b64([]byte(`{}`)) + "." + b64([]byte("tanda"))
			},
		},
		{
			nama:  "payload bukan objek JSON",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				return b64([]byte(`{"alg":"RS256","kid":"kid-uji"}`)) + "." + b64([]byte(`"bukan objek"`)) + "." + b64([]byte("tanda"))
			},
		},
		{
			nama:  "exp tidak ada",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				delete(klaim, "exp")
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "exp berupa teks",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string {
				klaim := klaimSah()
				klaim["exp"] = "besok"
				return buatToken(t, utama, headerSah(), klaim)
			},
		},
		{
			nama:  "token kosong",
			ingin: ErrMalformedToken,
			token: func(t *testing.T) string { return "" },
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			claims, err := v.Verify(context.Background(), k.token(t))
			if !errors.Is(err, k.ingin) {
				t.Fatalf("Verify galat = %v, ingin errors.Is %v", err, k.ingin)
			}
			if claims != (Claims{}) {
				t.Errorf("klaim dikembalikan meski verifikasi gagal: %+v", claims)
			}
			// Pesan galat tidak boleh memuat token mentah.
			if strings.Contains(err.Error(), ".") && strings.Contains(err.Error(), k.token(t)) {
				t.Errorf("pesan galat memuat token mentah: %v", err)
			}
		})
	}
}

// TestVerifyNonceTerpisah menegaskan nonce tidak diperiksa di dalam Verify:
// pemanggil yang memutuskan lewat CheckNonce.
func TestVerifyNonceTerpisah(t *testing.T) {
	v, _, utama := verifierUji(t, nil)

	klaim := klaimSah()
	delete(klaim, "nonce")
	claims, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim))
	if err != nil {
		t.Fatalf("Verify menolak token tanpa nonce: %v", err)
	}
	if err := claims.CheckNonce(""); err != nil {
		t.Errorf("CheckNonce(\"\") = %v, ingin nil", err)
	}
	if err := claims.CheckNonce("nonce-apa-pun"); !errors.Is(err, ErrNonceMismatch) {
		t.Errorf("CheckNonce dengan nonce kosong pada token = %v, ingin ErrNonceMismatch", err)
	}
	if err := claims.CheckNonce("nonce-uji"); !errors.Is(err, ErrNonceMismatch) {
		t.Errorf("CheckNonce = %v, ingin ErrNonceMismatch", err)
	}

	// Token dengan nonce berbeda dari yang diharapkan juga ditolak.
	beda := klaimSah()
	beda["nonce"] = "nonce-lain"
	claimsBeda, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), beda))
	if err != nil {
		t.Fatalf("Verify menolak token dengan nonce lain: %v", err)
	}
	if err := claimsBeda.CheckNonce("nonce-uji"); !errors.Is(err, ErrNonceMismatch) {
		t.Errorf("CheckNonce(token nonce lain) = %v, ingin ErrNonceMismatch", err)
	}
	if err := claimsBeda.CheckNonce("nonce-lain"); err != nil {
		t.Errorf("CheckNonce(nonce yang sama) = %v, ingin nil", err)
	}
}

// TestVerifyIssuerBawaan memastikan kedua bentuk issuer Google diterima saat
// Config.Issuers dikosongkan.
func TestVerifyIssuerBawaan(t *testing.T) {
	for _, iss := range []string{issuerGoogle, issuerGoogleHTTPS} {
		t.Run(iss, func(t *testing.T) {
			v, _, utama := verifierUji(t, nil)
			klaim := klaimSah()
			klaim["iss"] = iss
			if _, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim)); err != nil {
				t.Fatalf("Verify menolak issuer bawaan %q: %v", iss, err)
			}
		})
	}
}

// TestVerifyIssuerKhusus menegaskan issuer kustom menggantikan daftar bawaan.
func TestVerifyIssuerKhusus(t *testing.T) {
	v, _, utama := verifierUji(t, func(c *Config) {
		c.Issuers = []string{"https://issuer.kampus.example"}
	})

	klaim := klaimSah()
	klaim["iss"] = "https://issuer.kampus.example"
	if _, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim)); err != nil {
		t.Fatalf("Verify menolak issuer kustom: %v", err)
	}

	bawaan := klaimSah()
	if _, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), bawaan)); !errors.Is(err, ErrIssuerInvalid) {
		t.Errorf("Verify galat = %v, ingin ErrIssuerInvalid untuk issuer Google saat daftar dikustom", err)
	}
}

// TestVerifyAudienceGanda membuktikan klaim `aud` berbentuk larik diterima bila
// salah satu anggotanya diizinkan dan `azp` menunjuk audience yang diizinkan
// (kontrak §5.1: azp diperiksa bila token punya beberapa audience).
func TestVerifyAudienceGanda(t *testing.T) {
	v, _, utama := verifierUji(t, nil)

	klaim := klaimSah()
	klaim["aud"] = []string{"klien-lain", klienIDUji}
	klaim["azp"] = klienIDUji
	claims, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim))
	if err != nil {
		t.Fatalf("Verify menolak audience larik yang memuat klien diizinkan: %v", err)
	}
	if claims.Audience != klienIDUji {
		t.Errorf("Audience = %q, ingin %q", claims.Audience, klienIDUji)
	}
}

// TestVerifyAzpDanNbf menguji klaim `azp` dan `nbf` sesuai kontrak §5.1:
// `azp` diperiksa bila token punya beberapa audience, dan masa berlaku token
// (termasuk nbf) ditegakkan dengan toleransi waktu.
func TestVerifyAzpDanNbf(t *testing.T) {
	klienLain := "klien-lain.apps.googleusercontent.com"

	kasus := []struct {
		nama  string
		ubah  func(map[string]any)
		ingin error
	}{
		{
			nama:  "audience ganda azp salah",
			ubah:  func(k map[string]any) { k["aud"] = []string{klienIDUji, klienLain}; k["azp"] = klienLain },
			ingin: ErrAuthorizedPartyInvalid,
		},
		{
			nama: "audience ganda azp benar",
			ubah: func(k map[string]any) { k["aud"] = []string{klienLain, klienIDUji}; k["azp"] = klienIDUji },
		},
		{
			nama:  "audience ganda tanpa azp",
			ubah:  func(k map[string]any) { k["aud"] = []string{klienIDUji, klienLain} },
			ingin: ErrAuthorizedPartyInvalid,
		},
		{
			nama:  "audience ganda azp kosong",
			ubah:  func(k map[string]any) { k["aud"] = []string{klienIDUji, klienLain}; k["azp"] = "" },
			ingin: ErrAuthorizedPartyInvalid,
		},
		{
			nama:  "audience ganda azp tidak dikenal",
			ubah:  func(k map[string]any) { k["aud"] = []string{klienIDUji, klienLain}; k["azp"] = "klien-asing" },
			ingin: ErrAuthorizedPartyInvalid,
		},
		{
			nama: "audience ganda azp sama dengan audience kedua yang diizinkan",
			ubah: func(k map[string]any) {
				k["aud"] = []string{klienLain, klienIDUji, klienLain}
				k["azp"] = klienIDUji
			},
		},
		{
			nama: "audience tunggal tanpa azp",
			ubah: func(k map[string]any) {},
		},
		{
			nama: "audience tunggal dengan azp benar",
			ubah: func(k map[string]any) { k["azp"] = klienIDUji },
		},
		{
			// OIDC Core §2: `azp` boleh ada pada token ber-audience tunggal dan
			// boleh berbeda dari audience itu, jadi nilainya tidak diperiksa
			// pada kasus ini — kontrak §5.1 hanya menuntut pemeriksaan `azp`
			// saat token punya beberapa audience.
			nama: "audience tunggal dengan azp berbeda tetap diterima",
			ubah: func(k map[string]any) { k["azp"] = klienLain },
		},
		{
			nama:  "nbf di masa depan",
			ubah:  func(k map[string]any) { k["nbf"] = waktuUji.Add(time.Hour).Unix() },
			ingin: ErrNotBeforeInvalid,
		},
		{
			nama: "nbf di masa lalu",
			ubah: func(k map[string]any) { k["nbf"] = waktuUji.Add(-time.Hour).Unix() },
		},
		{
			nama: "nbf tidak ada",
			ubah: func(k map[string]any) {},
		},
		{
			nama: "nbf di masa depan dalam toleransi",
			ubah: func(k map[string]any) { k["nbf"] = waktuUji.Add(time.Minute).Unix() },
		},
		{
			nama:  "nbf di masa depan di luar toleransi",
			ubah:  func(k map[string]any) { k["nbf"] = waktuUji.Add(3 * time.Minute).Unix() },
			ingin: ErrNotBeforeInvalid,
		},
		{
			nama:  "nbf bukan angka",
			ubah:  func(k map[string]any) { k["nbf"] = "besok" },
			ingin: ErrMalformedToken,
		},
		{
			nama:  "nbf nol",
			ubah:  func(k map[string]any) { k["nbf"] = 0 },
			ingin: ErrMalformedToken,
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			v, _, utama := verifierUji(t, nil)
			klaim := klaimSah()
			k.ubah(klaim)

			claims, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim))
			if k.ingin == nil {
				if err != nil {
					t.Fatalf("Verify menolak token yang sah: %v", err)
				}
				if claims.Audience == "" {
					t.Error("Audience kosong pada token yang diterima")
				}
				return
			}
			if !errors.Is(err, k.ingin) {
				t.Fatalf("Verify galat = %v, ingin errors.Is %v", err, k.ingin)
			}
			if claims != (Claims{}) {
				t.Errorf("klaim dikembalikan meski verifikasi gagal: %+v", claims)
			}
		})
	}
}

// TestVerifyAzpAudienceTerpilih membuktikan audience yang dilaporkan adalah
// audience yang cocok, bukan azp, ketika keduanya berbeda namun sama-sama
// diizinkan.
func TestVerifyAzpAudienceTerpilih(t *testing.T) {
	klienKedua := "client-kedua.apps.googleusercontent.com"
	v, _, utama := verifierUji(t, func(c *Config) {
		c.Audiences = []string{klienIDUji, klienKedua}
	})

	klaim := klaimSah()
	klaim["aud"] = []string{klienKedua, klienIDUji}
	klaim["azp"] = klienKedua

	claims, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim))
	if err != nil {
		t.Fatalf("Verify menolak token multi-audience yang sah: %v", err)
	}
	if claims.Audience != klienKedua {
		t.Errorf("Audience = %q, ingin %q", claims.Audience, klienKedua)
	}
}

// TestVerifyLeeway membuktikan toleransi waktu berlaku pada exp dan iat, dan
// bahwa nilai di luar toleransi tetap ditolak.
func TestVerifyLeeway(t *testing.T) {
	v, _, utama := verifierUji(t, func(c *Config) {
		c.Leeway = 2 * time.Minute
	})

	kasus := []struct {
		nama  string
		ubah  func(map[string]any)
		ingin error
	}{
		{
			nama:  "exp lewat satu menit masih diterima",
			ubah:  func(k map[string]any) { k["exp"] = waktuUji.Add(-time.Minute).Unix() },
			ingin: nil,
		},
		{
			nama:  "exp lewat tepat dua menit ditolak",
			ubah:  func(k map[string]any) { k["exp"] = waktuUji.Add(-2 * time.Minute).Unix() },
			ingin: ErrTokenExpired,
		},
		{
			nama:  "exp lewat tiga menit ditolak",
			ubah:  func(k map[string]any) { k["exp"] = waktuUji.Add(-3 * time.Minute).Unix() },
			ingin: ErrTokenExpired,
		},
		{
			nama:  "iat maju satu menit masih diterima",
			ubah:  func(k map[string]any) { k["iat"] = waktuUji.Add(time.Minute).Unix() },
			ingin: nil,
		},
		{
			nama:  "iat maju tepat dua menit masih diterima",
			ubah:  func(k map[string]any) { k["iat"] = waktuUji.Add(2 * time.Minute).Unix() },
			ingin: nil,
		},
		{
			nama:  "iat maju tiga menit ditolak",
			ubah:  func(k map[string]any) { k["iat"] = waktuUji.Add(3 * time.Minute).Unix() },
			ingin: ErrIssuedAtInvalid,
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			klaim := klaimSah()
			k.ubah(klaim)
			_, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaim))
			if k.ingin == nil {
				if err != nil {
					t.Fatalf("Verify menolak token dalam toleransi: %v", err)
				}
				return
			}
			if !errors.Is(err, k.ingin) {
				t.Fatalf("Verify galat = %v, ingin errors.Is %v", err, k.ingin)
			}
		})
	}
}

// TestVerifyGalatSumberKunci memastikan galat pengambilan kunci tidak membuat
// token dianggap sah.
func TestVerifyGalatSumberKunci(t *testing.T) {
	galatAsal := errors.New("sumber kunci sedang tidak tersedia")
	v, sumber, utama := verifierUji(t, nil)
	sumber.galat = galatAsal

	_, err := v.Verify(context.Background(), buatToken(t, utama, headerSah(), klaimSah()))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("Verify galat = %v, ingin ErrSignatureInvalid", err)
	}
	if errors.Is(err, galatAsal) {
		t.Error("galat sumber kunci bocor tanpa dibungkus sentinel paket")
	}
}

// TestNewVerifierKonfigurasiTidakSah membuktikan konfigurasi yang membahayakan
// ditolak sebelum dipakai.
func TestNewVerifierKonfigurasiTidakSah(t *testing.T) {
	kunci, _ := kunciUji(t)
	sumber := &sumberKunciUji{kunci: map[string]crypto.PublicKey{"kid-uji": &kunci.PublicKey}}

	kasus := []struct {
		nama string
		cfg  Config
	}{
		{nama: "audience kosong", cfg: Config{Keys: sumber}},
		{nama: "audience berisi entri kosong", cfg: Config{Audiences: []string{""}, Keys: sumber}},
		{nama: "issuer berisi entri kosong", cfg: Config{Audiences: []string{klienIDUji}, Issuers: []string{""}, Keys: sumber}},
		{nama: "sumber kunci nil", cfg: Config{Audiences: []string{klienIDUji}}},
		{nama: "leeway negatif", cfg: Config{Audiences: []string{klienIDUji}, Keys: sumber, Leeway: -time.Second}},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			v, err := NewVerifier(k.cfg)
			if !errors.Is(err, ErrConfigInvalid) {
				t.Fatalf("NewVerifier galat = %v, ingin ErrConfigInvalid", err)
			}
			if v != nil {
				t.Error("NewVerifier mengembalikan verifier meski konfigurasi tidak sah")
			}
		})
	}
}

// TestCheckNoncePanjangBerbeda menegaskan pembandingan waktu tetap tidak
// menerima nonce yang hanya berbagi awalan.
func TestCheckNoncePanjangBerbeda(t *testing.T) {
	claims := Claims{Nonce: "abcdef"}
	if err := claims.CheckNonce("abc"); !errors.Is(err, ErrNonceMismatch) {
		t.Errorf("CheckNonce(awalan) = %v, ingin ErrNonceMismatch", err)
	}
	if err := claims.CheckNonce("abcdefg"); !errors.Is(err, ErrNonceMismatch) {
		t.Errorf("CheckNonce(lebih panjang) = %v, ingin ErrNonceMismatch", err)
	}
	if err := claims.CheckNonce("abcdef"); err != nil {
		t.Errorf("CheckNonce(cocok) = %v, ingin nil", err)
	}
}
