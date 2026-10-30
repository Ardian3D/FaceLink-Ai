package smtpemail

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func certUji(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kb, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb}))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool.AddCert(c)
	return cert, pool
}

// Server SMTP nyata di loopback: merekam hanya apakah AUTH/pesan terjadi, tidak payloadnya.
func serverUji(t *testing.T, cert tls.Certificate, offerTLS bool, rejectRecipient bool) (int, *atomic.Bool, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var authed atomic.Bool
	body := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		tp := textproto.NewConn(conn)
		defer tp.Close()
		_ = tp.PrintfLine("220 localhost SMTP test")
		for {
			line, err := tp.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				if offerTLS {
					_ = tp.PrintfLine("250-localhost\r\n250-STARTTLS\r\n250 AUTH PLAIN")
				} else {
					_ = tp.PrintfLine("250-localhost\r\n250 AUTH PLAIN")
				}
			case line == "STARTTLS":
				_ = tp.PrintfLine("220 Begin TLS")
				tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
				if err := tc.Handshake(); err != nil {
					return
				}
				conn = tc
				tp = textproto.NewConn(tc)
			case strings.HasPrefix(line, "AUTH"):
				authed.Store(true)
				_ = tp.PrintfLine("235 accepted")
			case strings.HasPrefix(line, "MAIL"):
				_ = tp.PrintfLine("250 accepted")
			case strings.HasPrefix(line, "RCPT"):
				if rejectRecipient {
					_ = tp.PrintfLine("550 rejected private detail should not escape")
				} else {
					_ = tp.PrintfLine("250 accepted")
				}
			case line == "DATA":
				_ = tp.PrintfLine("354 Send data")
				b, err := tp.ReadDotBytes()
				if err != nil {
					return
				}
				body <- string(b)
				_ = tp.PrintfLine("250 queued")
			case line == "QUIT":
				_ = tp.PrintfLine("221 goodbye")
				return
			default:
				_ = tp.PrintfLine("500 unknown")
			}
		}
	}()
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("SMTP test server tidak berhenti")
		}
	})
	return ln.Addr().(*net.TCPAddr).Port, &authed, body
}

func TestSendMemerlukanTLSChainDanHostname(t *testing.T) {
	for _, tc := range []struct {
		name        string
		trust       bool
		host        string
		offerTLS    bool
		reject      bool
		wantAuth    bool
		wantSuccess bool
	}{
		{"trusted", true, "localhost", true, false, true, true},
		{"untrusted", false, "localhost", true, false, false, false},
		{"hostname mismatch", true, "wrong.invalid", true, false, false, false},
		{"no STARTTLS", true, "localhost", false, false, false, false},
		{"SMTP failure", true, "localhost", true, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cert, pool := certUji(t)
			port, authed, body := serverUji(t, cert, tc.offerTLS, tc.reject)
			sender, err := New(Config{Host: "localhost", Port: 587, Username: "smtp-sender@example.test", AuthSecret: "test-secret", From: "smtp-sender@example.test"})
			if err != nil {
				t.Fatal(err)
			}
			sender.cfg.Port = port
			sender.tlsConfig.ServerName = tc.host
			if tc.trust {
				sender.tlsConfig.RootCAs = pool
			}
			err = sender.Send(context.Background(), "recipient@example.test", "01234567")
			if (err == nil) != tc.wantSuccess {
				t.Fatalf("success=%v, want %v (port %s)", err == nil, tc.wantSuccess, strconv.Itoa(port))
			}
			if authed.Load() != tc.wantAuth {
				t.Fatal("AUTH sebelum TLS tervalidasi atau AUTH tidak terjadi")
			}
			if err != nil && (strings.Contains(err.Error(), "test-secret") || strings.Contains(err.Error(), "01234567") || strings.Contains(err.Error(), "private detail")) {
				t.Fatal("error membocorkan payload/respons SMTP")
			}
			if tc.wantSuccess {
				select {
				case msg := <-body:
					if !strings.Contains(msg, "01234567") || !strings.Contains(msg, "Subject: Kode masuk FaceLink AI") {
						t.Fatal("pesan tidak memuat kode/instruksi")
					}
				case <-time.After(time.Second):
					t.Fatal("SMTP tidak menerima pesan")
				}
			}
		})
	}
}

func TestConfigMenolakDowngradeDanInjeksi(t *testing.T) {
	base := Config{Host: "smtp.gmail.com", Port: 587, Username: "smtp-sender@example.test", AuthSecret: "test", From: "smtp-sender@example.test"}
	for _, change := range []func(*Config){func(c *Config) { c.Port = 25 }, func(c *Config) { c.From = "admin@facelink.online" }, func(c *Config) { c.ReplyTo = "a@b.test\r\nBcc: bad@b.test" }, func(c *Config) { c.Host = "smtp.gmail.com:587" }, func(c *Config) { c.AuthSecret = "" }} {
		c := base
		change(&c)
		if _, err := New(c); err == nil {
			t.Fatal("konfigurasi berbahaya diterima")
		}
	}
}

func TestConfigMenerimaAliasTerverifikasi(t *testing.T) {
	base := Config{
		Host: "smtp.gmail.com", Port: 587,
		Username: "smtp-sender@example.test", AuthSecret: "test",
		From: "support@facelink.online", ReplyTo: "support@facelink.online",
		AllowedFromDomains: []string{"facelink.online"},
	}
	if _, err := New(base); err != nil {
		t.Fatalf("alias dengan allowlist eksplisit ditolak: %v", err)
	}

	for _, ubah := range []func(*Config){
		func(c *Config) { c.AllowedFromDomains = nil },
		func(c *Config) { c.AllowedFromDomains = []string{"contoh.test"} },
		func(c *Config) { c.AllowedFromDomains = []string{"facelink.online", "bad domain"} },
	} {
		c := base
		ubah(&c)
		if _, err := New(c); err == nil {
			t.Fatal("alias tanpa otorisasi domain diterima")
		}
	}

	// Penyedia tetap penentu akhir: allowlist bukan bukti kriptografis.
	// From sama dengan Username tidak memerlukan allowlist.
	if _, err := New(Config{Host: "smtp.gmail.com", Port: 587, Username: "a@b.test", AuthSecret: "x", From: "a@b.test"}); err != nil {
		t.Fatalf("From sama dengan Username ditolak: %v", err)
	}
}
