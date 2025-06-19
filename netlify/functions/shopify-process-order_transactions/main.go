package main

import (
	"context"
	"encoding/json"
	"fmt"

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

	if data["status"] != "success" {
		// Wait for transaction to succeed before processing
		return qfn.NetlifyLogAndResponse(200, "OK", nil)
	}

	if _, found := data["order_id"].(float64); !found {
		return qfn.NetlifyLogAndResponse(400, "Order ID not found in request body", nil)
	}
	if _, found := data["id"].(float64); !found {
		return qfn.NetlifyLogAndResponse(400, "Transaction ID not found in request body", nil)
	}

	orderId := fmt.Sprintf("gid://shopify/Order/%v", int(data["order_id"].(float64)))
	transactionId := fmt.Sprintf("gid://shopify/OrderTransaction/%v", int(data["id"].(float64)))

	odooId, isNew, err := shopifyodoo.ShopifyTransactionToOdoo(orderId, transactionId)
	if err != nil {
		return qfn.NetlifyLogAndResponse(500, "Error processing transaction", err)
	}

	return qfn.NetlifyLogAndJsonResponse(200, map[string]any{"id": odooId, "new": isNew}, nil)
}

func main() {
	lambda.Start(handler)
}
