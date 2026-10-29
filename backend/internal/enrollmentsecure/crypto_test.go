package enrollmentsecure

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

type testKeys struct {
	photo, template []byte
	unavailable     bool
}

func newTestKeys() *testKeys {
	return &testKeys{photo: bytes.Repeat([]byte{0x31}, 32), template: bytes.Repeat([]byte{0x72}, 32)}
}
func (k *testKeys) Active(ctx context.Context, p string) (string, []byte, error) {
	key, err := k.ByVersion(ctx, p, "1")
	return "1", key, err
}
func (k *testKeys) ByVersion(_ context.Context, p, v string) ([]byte, error) {
	if k.unavailable || v != "1" {
		return nil, ErrUnavailable
	}
	if p == PendingPhoto {
		return bytes.Clone(k.photo), nil
	}
	if p == Template {
		return bytes.Clone(k.template), nil
	}
	return nil, ErrUnavailable
}
func photoBinding() Binding {
	return Binding{Purpose: PendingPhoto, RecordID: "submission", PersonID: "person", UserID: "user", ConsentID: "consent"}
}
func cloneEnvelope(e Envelope) Envelope {
	e.Nonce = bytes.Clone(e.Nonce)
	e.Ciphertext = bytes.Clone(e.Ciphertext)
	return e
}
func TestEnvelopeAuthenticatesEveryTrustedField(t *testing.T) {
	ctx := context.Background()
	keys := newTestKeys()
	b := photoBinding()
	plain := []byte("synthetic private bytes")
	e, err := Seal(ctx, keys, b, plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(ctx, keys, b, e)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatal("round trip failed")
	}
	if bytes.Contains(e.Ciphertext, plain) {
		t.Fatal("plaintext exposed")
	}
	cases := map[string]func(*Envelope){
		"ciphertext": func(e *Envelope) { e.Ciphertext[0] ^= 1 }, "tag": func(e *Envelope) { e.Ciphertext[len(e.Ciphertext)-1] ^= 1 },
		"nonce": func(e *Envelope) { e.Nonce[0] ^= 1 }, "nonce length": func(e *Envelope) { e.Nonce = e.Nonce[:11] },
		"aad": func(e *Envelope) { e.AAD += "00" }, "algorithm": func(e *Envelope) { e.Algorithm = "AES-128-GCM" }, "version": func(e *Envelope) { e.Version = 2 },
		"key version": func(e *Envelope) { e.KeyVersion = "2" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			bad := cloneEnvelope(e)
			change(&bad)
			if _, err := Open(ctx, keys, b, bad); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("wanted integrity error, got %v", err)
			}
		})
	}
	bindings := []Binding{b, b, b, b, b}
	bindings[0].RecordID = "other"
	bindings[1].PersonID = "other"
	bindings[2].UserID = "other"
	bindings[3].ConsentID = "other"
	bindings[4].Purpose = Template
	for _, bad := range bindings {
		if _, err := Open(ctx, keys, bad, e); !errors.Is(err, ErrIntegrity) {
			t.Fatal("binding substitution accepted")
		}
	}
	wrong := newTestKeys()
	wrong.photo[0] ^= 1
	if _, err := Open(ctx, wrong, b, e); !errors.Is(err, ErrIntegrity) {
		t.Fatal("wrong key accepted")
	}
	keys.unavailable = true
	if _, err := Open(ctx, keys, b, e); !errors.Is(err, ErrUnavailable) {
		t.Fatal("missing key did not fail closed")
	}
}
func TestSealBoundsCancellationAndFreshNonces(t *testing.T) {
	ctx := context.Background()
	keys := newTestKeys()
	b := photoBinding()
	seen := map[string]bool{}
	for range 128 {
		e, err := Seal(ctx, keys, b, []byte("synthetic"))
		if err != nil {
			t.Fatal(err)
		}
		if seen[string(e.Nonce)] {
			t.Fatal("nonce reused")
		}
		seen[string(e.Nonce)] = true
	}
	for _, plain := range [][]byte{nil, make([]byte, MaxImageBytes+1)} {
		if _, err := Seal(ctx, keys, b, plain); !errors.Is(err, ErrIntegrity) {
			t.Fatal("bad payload size accepted")
		}
	}
	keys.photo = keys.photo[:16]
	if _, err := Seal(ctx, keys, b, []byte("x")); !errors.Is(err, ErrUnavailable) {
		t.Fatal("short key accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Seal(canceled, newTestKeys(), b, []byte("x")); !errors.Is(err, ErrUnavailable) {
		t.Fatal("canceled encryption accepted")
	}
}

type wipeKeys struct {
	key []byte
	err error
}

func (k *wipeKeys) Active(context.Context, string) (string, []byte, error) {
	return "1", k.key, k.err
}
func (k *wipeKeys) ByVersion(context.Context, string, string) ([]byte, error) {
	return k.key, k.err
}
func TestProviderKeyCopiesAreWiped(t *testing.T) {
	ctx := context.Background()
	binding := photoBinding()
	envelope, err := Seal(ctx, newTestKeys(), binding, []byte("synthetic"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, operation string
		size            int
		providerError   bool
		tamper          bool
		want            error
	}{
		{"seal-success", "seal", 32, false, false, nil},
		{"seal-provider-error", "seal", 32, true, false, ErrUnavailable},
		{"seal-short-key", "seal", 16, false, false, ErrUnavailable},
		{"open-success", "open", 32, false, false, nil},
		{"open-authentication-error", "open", 32, false, true, ErrIntegrity},
		{"open-provider-error", "open", 32, true, false, ErrUnavailable},
		{"open-short-key", "open", 16, false, false, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := &wipeKeys{key: bytes.Repeat([]byte{0x31}, tc.size)}
			if tc.providerError {
				k.err = ErrUnavailable
			}
			var operationErr error
			if tc.operation == "seal" {
				_, operationErr = Seal(ctx, k, binding, []byte("synthetic"))
			} else {
				candidate := cloneEnvelope(envelope)
				if tc.tamper {
					candidate.Ciphertext[len(candidate.Ciphertext)-1] ^= 1
				}
				plain, err := Open(ctx, k, binding, candidate)
				operationErr = err
				clear(plain)
			}
			if !errors.Is(operationErr, tc.want) || !bytes.Equal(k.key, make([]byte, tc.size)) {
				t.Fatal("crypto operation result or provider-owned key cleanup failed")
			}
		})
	}
}
