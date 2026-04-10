package main

import (
	"bufio"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// generateNoisePNG creates a random noise PNG whose dimensions are proportional
// to the absolute difference between start and end coordinates. The file is
// saved as <dir>/<x1>-<y1>_<x2>-<y2>-<unix-nano>.png and the path is returned.
func generateNoisePNG(dir string, startLng, startLat, endLng, endLat float64) (string, error) {
	const baseSize = 256
	const maxSize = 2048

	dLng := math.Abs(endLng - startLng)
	dLat := math.Abs(endLat - startLat)

	// Scale so that a 1-degree delta maps to baseSize pixels, clamped to maxSize.
	width := int(math.Round(dLng * baseSize))
	height := int(math.Round(dLat * baseSize))

	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if width > maxSize {
		width = maxSize
	}
	if height > maxSize {
		height = maxSize
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Write directly into the Pix slice -- avoids the overhead of
	// img.SetRGBA() which performs bounds checking on every call.
	pix := img.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = uint8(rand.Intn(256)) // R
		pix[i+1] = uint8(rand.Intn(256)) // G
		pix[i+2] = uint8(rand.Intn(256)) // B
		pix[i+3] = 255                   // A
	}

	ts := time.Now().UnixNano()
	filename := fmt.Sprintf("%.6f-%.6f_%.6f-%.6f-%d.png",
		startLng, startLat, endLng, endLat, ts)
	path := filepath.Join(dir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create png file: %w", err)
	}
	defer f.Close()

	// Buffer writes and use fast compression -- noise is incompressible anyway,
	// so spending CPU on compression is pure waste.
	bw := bufio.NewWriterSize(f, 32*1024)
	enc := &png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(bw, img); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return "", fmt.Errorf("flush png: %w", err)
	}

	return path, nil
}
