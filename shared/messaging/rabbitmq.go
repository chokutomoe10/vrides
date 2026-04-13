package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"vrides/shared/contracts"
	"vrides/shared/tracing"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	TripExchange = "trip"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewRabbitMQ(uri string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create channel: %v", err)
	}

	rmq := &RabbitMQ{
		conn:    conn,
		Channel: ch,
	}

	if err := rmq.setupExchangesAndQueues(); err != nil {
		rmq.Close()
		return nil, fmt.Errorf("failed to setup exchanges and queues: %v", err)
	}

	return rmq, nil
}

type MessageHandler func(context.Context, amqp.Delivery) error

func (r *RabbitMQ) ConsumeMessages(queueName string, handler MessageHandler) error {
	err := r.Channel.Qos(
		1,
		0,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %v", err)
	}

	msgs, err := r.Channel.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			log.Printf("Received a message: %s", msg.Body)

			if err := tracing.TraceConsumer(msg, func(ctx context.Context, d amqp.Delivery) error {
				if err := handler(ctx, msg); err != nil {
					log.Printf("ERROR: Failed to handle message: %v. Message body: %s", err, msg.Body)
					if nackErr := msg.Nack(false, false); nackErr != nil {
						log.Printf("ERROR: Failed to Nack message: %v", nackErr)
					}

					return err
				}

				if err := msg.Ack(false); err != nil {
					log.Printf("ERROR: Failed to Ack message: %v. Message body: %s", err, msg.Body)
				}

				return nil
			}); err != nil {
				log.Printf("Error processing message: %v", err)
			}
		}
	}()

	return nil
}

func (r *RabbitMQ) PublishMessage(ctx context.Context, routingKey string, message contracts.AmqpMessage) error {
	log.Printf("Publishing message with routing key: %s", routingKey)

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	msg := amqp.Publishing{
		ContentType:  "text/plain",
		Body:         jsonMsg,
		DeliveryMode: amqp.Persistent,
	}

	return tracing.TracePublish(ctx, TripExchange, routingKey, msg, r.publish)
}

func (r *RabbitMQ) publish(ctx context.Context, exchange, routingKey string, msg amqp.Publishing) error {
	return r.Channel.PublishWithContext(ctx,
		exchange,
		routingKey,
		false,
		false,
		msg,
	)
}

func (r *RabbitMQ) setupExchangesAndQueues() error {
	err := r.Channel.ExchangeDeclare(
		TripExchange,
		"topic",
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return fmt.Errorf("failed to declare exchange: %s: %v", TripExchange, err)
	}

	err = r.declareAndBindQueue(FindAvailableDriversQueue, []string{
		contracts.TripEventCreated, contracts.TripEventDriverNotInterested,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(DriverCmdTripRequestQueue, []string{
		contracts.DriverCmdTripRequest,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(DriverTripResponseQueue, []string{
		contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(NotifyDriverNoDriversFoundQueue, []string{
		contracts.TripEventNoDriversFound,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(NotifyDriverAssignQueue, []string{
		contracts.TripEventDriverAssigned,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(PaymentTripResponseQueue, []string{
		contracts.PaymentCmdCreateSession,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(NotifyPaymentSessionCreatedQueue, []string{
		contracts.PaymentEventSessionCreated,
	}, TripExchange)

	if err != nil {
		return err
	}

	err = r.declareAndBindQueue(NotifyPaymentSuccessQueue, []string{
		contracts.PaymentEventSuccess,
	}, TripExchange)

	if err != nil {
		return err
	}

	return nil
}

func (r *RabbitMQ) declareAndBindQueue(queueName string, routingKeys []string, exchange string) error {
	q, err := r.Channel.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)

	if err != nil {
		return err
	}

	for _, rk := range routingKeys {
		err = r.Channel.QueueBind(
			q.Name,
			rk,
			exchange,
			false,
			nil,
		)

		if err != nil {
			return fmt.Errorf("failed to bind queue to %s: %v", queueName, err)
		}
	}

	return nil
}

func (r *RabbitMQ) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
	if r.Channel != nil {
		r.Channel.Close()
	}
}
