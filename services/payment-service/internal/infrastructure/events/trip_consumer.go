package events

import (
	"context"
	"encoding/json"
	"log"
	"vrides/services/payment-service/internal/domain"
	"vrides/shared/contracts"
	"vrides/shared/messaging"

	"github.com/rabbitmq/amqp091-go"
)

type tripConsumer struct {
	rabbitmq *messaging.RabbitMQ
	service  domain.Service
}

func NewTripConsumer(rabbitmq *messaging.RabbitMQ, service domain.Service) *tripConsumer {
	return &tripConsumer{
		rabbitmq: rabbitmq,
		service:  service,
	}
}

func (c *tripConsumer) Listen() error {
	return c.rabbitmq.ConsumeMessages(messaging.PaymentTripResponseQueue, func(ctx context.Context, msg amqp091.Delivery) error {
		var message contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &message); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return err
		}

		var payload messaging.PaymentTripResponseData
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return err
		}

		log.Printf("trip response received message: %+v", payload)

		switch msg.RoutingKey {
		case contracts.PaymentCmdCreateSession:
			if err := c.handleTripAccepted(ctx, payload); err != nil {
				log.Printf("Failed to handle the trip accept: %v", err)
				return err
			}
		default:
			log.Println("unknown message type")
		}

		return nil
	})
}

func (c *tripConsumer) handleTripAccepted(ctx context.Context, payload messaging.PaymentTripResponseData) error {
	log.Printf("Handling trip accepted by driver: %s", payload.TripID)

	paymentSession, err := c.service.CreatePaymentSession(ctx, payload.TripID, payload.UserID, payload.DriverID, int64(payload.Amount), payload.Currency)
	if err != nil {
		log.Printf("Failed to create payment session: %v", err)
		return err
	}

	log.Printf("Payment session created: %s", paymentSession.StripeSessionID)

	paymentPayload := messaging.PaymentEventSessionCreatedData{
		TripID:    payload.TripID,
		SessionID: paymentSession.StripeSessionID,
		Amount:    float64(paymentSession.Amount) / 100.0,
		Currency:  paymentSession.Currency,
	}

	mPayment, err := json.Marshal(paymentPayload)
	if err != nil {
		log.Printf("Failed to marshal payment session payload: %v", err)
		return err
	}

	err = c.rabbitmq.PublishMessage(ctx, contracts.PaymentEventSessionCreated, contracts.AmqpMessage{
		OwnerID: payload.UserID,
		Data:    mPayment,
	})

	if err != nil {
		log.Printf("Failed to publish payment session created event: %v", err)
		return err
	}

	log.Printf("Published payment session created event for trip: %s", payload.TripID)
	return nil
}
