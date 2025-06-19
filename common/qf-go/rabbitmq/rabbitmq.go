package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishMessage(exchange, routingKey, body string) error {
	rHost := os.Getenv("RABBITMQ_HOST")
	rUser := os.Getenv("RABBITMQ_USER")
	rPass := os.Getenv("RABBITMQ_PASSWORD")
	if !(rHost != "" && rUser != "" && rPass != "") {
		return fmt.Errorf("invalid or incomplete RabbitMQ environment variables")
	}

	rUrl := fmt.Sprintf("amqp://%s:%s@%s", rUser, rPass, rHost)
	conn, err := amqp.Dial(rUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ:\n>>> %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel to RabbitMQ:\n>>> %w", err)
	}
	defer ch.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "text/plain",
		Body:         []byte(body),
		DeliveryMode: amqp.Persistent,
	})
	if err != nil {
		return fmt.Errorf("failed to publish a message to RabbitMQ:\n>>> %w", err)
	}

	log.Printf("Published message to RabbitMQ: exchange=%s key=%s", exchange, routingKey)

	return nil
}
