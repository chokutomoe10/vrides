package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vrides/services/payment-service/internal/infrastructure/events"
	"vrides/services/payment-service/internal/infrastructure/repository"
	"vrides/services/payment-service/internal/service"
	"vrides/services/payment-service/pkg/types"
	"vrides/shared/env"
	"vrides/shared/messaging"
	"vrides/shared/tracing"
)

var GrpcAddr = ":9004"

func main() {
	sh, err := tracing.InitTracer(tracing.Config{
		ServiceName:    "payment-service",
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

	stripeCfg := &types.PaymentConfig{
		StripeSecretKey: env.GetString("STRIPE_SECRET_KEY", ""),
	}

	if stripeCfg.StripeSecretKey == "" {
		log.Fatalf("STRIPE_SECRET_KEY is not set")
		return
	}

	client := repository.NewStripeClient(stripeCfg)
	svc := service.NewPaymentService(client)

	rabbitmq, err := messaging.NewRabbitMQ(rabbitMqURI)
	if err != nil {
		log.Fatal(err)
	}
	defer rabbitmq.Close()

	log.Println("Starting RabbitMQ connection")

	consumer := events.NewTripConsumer(rabbitmq, svc)
	go consumer.Listen()

	<-ctx.Done()
	log.Println("Shutting down payment service...")
}
