package auth

import "errors"

var (
	ErrSessionAlreadyOpen   = errors.New("auth: sesi absensi sudah terbuka")
	ErrSessionClosed        = errors.New("auth: sesi absensi sudah ditutup")
	ErrSessionNotOpen       = errors.New("auth: sesi absensi belum dibuka")
	ErrDeviceAlreadyBound   = errors.New("auth: perangkat sudah terikat sesi terbuka")
	ErrDeviceUnavailable    = errors.New("auth: perangkat tidak tersedia")
	ErrClassMismatch        = errors.New("auth: peserta bukan anggota kelas sesi")
	ErrReasonRequired       = errors.New("auth: alasan wajib diisi")
	ErrAlreadyCheckedIn     = errors.New("auth: kehadiran sudah tercatat")
	ErrIdempotencyKeyReused = errors.New("auth: kunci idempotensi telah dipakai untuk permintaan berbeda")
	ErrAttendanceValidation = errors.New("auth: waktu atau status kehadiran tidak sesuai aturan sesi")
)
