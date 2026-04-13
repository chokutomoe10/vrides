package http

import (
	"encoding/json"
	"log"
	"net/http"
	"vrides/services/trip-service/internal/domain"
	"vrides/shared/contracts"
	"vrides/shared/types"
)

type HttpHandler struct {
	Service domain.TripService
}

type previewTripRequest struct {
	UserID      string           `json:"userID"`
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}

func (s *HttpHandler) HandlePreviewTrip(w http.ResponseWriter, r *http.Request) {
	var req previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	res, err := s.Service.GetRoute(ctx, &req.Pickup, &req.Destination, true)
	if err != nil {
		log.Println(err)
		return
	}

	response := contracts.APIResponse{Data: res}

	writeJSON(w, http.StatusOK, response)
}

func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}
