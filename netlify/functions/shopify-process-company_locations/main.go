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

	dataCompany, ok := data["company"].(map[string]any)
	if !ok {
		return qfn.NetlifyLogAndResponse(400, "Company data not in request body", nil)
	}
	companyId, ok := dataCompany["admin_graphql_api_id"]
	if !ok {
		return qfn.NetlifyLogAndResponse(400, "Company GraphQL ID not in request body", nil)
	}

	odooId, isNew, err := shopifyodoo.ShopifyCompanyToOdoo(companyId.(string))
	if err != nil {
		return qfn.NetlifyLogAndResponse(500, "Error processing company", err)
	}

	return qfn.NetlifyLogAndJsonResponse(200, map[string]any{"id": odooId, "new": isNew}, nil)
}

func main() {
	lambda.Start(qfn.AuthMiddleware(handler))
}
