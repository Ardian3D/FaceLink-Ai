package enrollmentsecure

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"io"
)

var imageSlots = make(chan struct{}, 2)

// NormalizeJPEG decodes at most two bounded images concurrently, only in memory.
// Nonidentity orientation and EXIF sub-IFDs/thumbnails are deliberately rejected,
// not silently interpreted. All accepted metadata is removed by re-encoding.
func NormalizeJPEG(input []byte) (canonical []byte, width, height int, err error) {
	if len(input) == 0 || len(input) > MaxImageBytes || !jpegStructure(input) {
		return nil, 0, 0, ErrImage
	}
	config, e := jpeg.DecodeConfig(bytes.NewReader(input))
	if e != nil || config.Width < 112 || config.Height < 112 || config.Width > 1920 || config.Height > 1920 || config.Width*config.Height > MaxPixels {
		return nil, 0, 0, ErrImage
	}
	imageSlots <- struct{}{}
	defer func() { <-imageSlots }()
	img, e := jpeg.Decode(bytes.NewReader(input))
	if e != nil {
		return nil, 0, 0, ErrImage
	}
	if img.Bounds().Dx() != config.Width || img.Bounds().Dy() != config.Height {
		return nil, 0, 0, ErrImage
	}
	out := &boundedJPEGWriter{}
	if jpeg.Encode(out, img, &jpeg.Options{Quality: 85}) != nil {
		clear(out.b)
		return nil, 0, 0, ErrImage
	}
	return out.b, config.Width, config.Height, nil
}

type boundedJPEGWriter struct{ b []byte }

func (w *boundedJPEGWriter) Write(p []byte) (int, error) {
	if len(p) > MaxImageBytes-len(w.b) {
		return 0, io.ErrShortWrite
	}
	w.b = append(w.b, p...)
	return len(p), nil
}

// Parse every marker, including entropy-coded segments, to reject concatenation,
// trailing bytes, truncated segments, repeated SOI and unbounded progressive scans.
func jpegStructure(b []byte) bool {
	if len(b) < 4 || b[0] != 0xff || b[1] != 0xd8 {
		return false
	}
	p, markers, scans := 2, 0, 0
	seenFrame := false
	for p < len(b) {
		markers++
		if markers > 4096 || b[p] != 0xff {
			return false
		}
		for p < len(b) && b[p] == 0xff {
			p++
		}
		if p == len(b) {
			return false
		}
		marker := b[p]
		p++
		if marker == 0xd9 {
			return seenFrame && scans > 0 && p == len(b)
		}
		if marker == 0xd8 || marker == 0x00 || marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			return false
		}
		if p+2 > len(b) {
			return false
		}
		n := int(binary.BigEndian.Uint16(b[p : p+2]))
		if n < 2 || n > len(b)-p {
			return false
		}
		segment := b[p+2 : p+n]
		p += n
		switch marker {
		case 0xc0, 0xc1, 0xc2:
			if seenFrame {
				return false
			}
			seenFrame = true
		case 0xda:
			if !seenFrame {
				return false
			}
			scans++
			if scans > 128 {
				return false
			}
			// Byte-stuffed FF00 and restart markers belong to the entropy stream.
			for p < len(b) {
				if b[p] != 0xff {
					p++
					continue
				}
				start := p
				p++
				for p < len(b) && b[p] == 0xff {
					p++
				}
				if p == len(b) {
					return false
				}
				if b[p] == 0 || b[p] >= 0xd0 && b[p] <= 0xd7 {
					p++
					continue
				}
				p = start
				break
			}
		case 0xe1:
			if bytes.HasPrefix(segment, []byte("Exif")) && (!bytes.HasPrefix(segment, []byte("Exif\x00\x00")) || !safeEXIF(segment[6:])) {
				return false
			}
		case 0xc4, 0xdb, 0xdd, 0xfe:
		default:
			if marker < 0xe0 || marker > 0xef {
				return false
			}
		}
	}
	return false
}

func safeEXIF(b []byte) bool {
	if len(b) < 8 {
		return false
	}
	var order binary.ByteOrder
	if string(b[:2]) == "II" {
		order = binary.LittleEndian
	} else if string(b[:2]) == "MM" {
		order = binary.BigEndian
	} else {
		return false
	}
	if order.Uint16(b[2:4]) != 42 {
		return false
	}
	offset := uint64(order.Uint32(b[4:8]))
	if offset < 8 || offset+2 > uint64(len(b)) {
		return false
	}
	count := uint64(order.Uint16(b[offset : offset+2]))
	if count > 256 || offset+2+12*count+4 > uint64(len(b)) {
		return false
	}
	widths := [...]uint64{0, 1, 1, 2, 4, 8, 1, 1, 2, 4, 8, 4, 8}
	orientationSeen := false
	for i := range count {
		entry := b[offset+2+12*i : offset+2+12*(i+1)]
		tag, kind, n := order.Uint16(entry[:2]), order.Uint16(entry[2:4]), uint64(order.Uint32(entry[4:8]))
		if kind == 0 || int(kind) >= len(widths) || n == 0 {
			return false
		}
		size := n * widths[kind]
		if size > 4 {
			at := uint64(order.Uint32(entry[8:12]))
			if at < 8 || at > uint64(len(b)) || size > uint64(len(b))-at {
				return false
			}
		}
		if tag == 0x8769 || tag == 0x8825 || tag == 0xa005 || tag == 0x0201 || tag == 0x0202 {
			return false
		}
		if tag == 0x0112 {
			if orientationSeen || kind != 3 || n != 1 || order.Uint16(entry[8:10]) != 1 {
				return false
			}
			orientationSeen = true
		}
	}
	return order.Uint32(b[offset+2+12*count:offset+2+12*count+4]) == 0
}
