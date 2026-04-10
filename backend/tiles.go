package main

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
)

const (
	tileSize = 256
	wmsBase  = "https://kartta.hel.fi/ws/geoserver/avoindata/wms"
	wmsLayer = "avoindata:Ortoilmakuva_2025_5cm"
	earthR   = 6378137.0 // WGS84 semi-major axis, metres

	// maxConcurrentFetches limits the number of parallel WMS HTTP requests
	// to avoid overwhelming the upstream server while still getting massive
	// speedup over sequential fetching.
	maxConcurrentFetches = 16
)

// wmsClient is tuned for high-throughput tile fetching: aggressive connection
// pooling, keep-alive, and sensible timeouts.
var wmsClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   32, // default is 2 -- way too low
		MaxConnsPerHost:       maxConcurrentFetches,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ForceAttemptHTTP2:     true, // multiplex requests on a single TCP conn
	},
}

// tileJob is a unit of work for the worker pool.
type tileJob struct {
	tile   maptile.Tile
	west   float64
	east   float64
	south  float64
	north  float64
	outDir string
}

// GenerateTiles fetches XYZ tiles for zoom levels minZoom..maxZoom that cover
// the given WGS84 bounding box from the Helsinki WMS service, colour-inverts
// only the pixels that fall within the bbox, and writes them to
// outDir/<z>/<x>/<y>.png.
//
// Tiles are fetched concurrently using a bounded worker pool for dramatically
// faster processing compared to sequential fetching.
func GenerateTiles(ctx context.Context, west, east, south, north float64, outDir string, minZoom, maxZoom maptile.Zoom) error {
	// Collect all tile jobs up-front so we know the total work.
	var jobs []tileJob
	for z := minZoom; z <= maxZoom; z++ {
		swTile := maptile.At(orb.Point{west, south}, z)
		neTile := maptile.At(orb.Point{east, north}, z)

		minX, maxX := swTile.X, neTile.X
		minY, maxY := neTile.Y, swTile.Y

		for tx := minX; tx <= maxX; tx++ {
			for ty := minY; ty <= maxY; ty++ {
				jobs = append(jobs, tileJob{
					tile:   maptile.New(tx, ty, z),
					west:   west,
					east:   east,
					south:  south,
					north:  north,
					outDir: outDir,
				})
			}
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	// Pre-create all directories so workers don't race on MkdirAll.
	dirSet := make(map[string]struct{}, len(jobs))
	for _, j := range jobs {
		dir := filepath.Join(j.outDir, fmt.Sprintf("%d/%d", j.tile.Z, j.tile.X))
		dirSet[dir] = struct{}{}
	}
	for dir := range dirSet {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir tile dir: %w", err)
		}
	}

	// Fan out work to a bounded pool of goroutines.
	workers := maxConcurrentFetches
	if len(jobs) < workers {
		workers = len(jobs)
	}

	jobCh := make(chan tileJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				// Bail out early if another worker already failed or context cancelled.
				select {
				case <-ctx.Done():
					errOnce.Do(func() { firstErr = ctx.Err() })
					return
				default:
				}
				if firstErr != nil {
					return
				}

				if err := processTile(ctx, j); err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("tile %d/%d/%d: %w", j.tile.Z, j.tile.X, j.tile.Y, err)
					})
					return
				}
			}
		}()
	}

	wg.Wait()
	return firstErr
}

// processTile fetches a single WMS tile, applies the colour-inversion mask,
// and writes the result to disk using a buffered writer to minimise syscalls.
func processTile(ctx context.Context, j tileJob) error {
	img, err := fetchWMSTile(ctx, j.tile)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	result := flipBBoxRegion(img, j.tile, j.west, j.east, j.south, j.north)

	path := filepath.Join(j.outDir, fmt.Sprintf("%d/%d/%d.png", j.tile.Z, j.tile.X, j.tile.Y))
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	bw := bufio.NewWriterSize(f, 32*1024) // 32 KiB write buffer

	enc := &png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(bw, result); err != nil {
		f.Close()
		return fmt.Errorf("encode png: %w", err)
	}

	if err := bw.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("flush writer: %w", err)
	}

	return f.Close()
}

// fetchWMSTile requests a 256x256 PNG from the Helsinki WMS for the given tile.
// The tile's WGS84 bounds are projected to EPSG:3857 for the BBOX parameter.
// The request is bound to the given context for cancellation support.
func fetchWMSTile(ctx context.Context, tile maptile.Tile) (image.Image, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := wmsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wms returned status %d", resp.StatusCode)
	}

	img, err := png.Decode(resp.Body)
	if err != nil {
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

// flipBBoxRegion produces a transparent 256x256 tile where only the pixels
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

	// Process the region using direct Pix slice access instead of per-pixel
	// method calls. Each pixel is 4 bytes (R, G, B, A) in the Pix slice.
	srcStride := srcNRGBA.Stride
	dstStride := dst.Stride
	srcPix := srcNRGBA.Pix
	dstPix := dst.Pix

	for y := y0; y < y1; y++ {
		srcRowOff := (y-srcNRGBA.Rect.Min.Y)*srcStride + (x0-srcNRGBA.Rect.Min.X)*4
		dstRowOff := (y-dst.Rect.Min.Y)*dstStride + (x0-dst.Rect.Min.X)*4
		for x := x0; x < x1; x++ {
			dstPix[dstRowOff+0] = 255 - srcPix[srcRowOff+0] // R
			dstPix[dstRowOff+1] = 255 - srcPix[srcRowOff+1] // G
			dstPix[dstRowOff+2] = 255 - srcPix[srcRowOff+2] // B
			dstPix[dstRowOff+3] = srcPix[srcRowOff+3]       // A
			srcRowOff += 4
			dstRowOff += 4
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
