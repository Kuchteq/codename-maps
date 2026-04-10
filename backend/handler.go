package main

import (
	"context"
	"fmt"

	"kuchta.dev/codename-maps-edit-service/api"
	"kuchta.dev/codename-maps-edit-service/data"
)

// Handler implements api.Handler using sqlc-generated queries.
type Handler struct {
	api.UnimplementedHandler
	q *data.Queries
}

func NewHandler(q *data.Queries) *Handler {
	return &Handler{q: q}
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

	_, err := h.q.CreateEdit(ctx, data.CreateEditParams{
		Name:     req.GetName(),
		Author:   req.GetAuthor(),
		Prompt:   req.GetPrompt(),
		StartLng: startCoords[0],
		StartLat: startCoords[1],
		EndLng:   endCoords[0],
		EndLat:   endCoords[1],
	})
	if err != nil {
		return nil, fmt.Errorf("create edit: %w", err)
	}

	return &api.CreateEditAccepted{}, nil
}
