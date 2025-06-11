package shopify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

func ValidateWebhook(request events.APIGatewayProxyRequest) error {
	shopDomain, okDomain := request.Headers["x-shopify-shop-domain"]
	hmacHeader, okHeader := request.Headers["x-shopify-hmac-sha256"]
	shopifyTopic, okTopic := request.Headers["x-shopify-topic"]
	if !(okDomain && okHeader && okTopic && shopDomain != "" && hmacHeader != "" && shopifyTopic != "") {
		return fmt.Errorf("invalid or incomplete Shopify headers")
	}

	domainQF := os.Getenv("SHOPIFY_DOMAIN_QF")
	domainFM := os.Getenv("SHOPIFY_DOMAIN_FM")
	secretQF := os.Getenv("SHOPIFY_SECRET_QF")
	secretFM := os.Getenv("SHOPIFY_SECRET_FM")
	if !(domainQF != "" && domainFM != "" && secretQF != "" && secretFM != "") {
		return fmt.Errorf("invalid or incomplete Shopify environment variables")
	}

	signature, signatureOk := map[string]string{
		domainQF: secretQF,
		domainFM: secretFM,
	}[shopDomain]
	if !(signatureOk && signature != "") {
		return fmt.Errorf("could not determine correct Shopify signature")
	}

	if len(request.Body) == 0 {
		return fmt.Errorf("empty request")
	}

	mac := hmac.New(sha256.New, []byte(signature))
	mac.Write([]byte(request.Body))
	calculatedHMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	isValid := hmac.Equal([]byte(calculatedHMAC), []byte(hmacHeader))

	if !isValid {
		return fmt.Errorf("the Shopify webhook is not valid")
	}

	return nil
}
