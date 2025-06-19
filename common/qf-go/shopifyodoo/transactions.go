package shopifyodoo

import (
	"fmt"
	"qf/go/helpers"
	"qf/go/odoo"
	"qf/go/shopify/adminapi"
	"qf/go/shopify/adminapi/types"
	"strings"
	"time"
)

func shopifyTransactionToOdoo[T types.OrderTransactionInterface](order *types.Order, transaction T, setState string) (txOdooId int, isNew bool, err error) {
	txShopifyId := *transaction.GetId()
	txOdooXid, _ := ShopifyIdToOdooXid(txShopifyId)
	txOdooRes, err := odoo.ReadRecordByXID("payment.transaction", txOdooXid, []string{"id", "state"})
	if err != nil {
		return 0, false, fmt.Errorf("error getting transaction %v from Odoo\nERROR=%w", txOdooXid, err)
	}
	txOdooId = int(helpers.Traverse(txOdooRes, []any{"id"}, 0.0))
	txOdooState := helpers.Traverse(txOdooRes, []any{"state"}, "")
	if txOdooId != 0 && txOdooState == "" {
		return 0, false, fmt.Errorf("incorrect data from transaction %v from Odoo (id: %v, state: %v)", txOdooXid, txOdooId, txOdooState)
	}

	// Return as we cannot update it anymore
	if txOdooId != 0 && txOdooState != "authorized" {
		return txOdooId, false, nil
	}

	orderOdooXid, _ := ShopifyIdToOdooXid(*order.Id)
	orderOdooRes, err := odoo.ReadRecordByXID("sale.order", orderOdooXid, []string{"id", "company_id", "commercial_partner_id", "name"})
	if err != nil || orderOdooRes == nil {
		return 0, false, fmt.Errorf("error getting order %v from Odoo\nERROR=%w", orderOdooXid, err)
	}
	orderOdooId := int(helpers.Traverse(orderOdooRes, []any{"id"}, 0.0))
	companyOdooId := int(helpers.Traverse(orderOdooRes, []any{"company_id", 0}, 0.0))
	partnerOdooId := int(helpers.Traverse(orderOdooRes, []any{"commercial_partner_id", 0}, 0.0))
	orderName := helpers.Traverse(orderOdooRes, []any{"name"}, "")
	if orderOdooId == 0 || companyOdooId == 0 || partnerOdooId == 0 || orderName == "" {
		return 0, false, fmt.Errorf("incorrect data from order %v from Odoo (id: %v, company_id: %v, commercial_partner_id: %v, name: %v)", orderOdooXid, orderOdooId, companyOdooId, partnerOdooId, orderName)
	}

	defer odoo.GlobalContext(map[string]any{"allowed_company_ids": []int{companyOdooId}})()

	currency, err := odoo.SearchId("res.currency", []any{[]any{"name", "=", "CAD"}}, nil)
	if err != nil {
		return 0, false, fmt.Errorf("error getting currency CAD from Odoo\nERROR=%w", err)
	}

	acquirer, err := odoo.SearchFirstId("payment.acquirer", []any{[]any{"company_id", "=", companyOdooId}, []any{"name", "=ilike", "shopify"}}, nil)
	if err != nil || acquirer == 0 {
		return 0, false, fmt.Errorf("error getting acquirer Shopify from Odoo\nERROR=%w", err)
	}

	amount := transaction.GetUnsettledAmount()
	if setState == "done" {
		amount = transaction.GetAmount()
	}

	txShopifyIdNumber := strings.Replace(txShopifyId, "gid://shopify/OrderTransaction/", "", 1)
	txReference := fmt.Sprintf("%s-%s", orderName, txShopifyIdNumber)

	txData := map[string]any{
		"reference":          txReference,
		"sale_order_ids":     []any{odoo.Command.Set([]int{orderOdooId})},
		"acquirer_id":        acquirer,
		"currency_id":        currency,
		"amount":             amount,
		"partner_id":         partnerOdooId,
		"acquirer_reference": txShopifyIdNumber,
		"state":              setState,
		"last_state_change":  time.Now().UTC().Format(odoo.DateFormat),
	}
	if amount == 0 || setState == "cancel" {
		txData["state"] = "cancel"
	}

	if txOdooId == 0 {
		isNew = true
		txOdooId, err = odoo.Create("payment.transaction", txData, map[string]any{"xid": txOdooXid})
		if err != nil {
			return 0, false, fmt.Errorf("error creating transaction %v in Odoo\nERROR=%w", txOdooXid, err)
		}
	} else {
		err := odoo.Write("payment.transaction", txOdooId, txData, nil)
		if err != nil {
			return 0, false, fmt.Errorf("error updating transaction %v in Odoo\nERROR=%w", txOdooXid, err)
		}
	}

	return txOdooId, isNew, nil
}

func handleTransactionAuthorization(order *types.Order, transaction types.OrderTransaction) (odooId int, isNew bool, err error) {
	return shopifyTransactionToOdoo(order, transaction, "authorized")
}

func handleTransactionCapture(order *types.Order, transaction types.OrderTransaction) (odooId int, isNew bool, err error) {
	odooId, isNew, err = shopifyTransactionToOdoo(order, transaction, "done")
	if err != nil {
		return odooId, isNew, err
	}
	if transaction.ParentTransaction.Id != nil {
		_, _, err = shopifyTransactionToOdoo(order, transaction.ParentTransaction, "authorized")
	}
	return odooId, isNew, err
}

func handleTransactionSale(order *types.Order, transaction types.OrderTransaction) (odooId int, isNew bool, err error) {
	return shopifyTransactionToOdoo(order, transaction, "done")
}

func handleTransactionVoid(order *types.Order, transaction types.OrderTransaction) (odooId int, isNew bool, err error) {
	if transaction.ParentTransaction.Id != nil {
		return shopifyTransactionToOdoo(order, transaction.ParentTransaction, "cancel")
	}
	return 0, false, nil
}

func ShopifyTransactionToOdoo(orderShopifyId string, transactionShopifyId string) (odooId int, isNew bool, err error) {
	order, err := adminapi.OrderWithTransactionsById(orderShopifyId)
	if err != nil {
		return 0, false, fmt.Errorf("error getting order %v from Shopify Admin API\nERROR=%w", orderShopifyId, err)
	}

	var transaction types.OrderTransaction
	for _, tx := range order.Transactions {
		if tx.Id != nil && *tx.Id == transactionShopifyId {
			transaction = tx
		}
	}
	if transaction.Id == nil {
		return 0, false, fmt.Errorf("transaction %v not found for order %v in Shopify Admin API\nERROR=%w", transactionShopifyId, orderShopifyId, err)
	}

	if transaction.Status != "SUCCESS" {
		// Wait for transaction to succeed before proceeding
		return 0, false, nil
	}

	switch transaction.Kind {
	case "AUTHORIZATION":
		return handleTransactionAuthorization(order, transaction)
	case "CAPTURE":
		return handleTransactionCapture(order, transaction)
	case "SALE":
		return handleTransactionSale(order, transaction)
	case "VOID":
		return handleTransactionVoid(order, transaction)
	}

	// Unsupported transaction, ignore
	return 0, false, nil
}
