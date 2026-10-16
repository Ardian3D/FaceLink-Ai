package oidc

import "errors"

// Sentinel galat paket oidc. Pemanggil memakai errors.Is; setiap galat yang
// dikembalikan dibungkus dengan fmt.Errorf("...: %w", err) agar konteks tetap
// terbaca pada log tanpa kehilangan kemampuan pencocokan. Pesan galat tidak
// pernah memuat token mentah maupun isi JWKS.
var (
	// ErrConfigInvalid dikembalikan NewVerifier bila konfigurasi tidak lengkap
	// atau tidak masuk akal.
	ErrConfigInvalid = errors.New("oidc: konfigurasi verifier tidak sah")

	// ErrMalformedToken dikembalikan bila token bukan tiga bagian base64url,
	// header atau payload bukan JSON objek, atau klaim wajib tidak berbentuk
	// seperti yang diharapkan.
	ErrMalformedToken = errors.New("oidc: token cacat")

	// ErrUnsupportedAlgorithm dikembalikan bila `alg` bukan RS256, termasuk
	// `none` dan HS256.
	ErrUnsupportedAlgorithm = errors.New("oidc: algoritma tanda tangan tidak didukung")

	// ErrSignatureInvalid dikembalikan bila tanda tangan tidak cocok dengan
	// kunci yang dipakai.
	ErrSignatureInvalid = errors.New("oidc: tanda tangan token tidak sah")

	// ErrIssuerInvalid dikembalikan bila `iss` tidak termasuk issuer yang
	// diterima.
	ErrIssuerInvalid = errors.New("oidc: issuer token tidak diterima")

	// ErrAudienceInvalid dikembalikan bila tidak ada nilai `aud` yang cocok
	// dengan audience yang diizinkan.
	ErrAudienceInvalid = errors.New("oidc: audience token tidak diterima")

	// ErrTokenExpired dikembalikan bila `exp` sudah lewat, memperhitungkan
	// toleransi waktu.
	ErrTokenExpired = errors.New("oidc: token kedaluwarsa")

	// ErrIssuedAtInvalid dikembalikan bila `iat` berada di masa depan di luar
	// toleransi waktu.
	ErrIssuedAtInvalid = errors.New("oidc: klaim iat tidak sah")

	// ErrNotBeforeInvalid dikembalikan bila `nbf` berada di masa depan di luar
	// toleransi waktu, yaitu token dipakai sebelum masa berlakunya.
	ErrNotBeforeInvalid = errors.New("oidc: klaim nbf belum berlaku")

	// ErrAuthorizedPartyInvalid dikembalikan bila token memuat lebih dari satu
	// audience tetapi klaim `azp` tidak ada atau bukan salah satu audience yang
	// diizinkan (OIDC Core 1.0 §3.1.3.7).
	ErrAuthorizedPartyInvalid = errors.New("oidc: klaim azp tidak sah")

	// ErrSubjectMissing dikembalikan bila `sub` kosong.
	ErrSubjectMissing = errors.New("oidc: klaim sub kosong")

	// ErrEmailNotVerified dikembalikan bila `email_verified` bukan true.
	ErrEmailNotVerified = errors.New("oidc: email belum terverifikasi")

	// ErrEmailMissing dikembalikan bila `email` kosong setelah dinormalkan.
	ErrEmailMissing = errors.New("oidc: klaim email kosong")

	// ErrNonceMismatch dikembalikan CheckNonce bila nonce yang diharapkan
	// berbeda dari klaim token.
	ErrNonceMismatch = errors.New("oidc: nonce tidak cocok")

	// ErrKeyNotFound dikembalikan bila kid yang diminta tidak ada pada sumber
	// kunci.
	ErrKeyNotFound = errors.New("oidc: kunci tanda tangan tidak ditemukan")

	// ErrKeyInvalid dikembalikan bila kunci yang diperoleh bukan kunci RSA yang
	// dapat dipakai.
	ErrKeyInvalid = errors.New("oidc: kunci tanda tangan tidak sah")

	// ErrJWKSUnavailable dikembalikan bila JWKS tidak dapat diambil: permintaan
	// gagal, status bukan 200, atau badan respons tidak terbaca.
	ErrJWKSUnavailable = errors.New("oidc: JWKS tidak dapat diambil")

	// ErrJWKSInvalid dikembalikan bila isi JWKS rusak: bukan JSON yang sah,
	// melebihi batas ukuran, atau memuat kunci RSA dengan n/e tidak sah.
	ErrJWKSInvalid = errors.New("oidc: isi JWKS tidak sah")
)
