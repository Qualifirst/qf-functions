package main

import (
	"context"
	"log"
	"strings"

	qfn "qf/go/netlify"
	"qf/go/rabbitmq"
	"qf/go/shopify"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	err := shopify.ValidateWebhook(request)
	if err != nil {
		errMsg := "Error! Invalid Shopify webhook"
		log.Printf("%s: %v", errMsg, err)
		return &events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       errMsg,
		}, nil
	}

	err = rabbitmq.PublishMessage(
		"shopify.webhook",
		strings.ReplaceAll(request.Headers["x-shopify-topic"], "/", "."),
		request.Body,
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
	lambda.Start(qfn.CheckEnvMiddleware(handler))
}
