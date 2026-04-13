package domain

import (
	"context"
	t "vrides/services/trip-service/pkg/types"
	pbd "vrides/shared/proto/driver"
	pb "vrides/shared/proto/trip"
	"vrides/shared/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RideFareModel struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	UserID            string             `bson:"userID"`
	PackageSlug       string             `bson:"packageSlug"`
	TotalPriceInCents float64            `bson:"totalPriceInCents"`
	Route             *t.OsrmApiResponse `bson:"route"`
}

type TripModel struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	UserID   string             `bson:"userID"`
	Status   string             `bson:"status"`
	RideFare *RideFareModel     `bson:"rideFare"`
	Driver   *pb.TripDriver     `bson:"driver"`
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	SaveRideFare(ctx context.Context, f *RideFareModel) error
	GetRideFareByID(ctx context.Context, id string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *types.Coordinate, useOSRMApi bool) (*t.OsrmApiResponse, error)
	EstimatePackagesPriceWithRoute(route *t.OsrmApiResponse) []*RideFareModel
	GenerateTripFares(ctx context.Context, fares []*RideFareModel, route *t.OsrmApiResponse) ([]*RideFareModel, error)
	GetAndValidateFare(ctx context.Context, fareID string, userID string) (*RideFareModel, error)
	GetTripByID(ctx context.Context, id string) (*TripModel, error)
	UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error
}

func (r *TripModel) ToTripProto() *pb.Trip {
	return &pb.Trip{
		Id:           r.ID.Hex(),
		SelectedFare: r.RideFare.ToProto(),
		Route:        r.RideFare.Route.ToProto(),
		Status:       r.Status,
		UserID:       r.UserID,
		Driver:       r.Driver,
	}
}

func (r *RideFareModel) ToProto() *pb.RideFare {
	return &pb.RideFare{
		Id:                r.ID.Hex(),
		UserID:            r.UserID,
		PackageSlug:       r.PackageSlug,
		TotalPriceInCents: r.TotalPriceInCents,
	}
}

func ToRideFaresProto(fares []*RideFareModel) []*pb.RideFare {
	pFares := make([]*pb.RideFare, len(fares))
	for _, f := range fares {
		pFares = append(pFares, f.ToProto())
	}

	return pFares
}
