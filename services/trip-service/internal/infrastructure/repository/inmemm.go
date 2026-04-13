package repository

import (
	"context"
	"vrides/services/trip-service/internal/domain"
	pbd "vrides/shared/proto/driver"
	pb "vrides/shared/proto/trip"
)

type inMemmRepository struct {
	trips map[string]*domain.TripModel
	fares map[string]*domain.RideFareModel
}

func NewInMemmRepository() *inMemmRepository {
	return &inMemmRepository{
		trips: make(map[string]*domain.TripModel),
		fares: make(map[string]*domain.RideFareModel),
	}
}

func (mr *inMemmRepository) CreateTrip(ctx context.Context, trip *domain.TripModel) (*domain.TripModel, error) {
	mr.trips[trip.ID.Hex()] = trip
	return trip, nil
}

func (mr *inMemmRepository) SaveRideFare(ctx context.Context, f *domain.RideFareModel) error {
	mr.fares[f.ID.Hex()] = f
	return nil
}

func (mr *inMemmRepository) GetRideFareByID(ctx context.Context, id string) (*domain.RideFareModel, error) {
	fare := mr.fares[id]

	return fare, nil
}

func (mr *inMemmRepository) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	trip := mr.trips[id]

	return trip, nil
}

func (mr *inMemmRepository) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	trip := mr.trips[tripID]

	if trip != nil {
		trip.Status = status
	}

	if driver != nil {
		trip.Driver = &pb.TripDriver{
			Id:             trip.Driver.Id,
			Name:           trip.Driver.Name,
			ProfilePicture: trip.Driver.ProfilePicture,
			CarPlate:       trip.Driver.CarPlate,
		}
	}

	return nil
}
