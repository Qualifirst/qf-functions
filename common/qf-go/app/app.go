package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/trace"
	"slices"
	"strings"
	"time"

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

func ProfilingMiddleware(function NetlifyFunction, filename string) NetlifyFunction {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		if os.Getenv("PROFILING") == "1" && os.Getenv("ENV") == "LOCAL" {
			path := os.Getenv("PROFILING_PATH")
			if path != "" {
				if string(path[len(path)-1]) != "/" {
					path += "/"
				}
				filename = path + filename
			}
			filename += ".out"
			f, err := os.Create(filename)
			if err != nil {
				log.Printf("!!! Could not create trace profile for %v: %v", filename, err)
			} else {
				defer f.Close()
				if err := trace.Start(f); err != nil {
					f.Close()
					log.Printf("!!! Could not start trace profile for %v: %v", filename, err)
				} else {
					defer trace.Stop()
					fmt.Printf("!!! Tracing on for: %v\n", filename)
				}
			}
		}

		return function(ctx, request)
	}
}

func TimeoutMiddleware(function NetlifyFunction) NetlifyFunction {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 9500*time.Millisecond)
		defer cancel()

		type result struct {
			Response *events.APIGatewayProxyResponse
			Error    error
		}

		resultChan := make(chan result, 1)

		go func() {
			response, err := function(timeoutCtx, request)
			resultChan <- result{
				Response: response,
				Error:    err,
			}
		}()

		select {
		case res := <-resultChan:
			return res.Response, res.Error
		case <-timeoutCtx.Done():
			return NetlifyResponse(int(http.StatusGatewayTimeout), "Request timed out")
		}
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
