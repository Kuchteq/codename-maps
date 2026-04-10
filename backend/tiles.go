package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
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

var wmsClient = &http.Client{Timeout: 30 * time.Second}

// GenerateTiles fetches XYZ tiles for zoom levels minZoom..maxZoom that cover
// the given WGS84 bounding box from the Helsinki WMS service, colour-inverts
// only the pixels that fall within the bbox, and writes them to
// outDir/<z>/<x>/<y>.png.
func GenerateTiles(west, east, south, north float64, outDir string, minZoom, maxZoom maptile.Zoom) error {
	for z := minZoom; z <= maxZoom; z++ {
		swTile := maptile.At(orb.Point{west, south}, z)
		neTile := maptile.At(orb.Point{east, north}, z)

		// Y axis is flipped: north → smaller Y index.
		minX, maxX := swTile.X, neTile.X
		minY, maxY := neTile.Y, swTile.Y

		for tx := minX; tx <= maxX; tx++ {
			for ty := minY; ty <= maxY; ty++ {
				tile := maptile.New(tx, ty, z)

				img, err := fetchWMSTile(tile)
				if err != nil {
					return fmt.Errorf("fetch wms tile %d/%d/%d: %w", z, tx, ty, err)
				}

				result := flipBBoxRegion(img, tile, west, east, south, north)

				dir := filepath.Join(outDir, fmt.Sprintf("%d/%d", z, tx))
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("mkdir tile dir: %w", err)
				}

				path := filepath.Join(dir, fmt.Sprintf("%d.png", ty))
				f, err := os.Create(path)
				if err != nil {
					return fmt.Errorf("create tile file: %w", err)
				}

				if err := png.Encode(f, result); err != nil {
					f.Close()
					return fmt.Errorf("encode tile png: %w", err)
				}
				f.Close()
			}
		}
	}
	return nil
}

// fetchWMSTile requests a 256×256 PNG from the Helsinki WMS for the given tile.
// The tile's WGS84 bounds are projected to EPSG:3857 for the BBOX parameter.
func fetchWMSTile(tile maptile.Tile) (image.Image, error) {
	b := tile.Bound()
	minX, minY := lngLatTo3857(b.Min[0], b.Min[1])
	maxX, maxY := lngLatTo3857(b.Max[0], b.Max[1])

	url := fmt.Sprintf(
		"%s?SERVICE=WMS&VERSION=1.3.0&REQUEST=GetMap"+
			"&LAYERS=%s&STYLES=&FORMAT=image/png&TRANSPARENT=TRUE"+
			"&CRS=EPSG:3857&WIDTH=%d&HEIGHT=%d"+
			"&BBOX=%.6f,%.6f,%.6f,%.6f",
		wmsBase, wmsLayer, tileSize, tileSize,
		minX, minY, maxX, maxY,
	)

	resp, err := wmsClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wms returned status %d", resp.StatusCode)
	}

	img, err := png.Decode(resp.Body)
	if err != nil {
		// Try reading as bytes first in case the body was partially consumed.
		return nil, fmt.Errorf("decode png response: %w", err)
	}

	return img, nil
}

// lngLatTo3857 converts WGS84 longitude/latitude (degrees) to EPSG:3857
// (Web Mercator) x/y in metres.
func lngLatTo3857(lng, lat float64) (x, y float64) {
	x = lng * math.Pi / 180.0 * earthR
	y = math.Log(math.Tan(math.Pi/4.0+lat*math.Pi/360.0)) * earthR
	return
}

// flipBBoxRegion produces a transparent 256×256 tile where only the pixels
// that fall within the WGS84 bounding box [west,east,south,north] are filled
// with the colour-inverted WMS imagery. All other pixels are fully transparent.
func flipBBoxRegion(src image.Image, tile maptile.Tile, west, east, south, north float64) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b) // zero-value = fully transparent

	n := float64(uint32(1) << uint32(tile.Z))

	lngToPixX := func(lng float64) float64 {
		fracWorld := (lng/360.0 + 0.5) * n
		return (fracWorld - float64(tile.X)) * float64(tileSize)
	}
	latToPixY := func(lat float64) float64 {
		sinLat := math.Sin(lat * math.Pi / 180.0)
		fracWorld := (0.5 - math.Log((1+sinLat)/(1-sinLat))/(4*math.Pi)) * n
		return (fracWorld - float64(tile.Y)) * float64(tileSize)
	}

	x0 := int(math.Floor(lngToPixX(west)))
	x1 := int(math.Ceil(lngToPixX(east)))
	y0 := int(math.Floor(latToPixY(north)))
	y1 := int(math.Ceil(latToPixY(south)))

	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}

	srcNRGBA := toNRGBA(src)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := srcNRGBA.NRGBAAt(x, y)
			dst.SetNRGBA(x, y, color.NRGBA{
				R: 255 - c.R,
				G: 255 - c.G,
				B: 255 - c.B,
				A: c.A,
			})
		}
	}

	return dst
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
