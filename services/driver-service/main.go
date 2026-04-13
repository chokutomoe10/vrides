package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"vrides/shared/env"
	"vrides/shared/messaging"
	"vrides/shared/tracing"
	"syscall"

	"google.golang.org/grpc"
)

var (
	GrpcAddr = ":9092"
)

func main() {
	sh, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "driver-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "http://jaeger:14268/api/traces"),
	})

	if err != nil {
		log.Fatalf("Failed to initialize the tracer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer sh(ctx)

	rabbitMqURI := env.GetString("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/")

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	svc := NewService()

	rabbitmq, err := messaging.NewRabbitMQ(rabbitMqURI)
	if err != nil {
		log.Fatal(err)
	}

	defer rabbitmq.Close()

	log.Println("Starting RabbitMQ connection")

	srv := grpc.NewServer(tracing.WithTracingInterceptors()...)
	NewGRPCHandler(srv, svc)

	listener, err := net.Listen("tcp", GrpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	consumer := NewTripConsumer(rabbitmq, svc)
	go func() {
		if err := consumer.Listen(); err != nil {
			log.Fatalf("Failed to listen to the message: %v", err)
		}
	}()

	log.Printf("Starting gRPC server Driver service on port %s", listener.Addr().String())

	go func() {
		if err := srv.Serve(listener); err != nil {
			log.Printf("failed to serve: %v", err)
		}
		cancel()
	}()

	<-ctx.Done()
	log.Println("Shutting down the server...")
	srv.GracefulStop()
}
