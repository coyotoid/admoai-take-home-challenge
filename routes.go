package adspots

import (
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) CreateAdSpot(w http.ResponseWriter, req *http.Request) {
	var payload CreatePayload
	d := json.NewDecoder(req.Body)
	d.DisallowUnknownFields()

	if err := d.Decode(&payload); err != nil {
		JSONError(w, map[string]any{
			"what":    "Failed to decode JSON payload",
			"context": err.Error(),
		}, http.StatusBadRequest)
		return
	}

	var validationErrors []string
	if payload.Title == nil {
		validationErrors = append(validationErrors, "Title field cannot be missing")
	}
	if payload.ImageURL == nil {
		validationErrors = append(validationErrors, "Image URL field cannot be missing")
	}
	if payload.Placement == nil {
		validationErrors = append(validationErrors, "Placement field cannot be missing")
	}
	var place Placement
	if err := place.Parse(*payload.Placement); err != nil {
		validationErrors = append(validationErrors, "Invalid value for placement field")
	}

	if len(validationErrors) > 0 {
		JSONError(w, map[string]any{
			"what":    "Failed to validate JSON payload",
			"context": validationErrors,
		}, http.StatusBadRequest)
		return
	}

	adspot := AdSpot{
		ID:         uuid.NewString(),
		Title:      *payload.Title,
		ImageURL:   *payload.ImageURL,
		Placement:  place,
		Status:     StatusActive,
		TTLMinutes: payload.TTLMinutes,
		CreatedAt:  ISO8601(time.Now()),
	}

	if err := gorm.G[AdSpot](s.db).Create(req.Context(), &adspot); err != nil {
		JSONError(w, map[string]any{
			"what":    "Failed to persist ad spot",
			"context": err.Error(),
		}, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	e := json.NewEncoder(w)
	e.Encode(adspot)
}

func (s *Server) GetAdSpot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		JSONError(w, map[string]string{
			"what": "ID parameter is missing",
		}, http.StatusBadRequest)
		return
	}

	spots, err := gorm.G[AdSpot](s.db).Where("id = ?", id).Find(r.Context())
	if err != nil {
		JSONError(w, map[string]string{
			"what":    "Database request failed",
			"context": err.Error(),
		}, http.StatusInternalServerError)
		return
	}

	if len(spots) == 0 {
		JSONError(w, map[string]string{
			"what": "Could not find ad spot with requested ID",
			"id":   id,
		}, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	e := json.NewEncoder(w)
	e.Encode(spots[0])
}

func (s *Server) DeactivateAdSpot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		JSONError(w, map[string]string{
			"what": "ID parameter is missing",
		}, http.StatusBadRequest)
		return
	}

	now := ISO8601(time.Now())
	rows, err := gorm.G[AdSpot](s.db).
		Where("id = ? AND status = ?", id, true).
		Updates(r.Context(), AdSpot{Status: StatusInactive, DeactivatedAt: &now})

	if err != nil {
		JSONError(w, map[string]string{
			"what":    "Failed to execute database update",
			"context": err.Error(),
		}, http.StatusInternalServerError)
		return
	}

	if rows == 0 {
		JSONError(w, map[string]string{
			"what": "Ad spot was not found, or was already inactive",
		}, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Ad spot deactivated successfully",
		"id":      id,
	})
}

func (s *Server) ListAdSpots(w http.ResponseWriter, r *http.Request) {
	placement := r.URL.Query().Get("placement")
	status := r.URL.Query().Get("status")

	query := gorm.G[AdSpot](s.db).Order("created_at desc")

	if placement != "" {
		var p Placement
		err := p.Parse(placement)
		if err != nil {
			JSONError(w, map[string]string{
				"what": "Invalid value for placement field",
			}, http.StatusBadRequest)
			return
		}
		query = query.Where("placement = ?", p)
	}

	if status != "" {
		if status != "active" && status != "inactive" {
			JSONError(w, map[string]string{
				"what": "Invalid value for status field",
			}, http.StatusBadRequest)
			return
		}
		query = query.Where("status = ?", status == "active")
	}

	rows, err := query.Find(r.Context())
	if err != nil {
		JSONError(w, map[string]string{
			"what":    "Failed to execute database query",
			"context": err.Error(),
		}, http.StatusInternalServerError)
		return
	}

	rowsFiltered := slices.DeleteFunc(
		rows,
		func(a AdSpot) bool {
			return status == "active" && a.IsExpired()
		},
	)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	e := json.NewEncoder(w)
	e.Encode(rowsFiltered)
}
