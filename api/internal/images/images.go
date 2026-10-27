// Package images checks an uploaded photo and makes the copies that the site shows: a full image and a thumbnail,
// both re-encoded as JPEG, so no metadata of the original (GPS, camera, editing history) survives.
package images

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

const (
	MaxSize      = 10 << 20 // the largest photo file, in bytes
	maxSide      = 8192     // the longest side of a photo that the check decodes
	maxPixels    = 50_000_000
	fullSide     = 2048 // the long side of the full image
	thumbSide    = 480  // the long side of the thumbnail
	fullQuality  = 85
	thumbQuality = 80
)

// Photo is a checked photo: the full image and the thumbnail as JPEG, and the full image's size after rotation.
type Photo struct {
	Full, Thumb   []byte
	Width, Height int
}

var (
	errType      = errors.New("the file is not a JPEG or PNG photo")
	errOtherType = errors.New("this photo format is not supported yet: save it as JPEG or PNG")
	errBroken    = errors.New("the photo could not be read")
	errSide      = errors.New("a photo can have at most 8192 pixels on a side")
	errPixels    = errors.New("a photo can have at most 50 megapixels")
)

// Process checks a photo by its content, reads its size before it decodes it, applies the EXIF orientation, and
// returns the re-encoded full image and thumbnail. The upload check calls it; the error text is for the uploader.
func Process(data []byte) (*Photo, error) {
	isJPEG := bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF})
	isPNG := bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	switch {
	case isJPEG, isPNG:
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP",
		len(data) >= 12 && string(data[4:8]) == "ftyp": // HEIC, HEIF and AVIF
		return nil, errOtherType
	default:
		return nil, errType
	}

	// The size first: decoding allocates for every pixel, so a small file with a huge size must not get that far.
	var cfg image.Config
	var err error
	if isJPEG {
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	} else {
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	}
	switch {
	case err != nil:
		return nil, errBroken
	case cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSide || cfg.Height > maxSide:
		return nil, errSide
	case cfg.Width*cfg.Height > maxPixels:
		return nil, errPixels
	}

	var src image.Image
	if isJPEG {
		src, err = jpeg.Decode(bytes.NewReader(data))
	} else {
		src, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, errBroken
	}
	o := 1
	if isJPEG {
		o = orientation(data)
	}
	// The rotation runs on the small copy, after the downscale, so it never copies the large photo.
	full := orient(downscale(src, fullSide), o)
	thumb := downscale(full, thumbSide)

	p := &Photo{Width: full.Rect.Dx(), Height: full.Rect.Dy()}
	if p.Full, err = encode(full, fullQuality); err != nil {
		return nil, err
	}
	if p.Thumb, err = encode(thumb, thumbQuality); err != nil {
		return nil, err
	}
	return p, nil
}

// encode writes an image as JPEG. Process calls it for both copies.
func encode(img image.Image, quality int) ([]byte, error) {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// downscale gives a copy of src whose long side is at most side, with every output pixel the mean of the source
// pixels under it (a box filter). It never scales up. Transparent pixels are put on white. Process calls it.
func downscale(src image.Image, side int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	ow, oh := w, h
	if w >= h && w > side {
		ow, oh = side, max(1, (h*side+w/2)/w)
	} else if h > w && h > side {
		ow, oh = max(1, (w*side+h/2)/h), side
	}
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	px := pixelReader(src)
	for oy := range oh {
		y0, y1 := oy*h/oh, max((oy+1)*h/oh, oy*h/oh+1)
		for ox := range ow {
			x0, x1 := ox*w/ow, max((ox+1)*w/ow, ox*w/ow+1)
			var r, g, bl, n uint64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					pr, pg, pb := px(b.Min.X+x, b.Min.Y+y)
					r, g, bl = r+uint64(pr), g+uint64(pg), bl+uint64(pb)
					n++
				}
			}
			i := out.PixOffset(ox, oy)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = uint8(r/n), uint8(g/n), uint8(bl/n), 255
		}
	}
	return out
}

// pixelReader gives a function that reads one pixel of img as 8-bit RGB on white. It reads the common types of the
// two decoders directly, because the generic At path allocates for every pixel. downscale uses it.
func pixelReader(img image.Image) func(x, y int) (uint8, uint8, uint8) {
	switch m := img.(type) {
	case *image.YCbCr:
		return func(x, y int) (uint8, uint8, uint8) {
			yi, ci := m.YOffset(x, y), m.COffset(x, y)
			return color.YCbCrToRGB(m.Y[yi], m.Cb[ci], m.Cr[ci])
		}
	case *image.RGBA: // premultiplied: on white, each channel gains 255 minus the alpha
		return func(x, y int) (uint8, uint8, uint8) {
			i := m.PixOffset(x, y)
			add := 255 - m.Pix[i+3]
			return m.Pix[i] + add, m.Pix[i+1] + add, m.Pix[i+2] + add
		}
	case *image.NRGBA:
		return func(x, y int) (uint8, uint8, uint8) {
			i := m.PixOffset(x, y)
			a := uint32(m.Pix[i+3])
			on := func(c uint8) uint8 { return uint8((uint32(c)*a + 255*(255-a) + 127) / 255) }
			return on(m.Pix[i]), on(m.Pix[i+1]), on(m.Pix[i+2])
		}
	}
	return func(x, y int) (uint8, uint8, uint8) {
		r, g, b, a := img.At(x, y).RGBA() // premultiplied, 16-bit
		return uint8((r + 0xffff - a) >> 8), uint8((g + 0xffff - a) >> 8), uint8((b + 0xffff - a) >> 8)
	}
}

// orient turns and flips img as the EXIF orientation o (1-8) says, so the photo shows upright. Process calls it on
// the downscaled copy.
func orient(img *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return img
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	ow, oh := w, h
	if o >= 5 {
		ow, oh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := range oh {
		for x := range ow {
			// (sx, sy) is the source pixel that lands on (x, y).
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			copy(out.Pix[out.PixOffset(x, y):][:4], img.Pix[img.PixOffset(sx, sy):][:4])
		}
	}
	return out
}

// orientation reads the EXIF orientation tag of a JPEG, or gives 1 (upright) when there is none or the block is
// broken. It reads only the segments before the image data. Process calls it.
func orientation(jpg []byte) int {
	i := 2 // after SOI
	for i+4 <= len(jpg) && jpg[i] == 0xFF {
		marker := jpg[i+1]
		if marker == 0xDA || marker == 0xD9 { // the image data or the end: no Exif block came first
			return 1
		}
		n := int(binary.BigEndian.Uint16(jpg[i+2:]))
		if n < 2 || i+2+n > len(jpg) {
			return 1
		}
		seg := jpg[i+4 : i+2+n]
		if marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i += 2 + n
	}
	return 1
}

// tiffOrientation finds tag 0x0112 in the first IFD of a TIFF block, or gives 1. orientation calls it.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[ifd:]))
	for k := range count {
		e := ifd + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 && order.Uint16(t[e+2:]) == 3 {
			if o := int(order.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}
