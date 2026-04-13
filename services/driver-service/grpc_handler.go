package main

import (
	"context"
	pb "vrides/shared/proto/driver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	pb.UnimplementedDriverServiceServer
	service *Service
}

func NewGRPCHandler(srv *grpc.Server, svc *Service) {
	handler := &gRPCHandler{
		service: svc,
	}

	pb.RegisterDriverServiceServer(srv, handler)
}

func (gh *gRPCHandler) RegisterDriver(ctx context.Context, req *pb.RegisterDriverReq) (*pb.RegisterDriverRes, error) {
	d, err := gh.service.RegisterDriver(req.GetDriverID(), req.GetPackageSlug())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register driver")
	}

	return &pb.RegisterDriverRes{
		Driver: d,
	}, nil
}

func (gh *gRPCHandler) UnregisterDriver(ctx context.Context, req *pb.RegisterDriverReq) (*pb.RegisterDriverRes, error) {
	gh.service.UnregisterDriver(req.GetDriverID())

	return &pb.RegisterDriverRes{
		Driver: &pb.Driver{
			Id: req.GetDriverID(),
		},
	}, nil
}
