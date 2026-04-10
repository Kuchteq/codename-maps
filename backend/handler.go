package main

import (
	"context"
	"fmt"
	"os"

	"github.com/paulmach/orb/maptile"
	"kuchta.dev/codename-maps-edit-service/api"
	"kuchta.dev/codename-maps-edit-service/data"
)

// Handler implements api.Handler using sqlc-generated queries.
type Handler struct {
	api.UnimplementedHandler
	q            *data.Queries
	generatedDir string
	tilesDir     string
}

func NewHandler(q *data.Queries) *Handler {
	generatedDir := "generated"
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create generated dir: %v", err))
	}

	tilesDir := os.Getenv("TILES_DIR")
	if tilesDir == "" {
		tilesDir = "tileservice/data/tiles"
	}
	if err := os.MkdirAll(tilesDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create tiles dir: %v", err))
	}

	return &Handler{q: q, generatedDir: generatedDir, tilesDir: tilesDir}
}

// CreateEdit implements POST /v1/edit.
func (h *Handler) CreateEdit(ctx context.Context, req *api.EditRequest) (api.CreateEditRes, error) {
	startCoords := req.Start.GetCoordinates()
	if len(startCoords) < 2 {
		return &api.CreateEditBadRequest{}, nil
	}

	endCoords := req.End.GetCoordinates()
	if len(endCoords) < 2 {
		return &api.CreateEditBadRequest{}, nil
	}

	startLng := startCoords[0]
	startLat := startCoords[1]
	endLng := endCoords[0]
	endLat := endCoords[1]

	// 1. Generate noise PNG.
	imagePath, err := generateNoisePNG(h.generatedDir, startLng, startLat, endLng, endLat)
	if err != nil {
		return nil, fmt.Errorf("generate noise png: %w", err)
	}

	// 2. Fetch WMS tiles for the bounding box and dim them.
	if err := h.generateTilesFromWMS(startLng, startLat, endLng, endLat); err != nil {
		return nil, fmt.Errorf("generate wms tiles: %w", err)
	}

	// 3. Persist the edit record.
	_, err = h.q.CreateEdit(ctx, data.CreateEditParams{
		Name:      req.GetName(),
		Author:    req.GetAuthor(),
		Prompt:    req.GetPrompt(),
		StartLng:  startLng,
		StartLat:  startLat,
		EndLng:    endLng,
		EndLat:    endLat,
		ImagePath: imagePath,
	})
	if err != nil {
		return nil, fmt.Errorf("create edit: %w", err)
	}

	return &api.CreateEditAccepted{}, nil
}

// generateTilesFromWMS fetches XYZ tiles (zoom 0–14) covering the given WGS84
// bounding box from the Helsinki WMS, dims them by 50%, and writes them into
// h.tilesDir.
func (h *Handler) generateTilesFromWMS(startLng, startLat, endLng, endLat float64) error {
	west := min(startLng, endLng)
	east := max(startLng, endLng)
	south := min(startLat, endLat)
	north := max(startLat, endLat)

	return GenerateTiles(west, east, south, north, h.tilesDir, maptile.Zoom(0), maptile.Zoom(14))
}
