package messaging

import (
	"encoding/json"
	"log"
	"vrides/shared/contracts"
)

type QueueConsumer struct {
	rb        *RabbitMQ
	cm        *ConnectionManager
	queueName string
}

func NewQueueConsumer(rb *RabbitMQ, cm *ConnectionManager, queueName string) *QueueConsumer {
	return &QueueConsumer{
		rb:        rb,
		cm:        cm,
		queueName: queueName,
	}
}

func (q *QueueConsumer) Start() error {
	msgs, err := q.rb.Channel.Consume(
		q.queueName, // queue
		"",          // consumer
		true,        // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)

	if err != nil {
		return err
	}

	go func() {
		for msg := range msgs {
			var driverEvent contracts.AmqpMessage
			if err := json.Unmarshal(msg.Body, &driverEvent); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				continue
			}

			userID := driverEvent.OwnerID

			var payload any
			if err := json.Unmarshal(driverEvent.Data, &payload); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				continue
			}

			clientMsg := contracts.WSMessage{
				Type: msg.RoutingKey,
				Data: payload,
			}

			err = q.cm.SendMessage(userID, clientMsg)
			if err != nil {
				log.Printf("Failed to send message to user %s: %v", userID, err)
			}
		}
	}()

	return nil
}
