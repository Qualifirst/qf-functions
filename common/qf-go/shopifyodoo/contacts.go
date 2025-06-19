package shopifyodoo

import (
	"fmt"
	"maps"
	"qf/go/helpers"
	"qf/go/odoo"
	"qf/go/shopify/adminapi"
	"qf/go/shopify/adminapi/types"
	"strings"
)

func mapShopifyAddressToOdoo(address *types.Address, extra map[string]any) map[string]any {
	addressMap := map[string]any{}
	if address != nil && address.Id != nil {
		maps.Copy(addressMap, map[string]any{
			"name":    address.Name,
			"street":  address.Address1,
			"street2": address.Address2,
			"city":    address.City,
			"zip":     address.Zip,
			"phone":   address.Phone,
			"mobile":  address.Phone,
		})
		countryId, stateId := odoo.GetCountryAndStateIds(address.CountryCode(), address.ProvinceCode())
		if countryId != 0 {
			addressMap["country_id"] = countryId
		}
		if stateId != 0 {
			addressMap["state_id"] = stateId
		}
	}
	if extra != nil {
		maps.Copy(addressMap, extra)
	}
	return addressMap
}

func mapShopifyCustomerToOdoo(customer *types.Customer, address *types.Address, extra map[string]any) map[string]any {
	splitId := strings.Split(*customer.Id, "/")
	ref := "SHCU" + splitId[len(splitId)-1]
	customerData := mapShopifyAddressToOdoo(address, map[string]any{
		"ref":         ref,
		"name":        customer.DisplayName,
		"phone":       customer.DefaultPhoneNumber.PhoneNumber,
		"mobile":      customer.DefaultPhoneNumber.PhoneNumber,
		"email":       customer.DefaultEmailAddress.EmailAddress,
		"active":      true,
		"is_company":  false,
		"is_customer": true,
		"company_id":  false,
	})
	if extra != nil {
		maps.Copy(customerData, extra)
	}
	return customerData
}

func ShopifyCustomerToOdoo(shopifyId string) (odooId int, isNew bool, err error) {
	customer, err := adminapi.CustomerById(shopifyId)
	if err != nil {
		return 0, false, fmt.Errorf("shopify Admin API error while getting customer information\nERROR: %w", err)
	}

	if len(customer.CompanyContacts) > 0 {
		return shopifyCompanyContactToOdoo(customer)
	}
	return shopifyIndividualToOdoo(customer)
}

func shopifyCompanyContactToOdoo(customer *types.Customer) (odooId int, isNew bool, err error) {
	contactDetails := customer.CompanyContacts[0]
	pCompanyId := contactDetails.Company.Id

	companyXid, _ := ShopifyIdToOdooXid(*pCompanyId)
	odooCompany, err := odoo.ReadRecordByXID("res.partner", companyXid, []string{"id"})
	if err != nil {
		return 0, false, fmt.Errorf("error getting company from Odoo (XID=%s)\nERROR=%w", companyXid, err)
	}
	if odooCompany == nil {
		return 0, false, fmt.Errorf("company not found in Odoo (XID=%s)", companyXid)
	}

	company, err := adminapi.CompanyById(*pCompanyId)
	if err != nil {
		return 0, false, fmt.Errorf("error getting company information from Shopify\nERROR=%w", err)
	}
	if company.Locations.Length() == 0 {
		return 0, false, fmt.Errorf("no locations found for company %s", *pCompanyId)
	}
	location := company.Locations.Get(0)

	var address types.Address
	if location.BillingAddress.Id != nil {
		address = location.BillingAddress
	} else if location.ShippingAddress.Id != nil {
		address = location.BillingAddress
	}
	if address.Id == nil {
		return 0, false, fmt.Errorf("no location address found for company %s", *pCompanyId)
	}

	extraData := map[string]any{
		"parent_id": odooCompany["id"],
		"type":      "contact",
		"function":  contactDetails.Title,
	}
	roleMain, roleError := odoo.GetIDByXID("res.partner.role", "qfg_fields.res_partner_role_wholesale")
	if roleError == nil && roleMain != 0 {
		if contactDetails.IsMainContact {
			extraData["contact_role_code_ids"] = []any{odoo.Command.Link(roleMain)}
		} else {
			extraData["contact_role_code_ids"] = []any{odoo.Command.Unlink(roleMain)}
		}
	}

	return shopifyCustomerToOdoo(customer, &address, extraData, nil)
}

func shopifyIndividualToOdoo(customer *types.Customer) (odooId int, isNew bool, err error) {
	createData := func() map[string]any {
		data := map[string]any{}
		if ctype, err := odoo.GetIDByXID("customer.type", "qfg_customer_type.customer_type_individual_consumer"); err == nil && ctype != 0 {
			data["customer_type_id"] = ctype
		}
		if ctype, err := odoo.SearchFirstId("account.payment.method", []any{[]any{"name", "=ilike", "shopify"}}, nil); err == nil && ctype != 0 {
			data["customer_payment_method_id"] = ctype
		}
		if teams, err := odoo.SearchRead("crm.team", []any{[]any{"code", "=ilike", "consumer"}}, []string{"id", "user_id"}, 1, nil); err == nil && len(teams) != 0 {
			data["team_id"] = int(helpers.Traverse(teams, []any{0, "id"}, 0.0))
			data["user_id"] = int(helpers.Traverse(teams, []any{0, "user_id", 0}, 0.0))
		}
		if qfw, err := odoo.GetIDByXID("website", "qfg.main_website"); err == nil && qfw != 0 {
			data["website_id"] = qfw
		}
		if qz, err := odoo.GetIDByXID("product.pricelist", "qfg.pricelist_qualizon"); err == nil && qz != 0 {
			maps.Copy(data, map[string]any{
				"qf_pricelist_id": qz,
				"fm_pricelist_id": qz,
			})
		}
		if src, err := odoo.FindFirstOrCreate("utm.source", []any{[]any{"name", "=ilike", "shopify"}}, map[string]any{"name": "Shopify"}, nil); err == nil {
			data["source_id"] = src
		}
		return data
	}

	return shopifyCustomerToOdoo(customer, &customer.DefaultAddress, nil, createData)
}

func shopifyCustomerToOdoo(customer *types.Customer, address *types.Address, extra map[string]any, createData func() map[string]any) (odooId int, isNew bool, err error) {
	customerXid, _ := ShopifyIdToOdooXid(*customer.Id)
	foundOdooCustomer, err := odoo.ReadRecordByXID("res.partner", customerXid, []string{"id"})
	if err != nil {
		return 0, false, fmt.Errorf("error checking customer from Odoo (XID=%s)\nERROR=%w", customerXid, err)
	}
	customerOdooData := mapShopifyCustomerToOdoo(customer, address, nil)
	maps.Copy(customerOdooData, extra)
	if customerOdooData["name"] == customerOdooData["email"] || customerOdooData["country_id"] == nil || customerOdooData["country_id"] == 0 {
		return 0, false, fmt.Errorf("missing information to process customer: name, email, and country are required")
	}
	if foundOdooCustomer == nil {
		if createData != nil {
			maps.Copy(customerOdooData, createData())
		}
		newId, err := odoo.Create("res.partner", customerOdooData, nil)
		if err != nil {
			return 0, false, fmt.Errorf("error creating new customer in Odoo\nERROR=%w", err)
		}
		err = odoo.AssignRecordXID("res.partner", newId, customerXid)
		if err != nil {
			odoo.Unlink("res.partner", newId, nil) // Try deleting newly created record as we won't be able to reference it without XID
			return 0, false, fmt.Errorf("error assigning XID %s to new customer in Odoo\nERROR=%w", customerXid, err)
		}
		return newId, true, nil
	}
	foundId := int(foundOdooCustomer["id"].(float64))
	err = odoo.Write("res.partner", foundId, customerOdooData, nil)
	if err != nil {
		return 0, false, fmt.Errorf("error writing customer data in Odoo\nERROR=%w", err)
	}
	return foundId, false, nil
}

func ShopifyCompanyToOdoo(shopifyId string) (odooId int, isNew bool, err error) {
	company, err := adminapi.CompanyById(shopifyId)
	if err != nil {
		return 0, false, fmt.Errorf("error getting company from Shopify Admin API\nERROR=%w", err)
	}
	xid, _ := ShopifyIdToOdooXid(*company.Id)
	found, err := odoo.ReadRecordByXID("res.partner", xid, []string{"id"})
	if err != nil {
		return 0, false, fmt.Errorf("error checking company from Odoo (XID=%s)\nERROR=%w", xid, err)
	}

	if company.Locations.Length() == 0 {
		return 0, false, fmt.Errorf("no locations found for company %s", *company.Id)
	}
	location := company.Locations.Get(0)

	var address *types.Address
	if location.BillingAddress.Id != nil {
		address = &location.BillingAddress
	} else if location.ShippingAddress.Id != nil {
		address = &location.ShippingAddress
	}
	if address.Id == nil {
		return 0, false, fmt.Errorf("no location address found for company %s", *company.Id)
	}

	splitId := strings.Split(*company.Id, "/")
	ref := "SHCC" + splitId[len(splitId)-1]

	countryId, stateId := odoo.GetCountryAndStateIds(address.CountryCode(), address.ProvinceCode())
	if countryId == 0 || stateId == 0 {
		return 0, false, fmt.Errorf("location address with invalid country or state for company %s (%d, %d)", *company.Id, countryId, stateId)
	}

	companyData := map[string]any{
		"ref":         ref,
		"name":        company.Name,
		"phone":       location.Phone,
		"mobile":      location.Phone,
		"email":       "", // TODO: Need to add email to shopify company model as metadata
		"active":      true,
		"is_company":  true,
		"is_customer": true,
		"company_id":  false,
		"website_id":  false,
		"street":      address.Address1,
		"street2":     address.Address2,
		"city":        address.City,
		"state_id":    stateId,
		"country_id":  countryId,
		"zip":         address.Zip,
	}
	if found == nil {
		createData := map[string]any{}
		if ctype, err := odoo.SearchFirstId("customer.type", []any{[]any{"name", "=ilike", "business"}}, nil); err == nil && ctype != 0 {
			createData["customer_type_id"] = ctype
		}
		if ctype, err := odoo.SearchFirstId("account.payment.method", []any{[]any{"name", "=ilike", "shopify"}}, nil); err == nil && ctype != 0 {
			createData["customer_payment_method_id"] = ctype
		}
		if teams, err := odoo.SearchRead("crm.team", []any{[]any{"code", "=ilike", "leads"}}, []string{"id", "user_id"}, 1, nil); err == nil && len(teams) != 0 {
			createData["team_id"] = int(helpers.Traverse(teams, []any{0, "id"}, 0.0))
			createData["user_id"] = int(helpers.Traverse(teams, []any{0, "user_id", 0}, 0.0))
		}
		if qfw, err := odoo.GetIDByXID("website", "qfg.main_website"); err == nil && qfw != 0 {
			createData["website_id"] = qfw
		}
		if wsp, err := odoo.GetIDByXID("product.pricelist", "qfg.pricelist_qf_wholesale"); err == nil && wsp != 0 {
			createData["qf_pricelist_id"] = wsp
		}
		if wsp, err := odoo.GetIDByXID("product.pricelist", "qfg.pricelist_fm_wholesale"); err == nil && wsp != 0 {
			createData["fm_pricelist_id"] = wsp
		}
		if src, err := odoo.FindFirstOrCreate("utm.source", []any{[]any{"name", "=ilike", "shopify"}}, map[string]any{"name": "Shopify"}, nil); err == nil {
			createData["source_id"] = src
		}
		maps.Copy(companyData, createData)
		newId, err := odoo.Create("res.partner", companyData, nil)
		if err != nil {
			return 0, false, fmt.Errorf("error creating new company in Odoo\nERROR=%w", err)
		}
		err = odoo.AssignRecordXID("res.partner", newId, xid)
		if err != nil {
			odoo.Unlink("res.partner", newId, nil) // Try deleting newly created record as we won't be able to reference it without XID
			return 0, false, fmt.Errorf("error assigning XID %s to new company in Odoo\nERROR=%w", xid, err)
		}
		return newId, true, nil
	}
	foundId := int(found["id"].(float64))
	err = odoo.Write("res.partner", foundId, companyData, nil)
	if err != nil {
		return 0, false, fmt.Errorf("error writing company data in Odoo\nERROR=%w", err)
	}
	return foundId, false, nil
}

func ensureShopifyCustomerAddressInOdoo(customerOdooId int, address *types.Address, addressType string) (addressId int, err error) {
	addressMap := mapShopifyAddressToOdoo(address, map[string]any{
		"parent_id": customerOdooId,
		"type":      addressType,
	})
	addressXid, _ := ShopifyIdToOdooXid(*address.Id)
	addressId, err = odoo.GetIDByXID("res.partner", addressXid)
	if err != nil {
		return 0, fmt.Errorf("error searching for address %v\nERROR=%w", addressXid, err)
	}
	action := "updating"
	if addressId == 0 {
		action = "creating"
		addressId, err = odoo.Create("res.partner", addressMap, map[string]any{"xid": addressXid})
	} else {
		err = odoo.Write("res.partner", addressId, addressMap, nil)
	}
	if err != nil {
		return addressId, fmt.Errorf("error %v address %v\nERROR=%w", action, addressXid, err)
	}
	return addressId, nil
}
