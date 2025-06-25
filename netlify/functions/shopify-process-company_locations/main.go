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

	dataCompany, ok := data["company"].(map[string]any)
	if !ok {
		return app.NetlifyLogAndResponse(400, "Company data not in request body", nil)
	}
	companyId, ok := dataCompany["admin_graphql_api_id"]
	if !ok {
		return app.NetlifyLogAndResponse(400, "Company GraphQL ID not in request body", nil)
	}

	odooId, isNew, err := shopifyodoo.ShopifyCompanyToOdoo(ctx, companyId.(string))
	if err != nil {
		return app.NetlifyLogAndResponse(500, "Error processing company", err)
	}

	return app.NetlifyLogAndJsonResponse(200, map[string]any{"id": odooId, "new": isNew}, nil)
}

func main() {
	lambda.Start(app.ProfilingMiddleware(
		app.TimeoutMiddleware(app.CacheMiddleware(app.CheckEnvMiddleware(app.AuthMiddleware(odoo.OdooDataMiddleware(handler))))),
		"shopify-process-company_locations",
	))
}
