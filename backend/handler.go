package main

import (
	"context"
	"fmt"
	"image"
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
	west := min(startLng, endLng)
	east := max(startLng, endLng)
	south := min(startLat, endLat)
	north := max(startLat, endLat)

	// 1. Fetch the selected map crop at 1024x1024 and edit it with Nano Banana.
	imagePath, editedImage, err := generateEditedPNG(h.generatedDir, req.GetPrompt(), west, east, south, north)
	if err != nil {
		return nil, fmt.Errorf("generate edited png: %w", err)
	}

	// 2. Cut the edited image into transparent map overlay tiles.
	if err := h.generateTilesFromImage(editedImage, west, east, south, north); err != nil {
		return nil, fmt.Errorf("generate edited image tiles: %w", err)
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

// ListEdits implements GET /v1/edits.
func (h *Handler) ListEdits(ctx context.Context) ([]api.Edit, error) {
	edits, err := h.q.ListEdits(ctx)
	if err != nil {
		return nil, fmt.Errorf("list edits: %w", err)
	}

	res := make([]api.Edit, 0, len(edits))
	for _, edit := range edits {
		res = append(res, api.Edit{
			ID:     edit.ID,
			Name:   edit.Name,
			Author: edit.Author,
			Prompt: edit.Prompt,
			Start: api.GeoJsonPoint{
				Type:        api.GeoJsonPointTypePoint,
				Coordinates: []float64{edit.StartLng, edit.StartLat},
			},
			End: api.GeoJsonPoint{
				Type:        api.GeoJsonPointTypePoint,
				Coordinates: []float64{edit.EndLng, edit.EndLat},
			},
			CreatedAt: edit.CreatedAt,
			ImagePath: edit.ImagePath,
		})
	}

	return res, nil
}

// generateTilesFromImage writes XYZ tiles (zoom 0-14) covering the given WGS84
// bounding box into h.tilesDir.
func (h *Handler) generateTilesFromImage(
	editedImage image.Image,
	west float64,
	east float64,
	south float64,
	north float64,
) error {
	return GenerateTilesFromImage(editedImage, west, east, south, north, h.tilesDir, maptile.Zoom(0), maptile.Zoom(14))
}
