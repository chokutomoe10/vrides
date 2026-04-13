package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"vrides/services/trip-service/internal/domain"
	t "vrides/services/trip-service/pkg/types"
	"vrides/shared/env"
	pbd "vrides/shared/proto/driver"
	pb "vrides/shared/proto/trip"
	"vrides/shared/types"

	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type service struct {
	repo domain.TripRepository
}

func NewService(repo domain.TripRepository) *service {
	return &service{repo: repo}
}

func (s *service) CreateTrip(ctx context.Context, fare *domain.RideFareModel) (*domain.TripModel, error) {
	t := &domain.TripModel{
		ID:       primitive.NewObjectID(),
		UserID:   fare.UserID,
		Status:   "pending",
		RideFare: fare,
		Driver:   &pb.TripDriver{},
	}

	return s.repo.CreateTrip(ctx, t)
}

func (s *service) GetRoute(ctx context.Context, pickup, destination *types.Coordinate, useOSRMApi bool) (*t.OsrmApiResponse, error) {
	if !useOSRMApi {
		return &t.OsrmApiResponse{
			Routes: []struct {
				Distance float64 `json:"distance"`
				Duration float64 `json:"duration"`
				Geometry struct {
					Coordinates [][]float64 `json:"coordinates"`
				} `json:"geometry"`
			}{
				{
					Distance: 5.0,
					Duration: 600,
					Geometry: struct {
						Coordinates [][]float64 `json:"coordinates"`
					}{
						Coordinates: [][]float64{
							{pickup.Latitude, pickup.Longitude},
							{destination.Latitude, destination.Longitude},
						},
					},
				},
			},
		}, nil
	}

	baseURL := env.GetString("OSRM_API", "http://router.project-osrm.org")

	url := fmt.Sprintf(
		"%s/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		baseURL,
		pickup.Latitude, pickup.Longitude,
		destination.Latitude, destination.Longitude,
	)

	log.Printf("Fetching from OSRM API: URL: %s", url)

	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("")
	}

	log.Printf("GOT RESPONSE FROM API %s", string(body))

	var or t.OsrmApiResponse
	if err := json.Unmarshal(body, &or); err != nil {
		return nil, fmt.Errorf("")
	}

	return &or, nil
}

func (s *service) EstimatePackagesPriceWithRoute(route *t.OsrmApiResponse) []*domain.RideFareModel {
	baseFares := getBaseFares()
	fares := make([]*domain.RideFareModel, len(baseFares))

	for i, f := range baseFares {
		fares[i] = estimateFareRoute(f, route)
	}

	return fares
}

func estimateFareRoute(f *domain.RideFareModel, route *t.OsrmApiResponse) *domain.RideFareModel {
	pc := t.DefaultPricingConfig()
	carPackagePrice := f.TotalPriceInCents

	distance := route.Routes[0].Distance
	duration := route.Routes[0].Duration

	totalDistance := distance * pc.PricePerUnitOfDistance
	totalDuration := duration * pc.PricingPerMinute
	totalPrice := carPackagePrice + totalDistance + totalDuration

	return &domain.RideFareModel{
		PackageSlug:       f.PackageSlug,
		TotalPriceInCents: totalPrice,
	}
}

func (s *service) GenerateTripFares(ctx context.Context, fares []*domain.RideFareModel, route *t.OsrmApiResponse) ([]*domain.RideFareModel, error) {
	tFares := make([]*domain.RideFareModel, len(fares))

	for i, f := range fares {
		fare := &domain.RideFareModel{
			ID:                primitive.NewObjectID(),
			UserID:            uuid.New().String(),
			PackageSlug:       f.PackageSlug,
			TotalPriceInCents: f.TotalPriceInCents,
			Route:             route,
		}

		if err := s.repo.SaveRideFare(ctx, fare); err != nil {
			return nil, fmt.Errorf("failed to save trip fare: %w", err)
		}

		tFares[i] = fare
	}

	return tFares, nil
}

func getBaseFares() []*domain.RideFareModel {
	return []*domain.RideFareModel{
		{
			PackageSlug:       "suv",
			TotalPriceInCents: 200,
		},
		{
			PackageSlug:       "sedan",
			TotalPriceInCents: 350,
		},
		{
			PackageSlug:       "van",
			TotalPriceInCents: 400,
		},
		{
			PackageSlug:       "luxury",
			TotalPriceInCents: 1000,
		},
	}
}

func (s *service) GetAndValidateFare(ctx context.Context, fareID string, userID string) (*domain.RideFareModel, error) {
	fare, err := s.repo.GetRideFareByID(ctx, fareID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trip fare: %w", err)
	}

	if fare == nil {
		return nil, fmt.Errorf("fare does not exist")
	}

	if fare.UserID != userID {
		return nil, fmt.Errorf("fare does not belong to the user")
	}

	return fare, nil
}

func (s *service) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	return s.repo.GetTripByID(ctx, id)
}

func (s *service) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	return s.repo.UpdateTrip(ctx, tripID, status, driver)
}
