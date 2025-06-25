package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishMessage(ctx context.Context, exchange, routingKey, body string, headers amqp.Table) error {
	rHost := os.Getenv("RABBITMQ_HOST")
	rUser := os.Getenv("RABBITMQ_USER")
	rPass := os.Getenv("RABBITMQ_PASSWORD")
	if !(rHost != "" && rUser != "" && rPass != "") {
		return fmt.Errorf("invalid or incomplete RabbitMQ environment variables")
	}

	rUrl := fmt.Sprintf("amqp://%s:%s@%s", rUser, rPass, rHost)
	config := amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	conn, err := amqp.DialConfig(rUrl, config)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ:\n>>> %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open a channel to RabbitMQ:\n>>> %w", err)
	}
	defer ch.Close()

	allHeaders := amqp.Table{}
	maps.Copy(allHeaders, headers)

	err = ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "text/plain",
		Body:         []byte(body),
		DeliveryMode: amqp.Persistent,
		Headers:      allHeaders,
	})
	if err != nil {
		return fmt.Errorf("failed to publish a message to RabbitMQ:\n>>> %w", err)
	}

	log.Printf("Published message to RabbitMQ: exchange=%s key=%s", exchange, routingKey)

	return nil
}
