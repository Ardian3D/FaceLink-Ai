// Package smtpemail mengirim kode OTP tanpa mencatat kredensial/payload.
package smtpemail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config hanya untuk backend. Jangan serialisasi nilai ini ke log.
type Config struct {
	Host       string
	Port       int
	Username   string
	AuthSecret string
	From       string
	ReplyTo    string
	// AllowedFromDomains adalah daftar eksplisit domain alias yang diizinkan
	// berbeda dari domain SMTP_USERNAME. Daftar ini adalah pernyataan operator
	// bahwa alias sudah terverifikasi di penyedia SMTP; ia bukan bukti
	// kriptografis. Penyedia tetap yang menolak atau mengganti From.
	AllowedFromDomains []string
}

// Validate menolak downgrade SMTP, injeksi header, dan From yang belum diotorisasi.
func (c Config) Validate() error {
	if c.Host == "" || strings.ContainsAny(c.Host, "\r\n /:@") {
		return errors.New("SMTP_HOST harus hostname tanpa port/skema")
	}
	if c.Port != 465 && c.Port != 587 {
		return errors.New("SMTP_PORT wajib 465 (TLS) atau 587 (STARTTLS wajib)")
	}
	if !validAddress(c.Username) || !validAddress(c.From) || (c.ReplyTo != "" && !validAddress(c.ReplyTo)) {
		return errors.New("alamat konfigurasi SMTP tidak sah")
	}
	if !strings.EqualFold(c.From, c.Username) {
		domain := alamatDomain(c.From)
		diizinkan := false
		for _, d := range c.AllowedFromDomains {
			d = strings.ToLower(strings.TrimSpace(d))
			if !validDomain(d) {
				return errors.New("SMTP_ALLOWED_FROM_DOMAINS memuat domain yang tidak sah")
			}
			if d == domain {
				diizinkan = true
			}
		}
		if !diizinkan {
			return errors.New("SMTP_FROM berbeda dari SMTP_USERNAME; tambahkan domainnya ke SMTP_ALLOWED_FROM_DOMAINS hanya jika alias sudah terverifikasi di penyedia SMTP")
		}
	}
	if strings.TrimSpace(c.AuthSecret) == "" || strings.ContainsAny(c.AuthSecret, "\r\n") {
		return errors.New("SMTP_AUTH_SECRET wajib diisi tanpa baris baru")
	}
	return nil
}

// alamatDomain mengembalikan domain huruf kecil dari alamat yang sudah valid.
func alamatDomain(alamat string) string {
	i := strings.LastIndex(alamat, "@")
	if i < 0 || i == len(alamat)-1 {
		return ""
	}
	return strings.ToLower(alamat[i+1:])
}

// validDomain membatasi allowlist pada bentuk domain DNS sederhana.
func validDomain(d string) bool {
	if d == "" || len(d) > 253 || strings.HasPrefix(d, ".") || strings.HasSuffix(d, ".") || strings.Contains(d, "..") {
		return false
	}
	for _, r := range d {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validAddress(s string) bool {
	if len(s) > 254 || strings.ContainsAny(s, "\r\n") {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s && strings.Contains(s, "@")
}

// Sender memverifikasi certificate chain dan hostname dengan trust store sistem.
type Sender struct {
	cfg       Config
	tlsConfig *tls.Config
}

func New(c Config) (*Sender, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &Sender{cfg: c, tlsConfig: &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}}, nil
}

// Send memiliki deadline dan hanya menjalankan AUTH setelah TLS tervalidasi.
// Error tahap sengaja tidak menyertakan respons SMTP, alamat, secret, atau kode.
func (s *Sender) Send(ctx context.Context, to, code string) error {
	if !validAddress(to) || len(code) != 8 || strings.IndexFunc(code, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return errors.New("smtp: payload tidak sah")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return errors.New("smtp: koneksi gagal")
	}
	defer conn.Close()
	if d, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(d); err != nil {
			return errors.New("smtp: deadline gagal")
		}
	}
	rawConn := conn
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	if s.cfg.Port == 465 {
		tc := tls.Client(conn, s.tlsConfig.Clone())
		if err = tc.HandshakeContext(ctx); err != nil {
			return errors.New("smtp: TLS tidak dapat diverifikasi")
		}
		conn = tc
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return errors.New("smtp: greeting gagal")
	}
	defer client.Close()
	if s.cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp: STARTTLS wajib tersedia")
		}
		if err = client.StartTLS(s.tlsConfig.Clone()); err != nil {
			return errors.New("smtp: TLS tidak dapat diverifikasi")
		}
	}
	state, ok := client.TLSConnectionState()
	if !ok || !state.HandshakeComplete {
		return errors.New("smtp: TLS wajib sebelum AUTH")
	}
	if err = client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.AuthSecret, s.cfg.Host)); err != nil {
		return errors.New("smtp: autentikasi gagal")
	}
	if err = client.Mail(s.cfg.From); err != nil {
		return errors.New("smtp: pengirim ditolak")
	}
	if err = client.Rcpt(to); err != nil {
		return errors.New("smtp: penerima ditolak")
	}
	w, err := client.Data()
	if err != nil {
		return errors.New("smtp: pesan ditolak")
	}
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Kode masuk FaceLink AI\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n", s.cfg.From, to)
	if s.cfg.ReplyTo != "" {
		headers += "Reply-To: " + s.cfg.ReplyTo + "\r\n"
	}
	_, err = io.WriteString(w, headers+"\r\nKode masuk FaceLink AI: "+code+"\r\n\r\nBerlaku 5 menit. Hanya kode terbaru yang berlaku. Jangan bagikan kode ini.\r\nJika Anda tidak meminta kode, abaikan email ini.\r\n")
	if err != nil {
		_ = w.Close()
		return errors.New("smtp: pengiriman gagal")
	}
	if err = w.Close(); err != nil {
		return errors.New("smtp: pesan tidak diterima server")
	}
	// 250 setelah DATA berarti server menerima pesan. Kegagalan QUIT sesudahnya
	// tidak membatalkan penerimaan itu dan tidak boleh membuat kode tak terpakai.
	_ = client.Quit()
	return nil
}
