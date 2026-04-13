package grpc_handler

import (
	"context"
	"log"
	"vrides/services/trip-service/internal/domain"
	"vrides/services/trip-service/internal/infrastructure/events"
	pb "vrides/shared/proto/trip"
	"vrides/shared/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	pb.UnimplementedTripServiceServer
	service   domain.TripService
	publisher *events.TripEventPublisher
}

func NewGRPCHandler(service domain.TripService, server *grpc.Server, publisher *events.TripEventPublisher) *gRPCHandler {
	handler := &gRPCHandler{
		service:   service,
		publisher: publisher,
	}

	pb.RegisterTripServiceServer(server, handler)
	return handler
}

func (gh *gRPCHandler) PreviewTrip(ctx context.Context, req *pb.PreviewTripReq) (*pb.PreviewTripRes, error) {
	pickup := &types.Coordinate{
		Latitude:  req.GetStartLocation().Latitude,
		Longitude: req.GetStartLocation().Longitude,
	}

	destination := &types.Coordinate{
		Latitude:  req.GetEndLocation().Latitude,
		Longitude: req.GetEndLocation().Longitude,
	}

	r, err := gh.service.GetRoute(ctx, pickup, destination, true)
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "failed to get route: %v", err)
	}

	ef := gh.service.EstimatePackagesPriceWithRoute(r)
	fares, err := gh.service.GenerateTripFares(ctx, ef, r)
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "failed to generate the ride fares: %v", err)
	}

	return &pb.PreviewTripRes{
		Route:     r.ToProto(),
		RideFares: domain.ToRideFaresProto(fares),
	}, nil
}

func (gh *gRPCHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	fare, err := gh.service.GetAndValidateFare(ctx, req.GetRideFareID(), req.GetUserID())
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "failed to validate the fare: %v", err)
	}

	trip, err := gh.service.CreateTrip(ctx, fare)
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "failed to create the trip: %v", err)
	}

	if err := gh.publisher.PublishTrip(ctx, trip); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish the trip created event: %v", err)
	}

	return &pb.CreateTripResponse{
		TripID: trip.ID.Hex(),
	}, nil
}
