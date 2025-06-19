package main

import (
	"context"
	"encoding/json"

	qfn "qf/go/netlify"
	"qf/go/shopifyodoo"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	var data map[string]any
	err := json.Unmarshal([]byte(request.Body), &data)
	if err != nil {
		return qfn.NetlifyLogAndResponse(400, "Invalid JSON in request body", err)
	}

	orderId, ok := data["admin_graphql_api_id"]
	if !ok {
		return qfn.NetlifyLogAndResponse(400, "Order Admin API ID not in request body", nil)
	}

	odooId, isNew, err := shopifyodoo.ShopifyOrderToOdoo(orderId.(string))
	if err != nil {
		return qfn.NetlifyLogAndResponse(500, "Error processing order", err)
	}

	return qfn.NetlifyLogAndJsonResponse(200, map[string]any{"id": odooId, "new": isNew}, nil)
}

func main() {
	lambda.Start(qfn.AuthMiddleware(handler))
}
