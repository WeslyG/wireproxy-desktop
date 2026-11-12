package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"runtime"
	"sync"

	xdraw "golang.org/x/image/draw"
)

var connectedTrayIcon = sync.OnceValue(func() []byte {
	source, err := png.Decode(bytes.NewReader(trayTemplate))
	if err != nil {
		return trayFallbackIcon()
	}
	const size = 32
	mask := image.NewAlpha(image.Rect(0, 0, size, size))
	logoBounds := image.Rect(1, 1, 28, 28)
	if runtime.GOOS == "darwin" {
		logoBounds = mask.Bounds()
	}
	xdraw.ApproxBiLinear.Scale(mask, logoBounds, source, source.Bounds(), draw.Src, nil)

	icon := image.NewNRGBA(mask.Bounds())
	for y := range size {
		for x := range size {
			alpha := mask.AlphaAt(x, y).A
			if runtime.GOOS == "darwin" {
				dx, dy := x-26, y-26
				distance := dx*dx + dy*dy
				switch {
				case distance <= 16:
					icon.SetNRGBA(x, y, color.NRGBA{R: 255, G: 69, B: 58, A: 255})
				case distance > 30 && alpha > 0:
					icon.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: alpha})
				}
				continue
			}
			edge := max(alpha, mask.AlphaAt(x-1, y).A, mask.AlphaAt(x+1, y).A,
				mask.AlphaAt(x, y-1).A, mask.AlphaAt(x, y+1).A)
			if edge > 0 {
				shade := uint8(245 - uint16(alpha)*215/uint16(edge))
				icon.SetNRGBA(x, y, color.NRGBA{R: shade, G: shade, B: shade, A: edge})
			}

			dx, dy := x-24, y-24
			radiusSquared := dx*dx + dy*dy
			if radiusSquared <= 49 {
				icon.SetNRGBA(x, y, color.NRGBA{R: 245, G: 245, B: 245, A: 255})
			}
			if radiusSquared <= 30 {
				icon.SetNRGBA(x, y, color.NRGBA{R: 230, G: 37, B: 48, A: 255})
			}
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, icon); err != nil {
		return trayFallbackIcon()
	}
	if runtime.GOOS != "windows" {
		return encoded.Bytes()
	}
	// A single PNG-compressed image in an ICO container.
	header := []byte{0, 0, 1, 0, 1, 0, size, size, 0, 0, 1, 0, 32, 0, 0, 0, 0, 0, 22, 0, 0, 0}
	binary.LittleEndian.PutUint32(header[14:18], uint32(encoded.Len()))
	return append(header, encoded.Bytes()...)
})

func trayFallbackIcon() []byte {
	if runtime.GOOS == "windows" {
		return trayIdleICO
	}
	return trayIdlePNG
}
