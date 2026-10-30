package enrollmentsecure

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func syntheticJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 77, 255})
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func withSegment(j []byte, marker byte, data []byte) []byte {
	out := append([]byte{}, j[:2]...)
	out = append(out, 0xff, marker)
	out = binary.BigEndian.AppendUint16(out, uint16(len(data)+2))
	out = append(out, data...)
	return append(out, j[2:]...)
}
func exifOrientation(value uint16) []byte {
	b := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(b[18:20], value)
	return append([]byte("Exif\x00\x00"), b...)
}
func TestNormalizeJPEGStripsMetadata(t *testing.T) {
	original := syntheticJPEG(t, 112, 113)
	input := withSegment(withSegment(withSegment(original, 0xe1, exifOrientation(1)), 0xfe, []byte("synthetic-private-comment")), 0xe2, []byte("ICC_PROFILE\x00synthetic"))
	out, w, h, err := NormalizeJPEG(input)
	if err != nil || w != 112 || h != 113 {
		t.Fatalf("valid image: %v %dx%d", err, w, h)
	}
	for _, metadata := range [][]byte{[]byte("Exif"), []byte("synthetic-private-comment"), []byte("ICC_PROFILE")} {
		if bytes.Contains(out, metadata) {
			t.Fatal("metadata retained")
		}
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, withSegment(withSegment(withSegment(original, 0xe1, exifOrientation(1)), 0xfe, []byte("synthetic-private-comment")), 0xe2, []byte("ICC_PROFILE\x00synthetic"))) {
		t.Fatal("input modified")
	}
}
func TestJPEGRejectsMalformedUnsafeAndOversize(t *testing.T) {
	valid := syntheticJPEG(t, 112, 112)
	malformedEXIF := exifOrientation(1)
	malformedEXIF[10] = 0xff
	malformedEXIF[11] = 0xff
	cases := map[string][]byte{
		"empty": nil, "not JPEG": []byte("synthetic PNG"), "oversize": make([]byte, MaxImageBytes+1),
		"truncated": valid[:len(valid)-1], "trailing": append(bytes.Clone(valid), 0), "multiple": append(bytes.Clone(valid), valid...),
		"orientation2": withSegment(valid, 0xe1, exifOrientation(2)), "orientation6": withSegment(valid, 0xe1, exifOrientation(6)),
		"malformed EXIF": withSegment(valid, 0xe1, malformedEXIF), "short EXIF": withSegment(valid, 0xe1, []byte("Exif")),
		"short side": syntheticJPEG(t, 111, 112), "long side": syntheticJPEG(t, 1921, 112), "pixel amplification": syntheticJPEG(t, 1920, 1081),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if out, _, _, err := NormalizeJPEG(input); err != ErrImage || out != nil {
				t.Fatal("unsafe image accepted")
			}
		})
	}
}
func TestJPEGAcceptsPixelBoundaryAndLimitsOutput(t *testing.T) {
	input := syntheticJPEG(t, 1920, 1080)
	out, w, h, err := NormalizeJPEG(input)
	if err != nil || w*h != MaxPixels || len(out) > MaxImageBytes {
		t.Fatalf("boundary image rejected: %v", err)
	}
	writer := &boundedJPEGWriter{}
	if _, err := writer.Write(make([]byte, MaxImageBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte{1}); err == nil || len(writer.b) != MaxImageBytes {
		t.Fatal("output limit not enforced")
	}
}
func TestEXIFRejectsPointersAndOutOfBoundsValues(t *testing.T) {
	b := exifOrientation(1)[6:]
	for _, tag := range []uint16{0x8769, 0x8825, 0xa005, 0x0201, 0x0202} {
		bad := bytes.Clone(b)
		binary.LittleEndian.PutUint16(bad[10:12], tag)
		if safeEXIF(bad) {
			t.Fatal("unsafe EXIF pointer accepted")
		}
	}
	bad := bytes.Clone(b)
	binary.LittleEndian.PutUint32(bad[len(bad)-4:], 8)
	if safeEXIF(bad) {
		t.Fatal("IFD cycle accepted")
	}
	bad = bytes.Clone(b)
	binary.LittleEndian.PutUint32(bad[4:8], 0xfffffff0)
	if safeEXIF(bad) {
		t.Fatal("EXIF offset overflow accepted")
	}
}
