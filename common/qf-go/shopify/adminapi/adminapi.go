package adminapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"qf/go/helpers"
	"qf/go/shopify/adminapi/queries"
	"qf/go/shopify/adminapi/types"
)

type queryConfig struct {
	DomainKey    string
	GraphQLQuery func(string, string, string, string, map[string]any) (any, error)
}

func (f *queryConfig) SetDomainKey(domainKey string, onlyIfEmpty bool) (reset func()) {
	current := f.DomainKey
	if onlyIfEmpty && current != "" {
		return func() {}
	}
	f.DomainKey = domainKey
	return func() {
		f.DomainKey = current
	}
}
func (f *queryConfig) SetGraphQLQuery(graphQLQuery func(string, string, string, string, map[string]any) (any, error), onlyIfNull bool) (reset func()) {
	current := f.GraphQLQuery
	if onlyIfNull && current != nil {
		return func() {}
	}
	f.GraphQLQuery = graphQLQuery
	return func() {
		f.GraphQLQuery = current
	}
}

var QueryConfig = queryConfig{}

type Query[T any] struct{}

func (f *Query[T]) CallGeneric(query queries.ShopifyQuery, variables map[string]any) (any, error) {
	defer QueryConfig.SetDomainKey("FM", true)()
	domain := os.Getenv(fmt.Sprintf("SHOPIFY_DOMAIN_%s", QueryConfig.DomainKey))
	token := os.Getenv(fmt.Sprintf("SHOPIFY_ADMIN_API_ACCESS_TOKEN_%s", QueryConfig.DomainKey))
	if domain == "" || token == "" {
		return nil, fmt.Errorf("missing necessary environment variables for Shopify Admin API call")
	}
	defer QueryConfig.SetGraphQLQuery(helpers.GraphQLQuery, true)()
	url := fmt.Sprintf("https://%s/admin/api/2025-04/graphql.json", domain)
	resp, err := QueryConfig.GraphQLQuery(url, "X-Shopify-Access-Token", token, query.Query, variables)
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

func (f *Query[T]) Call(query queries.ShopifyQuery, variables map[string]any) (*T, error) {
	resultAny, err := f.CallGeneric(query, variables)
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

func AsQF(callable func()) {
	defer QueryConfig.SetDomainKey("QF", false)()
	callable()
}

func CustomerById(id string) (*types.Customer, error) {
	return (&Query[types.Customer]{}).Call(queries.Customer, map[string]any{"id": id})
}
func CompanyById(id string) (*types.Company, error) {
	return (&Query[types.Company]{}).Call(queries.Company, map[string]any{"id": id})
}
func OrderMinimalById(id string) (*types.Order, error) {
	return (&Query[types.Order]{}).Call(queries.OrderMinimal, map[string]any{"id": id})
}
func OrderById(id string) (*types.Order, error) {
	return (&Query[types.Order]{}).Call(queries.Order, map[string]any{"id": id})
}
func OrderWithTransactionsById(id string) (*types.Order, error) {
	return (&Query[types.Order]{}).Call(queries.OrderWithTransactions, map[string]any{"id": id})
}
