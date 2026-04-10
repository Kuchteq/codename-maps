package main

import (
	"bufio"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
)

const (
	tileSize = 256
	wmsBase  = "https://kartta.hel.fi/ws/geoserver/avoindata/wms"
	wmsLayer = "avoindata:Ortoilmakuva_2025_5cm"
	earthR   = 6378137.0 // WGS84 semi-major axis, metres
)

var wmsClient = &http.Client{
	Timeout: 90 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   32,
		MaxConnsPerHost:       16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ForceAttemptHTTP2:     true,
	},
}

// GenerateTilesFromImage writes the edited 1024x1024 image into transparent XYZ
// overlay tiles covering the selected WGS84 bounding box.
func GenerateTilesFromImage(
	edited image.Image,
	west float64,
	east float64,
	south float64,
	north float64,
	outDir string,
	minZoom maptile.Zoom,
	maxZoom maptile.Zoom,
) error {
	for z := minZoom; z <= maxZoom; z++ {
		swTile := maptile.At(orb.Point{west, south}, z)
		neTile := maptile.At(orb.Point{east, north}, z)

		// Y axis is flipped: north -> smaller Y index.
		minX, maxX := swTile.X, neTile.X
		minY, maxY := neTile.Y, swTile.Y

		for tx := minX; tx <= maxX; tx++ {
			for ty := minY; ty <= maxY; ty++ {
				tile := maptile.New(tx, ty, z)
				result := renderEditedImageTile(edited, tile, west, east, south, north)

				dir := filepath.Join(outDir, fmt.Sprintf("%d/%d", z, tx))
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("mkdir tile dir: %w", err)
				}

				path := filepath.Join(dir, fmt.Sprintf("%d.png", ty))
				f, err := os.Create(path)
				if err != nil {
					return fmt.Errorf("create tile file: %w", err)
				}

				bw := bufio.NewWriterSize(f, 32*1024)
				enc := &png.Encoder{CompressionLevel: png.BestSpeed}
				if err := enc.Encode(bw, result); err != nil {
					f.Close()
					return fmt.Errorf("encode tile png: %w", err)
				}
				if err := bw.Flush(); err != nil {
					f.Close()
					return fmt.Errorf("flush tile png: %w", err)
				}
				if err := f.Close(); err != nil {
					return fmt.Errorf("close tile png: %w", err)
				}
			}
		}
	}
	return nil
}

// lngLatTo3857 converts WGS84 longitude/latitude (degrees) to EPSG:3857
// (Web Mercator) x/y in metres.
func lngLatTo3857(lng, lat float64) (x, y float64) {
	x = lng * math.Pi / 180.0 * earthR
	y = math.Log(math.Tan(math.Pi/4.0+lat*math.Pi/360.0)) * earthR
	return
}

// renderEditedImageTile produces a transparent 256x256 tile where only pixels
// inside the selected bounds are sampled from the edited 1024x1024 image.
func renderEditedImageTile(
	edited image.Image,
	tile maptile.Tile,
	west float64,
	east float64,
	south float64,
	north float64,
) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, tileSize, tileSize))
	editedNRGBA := toNRGBA(edited)
	editedBounds := editedNRGBA.Bounds()

	bboxMinX, bboxMinY := lngLatTo3857(west, south)
	bboxMaxX, bboxMaxY := lngLatTo3857(east, north)

	tileBounds := tile.Bound()
	tileMinX, tileMinY := lngLatTo3857(tileBounds.Min[0], tileBounds.Min[1])
	tileMaxX, tileMaxY := lngLatTo3857(tileBounds.Max[0], tileBounds.Max[1])

	for y := 0; y < tileSize; y++ {
		pctY := (float64(y) + 0.5) / float64(tileSize)
		mercatorY := tileMaxY - pctY*(tileMaxY-tileMinY)
		if mercatorY < bboxMinY || mercatorY > bboxMaxY {
			continue
		}

		sourceY := int(((bboxMaxY - mercatorY) / (bboxMaxY - bboxMinY)) * float64(editedBounds.Dy()))
		sourceY = clamp(sourceY, editedBounds.Min.Y, editedBounds.Max.Y-1)

		for x := 0; x < tileSize; x++ {
			pctX := (float64(x) + 0.5) / float64(tileSize)
			mercatorX := tileMinX + pctX*(tileMaxX-tileMinX)
			if mercatorX < bboxMinX || mercatorX > bboxMaxX {
				continue
			}

			sourceX := int(((mercatorX - bboxMinX) / (bboxMaxX - bboxMinX)) * float64(editedBounds.Dx()))
			sourceX = clamp(sourceX, editedBounds.Min.X, editedBounds.Max.X-1)

			dst.SetNRGBA(x, y, editedNRGBA.NRGBAAt(sourceX, sourceY))
		}
	}

	return dst
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

// toNRGBA converts any image.Image to *image.NRGBA.
func toNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok {
		return n
	}
	dst := image.NewNRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}
