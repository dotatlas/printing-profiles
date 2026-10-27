package images

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

// marked returns a w×h photo, white with a red block in the top-left corner, so a test can see where the corner went.
func marked(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{255, 255, 255, 255}
			if x < w/4 && y < h/4 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// withExif puts an APP1 Exif block with this orientation, and a GPS-like text, right after the JPEG's SOI marker.
func withExif(t *testing.T, jpg []byte, orientation uint16, bigEndian bool) []byte {
	t.Helper()
	var order binary.AppendByteOrder = binary.LittleEndian
	tiff := []byte("II*\x00")
	if bigEndian {
		order, tiff = binary.BigEndian, []byte("MM\x00*")
	}
	tiff = order.AppendUint32(tiff, 8) // IFD0 starts right after the 8-byte header.
	tiff = order.AppendUint16(tiff, 1) // one entry
	tiff = order.AppendUint16(tiff, 0x0112)
	tiff = order.AppendUint16(tiff, 3) // SHORT
	tiff = order.AppendUint32(tiff, 1)
	tiff = order.AppendUint16(tiff, orientation)
	tiff = order.AppendUint16(tiff, 0)
	tiff = order.AppendUint32(tiff, 0) // no next IFD
	tiff = append(tiff, "GPS 52.2297N 21.0122E home-secret"...)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2))
	seg = append(seg, payload...)
	return append(append(append([]byte{}, jpg[:2]...), seg...), jpg[2:]...)
}

// encodeJPEG and encodePNG make test files from an image.
func encodeJPEG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// redAt says whether the pixel at (x, y) of a JPEG is clearly red.
func redAt(t *testing.T, jpg []byte, x, y int) bool {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(x, y).RGBA()
	return r > 0xc000 && g < 0x6000 && b < 0x6000
}

func TestProcessAppliesEveryOrientation(t *testing.T) {
	src := encodeJPEG(t, marked(80, 40))
	// The corner where the red block lands, for each EXIF orientation of an 80×40 photo.
	cases := []struct {
		orientation uint16
		w, h        int
		cornerX     float64 // 0 = left, 1 = right
		cornerY     float64 // 0 = top, 1 = bottom
	}{
		{1, 80, 40, 0, 0}, {2, 80, 40, 1, 0}, {3, 80, 40, 1, 1}, {4, 80, 40, 0, 1},
		{5, 40, 80, 0, 0}, {6, 40, 80, 1, 0}, {7, 40, 80, 1, 1}, {8, 40, 80, 0, 1},
	}
	for _, c := range cases {
		for _, big := range []bool{false, true} {
			p, err := Process(withExif(t, src, c.orientation, big))
			if err != nil {
				t.Fatalf("orientation %d: %v", c.orientation, err)
			}
			if p.Width != c.w || p.Height != c.h {
				t.Fatalf("orientation %d: size %d×%d, want %d×%d", c.orientation, p.Width, p.Height, c.w, c.h)
			}
			x, y := int(c.cornerX*float64(c.w-1)), int(c.cornerY*float64(c.h-1))
			if !redAt(t, p.Full, x, y) {
				t.Fatalf("orientation %d (big endian %v): the red corner is not at (%d, %d)", c.orientation, big, x, y)
			}
		}
	}
}

func TestProcessDropsMetadata(t *testing.T) {
	p, err := Process(withExif(t, encodeJPEG(t, marked(64, 64)), 1, false))
	if err != nil {
		t.Fatal(err)
	}
	for name, out := range map[string][]byte{"full": p.Full, "thumb": p.Thumb} {
		if bytes.Contains(out, []byte("home-secret")) || bytes.Contains(out, []byte("Exif")) {
			t.Fatalf("the %s image keeps the metadata", name)
		}
	}
}

func TestProcessScalesDownAndNeverUp(t *testing.T) {
	p, err := Process(encodePNG(t, marked(3000, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Width != 2048 || p.Height != 683 {
		t.Fatalf("full %d×%d, want 2048×683", p.Width, p.Height)
	}
	thumb, err := jpeg.DecodeConfig(bytes.NewReader(p.Thumb))
	if err != nil || thumb.Width != 480 || thumb.Height != 160 {
		t.Fatalf("thumb %d×%d (%v), want 480×160", thumb.Width, thumb.Height, err)
	}

	small, err := Process(encodePNG(t, marked(100, 50)))
	if err != nil || small.Width != 100 || small.Height != 50 {
		t.Fatalf("a small photo became %d×%d (%v)", small.Width, small.Height, err)
	}
}

func TestProcessPutsTransparencyOnWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10)) // fully transparent
	p, err := Process(encodePNG(t, img))
	if err != nil {
		t.Fatal(err)
	}
	out, err := jpeg.Decode(bytes.NewReader(p.Full))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := out.At(5, 5).RGBA(); r < 0xf000 || g < 0xf000 || b < 0xf000 {
		t.Fatalf("a transparent pixel became %x %x %x, want white", r, g, b)
	}
}

// pngHeader is a PNG signature with a valid IHDR chunk for w×h and nothing more: enough for DecodeConfig.
func pngHeader(w, h uint32) []byte {
	ihdr := binary.BigEndian.AppendUint32([]byte("IHDR"), w)
	ihdr = binary.BigEndian.AppendUint32(ihdr, h)
	ihdr = append(ihdr, 8, 6, 0, 0, 0) // 8-bit RGBA
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, 13)
	out = append(out, ihdr...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(ihdr))
}

func TestProcessRefusesWhatItCannotTrust(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want string
	}{
		"webp":       {[]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), "save it as JPEG or PNG"},
		"heic":       {[]byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00"), "save it as JPEG or PNG"},
		"gif":        {[]byte("GIF89a\x01\x00\x01\x00"), "not a JPEG or PNG photo"},
		"text":       {[]byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), "not a JPEG or PNG photo"},
		"too wide":   {pngHeader(9000, 10), "at most 8192 pixels"},
		"too many":   {pngHeader(8000, 8000), "at most 50 megapixels"},
		"broken png": {pngHeader(100, 100), "could not be read"},
		"empty":      {nil, "not a JPEG or PNG photo"},
	}
	for name, c := range cases {
		_, err := Process(c.data)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", name, err, c.want)
		}
	}
}

func TestOrientationIgnoresBrokenExif(t *testing.T) {
	jpg := encodeJPEG(t, marked(8, 8))
	broken := append(append([]byte{}, jpg[:2]...), 0xFF, 0xE1, 0x00, 0x08, 'E', 'x', 'i', 'f', 0, 0)
	broken = append(broken, jpg[2:]...)
	if o := orientation(broken); o != 1 {
		t.Fatalf("orientation of a cut Exif block = %d, want 1", o)
	}
	if o := orientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF}); o != 1 {
		t.Fatalf("orientation of a segment longer than the file = %d, want 1", o)
	}
}

// BenchmarkProcess12MP measures a phone-sized photo: 4000×3000, with detail so the JPEG is a realistic size.
// Run: go test ./internal/images -bench . -benchmem -run '^$'
func BenchmarkProcess12MP(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 4000, 3000))
	for i := range img.Pix {
		img.Pix[i] = uint8((i * 7919) >> 3)
	}
	data := encodeJPEG(b, img)
	b.ReportMetric(float64(len(data))/1e6, "MB-in")
	for b.Loop() {
		if _, err := Process(data); err != nil {
			b.Fatal(err)
		}
	}
}
