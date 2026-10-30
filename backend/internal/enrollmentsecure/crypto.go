// Package enrollmentsecure protects synthetic enrollment payloads. It never logs payloads or keys.
package enrollmentsecure

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

const MaxImageBytes = 1 << 20
const MaxPixels = 2073600
const PendingPhoto = "pending-photo"
const Template = "template"
const TemplateVersion = 2

var ErrUnavailable = errors.New("enrollment secure storage unavailable")
var ErrIntegrity = errors.New("enrollment payload integrity failure")
var ErrUnsupported = fmt.Errorf("%w: unsupported enrollment envelope version", ErrIntegrity)
var ErrImage = errors.New("invalid enrollment JPEG")

// Providers return an owned copy: Seal/Open wipe it, including on failure.
type KeyProvider interface {
	Active(context.Context, string) (string, []byte, error)
	ByVersion(context.Context, string, string) ([]byte, error)
}

type Binding struct {
	Purpose, RecordID, SubmissionID, PersonID, UserID, ConsentID string
	ModelID, ModelVersion, ModelFingerprint                      string
}

type Envelope struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KeyVersion string `json:"key_version"`
	AAD        string `json:"aad"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

func validateBinding(b Binding) error {
	switch b.Purpose {
	case PendingPhoto:
		for _, field := range [...]string{b.RecordID, b.PersonID, b.UserID, b.ConsentID} {
			if field == "" || len(field) > 4096 {
				return ErrIntegrity
			}
		}
	case Template:
		if !canonicalUUID(b.RecordID) || !canonicalUUID(b.SubmissionID) || !canonicalUUID(b.PersonID) || !canonicalUUID(b.ConsentID) || !lowerHex(b.ModelFingerprint, 64) {
			return ErrIntegrity
		}
		for _, field := range [...]string{b.ModelID, b.ModelVersion} {
			if field == "" || len(field) > 4096 || !utf8.ValidString(field) {
				return ErrIntegrity
			}
		}
	default:
		return ErrIntegrity
	}
	return nil
}

func canonicalAAD(b Binding, version string) (string, error) {
	if err := validateBinding(b); err != nil {
		return "", err
	}
	if version == "" || len(version) > 128 || b.Purpose == Template && !utf8.ValidString(version) {
		return "", ErrIntegrity
	}
	var fields []string
	switch b.Purpose {
	case PendingPhoto:
		fields = []string{"FL-ENR01B", "1", PendingPhoto, b.RecordID, b.PersonID, b.UserID, b.ConsentID, "image/jpeg", version}
	case Template:
		fields = []string{"FaceLink", "2", Template, b.RecordID, b.SubmissionID, b.PersonID, b.ConsentID, b.ModelID, b.ModelVersion, b.ModelFingerprint, version}
	default:
		return "", ErrIntegrity
	}
	size := 0
	for _, field := range fields {
		size += 4 + len(field)
	}
	encoded := make([]byte, 0, size)
	for _, field := range fields {
		encoded = binary.BigEndian.AppendUint32(encoded, uint32(len(field)))
		encoded = append(encoded, field...)
	}
	return hex.EncodeToString(encoded), nil
}

func lowerHex(s string, size int) bool {
	if len(s) != size {
		return false
	}
	for i := range s {
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
	}
	return true
}

func canonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	nonzero := false
	for i := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if s[i] != '-' {
				return false
			}
			continue
		}
		if !(s[i] >= '0' && s[i] <= '9' || s[i] >= 'a' && s[i] <= 'f') {
			return false
		}
		nonzero = nonzero || s[i] != '0'
	}
	return nonzero
}

func Seal(ctx context.Context, k KeyProvider, b Binding, plain []byte) (Envelope, error) {
	if k == nil || ctx.Err() != nil {
		return Envelope{}, ErrUnavailable
	}
	if len(plain) == 0 || len(plain) > MaxImageBytes {
		return Envelope{}, ErrIntegrity
	}
	if err := validateBinding(b); err != nil {
		return Envelope{}, err
	}
	version, key, err := k.Active(ctx, b.Purpose)
	defer clear(key)
	if err != nil || len(key) != 32 {
		return Envelope{}, ErrUnavailable
	}
	aad, err := canonicalAAD(b, version)
	if err != nil {
		return Envelope{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Envelope{}, ErrUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, ErrUnavailable
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return Envelope{}, ErrUnavailable
	}
	envelopeVersion := 1
	if b.Purpose == Template {
		envelopeVersion = TemplateVersion
	}
	return Envelope{Version: envelopeVersion, Algorithm: "AES-256-GCM", KeyVersion: version, AAD: aad, Nonce: nonce, Ciphertext: gcm.Seal(nil, nonce, plain, []byte(aad))}, nil
}

func validEnvelope(e Envelope) bool {
	return (e.Version == 1 || e.Version == TemplateVersion) && e.Algorithm == "AES-256-GCM" && e.KeyVersion != "" && len(e.KeyVersion) <= 128 && len(e.Nonce) == 12 && len(e.Ciphertext) >= 16 && len(e.Ciphertext) <= MaxImageBytes+16 && len(e.AAD) > 0 && len(e.AAD) <= 40000
}

func Open(ctx context.Context, k KeyProvider, b Binding, e Envelope) ([]byte, error) {
	switch b.Purpose {
	case PendingPhoto:
		if e.Version != 1 {
			return nil, ErrUnsupported
		}
	case Template:
		if e.Version != TemplateVersion {
			return nil, ErrUnsupported
		}
	default:
		return nil, ErrIntegrity
	}
	if !validEnvelope(e) {
		return nil, ErrIntegrity
	}
	aad, err := canonicalAAD(b, e.KeyVersion)
	if err != nil || aad != e.AAD {
		return nil, ErrIntegrity
	}
	if k == nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	key, err := k.ByVersion(ctx, b.Purpose, e.KeyVersion)
	defer clear(key)
	if err != nil || len(key) != 32 {
		return nil, ErrUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrUnavailable
	}
	plain, err := gcm.Open(nil, e.Nonce, e.Ciphertext, []byte(aad))
	if err != nil {
		return nil, ErrIntegrity
	}
	return plain, nil
}
