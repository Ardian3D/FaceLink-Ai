package logging

import (
	"errors"
	"strings"
	"testing"
)

// TestRedactMenyensorBentukRahasia memastikan setiap bentuk rahasia yang lazim
// terbawa nilai panic atau pesan galat diganti, tanpa mengubah bagian yang
// memang dibutuhkan untuk diagnosis.
func TestRedactMenyensorBentukRahasia(t *testing.T) {
	const (
		jwtUji   = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImsxIn0.eyJzdWIiOiJzdWItMTIzNDU2Nzg5MCJ9.dGFuZGEtdGFuZ2Fu"
		tokenUji = "Qk9ndXNUb2tlbjEyMzQ1Njc4OTBhYmNkZWZnaGlqa2xtbm9wcXJz"
		surelUji = "budi.santoso@kampus.example"
		idSesi   = "550e8400-e29b-41d4-a716-446655440000"
		idReq    = "AbCdEfGhIj-000123"
	)

	kasus := []struct {
		nama       string
		masukan    string
		terlarang  []string
		harusAda   []string
		keterangan string
	}{
		{
			nama:      "id token google",
			masukan:   "verifikasi gagal: id token " + jwtUji,
			terlarang: []string{jwtUji, "eyJhbGciOiJSUzI1NiJ9"},
			harusAda:  []string{"<jwt>", "verifikasi gagal"},
		},
		{
			nama:      "tajuk otorisasi",
			masukan:   "Authorization: Bearer " + tokenUji,
			terlarang: []string{tokenUji, "Bearer", "Authorization: Bearer"},
			harusAda:  []string{"<tajuk-otorisasi>"},
		},
		{
			nama:      "tajuk basic",
			masukan:   "proxy menolak Basic YWRtaW46cmFoYXNpYQ==",
			terlarang: []string{"YWRtaW46cmFoYXNpYQ==", "Basic YWRtaW4"},
			harusAda:  []string{"<tajuk-otorisasi>", "proxy menolak"},
		},
		{
			nama:      "pasangan kunci galat",
			masukan:   "memutar sesi: refresh_token=" + tokenUji,
			terlarang: []string{tokenUji},
			harusAda:  []string{"refresh_token", "<nilai>", "memutar sesi"},
		},
		{
			nama:      "pasangan kunci json",
			masukan:   `{"access_token":"` + tokenUji + `","nonce":"abc123"}`,
			terlarang: []string{tokenUji, "abc123"},
			harusAda:  []string{"access_token", "nonce", "<nilai>"},
		},
		{
			nama:      "alamat surel",
			masukan:   "penautan akun " + surelUji + " ditolak",
			terlarang: []string{surelUji, "budi.santoso", "@kampus.example"},
			harusAda:  []string{"<surel>", "penautan akun"},
		},
		{
			nama:      "deret panjang tanpa pemisah",
			masukan:   "token tidak dikenali: " + tokenUji,
			terlarang: []string{tokenUji},
			harusAda:  []string{"<token>", "token tidak dikenali"},
		},
		{
			nama:       "uuid sesi dipertahankan",
			masukan:    "sesi: sesi " + idSesi + " telah dicabut",
			harusAda:   []string{idSesi},
			keterangan: "id sesi bukan rahasia; korelasi log harus tetap terbaca",
		},
		{
			nama:       "id permintaan chi dipertahankan",
			masukan:    "panic pada handler request_id=" + idReq,
			harusAda:   []string{idReq},
			keterangan: "penautan permintaan tidak boleh rusak",
		},
		{
			nama:       "kode galat runtime dipertahankan",
			masukan:    "runtime error: index out of range [5] with length 3",
			harusAda:   []string{"index out of range [5] with length 3"},
			keterangan: "diagnostik non-sensitif tidak boleh hilang",
		},
		{
			nama:       "kode galat domain dipertahankan",
			masukan:    "409 LAST_ADMIN_PROTECTED dan SELF_ACCOUNT_PROTECTED",
			harusAda:   []string{"LAST_ADMIN_PROTECTED", "SELF_ACCOUNT_PROTECTED"},
			keterangan: "kode galat tidak boleh disensor",
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			hasil := Redact(k.masukan)
			for _, terlarang := range k.terlarang {
				if strings.Contains(hasil, terlarang) {
					t.Errorf("Redact() masih memuat %q: %q", terlarang, hasil)
				}
			}
			for _, wajib := range k.harusAda {
				if !strings.Contains(hasil, wajib) {
					t.Errorf("Redact() kehilangan %q: %q", wajib, hasil)
				}
			}
		})
	}
}

// jenisPanikKustom membuktikan nilai panic yang bukan string/error hanya
// dilaporkan jenisnya, sehingga struktur besar tidak pernah masuk log.
type jenisPanikKustom struct {
	rahasia string
}

// jenisPanikStringer membuktikan tipe dengan String() tetap aman karena
// hasilnya disensor.
type jenisPanikStringer struct{}

func (jenisPanikStringer) String() string {
	return "stringer token=RAHASIA-STRINGER-1234567890"
}

func TestPanicSummaryMenyensorIsi(t *testing.T) {
	const tokenUji = "Qk9ndXNUb2tlbjEyMzQ1Njc4OTBhYmNkZWZnaGlqa2xtbm9wcXJz"

	t.Run("string", func(t *testing.T) {
		hasil := PanicSummary("gagal menautkan akun dengan token=" + tokenUji)
		if !strings.HasPrefix(hasil, "string: ") {
			t.Errorf("PanicSummary() = %q, ingin berawalan jenis nilai", hasil)
		}
		if strings.Contains(hasil, tokenUji) {
			t.Errorf("PanicSummary() membocorkan token: %q", hasil)
		}
		if !strings.Contains(hasil, "gagal menautkan akun") {
			t.Errorf("PanicSummary() kehilangan diagnostik: %q", hasil)
		}
	})

	t.Run("error", func(t *testing.T) {
		hasil := PanicSummary(errors.New("id token " + tokenUji + " ditolak"))
		if !strings.HasPrefix(hasil, "*errors.errorString: ") {
			t.Errorf("PanicSummary() = %q, ingin berawalan jenis nilai", hasil)
		}
		if strings.Contains(hasil, tokenUji) {
			t.Errorf("PanicSummary() membocorkan token: %q", hasil)
		}
		if !strings.Contains(hasil, "ditolak") {
			t.Errorf("PanicSummary() kehilangan diagnostik: %q", hasil)
		}
	})

	t.Run("stringer", func(t *testing.T) {
		hasil := PanicSummary(jenisPanikStringer{})
		if strings.Contains(hasil, "RAHASIA-STRINGER-1234567890") {
			t.Errorf("PanicSummary() membocorkan nilai Stringer: %q", hasil)
		}
		if !strings.Contains(hasil, "jenisPanikStringer") {
			t.Errorf("PanicSummary() kehilangan jenis nilai: %q", hasil)
		}
	})

	t.Run("struktur tanpa Stringer", func(t *testing.T) {
		hasil := PanicSummary(jenisPanikKustom{rahasia: tokenUji})
		if strings.Contains(hasil, tokenUji) || strings.Contains(hasil, "rahasia") {
			t.Errorf("PanicSummary() memuat isi struktur: %q", hasil)
		}
		if !strings.Contains(hasil, "jenisPanikKustom") || !strings.Contains(hasil, "<isi tidak dicatat>") {
			t.Errorf("PanicSummary() = %q, ingin jenis nilai dan penanda isi", hasil)
		}
	})

	t.Run("nil", func(t *testing.T) {
		if hasil := PanicSummary(nil); !strings.Contains(hasil, "<kosong>") {
			t.Errorf("PanicSummary(nil) = %q, ingin penanda kosong", hasil)
		}
	})

	t.Run("pesan panjang dipotong", func(t *testing.T) {
		hasil := PanicSummary(strings.Repeat("penautan gagal ", panjangPesanPanik))
		if !strings.HasSuffix(hasil, "…(dipotong)") {
			t.Errorf("PanicSummary() tidak dipotong: %q", hasil)
		}
		if len(hasil) > panjangPesanPanik+len("string: ")+len("…(dipotong)") {
			t.Errorf("panjang PanicSummary() = %d, melebihi batas", len(hasil))
		}
	})
}

// TestStackTraceTerbatas memastikan jejak tumpukan yang dicatat tetap memuat
// nama fungsi pemanggil namun tidak melebihi batas yang ditetapkan.
func TestStackTraceTerbatas(t *testing.T) {
	jejak := StackTrace()
	if !strings.Contains(jejak, "TestStackTraceTerbatas") {
		t.Errorf("StackTrace() tidak memuat fungsi pemanggil: %q", jejak[:min(len(jejak), 200)])
	}
	if len(jejak) > panjangTumpukan+len("…(dipotong)") {
		t.Errorf("panjang StackTrace() = %d, melebihi batas", len(jejak))
	}
}

// TestPotongMenjagaUTF8 memastikan pemotongan tidak menghasilkan JSON tak sah
// ketika batas jatuh di tengah rune.
func TestPotongMenjagaUTF8(t *testing.T) {
	masukan := strings.Repeat("é", 10)
	hasil := potong(masukan, 5)
	if strings.Contains(hasil, "\ufffd") {
		t.Errorf("potong() menghasilkan rune rusak: %q", hasil)
	}
	if len(hasil) > 5+len("…(dipotong)") {
		t.Errorf("potong() melampaui batas: %q", hasil)
	}
}
