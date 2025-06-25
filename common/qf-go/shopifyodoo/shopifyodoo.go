package shopifyodoo

import (
	"fmt"
	"qf/go/odoo"
	"strings"
)

func ShopifyIdToOdooIrModelData(shopifyId string, model string) (imd *odoo.IrModelData, err error) {
	parts := strings.Split(shopifyId, "/")
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid Shopify ID: %v", shopifyId)
	}
	idNumber := strings.Split(parts[4], "?")[0]
	objectType := strings.ToLower(parts[3])
	if idNumber == "" || objectType == "" {
		return nil, fmt.Errorf("invalid Shopify ID: %v", shopifyId)
	}
	module := "__export__"
	name := fmt.Sprintf("shopify_%s_%s", objectType, idNumber)
	imd = &odoo.IrModelData{
		Module: module,
		Name:   name,
		Xid:    module + "." + name,
		Model:  model,
	}
	return imd, nil
}

func ShopifyIdToOdooXid(shopifyId string) (string, error) {
	imd, err := ShopifyIdToOdooIrModelData(shopifyId, "")
	if err != nil {
		return "", err
	}
	return imd.Xid, nil
}
