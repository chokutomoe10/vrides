package main

import (
	"vrides/shared/proto/trip"
	"vrides/shared/types"
)

type previewTripReq struct {
	UserID      string           `json:"userID"`
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}

func (pr *previewTripReq) toProto() *trip.PreviewTripReq {
	return &trip.PreviewTripReq{
		UserID: pr.UserID,
		StartLocation: &trip.Coordinate{
			Latitude:  pr.Pickup.Latitude,
			Longitude: pr.Pickup.Longitude,
		},
		EndLocation: &trip.Coordinate{
			Latitude:  pr.Destination.Latitude,
			Longitude: pr.Destination.Longitude,
		},
	}
}

type startTripRequest struct {
	RideFareID string `json:"rideFareID"`
	UserID     string `json:"userID"`
}

func (c *startTripRequest) toProto() *trip.CreateTripRequest {
	return &trip.CreateTripRequest{
		RideFareID: c.RideFareID,
		UserID:     c.UserID,
	}
}
