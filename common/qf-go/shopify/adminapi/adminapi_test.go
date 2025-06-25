package adminapi

import (
	"context"
	"fmt"
	"qf/go/app"
	"qf/go/helpers"
	"qf/go/shopify/adminapi/queries"
	"qf/go/shopify/adminapi/types"
	"strings"
	"testing"
)

func fakeGraphQLQuery(_ context.Context, _ string, _ string, _ string, _ string, v map[string]any) (any, error) {
	res := v["response"]
	if res == "error" {
		return nil, fmt.Errorf("some GraphQL error")
	}
	return res, nil
}

func TestQueryCallGeneric_Errors(t *testing.T) {
	tests := []struct {
		Title           string
		ShopifyDomain   string
		ShopifyToken    string
		ExpectedError   string
		GraphQLResponse any
	}{
		{
			Title:         "No env vars",
			ExpectedError: "missing necessary environment variables",
		},
		{
			Title:         "No env var for domain",
			ExpectedError: "missing necessary environment variables",
			ShopifyToken:  "0",
		},
		{
			Title:         "No env var for token",
			ExpectedError: "missing necessary environment variables",
			ShopifyDomain: "D",
		},
		{
			Title:         "GraphQL error",
			ExpectedError: "some GraphQL error",
			ShopifyDomain: "D", ShopifyToken: "T",
			GraphQLResponse: "error",
		},
		{
			Title:         "Expect map",
			ExpectedError: "expected map",
			ShopifyDomain: "D", ShopifyToken: "T",
			GraphQLResponse: []int{1},
		},
		{
			Title:         "Errors in response",
			ExpectedError: "errors in",
			ShopifyDomain: "D", ShopifyToken: "T",
			GraphQLResponse: map[string]any{
				"errors": "some error here",
			},
		},
		{
			Title:         "Data key not found",
			ExpectedError: "data map not found",
			ShopifyDomain: "D", ShopifyToken: "T",
			GraphQLResponse: map[string]any{
				"no errors": "but no data",
			},
		},
		{
			Title:         "Result key not found",
			ExpectedError: "result key not found",
			ShopifyDomain: "D", ShopifyToken: "T",
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"yes data": "but no result",
				},
			},
		},
		{
			Title:         "Empty response",
			ExpectedError: "empty response",
			ShopifyDomain: "D", ShopifyToken: "T",
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Title, func(t *testing.T) {
			defer helpers.TempEnvVars(map[string]string{
				"SHOPIFY_DOMAIN_FM":                 tt.ShopifyDomain,
				"SHOPIFY_ADMIN_API_ACCESS_TOKEN_FM": tt.ShopifyToken,
			})()
			q := &Query[any]{}
			ctx := app.ContextWithCache(context.Background())
			defer app.SetCacheValue(ctx, []any{"Shopify", "GraphQLQuery"}, fakeGraphQLQuery)()
			res, err := q.Call(ctx, queries.ShopifyQuery{ResultKey: "result"}, map[string]any{"response": tt.GraphQLResponse})
			if err == nil {
				t.Fatalf("expected error, but received (%T) %+v", res, res)
			}
			if !strings.Contains(err.Error(), tt.ExpectedError) {
				t.Fatalf("expected '%s' in error, but got: %v", tt.ExpectedError, err)
			}
		})
	}
}

func TestSingularTypedQueryCall_Errors(t *testing.T) {
	tests := []struct {
		Title           string
		ExpectedErrors  []string
		GraphQLResponse any
	}{
		{
			Title:          "Unknown field",
			ExpectedErrors: []string{"error decoding result into struct", "unknown field \"Explode\""},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"Explode": "Here",
					},
				},
			},
		},
		{
			Title:          "Unexpected array",
			ExpectedErrors: []string{"error decoding result into struct", "cannot unmarshal array into Go value of type types.Customer"},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": []map[string]any{
						{"Explode": "Here"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Title, func(t *testing.T) {
			defer helpers.TempEnvVars(map[string]string{
				"SHOPIFY_DOMAIN_FM":                 "X",
				"SHOPIFY_ADMIN_API_ACCESS_TOKEN_FM": "X",
			})()
			q := &Query[types.Customer]{}
			ctx := app.ContextWithCache(context.Background())
			defer app.SetCacheValue(ctx, []any{"Shopify", "GraphQLQuery"}, fakeGraphQLQuery)()
			res, err := q.Call(ctx, queries.ShopifyQuery{ResultKey: "result"}, map[string]any{"response": tt.GraphQLResponse})
			if err == nil {
				t.Fatalf("expected error, but received (%T) %+v", res, res)
			}
			for _, errMsg := range tt.ExpectedErrors {
				if !strings.Contains(err.Error(), errMsg) {
					t.Fatalf("expected '%s' in error, but got: %v", errMsg, err)
				}
			}
		})
	}
}

func TestMultiTypedQueryCall_Errors(t *testing.T) {
	tests := []struct {
		Title           string
		ExpectedErrors  []string
		GraphQLResponse any
	}{
		{
			Title:          "Unexpected object",
			ExpectedErrors: []string{"error decoding result into struct", "cannot unmarshal object into Go value of type []types.Customer"},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"Explode": "Here",
					},
				},
			},
		},
		{
			Title:          "Unknown field",
			ExpectedErrors: []string{"error decoding result into struct", "unknown field \"Explode\""},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": []map[string]any{
						{"Id": "Valid"},
						{"Explode": "Here"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Title, func(t *testing.T) {
			defer helpers.TempEnvVars(map[string]string{
				"SHOPIFY_DOMAIN_FM":                 "X",
				"SHOPIFY_ADMIN_API_ACCESS_TOKEN_FM": "X",
			})()
			q := &Query[[]types.Customer]{}
			ctx := app.ContextWithCache(context.Background())
			defer app.SetCacheValue(ctx, []any{"Shopify", "GraphQLQuery"}, fakeGraphQLQuery)()
			res, err := q.Call(ctx, queries.ShopifyQuery{ResultKey: "result"}, map[string]any{"response": tt.GraphQLResponse})
			if err == nil {
				t.Fatalf("expected error, but received (%T) %+v", res, res)
			}
			for _, errMsg := range tt.ExpectedErrors {
				if !strings.Contains(err.Error(), errMsg) {
					t.Fatalf("expected '%s' in error, but got: %v", errMsg, err)
				}
			}
		})
	}
}

func TestEdgesTypedQueryCall_Errors(t *testing.T) {
	tests := []struct {
		Title           string
		ExpectedErrors  []string
		GraphQLResponse any
	}{
		{
			Title:          "Unexpected key",
			ExpectedErrors: []string{"error decoding result into struct", "unknown field \"Explode\""},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"Explode": "Here",
					},
				},
			},
		},
		{
			Title:          "Unexpected object",
			ExpectedErrors: []string{"error decoding result into struct", "cannot unmarshal object into Go struct field"},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"edges": map[string]any{
							"type": "incorrect",
						},
					},
				},
			},
		},
		{
			Title:          "Unknown field for edge",
			ExpectedErrors: []string{"error decoding result into struct", "unknown field \"notCursor\""},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"edges": []map[string]any{
							{"cursor": "Valid"},
							{"notCursor": "Invalid"},
						},
					},
				},
			},
		},
		{
			Title:          "Unknown field for node",
			ExpectedErrors: []string{"error decoding result into struct", "unknown field \"notId\""},
			GraphQLResponse: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"edges": []map[string]any{
							{
								"cursor": "Valid",
								"node": map[string]any{
									"id": "Valid",
								},
							},
							{
								"cursor": "Invalid",
								"node": map[string]any{
									"notId": "Invalid",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Title, func(t *testing.T) {
			defer helpers.TempEnvVars(map[string]string{
				"SHOPIFY_DOMAIN_FM":                 "X",
				"SHOPIFY_ADMIN_API_ACCESS_TOKEN_FM": "X",
			})()
			q := &Query[types.Edges[types.Customer]]{}
			ctx := app.ContextWithCache(context.Background())
			defer app.SetCacheValue(ctx, []any{"Shopify", "GraphQLQuery"}, fakeGraphQLQuery)()
			res, err := q.Call(ctx, queries.ShopifyQuery{ResultKey: "result"}, map[string]any{"response": tt.GraphQLResponse})
			if err == nil {
				t.Fatalf("expected error, but received (%T) %+v", res, res)
			}
			for _, errMsg := range tt.ExpectedErrors {
				if !strings.Contains(err.Error(), errMsg) {
					t.Fatalf("expected '%s' in error, but got: %v", errMsg, err)
				}
			}
		})
	}
}
