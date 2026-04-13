package main

import (
	"encoding/json"
	"log"
	"net/http"
	"vrides/services/api-gateway/grpc_clients"
	"vrides/shared/contracts"
	"vrides/shared/messaging"
	"vrides/shared/proto/driver"
)

var (
	connManager = messaging.NewConnectionManager()
)

func handleRidersWebSocket(w http.ResponseWriter, r *http.Request, rb *messaging.RabbitMQ) {
	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("user ID is required")
		return
	}

	connManager.Add(userID, conn)
	defer connManager.Remove(userID)

	queues := []string{
		messaging.NotifyDriverNoDriversFoundQueue,
		messaging.NotifyDriverAssignQueue,
		messaging.NotifyPaymentSessionCreatedQueue,
	}

	for _, q := range queues {
		consumer := messaging.NewQueueConsumer(rb, connManager, q)

		if err := consumer.Start(); err != nil {
			log.Printf("Failed to start consumer for queue: %s: err: %v", q, err)
		}
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		log.Printf("Received message: %s", message)
	}
}

func handleDriversWebSocket(w http.ResponseWriter, r *http.Request, rb *messaging.RabbitMQ) {
	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("user ID is required")
		return
	}

	ps := r.URL.Query().Get("packageSlug")
	if ps == "" {
		log.Println("package slug is required")
		return
	}

	connManager.Add(userID, conn)

	ctx := r.Context()

	driverService, err := grpc_clients.NewDriverServiceClient()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		connManager.Remove(userID)

		driverService.Client.UnregisterDriver(ctx, &driver.RegisterDriverReq{
			DriverID:    userID,
			PackageSlug: ps,
		})

		driverService.Close()

		log.Println("Driver unregistered: ", userID)
	}()

	driverData, err := driverService.Client.RegisterDriver(ctx, &driver.RegisterDriverReq{
		DriverID:    userID,
		PackageSlug: ps,
	})
	if err != nil {
		log.Printf("Error registering driver: %v", err)
		return
	}

	msg := contracts.WSMessage{
		Type: contracts.DriverCmdRegister,
		Data: driverData.Driver,
	}

	if err := connManager.SendMessage(userID, msg); err != nil {
		log.Printf("Error sending message: %v", err)
		return
	}

	queues := []string{
		messaging.DriverCmdTripRequestQueue,
	}

	for _, q := range queues {
		consumer := messaging.NewQueueConsumer(rb, connManager, q)

		if err := consumer.Start(); err != nil {
			log.Printf("Failed to start consumer for queue: %s: err: %v", q, err)
		}
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		log.Printf("Received message: %s", message)

		type driverMsg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		var driver driverMsg

		if err := json.Unmarshal(message, &driver); err != nil {
			log.Printf("Error unmarshaling driver message: %v", err)
			continue
		}

		switch driver.Type {
		case contracts.DriverCmdLocation:
			continue
		case contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline:
			err = rb.PublishMessage(ctx, driver.Type, contracts.AmqpMessage{
				OwnerID: userID,
				Data:    driver.Data,
			})
			if err != nil {
				log.Printf("Error publishing message to RabbitMQ: %v", err)
			}
		default:
			log.Printf("unknown type message: %s", driver.Type)
		}
	}
}
