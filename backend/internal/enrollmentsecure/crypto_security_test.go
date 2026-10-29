package enrollmentsecure

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func templateSecurityBinding() Binding {
	return Binding{
		Purpose:          Template,
		RecordID:         "10000000-0000-4000-8000-000000000001",
		SubmissionID:     "40000000-0000-4000-8000-000000000001",
		PersonID:         "20000000-0000-4000-8000-000000000001",
		ConsentID:        "30000000-0000-4000-8000-000000000001",
		ModelID:          "synthetic-model",
		ModelVersion:     "1",
		ModelFingerprint: strings.Repeat("ab", 32),
	}
}

func requireIntegrityRejection(t *testing.T, keys KeyProvider, binding Binding, envelope Envelope) {
	t.Helper()
	plain, err := Open(context.Background(), keys, binding, envelope)
	if !errors.Is(err, ErrIntegrity) || len(plain) != 0 {
		t.Fatalf("rejection returned plaintext=%t, error=%v", len(plain) != 0, err)
	}
}

func sealSecurityTemplate(t *testing.T, keys KeyProvider, binding Binding) Envelope {
	t.Helper()
	envelope, err := Seal(context.Background(), keys, binding, []byte("synthetic private template"))
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

// A different record for the same person and model is not the record that was sealed.
func TestTemplateRejectsSamePersonDifferentRecordSwap(t *testing.T) {
	ctx := context.Background()
	keys := newTestKeys()
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	binding.RecordID = "10000000-0000-4000-8000-000000000002"
	plain, err := Open(ctx, keys, binding, envelope)
	if !errors.Is(err, ErrIntegrity) || len(plain) != 0 {
		t.Fatalf("cross-record template swap returned plaintext=%t, error=%v", len(plain) != 0, err)
	}
}

func TestTemplateV2RoundTripAndEveryBindingMismatch(t *testing.T) {
	keys := newTestKeys()
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	if envelope.Version != TemplateVersion || TemplateVersion != 2 {
		t.Fatal("template write did not use v2")
	}
	plain, err := Open(context.Background(), keys, binding, envelope)
	if err != nil || !bytes.Equal(plain, []byte("synthetic private template")) {
		t.Fatal("v2 round trip failed")
	}
	changes := map[string]func(*Binding){
		"record":        func(b *Binding) { b.RecordID = "10000000-0000-4000-8000-000000000002" },
		"submission":    func(b *Binding) { b.SubmissionID = "40000000-0000-4000-8000-000000000002" },
		"person":        func(b *Binding) { b.PersonID = "20000000-0000-4000-8000-000000000002" },
		"consent":       func(b *Binding) { b.ConsentID = "30000000-0000-4000-8000-000000000002" },
		"model":         func(b *Binding) { b.ModelID = "other-synthetic-model" },
		"model version": func(b *Binding) { b.ModelVersion = "2" },
		"fingerprint":   func(b *Binding) { b.ModelFingerprint = strings.Repeat("cd", 32) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			other := binding
			change(&other)
			requireIntegrityRejection(t, keys, other, envelope)

			// The attacker can replace stored AAD with the exact AAD expected by the
			// destination row. It still cannot authenticate the original ciphertext.
			otherEnvelope := sealSecurityTemplate(t, keys, other)
			forged := cloneEnvelope(envelope)
			forged.AAD = otherEnvelope.AAD
			requireIntegrityRejection(t, keys, other, forged)
		})
	}
}

func TestTemplateRejectsCiphertextAndEnvelopeTampering(t *testing.T) {
	keys := newTestKeys()
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	changes := map[string]func(*Envelope){
		"ciphertext":   func(e *Envelope) { e.Ciphertext[0] ^= 1 },
		"tag":          func(e *Envelope) { e.Ciphertext[len(e.Ciphertext)-1] ^= 1 },
		"nonce":        func(e *Envelope) { e.Nonce[0] ^= 1 },
		"short nonce":  func(e *Envelope) { e.Nonce = e.Nonce[:11] },
		"stored AAD":   func(e *Envelope) { e.AAD += "00" },
		"missing AAD":  func(e *Envelope) { e.AAD = "" },
		"oversize AAD": func(e *Envelope) { e.AAD = strings.Repeat("0", 40001) },
		"algorithm":    func(e *Envelope) { e.Algorithm = "AES-128-GCM" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			forged := cloneEnvelope(envelope)
			change(&forged)
			requireIntegrityRejection(t, keys, binding, forged)
		})
	}
	wrongKeys := newTestKeys()
	wrongKeys.template[0] ^= 1
	requireIntegrityRejection(t, wrongKeys, binding, envelope)
}

// Both versions deliberately resolve to the same key: rejection must come from
// authenticated key-version binding, not merely a missing or different key.
type securityVersionKeys struct {
	*testKeys
	version string
}

func (k securityVersionKeys) Active(ctx context.Context, purpose string) (string, []byte, error) {
	key, err := k.ByVersion(ctx, purpose, k.version)
	return k.version, key, err
}

func (k securityVersionKeys) ByVersion(ctx context.Context, purpose, _ string) ([]byte, error) {
	return k.testKeys.ByVersion(ctx, purpose, "1")
}

func TestTemplateAuthenticatesKeyVersionEvenWhenKeysMatch(t *testing.T) {
	binding := templateSecurityBinding()
	keys := securityVersionKeys{testKeys: newTestKeys(), version: "1"}
	envelope := sealSecurityTemplate(t, keys, binding)
	otherKeys := keys
	otherKeys.version = "2"
	other := sealSecurityTemplate(t, otherKeys, binding)
	forged := cloneEnvelope(envelope)
	forged.KeyVersion = "2"
	requireIntegrityRejection(t, otherKeys, binding, forged)
	forged.AAD = other.AAD
	requireIntegrityRejection(t, otherKeys, binding, forged)
}

func encryptSecurityAAD(t *testing.T, key []byte, envelope Envelope, aad string) Envelope {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	envelope.AAD = aad
	envelope.Ciphertext = gcm.Seal(nil, envelope.Nonce, []byte("synthetic private template"), []byte(aad))
	plain, err := gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, []byte(aad))
	if err != nil || !bytes.Equal(plain, []byte("synthetic private template")) {
		t.Fatal("attack fixture is not authentic AES-GCM")
	}
	return envelope
}

func TestTemplateRejectsWrongDomainAndCrossPurposeEvenWithSameKey(t *testing.T) {
	keys := newTestKeys()
	keys.photo = bytes.Clone(keys.template)
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	// Authenticate under a different protocol domain, then forge stored AAD to
	// the victim's expected value. GCM must not accept the different domain.
	wrongDomain := encryptSecurityAAD(t, keys.template, cloneEnvelope(envelope), "different-domain:"+envelope.AAD)
	requireIntegrityRejection(t, keys, binding, wrongDomain)
	wrongDomain.AAD = envelope.AAD
	requireIntegrityRejection(t, keys, binding, wrongDomain)

	photo := photoBinding()
	photoEnvelope, err := Seal(context.Background(), keys, photo, []byte("synthetic private template"))
	if err != nil || photoEnvelope.Version != 1 {
		t.Fatal("pending photo v1 write changed")
	}
	requireIntegrityRejection(t, keys, photo, envelope)
	requireIntegrityRejection(t, keys, binding, photoEnvelope)
	// Rewrite both the purpose-specific version and stored AAD, while retaining
	// the authentic ciphertext from the other purpose and the same AES key.
	forged := cloneEnvelope(envelope)
	forged.Version = 1
	forged.AAD = photoEnvelope.AAD
	requireIntegrityRejection(t, keys, photo, forged)
	forged = cloneEnvelope(photoEnvelope)
	forged.Version = TemplateVersion
	forged.AAD = envelope.AAD
	requireIntegrityRejection(t, keys, binding, forged)
}

func TestTemplateRejectsAuthenticLegacyWithoutFallback(t *testing.T) {
	keys := newTestKeys()
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	// Reproduce the historical authenticated payload, not a version-tampered v2.
	// This fixture only establishes compatibility refusal; it does not pin v2's
	// incidental serialization.
	var legacyAAD []byte
	for _, field := range []string{binding.PersonID, binding.ModelID, binding.ModelVersion, envelope.KeyVersion} {
		legacyAAD = binary.BigEndian.AppendUint32(legacyAAD, uint32(len(field)))
		legacyAAD = append(legacyAAD, field...)
	}
	legacy := encryptSecurityAAD(t, keys.template, cloneEnvelope(envelope), hex.EncodeToString(legacyAAD))
	legacy.Version = 1
	plain, err := Open(context.Background(), keys, binding, legacy)
	if len(plain) != 0 || !errors.Is(err, ErrUnsupported) || !errors.Is(err, ErrIntegrity) {
		t.Fatalf("authentic legacy template not quarantined: plaintext=%t, error=%v", len(plain) != 0, err)
	}
	// Relabeling authentic v1 data as v2 is not an upgrade or a fallback path.
	legacy.Version = TemplateVersion
	requireIntegrityRejection(t, keys, binding, legacy)
	legacy.AAD = envelope.AAD
	requireIntegrityRejection(t, keys, binding, legacy)
}

func TestEnvelopeVersionsArePurposeSpecificAndFailBeforeKeyLookup(t *testing.T) {
	keys := newTestKeys()
	template := sealSecurityTemplate(t, keys, templateSecurityBinding())
	photo, err := Seal(context.Background(), keys, photoBinding(), []byte("synthetic photo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		binding  Binding
		envelope Envelope
		versions []int
	}{
		{"template", templateSecurityBinding(), template, []int{-1, 0, 1, 3, 99}},
		{"photo", photoBinding(), photo, []int{-1, 0, TemplateVersion, 3, 99}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, version := range tc.versions {
				envelope := cloneEnvelope(tc.envelope)
				envelope.Version = version
				// A nil provider proves unsupported versions are rejected before
				// any fallback can obtain a key or decrypt a payload.
				plain, err := Open(context.Background(), nil, tc.binding, envelope)
				if len(plain) != 0 || !errors.Is(err, ErrUnsupported) || !errors.Is(err, ErrIntegrity) {
					t.Fatalf("version %d: plaintext=%t, error=%v", version, len(plain) != 0, err)
				}
			}
		})
	}
}

func TestTemplateRequiresCompleteBoundedCanonicalBinding(t *testing.T) {
	keys := newTestKeys()
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	fields := map[string]func(*Binding) *string{
		"record":        func(b *Binding) *string { return &b.RecordID },
		"submission":    func(b *Binding) *string { return &b.SubmissionID },
		"person":        func(b *Binding) *string { return &b.PersonID },
		"consent":       func(b *Binding) *string { return &b.ConsentID },
		"model":         func(b *Binding) *string { return &b.ModelID },
		"model version": func(b *Binding) *string { return &b.ModelVersion },
		"fingerprint":   func(b *Binding) *string { return &b.ModelFingerprint },
	}
	for name, field := range fields {
		t.Run("missing "+name, func(t *testing.T) {
			bad := binding
			*field(&bad) = ""
			sealed, err := Seal(context.Background(), keys, bad, []byte("synthetic"))
			if !errors.Is(err, ErrIntegrity) || len(sealed.Ciphertext) != 0 {
				t.Fatalf("incomplete binding accepted: error=%v", err)
			}
			requireIntegrityRejection(t, keys, bad, envelope)
		})
	}
	for _, field := range []func(*Binding) *string{fields["record"], fields["submission"], fields["person"], fields["consent"]} {
		for _, invalid := range []string{
			"00000000-0000-0000-0000-000000000000",
			"10000000-0000-4000-8000-00000000000A",
			"10000000000040008000000000000001",
			"10000000-0000-4000-8000-00000000000g",
		} {
			bad := binding
			*field(&bad) = invalid
			if _, err := Seal(context.Background(), keys, bad, []byte("synthetic")); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("noncanonical UUID accepted: %q, error=%v", invalid, err)
			}
			requireIntegrityRejection(t, keys, bad, envelope)
		}
	}
	for _, invalid := range []string{strings.Repeat("a", 63), strings.Repeat("a", 65), strings.Repeat("A", 64), strings.Repeat("g", 64)} {
		bad := binding
		bad.ModelFingerprint = invalid
		if _, err := Seal(context.Background(), keys, bad, []byte("synthetic")); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("invalid fingerprint accepted: error=%v", err)
		}
		requireIntegrityRejection(t, keys, bad, envelope)
	}
	for _, field := range []func(*Binding) *string{fields["model"], fields["model version"]} {
		for _, invalid := range []string{strings.Repeat("x", 4097), string([]byte{0xff})} {
			bad := binding
			*field(&bad) = invalid
			if _, err := Seal(context.Background(), keys, bad, []byte("synthetic")); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("unbounded/non-UTF8 model identity accepted: error=%v", err)
			}
			requireIntegrityRejection(t, keys, bad, envelope)
		}
	}
	for _, invalid := range []string{"", strings.Repeat("v", 129), string([]byte{0xff})} {
		badKeys := securityVersionKeys{testKeys: newTestKeys(), version: invalid}
		if _, err := Seal(context.Background(), badKeys, binding, []byte("synthetic")); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("invalid active key version accepted: error=%v", err)
		}
		forged := cloneEnvelope(envelope)
		forged.KeyVersion = invalid
		requireIntegrityRejection(t, badKeys, binding, forged)
	}
	boundary := binding
	boundary.ModelID = strings.Repeat("m", 4096)
	boundary.ModelVersion = strings.Repeat("v", 4096)
	boundaryKeys := securityVersionKeys{testKeys: newTestKeys(), version: strings.Repeat("k", 128)}
	bounded := sealSecurityTemplate(t, boundaryKeys, boundary)
	if plain, err := Open(context.Background(), boundaryKeys, boundary, bounded); err != nil || len(plain) == 0 {
		t.Fatalf("valid bounded identity failed: error=%v", err)
	}
}

func TestTemplateBindingCannotBeConfusedByFieldDelimiters(t *testing.T) {
	for _, fields := range [][4]string{
		{"synthetic|pipeline", "revision", "synthetic", "pipeline|revision"},
		{"synthetic", "pipeline", "syntheticpipe", "line"},
	} {
		keys := newTestKeys()
		binding := templateSecurityBinding()
		binding.ModelID, binding.ModelVersion = fields[0], fields[1]
		envelope := sealSecurityTemplate(t, keys, binding)
		if repeat := sealSecurityTemplate(t, keys, binding); repeat.AAD != envelope.AAD {
			t.Fatal("identical record metadata produced nondeterministic AAD")
		}
		other := binding
		other.ModelID, other.ModelVersion = fields[2], fields[3]
		requireIntegrityRejection(t, keys, other, envelope)
		forged := cloneEnvelope(envelope)
		forged.AAD = sealSecurityTemplate(t, keys, other).AAD
		requireIntegrityRejection(t, keys, other, forged)
	}
}

func TestTemplateSameBindingReplayRequiresExternalFreshnessPolicy(t *testing.T) {
	keys := newTestKeys()
	binding := templateSecurityBinding()
	envelope := sealSecurityTemplate(t, keys, binding)
	// AEAD authenticates the supplied identity; it is not an anti-replay ledger.
	// A rollback of both ciphertext and the same trusted metadata remains valid.
	// Callers must enforce activation/consent/freshness outside this primitive.
	for range 2 {
		plain, err := Open(context.Background(), keys, binding, envelope)
		if err != nil || !bytes.Equal(plain, []byte("synthetic private template")) {
			t.Fatal("same-binding authenticated replay unexpectedly changed behavior")
		}
	}
}
