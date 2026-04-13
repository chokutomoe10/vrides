package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"vrides/services/trip-service/internal/domain"
	"vrides/shared/contracts"
	"vrides/shared/messaging"
	pbd "vrides/shared/proto/driver"

	"github.com/rabbitmq/amqp091-go"
)

type driverConsumer struct {
	rabbitmq *messaging.RabbitMQ
	service  domain.TripService
}

func NewDriverConsumer(rabbitmq *messaging.RabbitMQ, service domain.TripService) *driverConsumer {
	return &driverConsumer{
		rabbitmq: rabbitmq,
		service:  service,
	}
}

func (c *driverConsumer) Listen() error {
	return c.rabbitmq.ConsumeMessages(messaging.DriverTripResponseQueue, func(ctx context.Context, msg amqp091.Delivery) error {
		var message contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &message); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return err
		}

		var payload messaging.DriverTripResponseData
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return err
		}

		switch msg.RoutingKey {
		case contracts.DriverCmdTripAccept:
			if err := c.handleTripAccepted(ctx, payload.TripID, payload.Driver); err != nil {
				log.Printf("Failed to handle the trip accept: %v", err)
				return err
			}
		case contracts.DriverCmdTripDecline:
			if err := c.handleTripDeclined(ctx, payload.TripID, payload.RiderID); err != nil {
				log.Printf("Failed to handle the trip decline: %v", err)
				return err
			}
		}

		return nil
	})
}

func (c *driverConsumer) handleTripAccepted(ctx context.Context, tripID string, driver *pbd.Driver) error {
	trip, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	if trip == nil {
		return fmt.Errorf("Trip was not found %s", tripID)
	}

	if err := c.service.UpdateTrip(ctx, tripID, "accepted", driver); err != nil {
		log.Printf("Failed to update the trip: %v", err)
		return err
	}

	tripData, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	mTrip, err := json.Marshal(tripData)
	if err != nil {
		return err
	}

	err = c.rabbitmq.PublishMessage(ctx, contracts.TripEventDriverAssigned, contracts.AmqpMessage{
		OwnerID: tripData.UserID,
		Data:    mTrip,
	})

	if err != nil {
		return err
	}

	mPayment, err := json.Marshal(messaging.PaymentTripResponseData{
		TripID:   tripID,
		DriverID: driver.Id,
		UserID:   tripData.UserID,
		Amount:   tripData.RideFare.TotalPriceInCents,
		Currency: "USD",
	})

	err = c.rabbitmq.PublishMessage(ctx, contracts.PaymentCmdCreateSession, contracts.AmqpMessage{
		OwnerID: tripData.UserID,
		Data:    mPayment,
	})

	if err != nil {
		return err
	}

	return nil
}

func (c *driverConsumer) handleTripDeclined(ctx context.Context, tripID string, riderID string) error {
	trip, err := c.service.GetTripByID(ctx, tripID)
	if err != nil {
		return err
	}

	newPayload := messaging.TripEventData{
		Trip: trip.ToTripProto(),
	}

	mPayload, err := json.Marshal(newPayload)
	if err != nil {
		return err
	}

	err = c.rabbitmq.PublishMessage(ctx, contracts.TripEventDriverNotInterested, contracts.AmqpMessage{
		OwnerID: riderID,
		Data:    mPayload,
	})

	if err != nil {
		return err
	}

	return nil
}
