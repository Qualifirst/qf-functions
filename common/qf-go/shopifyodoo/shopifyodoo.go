package shopifyodoo

import (
	"fmt"
	"strings"
)

func ShopifyIdToOdooXid(shopifyId string) (string, error) {
	parts := strings.Split(shopifyId, "/")
	if len(parts) != 5 {
		return "", fmt.Errorf("invalid Shopify ID: %v", shopifyId)
	}
	idNumber := strings.Split(parts[4], "?")[0]
	objectType := strings.ToLower(parts[3])
	if idNumber == "" || objectType == "" {
		return "", fmt.Errorf("invalid Shopify ID: %v", shopifyId)
	}
	return fmt.Sprintf("__export__.shopify_%s_%s", objectType, idNumber), nil
}
