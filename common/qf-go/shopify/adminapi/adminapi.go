package adminapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"qf/go/app"
	"qf/go/helpers"
	"qf/go/shopify/adminapi/queries"
	"qf/go/shopify/adminapi/types"
)

type Query[T any] struct{}

func (f *Query[T]) CallGeneric(ctx context.Context, query queries.ShopifyQuery, variables map[string]any) (any, error) {
	domainKey, _ := app.GetCacheValue(ctx, []any{"Shopify", "DomainKey"}, "FM")
	domain := os.Getenv(fmt.Sprintf("SHOPIFY_DOMAIN_%s", domainKey))
	token := os.Getenv(fmt.Sprintf("SHOPIFY_ADMIN_API_ACCESS_TOKEN_%s", domainKey))
	if domain == "" || token == "" {
		return nil, fmt.Errorf("missing necessary environment variables for Shopify Admin API call")
	}
	graphQLQuery, _ := app.GetCacheValue(ctx, []any{"Shopify", "GraphQLQuery"}, helpers.GraphQLQuery)
	url := fmt.Sprintf("https://%s/admin/api/2025-04/graphql.json", domain)
	resp, err := graphQLQuery(ctx, url, "X-Shopify-Access-Token", token, query.Query, variables)
	if err != nil {
		return nil, err
	}
	respMap, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid Shopify Admin API query response, expected map, got: %v", resp)
	}
	if _, foundErrors := respMap["errors"]; foundErrors {
		return nil, fmt.Errorf("errors in Shopify Admin API query response: %v", respMap)
	}
	data, dataOk := respMap["data"].(map[string]any)
	if !dataOk {
		return nil, fmt.Errorf("data map not found in Shopify Admin API query response: %v", respMap)
	}
	resultData, resultFound := data[query.ResultKey]
	if !resultFound {
		return nil, fmt.Errorf("result key not found in Shopify Admin API query response (%v): %v", query.ResultKey, respMap)
	}
	if resultData == nil {
		return nil, fmt.Errorf("empty response from Shopify Admin API query response (%v): %v", query.ResultKey, respMap)
	}
	return resultData, nil
}

func (f *Query[T]) Call(ctx context.Context, query queries.ShopifyQuery, variables map[string]any) (*T, error) {
	resultAny, err := f.CallGeneric(ctx, query, variables)
	if err != nil {
		return nil, err
	}
	resultJson, err := json.Marshal(resultAny)
	if err != nil {
		return nil, fmt.Errorf("error re-marshalling result from Shopify Admin API query response:\n>>> %v\n>>> %w", resultAny, err)
	}
	var result T
	decoder := json.NewDecoder(bytes.NewReader(resultJson))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding result into struct from Shopify Admin API query response:\n>>> %v\n>>> %w", string(resultJson), err)
	}
	return &result, nil
}

func AsQF(ctx context.Context, callable func()) {
	defer app.SetCacheValue(ctx, []any{"Shopify", "DomainKey"}, "QF")()
	callable()
}

func CustomerById(ctx context.Context, id string) (*types.Customer, error) {
	return (&Query[types.Customer]{}).Call(ctx, queries.Customer, map[string]any{"id": id})
}
func CompanyById(ctx context.Context, id string) (*types.Company, error) {
	return (&Query[types.Company]{}).Call(ctx, queries.Company, map[string]any{"id": id})
}
func OrderMinimalById(ctx context.Context, id string) (*types.Order, error) {
	return (&Query[types.Order]{}).Call(ctx, queries.OrderMinimal, map[string]any{"id": id})
}
func OrderById(ctx context.Context, id string) (*types.Order, error) {
	return (&Query[types.Order]{}).Call(ctx, queries.Order, map[string]any{"id": id})
}
func OrderWithTransactionsById(ctx context.Context, id string) (*types.Order, error) {
	return (&Query[types.Order]{}).Call(ctx, queries.OrderWithTransactions, map[string]any{"id": id})
}
