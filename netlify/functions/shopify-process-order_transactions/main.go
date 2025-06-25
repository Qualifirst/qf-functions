package main

import (
	"context"
	"encoding/json"
	"fmt"

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

	if data["status"] != "success" {
		// Wait for transaction to succeed before processing
		return app.NetlifyLogAndResponse(200, "OK", nil)
	}

	if _, found := data["order_id"].(float64); !found {
		return app.NetlifyLogAndResponse(400, "Order ID not found in request body", nil)
	}
	if _, found := data["id"].(float64); !found {
		return app.NetlifyLogAndResponse(400, "Transaction ID not found in request body", nil)
	}

	orderId := fmt.Sprintf("gid://shopify/Order/%v", int(data["order_id"].(float64)))
	transactionId := fmt.Sprintf("gid://shopify/OrderTransaction/%v", int(data["id"].(float64)))

	odooId, isNew, err := shopifyodoo.ShopifyTransactionToOdoo(ctx, orderId, transactionId)
	if err != nil {
		return app.NetlifyLogAndResponse(500, "Error processing transaction", err)
	}

	return app.NetlifyLogAndJsonResponse(200, map[string]any{"id": odooId, "new": isNew}, nil)
}

func main() {
	lambda.Start(app.ProfilingMiddleware(
		app.TimeoutMiddleware(app.CacheMiddleware(app.CheckEnvMiddleware(app.AuthMiddleware(odoo.OdooDataMiddleware(handler))))),
		"shopify-process-order_transactions",
	))
}
