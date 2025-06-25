package shopifyodoo

import (
	"context"
	"fmt"
	"maps"
	"qf/go/helpers"
	"qf/go/odoo"
	"qf/go/shopify/adminapi"
	"qf/go/shopify/adminapi/types"
	"slices"
	"strings"
	"time"
)

func computeScheduledDate(orderDate time.Time, companyId int, address *types.Address) (scheduledDate time.Time, err error) {
	type LocationConfig struct {
		Timezone string
		Cities   []string
	}
	locationConfig := map[int]LocationConfig{
		// QF
		2: {
			Timezone: "Canada/Eastern",
			Cities:   []string{"Etobicoke, ON", "Markham, ON", "Missisauga, ON", "Richmond Hill, ON", "Scarborough, ON", "Toronto, ON", "Vaughan, ON"},
		},
		// FM
		3: {
			Timezone: "Canada/Pacific",
			Cities:   []string{"Burnaby, BC", "New Westminster, BC", "Richmond, BC", "Vancouver, BC"},
		},
	}[companyId]

	inTown, err := helpers.StringInSlice(fmt.Sprintf("%s, %s", address.City, address.ProvinceCode()), locationConfig.Cities)
	if err != nil {
		return scheduledDate, fmt.Errorf("error checking cities for scheduled date computation\nERROR=%w", err)
	}

	location, _ := time.LoadLocation(locationConfig.Timezone)
	localizedOrderDate := orderDate.In(location)
	hour := localizedOrderDate.Hour()
	afterFive := hour >= 17
	weekday := localizedOrderDate.Weekday()
	weekdayInt := int(weekday)
	if weekdayInt == 0 {
		weekdayInt = 7
	}
	daysUntilMonday := (weekdayInt - 8) * -1
	addDays := 0

	// If friday after 5pm, or saturday or sunday
	if (afterFive && weekday == time.Friday) || slices.Contains([]time.Weekday{time.Saturday, time.Sunday}, weekday) {
		// Schedule for next monday
		addDays = daysUntilMonday
		if inTown {
			// For next tuesday for in town
			addDays += 1
		}
	} else
	// If monday, tuesday, or wednesday after 5pm
	if afterFive && slices.Contains([]time.Weekday{time.Monday, time.Tuesday, time.Wednesday}, weekday) {
		// Schedule for next day
		addDays = 1
		if inTown {
			// 2 days after for in town
			addDays = 2
		}
	} else
	// If thursday, after 5pm
	if afterFive && weekday == time.Thursday {
		// Schedule for next day
		addDays = 1
		if inTown {
			// Next monday for in town
			addDays = daysUntilMonday
		}
	} else
	// If monday-thursday, before 5pm, and in town
	if inTown && !afterFive && slices.Contains([]time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday}, weekday) {
		// Schedule for next day
		addDays = 1
	} else
	// If friday, before 5pm, and in town
	if inTown && !afterFive && weekday == time.Friday {
		// Schedule for next monday
		addDays = daysUntilMonday
	}

	localizedScheduled := localizedOrderDate.AddDate(0, 0, addDays)
	localizedScheduled = time.Date(
		localizedScheduled.Year(),
		localizedScheduled.Month(),
		localizedScheduled.Day(),
		12, 0, 0, 0,
		localizedScheduled.Location(),
	)

	scheduledDate = localizedScheduled.UTC()
	return scheduledDate, nil
}

func shopifyTaxLinesToOdooIds(ctx context.Context, taxLines *[]types.OrderTaxLine, companyId int) ([]int, error) {
	taxes := make([]int, 0, len(*taxLines))
	for _, taxLine := range *taxLines {
		taxName := strings.ReplaceAll(fmt.Sprintf("%s %.2f%%", taxLine.Title, taxLine.RatePercentage), ".00%", "%")
		taxId, err := odoo.OdooDataManager.GetTax(ctx, taxName, taxLine.RatePercentage, companyId)
		if err != nil || taxId == 0 {
			return nil, fmt.Errorf("error getting taxes %v for company %v\nERROR=%w", taxName, companyId, err)
		}
		taxes = append(taxes, taxId)
	}
	return taxes, nil
}

func shopifyOrderToOdoo(ctx context.Context, order *types.Order, customerOdooId int) (odooId int, isNew bool, err error) {
	orderOdooXid, _ := ShopifyIdToOdooXid(*order.Id)
	odooId, err = odoo.GetIDByXID(ctx, "sale.order", orderOdooXid)
	if err != nil {
		return 0, false, fmt.Errorf("error reading XID %v from Odoo\nERROR=%w", orderOdooXid, err)
	}

	shippingAddressOdooId, err := ensureShopifyCustomerAddressInOdoo(ctx, customerOdooId, &order.ShippingAddress, "delivery")
	if err != nil {
		return 0, false, fmt.Errorf("error getting the shipping address from Odoo\nERROR=%w", err)
	}
	billingAddressOdooId := shippingAddressOdooId
	if *order.BillingAddress.Id != *order.ShippingAddress.Id {
		billingAddressOdooId, err = ensureShopifyCustomerAddressInOdoo(ctx, customerOdooId, &order.BillingAddress, "invoice")
		if err != nil {
			return 0, false, fmt.Errorf("error getting the billing address from Odoo\nERROR=%w", err)
		}
	}

	companyId := odoo.CompanyFM
	if strings.Contains(order.Name, "QF") {
		companyId = odoo.CompanyQF
	}
	defer odoo.WithContext(ctx, map[string]any{"allowed_company_ids": []int{int(companyId)}})()

	orderData := map[string]any{
		"partner_id":                     customerOdooId,
		"partner_invoice_id":             billingAddressOdooId,
		"partner_shipping_id":            shippingAddressOdooId,
		"origin":                         order.Name,
		"date_order":                     order.CreatedAt.Format(odoo.DateFormat),
		"company_id":                     companyId,
		"customer_delivery_instructions": order.DeliveryInstructions.Value,
		"client_order_ref":               order.PurchaseOrderNumber.Value,
		"recompute_delivery_price":       false,
		"amount_delivery":                0,
		"no_handling_fee_reason":         "Shopify",
	}
	if odoo.OdooDataManager.Data.Sources.Shopify != 0 {
		orderData["source_id"] = int(odoo.OdooDataManager.Data.Sources.Shopify)
	}

	allSkus := make([]string, 0, order.Lines.Length()+1)
	for _, line := range order.Lines.Iter {
		allSkus = append(allSkus, line.Sku)
	}
	shippingSku := odoo.ShippingSku
	if order.ShippingLine.Id != nil && strings.Contains(strings.ToLower(order.ShippingLine.Source), "2ship") {
		shippingSku = odoo.TwoshipSku
	}
	allSkus = append(allSkus, shippingSku)
	slices.Sort(allSkus)
	allSkus = slices.Compact(allSkus)

	idsBySku := map[string]int{}
	skusById := map[int]string{}
	if len(allSkus) != 0 {
		products, err := odoo.SearchRead(ctx, "product.product", []any{[]any{"default_code", "in", allSkus}}, []string{"id", "default_code"}, 0, map[string]any{"active_test": false})
		if err != nil {
			return 0, false, fmt.Errorf("error reading products for the order %v in Odoo\nERROR=%w", orderOdooXid, err)
		}
		if len(allSkus) != len(products) {
			return 0, false, fmt.Errorf("not all order products were found in Odoo: %v", allSkus)
		}
		for _, product := range products {
			idsBySku[product["default_code"].(string)] = int(product["id"].(float64))
			skusById[int(product["id"].(float64))] = product["default_code"].(string)
		}
	}

	odooLineIds, err := odoo.SearchIds(ctx, "sale.order.line", []any{[]any{"order_id", "=", odooId}}, nil)
	if err != nil {
		return 0, false, fmt.Errorf("error reading lines for the order %v in Odoo\nERROR=%w", orderOdooXid, err)
	}

	sequence := 1
	odooOrderLines := make([]any, 0, max(order.Lines.Length(), len(odooLineIds)))
	foundOdooLineIds := make([]int, 0, order.Lines.Length())
	odooNewLinesData := map[string]map[string]any{}
	for _, shopifyLine := range order.Lines.Iter {
		shopifyLineXid, _ := ShopifyIdToOdooXid(*shopifyLine.Id)
		odooLineId, err := odoo.GetIDByXID(ctx, "sale.order.line", shopifyLineXid)
		if err != nil {
			return 0, false, fmt.Errorf("error reading line %v from Odoo Order %v\nERROR=%w", shopifyLineXid, orderOdooXid, err)
		}
		odooLineData := map[string]any{
			"product_id":      idsBySku[shopifyLine.Sku],
			"name":            shopifyLine.Name,
			"product_uom_qty": shopifyLine.Quantity,
			"price_unit":      shopifyLine.UnitPrice.Amount(),
			"sequence":        sequence,
		}
		sequence += 1
		taxes, err := shopifyTaxLinesToOdooIds(ctx, &shopifyLine.TaxLines, int(companyId))
		if err != nil {
			return 0, false, fmt.Errorf("error creating line %v from Odoo Order %v\nERROR=%w", shopifyLineXid, orderOdooXid, err)
		}
		odooLineData["tax_id"] = []any{odoo.Command.Set(taxes)}
		if odooLineId != 0 {
			foundOdooLineIds = append(foundOdooLineIds, odooLineId)
			odooOrderLines = append(odooOrderLines, odoo.Command.Update(odooLineId, odooLineData))
		} else {
			odooNewLinesData[shopifyLineXid] = odooLineData
		}
	}
	if order.ShippingLine.Id != nil {
		shopifyLineXid, _ := ShopifyIdToOdooXid(*order.ShippingLine.Id)
		odooLineId, err := odoo.GetIDByXID(ctx, "sale.order.line", shopifyLineXid)
		if err != nil {
			return 0, false, fmt.Errorf("error reading line %v from Odoo Order %v\nERROR=%w", shopifyLineXid, orderOdooXid, err)
		}
		carrierName := order.ShippingLine.Title
		deliveryType := "base_on_rule"
		carrierProductId := odoo.OdooDataManager.Data.DeliveryProducts.Webship
		if shippingSku == odoo.TwoshipSku {
			carrierName = "2Ship"
			deliveryType = "twoship"
			carrierProductId = odoo.OdooDataManager.Data.DeliveryProducts.Twoship
			orderData["customer_delivery_instructions"] = strings.Trim(orderData["customer_delivery_instructions"].(string)+"\n"+order.ShippingLine.Title, " \n")
		}
		carrier, err := odoo.OdooDataManager.GetDeliveryCarrier(ctx, carrierName, deliveryType, int(carrierProductId))
		if err != nil {
			return 0, false, fmt.Errorf("error searching for delivery carrier %v for Odoo Order %v\nERROR=%w", carrierName, orderOdooXid, err)
		}
		orderData["carrier_id"] = carrier
		orderData["amount_delivery"] = order.ShippingLine.Price.Amount()
		odooLineData := map[string]any{
			"product_id":      carrierProductId,
			"name":            order.ShippingLine.Title,
			"product_uom_qty": 1,
			"price_unit":      order.ShippingLine.Price.Amount(),
			"is_delivery":     true,
			"sequence":        sequence,
		}
		sequence += 1
		taxes, err := shopifyTaxLinesToOdooIds(ctx, &order.ShippingLine.TaxLines, int(companyId))
		if err != nil {
			return 0, false, fmt.Errorf("error creating line %v from Odoo Order %v\nERROR=%w", shopifyLineXid, orderOdooXid, err)
		}
		odooLineData["tax_id"] = []any{odoo.Command.Set(taxes)}
		if odooLineId != 0 {
			foundOdooLineIds = append(foundOdooLineIds, odooLineId)
			odooOrderLines = append(odooOrderLines, odoo.Command.Update(odooLineId, odooLineData))
		} else {
			odooNewLinesData[shopifyLineXid] = odooLineData
		}
	}
	for _, odooLineId := range odooLineIds {
		if !slices.Contains(foundOdooLineIds, odooLineId) {
			odooOrderLines = append(odooOrderLines, odoo.Command.Delete(odooLineId))
		}
	}
	orderData["order_line"] = odooOrderLines

	if odooId == 0 {
		createData := map[string]any{}
		if scheduledDate, err := computeScheduledDate(order.CreatedAt, int(companyId), &order.ShippingAddress); err == nil {
			createData["commitment_date"] = scheduledDate.Format(odoo.DateFormat)
		}
		maps.Copy(orderData, createData)
		isNew = true
		odooId, err = odoo.Create(ctx, "sale.order", orderData, map[string]any{"xid": orderOdooXid})
		if err != nil {
			return 0, false, fmt.Errorf("error creating the order %v in Odoo\nERROR=%w", orderOdooXid, err)
		}
	} else {
		err = odoo.Write(ctx, "sale.order", odooId, orderData, nil)
		if err != nil {
			return 0, false, fmt.Errorf("error updating the order %v in Odoo\nERROR=%w", orderOdooXid, err)
		}
	}

	lineErrors := ""
	for lineXid, lineData := range odooNewLinesData {
		lineData["order_id"] = odooId
		_, err := odoo.Create(ctx, "sale.order.line", lineData, map[string]any{"xid": lineXid})
		if err != nil {
			lineErrors += fmt.Sprintf("error creating new line %v for order %v in Odoo\nERROR=%v", lineXid, orderOdooXid, err)
		}
	}
	lineErrors = strings.Trim(lineErrors, " \n")
	if lineErrors != "" {
		return odooId, false, fmt.Errorf("error syncing lines for order %v in Odoo\nERROR=%v", orderOdooXid, lineErrors)
	}

	if isNew {
		confirmationContext := map[string]any{"followup_validation": false, "skip_preauth_payment": true}
		confirmationRes, err := odoo.JsonRpcExecuteKw(ctx, "sale.order", "action_confirm", []any{[]any{odooId}}, map[string]any{"context": confirmationContext})
		if err != nil {
			odoo.Unlink(ctx, "sale.order", odooId, nil) // Try to delete order as we could not confirm it
			return 0, false, fmt.Errorf("error confirming the order %v in Odoo\nERROR=%w", orderOdooXid, err)
		}
		res, err := odoo.SearchReadById(ctx, "sale.order", odooId, []string{"state"}, nil)
		if err != nil {
			odoo.Unlink(ctx, "sale.order", odooId, nil) // Try to delete order as we could not validate the confirmation
			return 0, false, fmt.Errorf("error validating order confirmation %v in Odoo\nERROR=%w", orderOdooXid, err)
		}
		if res["state"].(string) != "sale" {
			odoo.Unlink(ctx, "sale.order", odooId, nil) // Try to delete order as we could not validate the confirmation
			return 0, false, fmt.Errorf("could not validate order confirmation %v in Odoo. Expected=sale, Got=%v. Result: %v", orderOdooXid, res["state"], confirmationRes)
		}
	}

	return odooId, isNew, nil
}

func prefetchXidsForOrder(ctx context.Context, order *types.Order) error {
	xids := make([]odoo.IrModelData, 0, 4+order.Lines.Length())
	orderOdooXid, _ := ShopifyIdToOdooIrModelData(*order.Id, "sale.order")
	xids = append(xids, *orderOdooXid)
	for _, shopifyLine := range order.Lines.Iter {
		shopifyLineXid, _ := ShopifyIdToOdooIrModelData(*shopifyLine.Id, "sale.order.line")
		xids = append(xids, *shopifyLineXid)
	}
	if order.ShippingLine.Id != nil {
		shopifyLineXid, _ := ShopifyIdToOdooIrModelData(*order.ShippingLine.Id, "sale.order.line")
		xids = append(xids, *shopifyLineXid)
	}
	if order.ShippingAddress.Id != nil {
		addressXid, _ := ShopifyIdToOdooIrModelData(*order.ShippingAddress.Id, "res.partner")
		xids = append(xids, *addressXid)
	}
	if order.BillingAddress.Id != nil && *order.BillingAddress.Id != *order.ShippingAddress.Id {
		addressXid, _ := ShopifyIdToOdooIrModelData(*order.BillingAddress.Id, "res.partner")
		xids = append(xids, *addressXid)
	}
	return odoo.PrefetchIrModelData(ctx, xids)
}

func ShopifyOrderToOdoo(ctx context.Context, shopifyId string) (odooId int, isNew bool, err error) {
	order, err := adminapi.OrderMinimalById(ctx, shopifyId)
	if err != nil {
		return 0, false, fmt.Errorf("error getting order %v from Shopify Admin API\nERROR=%w", shopifyId, err)
	}

	customerOdooXid, _ := ShopifyIdToOdooXid(*order.Customer.Id)
	customerOdooId, err := odoo.GetIDByXID(ctx, "res.partner", customerOdooXid)
	if err != nil {
		return 0, false, fmt.Errorf("error getting customer %w from Odoo", err)
	}
	if customerOdooId == 0 {
		return 0, false, fmt.Errorf("customer %v not found in Odoo", customerOdooXid)
	}

	var fullOrder *types.Order
	qfId := order.CustomAttribute("FarMetOrderId")
	if qfId != "" {
		qfShopifyId := "gid://shopify/Order/" + qfId
		adminapi.AsQF(ctx, func() {
			fullOrder, err = adminapi.OrderById(ctx, qfShopifyId)
		})
		if err != nil {
			return 0, false, fmt.Errorf("error getting QF order %v from Shopify Admin API\nERROR=%w", qfShopifyId, err)
		}
	} else {
		fullOrder, err = adminapi.OrderById(ctx, shopifyId)
		if err != nil {
			return 0, false, fmt.Errorf("error getting FM order %v from Shopify Admin API\nERROR=%w", shopifyId, err)
		}
	}

	if err := prefetchXidsForOrder(ctx, fullOrder); err != nil {
		return 0, false, err
	}
	return shopifyOrderToOdoo(ctx, fullOrder, customerOdooId)
}
