package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"vrides/services/trip-service/internal/infrastructure/events"
	"vrides/services/trip-service/internal/infrastructure/grpc_handler"
	"vrides/services/trip-service/internal/infrastructure/repository"
	"vrides/services/trip-service/internal/service"
	"vrides/shared/db"
	"vrides/shared/env"
	"vrides/shared/messaging"
	"vrides/shared/tracing"

	"google.golang.org/grpc"
)

var (
	GrpcAddr = ":9093"
)

func main() {
	sh, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "trip-service",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "http://jaeger:14268/api/traces"),
	})

	if err != nil {
		log.Fatalf("Failed to initialize the tracer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	defer sh(ctx)

	client, err := db.NewMongoClient(ctx, db.NewMongoDefaultConfig())
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB, err: %v", err)
	}

	defer client.Disconnect(ctx)

	mongodb := db.GetDatabase(client, db.NewMongoDefaultConfig())

	log.Println(mongodb.Name())

	rabbitMqURI := env.GetString("RABBITMQ_URI", "amqp://guest:guest@rabbitmq:5672/")
	mr := repository.NewMongoRepository(mongodb)
	svc := service.NewService(mr)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	srv := grpc.NewServer(tracing.WithTracingInterceptors()...)

	rabbitmq, err := messaging.NewRabbitMQ(rabbitMqURI)
	if err != nil {
		log.Fatal(err)
	}

	defer rabbitmq.Close()

	log.Println("Starting RabbitMQ connection")

	publisher := events.NewTripEventPublisher(rabbitmq)

	consumer := events.NewDriverConsumer(rabbitmq, svc)
	go consumer.Listen()

	paymentConsumer := events.NewPaymentConsumer(rabbitmq, svc)
	go paymentConsumer.Listen()

	grpc_handler.NewGRPCHandler(svc, srv, publisher)

	listener, err := net.Listen("tcp", GrpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("Starting gRPC server Trip service on port %s", listener.Addr().String())

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
