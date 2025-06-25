package main

import (
	"context"
	"encoding/json"

	"qf/go/app"
	"qf/go/odoo"
	"qf/go/shopifyodoo"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	var data map[string]any
	err := json.Unmarshal([]byte(request.Body), &data)
	if err != nil {
		return app.NetlifyLogAndResponse(400, "Invalid JSON in request body", err)
	}

	orderId, ok := data["admin_graphql_api_id"]
	if !ok {
		return app.NetlifyLogAndResponse(400, "Order Admin API ID not in request body", nil)
	}

	odooId, isNew, err := shopifyodoo.ShopifyOrderToOdoo(ctx, orderId.(string))
	if err != nil {
		return app.NetlifyLogAndResponse(500, "Error processing order", err)
	}

	return app.NetlifyLogAndJsonResponse(200, map[string]any{"id": odooId, "new": isNew}, nil)
}

func main() {
	lambda.Start(app.ProfilingMiddleware(
		app.TimeoutMiddleware(app.CacheMiddleware(app.CheckEnvMiddleware(app.AuthMiddleware(odoo.OdooDataMiddleware(handler))))),
		"shopify-process-orders",
	))
}
