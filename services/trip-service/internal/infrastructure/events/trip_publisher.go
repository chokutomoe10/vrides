package events

import (
	"context"
	"encoding/json"
	"vrides/services/trip-service/internal/domain"
	"vrides/shared/contracts"
	"vrides/shared/messaging"
)

type TripEventPublisher struct {
	rabbitmq *messaging.RabbitMQ
}

func NewTripEventPublisher(rabbitmq *messaging.RabbitMQ) *TripEventPublisher {
	return &TripEventPublisher{rabbitmq: rabbitmq}
}

func (p *TripEventPublisher) PublishTrip(ctx context.Context, trip *domain.TripModel) error {
	payload := messaging.TripEventData{
		Trip: trip.ToTripProto(),
	}

	tripEventJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	tripMsg := contracts.AmqpMessage{
		OwnerID: trip.UserID,
		Data:    tripEventJSON,
	}

	return p.rabbitmq.PublishMessage(ctx, contracts.TripEventCreated, tripMsg)
}
