// Package ids membuat identifier acak tanpa makna untuk baris database.
// Formatnya UUID v4 seperti yang disyaratkan docs/DATA_MODEL.md §3: identifier
// tidak boleh mengandung makna apa pun (bukan NIK, bukan nomor absen).
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New mengembalikan UUID v4 acak. Sumbernya crypto/rand; bila sumber acak gagal
// fungsi ini panik, karena melanjutkan tanpa identifier acak akan menghasilkan
// tabrakan data yang sulit dilacak.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("ids: sumber acak tidak tersedia: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // versi 4
	b[8] = (b[8] & 0x3f) | 0x80 // varian RFC 4122

	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}
