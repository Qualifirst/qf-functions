package shopifyodoo

import (
	"context"
	"fmt"
	"maps"
	"qf/go/odoo"
	"qf/go/shopify/adminapi"
	"qf/go/shopify/adminapi/types"
	"strings"
)

func mapShopifyAddressToOdoo(ctx context.Context, address *types.Address, extra map[string]any) map[string]any {
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
		countryId, stateId := odoo.OdooDataManager.GetCountryAndStateIds(ctx, address.CountryCode(), address.ProvinceCode())
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

func mapShopifyCustomerToOdoo(ctx context.Context, customer *types.Customer, address *types.Address, extra map[string]any) map[string]any {
	splitId := strings.Split(*customer.Id, "/")
	ref := "SHCU" + splitId[len(splitId)-1]
	customerData := mapShopifyAddressToOdoo(ctx, address, map[string]any{
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

func ShopifyCustomerToOdoo(ctx context.Context, shopifyId string) (odooId int, isNew bool, err error) {
	customer, err := adminapi.CustomerById(ctx, shopifyId)
	if err != nil {
		return 0, false, fmt.Errorf("shopify Admin API error while getting customer information\nERROR: %w", err)
	}

	if len(customer.CompanyContacts) > 0 {
		return shopifyCompanyContactToOdoo(ctx, customer)
	}
	return shopifyIndividualToOdoo(ctx, customer)
}

func shopifyCompanyContactToOdoo(ctx context.Context, customer *types.Customer) (odooId int, isNew bool, err error) {
	contactDetails := customer.CompanyContacts[0]
	pCompanyId := contactDetails.Company.Id

	companyXid, _ := ShopifyIdToOdooXid(*pCompanyId)
	odooCompany, err := odoo.GetIDByXID(ctx, "res.partner", companyXid)
	if err != nil {
		return 0, false, fmt.Errorf("error getting company from Odoo (XID=%s)\nERROR=%w", companyXid, err)
	}
	if odooCompany == 0 {
		return 0, false, fmt.Errorf("company not found in Odoo (XID=%s)", companyXid)
	}

	company, err := adminapi.CompanyById(ctx, *pCompanyId)
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
		"parent_id": odooCompany,
		"type":      "contact",
		"function":  contactDetails.Title,
	}
	roleMain := odoo.OdooDataManager.Data.PartnerRoles.Wholesale
	if roleMain != 0 {
		if contactDetails.IsMainContact {
			extraData["contact_role_code_ids"] = []any{odoo.Command.Link(int(roleMain))}
		} else {
			extraData["contact_role_code_ids"] = []any{odoo.Command.Unlink(int(roleMain))}
		}
	}

	return shopifyCustomerToOdoo(ctx, customer, &address, extraData, nil)
}

func shopifyIndividualToOdoo(ctx context.Context, customer *types.Customer) (odooId int, isNew bool, err error) {
	createData := func() map[string]any {
		data := map[string]any{}
		if odoo.OdooDataManager.Data.CustomerTypes.Individual != 0 {
			data["customer_type_id"] = int(odoo.OdooDataManager.Data.CustomerTypes.Individual)
		}
		if odoo.OdooDataManager.Data.PaymentMethods.Shopify != 0 {
			data["customer_payment_method_id"] = int(odoo.OdooDataManager.Data.PaymentMethods.Shopify)
		}
		if odoo.OdooDataManager.Data.SalesTeams.Consumer.Id != 0 {
			data["team_id"] = int(odoo.OdooDataManager.Data.SalesTeams.Consumer.Id)
		}
		if odoo.OdooDataManager.Data.SalesTeams.Consumer.UserId != 0 {
			data["user_id"] = int(odoo.OdooDataManager.Data.SalesTeams.Consumer.UserId)
		}
		if odoo.OdooDataManager.Data.Websites.Qualifirst != 0 {
			data["website_id"] = int(odoo.OdooDataManager.Data.Websites.Qualifirst)
		}
		if odoo.OdooDataManager.Data.Pricelists.Qualizon != 0 {
			data["qf_pricelist_id"] = int(odoo.OdooDataManager.Data.Pricelists.Qualizon)
			data["fm_pricelist_id"] = int(odoo.OdooDataManager.Data.Pricelists.Qualizon)
		}
		if odoo.OdooDataManager.Data.Sources.Shopify != 0 {
			data["source_id"] = int(odoo.OdooDataManager.Data.Sources.Shopify)
		}
		return data
	}

	return shopifyCustomerToOdoo(ctx, customer, &customer.DefaultAddress, nil, createData)
}

func shopifyCustomerToOdoo(ctx context.Context, customer *types.Customer, address *types.Address, extra map[string]any, createData func() map[string]any) (odooId int, isNew bool, err error) {
	customerXid, _ := ShopifyIdToOdooXid(*customer.Id)
	foundOdooCustomer, err := odoo.GetIDByXID(ctx, "res.partner", customerXid)
	if err != nil {
		return 0, false, fmt.Errorf("error checking customer from Odoo (XID=%s)\nERROR=%w", customerXid, err)
	}
	customerOdooData := mapShopifyCustomerToOdoo(ctx, customer, address, nil)
	maps.Copy(customerOdooData, extra)
	if customerOdooData["name"] == customerOdooData["email"] || customerOdooData["country_id"] == nil || customerOdooData["country_id"] == 0 {
		return 0, false, fmt.Errorf("missing information to process customer: name, email, and country are required")
	}
	if foundOdooCustomer == 0 {
		if createData != nil {
			maps.Copy(customerOdooData, createData())
		}
		newId, err := odoo.Create(ctx, "res.partner", customerOdooData, map[string]any{"xid": customerXid})
		if err != nil {
			return 0, false, fmt.Errorf("error creating new customer in Odoo\nERROR=%w", err)
		}
		return newId, true, nil
	}
	err = odoo.Write(ctx, "res.partner", foundOdooCustomer, customerOdooData, nil)
	if err != nil {
		return 0, false, fmt.Errorf("error writing customer data in Odoo\nERROR=%w", err)
	}
	return foundOdooCustomer, false, nil
}

func ShopifyCompanyToOdoo(ctx context.Context, shopifyId string) (odooId int, isNew bool, err error) {
	company, err := adminapi.CompanyById(ctx, shopifyId)
	if err != nil {
		return 0, false, fmt.Errorf("error getting company from Shopify Admin API\nERROR=%w", err)
	}
	xid, _ := ShopifyIdToOdooXid(*company.Id)
	foundId, err := odoo.GetIDByXID(ctx, "res.partner", xid)
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

	countryId, stateId := odoo.OdooDataManager.GetCountryAndStateIds(ctx, address.CountryCode(), address.ProvinceCode())
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
	if foundId == 0 {
		if odoo.OdooDataManager.Data.CustomerTypes.Business != 0 {
			companyData["customer_type_id"] = int(odoo.OdooDataManager.Data.CustomerTypes.Business)
		}
		if odoo.OdooDataManager.Data.PaymentMethods.Shopify != 0 {
			companyData["customer_payment_method_id"] = int(odoo.OdooDataManager.Data.PaymentMethods.Shopify)
		}
		if odoo.OdooDataManager.Data.SalesTeams.Leads.Id != 0 {
			companyData["team_id"] = int(odoo.OdooDataManager.Data.SalesTeams.Leads.Id)
		}
		if odoo.OdooDataManager.Data.SalesTeams.Leads.UserId != 0 {
			companyData["user_id"] = int(odoo.OdooDataManager.Data.SalesTeams.Leads.UserId)
		}
		if odoo.OdooDataManager.Data.Websites.Qualifirst != 0 {
			companyData["website_id"] = int(odoo.OdooDataManager.Data.Websites.Qualifirst)
		}
		if odoo.OdooDataManager.Data.Pricelists.QfWholesale != 0 {
			companyData["qf_pricelist_id"] = int(odoo.OdooDataManager.Data.Pricelists.QfWholesale)
		}
		if odoo.OdooDataManager.Data.Pricelists.FmWholesale != 0 {
			companyData["fm_pricelist_id"] = int(odoo.OdooDataManager.Data.Pricelists.FmWholesale)
		}
		if odoo.OdooDataManager.Data.Sources.Shopify != 0 {
			companyData["source_id"] = int(odoo.OdooDataManager.Data.Sources.Shopify)
		}
		newId, err := odoo.Create(ctx, "res.partner", companyData, map[string]any{"xid": xid})
		if err != nil {
			return 0, false, fmt.Errorf("error creating new company in Odoo\nERROR=%w", err)
		}
		return newId, true, nil
	}
	err = odoo.Write(ctx, "res.partner", foundId, companyData, nil)
	if err != nil {
		return 0, false, fmt.Errorf("error writing company data in Odoo\nERROR=%w", err)
	}
	return foundId, false, nil
}

func ensureShopifyCustomerAddressInOdoo(ctx context.Context, customerOdooId int, address *types.Address, addressType string) (addressId int, err error) {
	addressMap := mapShopifyAddressToOdoo(ctx, address, map[string]any{
		"parent_id": customerOdooId,
		"type":      addressType,
	})
	addressXid, _ := ShopifyIdToOdooXid(*address.Id)
	addressId, err = odoo.GetIDByXID(ctx, "res.partner", addressXid)
	if err != nil {
		return 0, fmt.Errorf("error searching for address %v\nERROR=%w", addressXid, err)
	}
	action := "updating"
	if addressId == 0 {
		action = "creating"
		addressId, err = odoo.Create(ctx, "res.partner", addressMap, map[string]any{"xid": addressXid})
	} else {
		err = odoo.Write(ctx, "res.partner", addressId, addressMap, nil)
	}
	if err != nil {
		return addressId, fmt.Errorf("error %v address %v\nERROR=%w", action, addressXid, err)
	}
	return addressId, nil
}
