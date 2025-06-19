package netlify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

type NetlifyFunction func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error)

func AuthMiddleware(function NetlifyFunction) NetlifyFunction {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		expectedToken := os.Getenv("AUTH_KEY")
		if expectedToken == "" {
			return &events.APIGatewayProxyResponse{
				StatusCode: 401,
				Body:       "Unauthorized",
			}, nil
		}
		expectedToken = fmt.Sprintf("Bearer %s", expectedToken)
		token, tokenFound := request.Headers["authorization"]
		if !tokenFound || token != expectedToken {
			return &events.APIGatewayProxyResponse{
				StatusCode: 401,
				Body:       "Unauthorized",
			}, nil
		}

		return function(ctx, request)
	}
}

func CheckEnvMiddleware(function NetlifyFunction) NetlifyFunction {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		currentEnv := os.Getenv("ENV")
		disabledEnvs := os.Getenv("ENV_DISABLE")
		if currentEnv == "" || (disabledEnvs != "" && slices.Contains(strings.Split(disabledEnvs, ","), currentEnv)) {
			return &events.APIGatewayProxyResponse{
				StatusCode: 404,
				Body:       "Not Found",
			}, nil
		}

		return function(ctx, request)
	}
}

func NetlifyResponseWithHeaders(statusCode int, body string, headers map[string]string) (*events.APIGatewayProxyResponse, error) {
	return &events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       body,
		Headers:    headers,
	}, nil
}

func NetlifyResponse(statusCode int, body string) (*events.APIGatewayProxyResponse, error) {
	return NetlifyResponseWithHeaders(statusCode, body, nil)
}

func NetlifyJsonResponse(statusCode int, data any) (*events.APIGatewayProxyResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshalling Netlify JSON response: %v", err)
		return NetlifyResponse(500, "Internal Error")
	}
	return NetlifyResponseWithHeaders(statusCode, string(jsonData), map[string]string{
		"Content-Type": "application/json",
	})
}

func logBodyAndError(body any, err error) {
	if err != nil {
		log.Printf("%v\n>>> %v", body, err)
	} else {
		log.Println(body)
	}
}

func NetlifyLogAndResponse(statusCode int, body string, err error) (*events.APIGatewayProxyResponse, error) {
	logBodyAndError(body, err)
	return NetlifyResponse(statusCode, body)
}

func NetlifyLogAndJsonResponse(statusCode int, body any, err error) (*events.APIGatewayProxyResponse, error) {
	logBodyAndError(body, err)
	return NetlifyJsonResponse(statusCode, body)
}
