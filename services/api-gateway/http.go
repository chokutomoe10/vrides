package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"vrides/services/api-gateway/grpc_clients"
	"vrides/shared/contracts"
	"vrides/shared/env"
	"vrides/shared/messaging"
	"vrides/shared/tracing"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

var tracer = tracing.GetTracer("api-gateway")

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handleTripPreviewSpan")
	defer span.End()

	var req previewTripReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Fatal(err)
		return
	}

	defer tripService.Close()

	tripRes, err := tripService.Client.PreviewTrip(ctx, req.toProto())
	if err != nil {
		log.Printf("Failed to preview a trip: %v", err)
		http.Error(w, "Failed to preview trip", http.StatusInternalServerError)
		return
	}

	res := contracts.APIResponse{
		Data: tripRes,
	}

	writeJSON(w, http.StatusCreated, res)
}

func handleTripStart(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "handleTripStartSpan")
	defer span.End()

	var req startTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "failed to parse JSON data", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		log.Fatal(err)
		return
	}

	defer tripService.Close()

	tripRes, err := tripService.Client.CreateTrip(ctx, req.toProto())
	if err != nil {
		log.Printf("Failed to start a trip: %v", err)
		http.Error(w, "Failed to start trip", http.StatusInternalServerError)
		return
	}

	res := contracts.APIResponse{
		Data: tripRes,
	}

	writeJSON(w, http.StatusCreated, res)

}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request, rb *messaging.RabbitMQ) {
	ctx, span := tracer.Start(r.Context(), "handleStripeWebhookSpan")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()

	webhookKey := env.GetString("STRIPE_WEBHOOK_KEY", "")
	if webhookKey == "" {
		log.Printf("Webhook key is required")
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		body, r.Header.Get("Stripe-Signature"), webhookKey, webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)

	if err != nil {
		log.Printf("Error verifying webhook signature: %v", err)
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("Received Stripe event: %v", event)

	switch event.Type {
	case "checkout.session.completed":
		var chSession stripe.CheckoutSession

		if err := json.Unmarshal(event.Data.Raw, &chSession); err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		payload := messaging.PaymentStatusUpdateData{
			TripID:   chSession.Metadata["trip_id"],
			UserID:   chSession.Metadata["user_id"],
			DriverID: chSession.Metadata["driver_id"],
		}

		mSession, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling payload: %v", err)
			http.Error(w, "Failed to marshal payload", http.StatusInternalServerError)
			return
		}

		if err := rb.PublishMessage(ctx, contracts.PaymentEventSuccess, contracts.AmqpMessage{
			OwnerID: chSession.Metadata["user_id"],
			Data:    mSession,
		}); err != nil {
			log.Printf("Error publishing payment event: %v", err)
			http.Error(w, "Failed to publish payment event", http.StatusInternalServerError)
			return
		}
	}

}
