package session

import "errors"

// Sentinel galat paket sesi. Pemanggil memakai errors.Is; setiap galat yang
// dikembalikan dibungkus dengan konteks yang tidak memuat token.
var (
	// ErrUnauthenticated dikembalikan bila token akses tidak cocok dengan sesi
	// mana pun.
	ErrUnauthenticated = errors.New("sesi: token akses tidak dikenali")

	// ErrAccessExpired dikembalikan bila sesi ditemukan tetapi token aksesnya
	// sudah lewat masa berlaku.
	ErrAccessExpired = errors.New("sesi: token akses kedaluwarsa")

	// ErrSessionRevoked dikembalikan bila sesi sudah dicabut.
	ErrSessionRevoked = errors.New("sesi: sesi telah dicabut")

	// ErrRefreshInvalid dikembalikan bila token penyegar tidak dikenali, sesinya
	// sudah dicabut, atau batas absolut sesi sudah lewat.
	ErrRefreshInvalid = errors.New("sesi: token penyegar tidak sah")

	// ErrRefreshReused dikembalikan bila token penyegar yang sudah diputar
	// dipakai lagi. Sesi pelakunya dicabut sebelum galat ini dikembalikan.
	ErrRefreshReused = errors.New("sesi: token penyegar dipakai ulang")

	// ErrNotFound dikembalikan bila sesi yang dituju tidak ada. Sesi yang sudah
	// tercabut bukan ErrNotFound: pencabutan bersifat idempoten.
	ErrNotFound = errors.New("sesi: sesi tidak ditemukan")
)

// Alasan pencabutan yang ditulis ke kolom `revoked_reason`.
const (
	alasanRefreshReuse   = "refresh_reuse"
	alasanRefreshExpired = "refresh_expired"
)
