package main

import (
	"context"
	"log"
	"strings"

	"qf/go/app"
	"qf/go/rabbitmq"
	"qf/go/shopify"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	err := shopify.ValidateWebhook(ctx, request)
	if err != nil {
		errMsg := "Error! Invalid Shopify webhook"
		log.Printf("%s: %v", errMsg, err)
		return &events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       errMsg,
		}, nil
	}

	topic := strings.ReplaceAll(request.Headers["x-shopify-topic"], "/", ".")
	err = rabbitmq.PublishMessage(
		ctx,
		"shopify.webhook",
		topic,
		request.Body,
		amqp.Table{
			"X-Shopify-Topic": topic,
		},
	)
	if err != nil {
		errMsg := "Error! Could not publish message to RabbitMQ"
		log.Printf("%s: %v", errMsg, err)
		return &events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       errMsg,
		}, nil
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "OK",
	}, nil
}

func main() {
	lambda.Start(app.ProfilingMiddleware(
		app.TimeoutMiddleware(app.CacheMiddleware(app.CheckEnvMiddleware(handler))),
		"shopify-webhook",
	))
}
