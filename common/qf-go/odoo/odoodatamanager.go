package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"qf/go/app"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

const odooDataRetryCooldown = 10 * time.Second

type odooDataPartnerRoles struct {
	Wholesale float64 `json:"wholesale"`
}

type odooDataWebsites struct {
	Qualifirst float64 `json:"qf"`
}

type odooDataPricelists struct {
	Qualizon    float64 `json:"qualizon"`
	QfWholesale float64 `json:"qf_wholesale"`
	FmWholesale float64 `json:"fm_wholesale"`
}

type odooDataCustomerTypes struct {
	Individual float64 `json:"individual"`
	Business   float64 `json:"business"`
}

type odooDataPaymentMethods struct {
	Shopify float64 `json:"shopify"`
}

type odooDataPaymentAcquirers struct {
	Shopify float64 `json:"shopify"`
}

type odooDataPaymentAcquirersPerCompany struct {
	QF odooDataPaymentAcquirers `json:"QF"`
	FM odooDataPaymentAcquirers `json:"FM"`
}

type odooDataSalesTeam struct {
	Id     float64 `json:"id"`
	UserId float64 `json:"user_id"`
}

type odooDataSalesTeams struct {
	Consumer odooDataSalesTeam `json:"consumer"`
	Leads    odooDataSalesTeam `json:"leads"`
}

type odooDataSources struct {
	Shopify float64 `json:"shopify"`
}

type odooDataTax struct {
	Id          float64 `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

type odooDataTaxesPerCompany struct {
	QF []odooDataTax `json:"QF"`
	FM []odooDataTax `json:"FM"`
}

type odooDataState struct {
	Id float64 `json:"id"`
}

type odooDataCountry struct {
	Id     float64                  `json:"id"`
	States map[string]odooDataState `json:"states"`
}

type odooDataDeliveryProducts struct {
	Webship float64 `json:"webship"`
	Twoship float64 `json:"twoship"`
}

type odooDataDeliveryCarrier struct {
	Id           float64 `json:"id"`
	Name         string  `json:"name"`
	ProductId    float64 `json:"product_id"`
	DeliveryType string  `json:"delivery_type"`
}

type odooDataStruct struct {
	CsrfToken        string                             `json:"csrf_token"`
	PartnerRoles     odooDataPartnerRoles               `json:"partner_roles"`
	Websites         odooDataWebsites                   `json:"websites"`
	Pricelists       odooDataPricelists                 `json:"pricelists"`
	CustomerTypes    odooDataCustomerTypes              `json:"customer_types"`
	DeliveryProducts odooDataDeliveryProducts           `json:"delivery_products"`
	DeliveryCarriers []odooDataDeliveryCarrier          `json:"delivery_carriers"`
	PaymentMethods   odooDataPaymentMethods             `json:"payment_methods"`
	PaymentAcquirers odooDataPaymentAcquirersPerCompany `json:"payment_acquirers"`
	SalesTeams       odooDataSalesTeams                 `json:"sales_teams"`
	Sources          odooDataSources                    `json:"sources"`
	Taxes            odooDataTaxesPerCompany            `json:"taxes"`
	Countries        map[string]odooDataCountry         `json:"countries"`
}

type odooDataManager struct {
	mu        sync.RWMutex
	Data      *odooDataStruct
	err       error
	lastFetch time.Time
}

func (m *odooDataManager) load(ctx context.Context) error {
	m.mu.RLock()
	if m.Data != nil {
		m.mu.RUnlock()
		return nil
	}
	if m.err != nil && time.Since(m.lastFetch) < odooDataRetryCooldown {
		return m.err
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Data != nil {
		return nil
	}
	if m.err != nil && time.Since(m.lastFetch) < odooDataRetryCooldown {
		return m.err
	}

	m.lastFetch = time.Now()

	domain := os.Getenv("ODOO_DOMAIN")
	key := os.Getenv("ODOO_ACCESS_KEY")
	cfKey := os.Getenv("CLOUDFLARE_BYPASS_WAF")
	if !(domain != "" && key != "" && cfKey != "") {
		m.err = fmt.Errorf("error loading Odoo Master Data: invalid or incomplete Odoo environment variables")
		return m.err
	}

	url := fmt.Sprintf("https://%s/website/action/shopify-master-data", domain)

	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		m.err = fmt.Errorf("error loading Odoo Master Data: error creating request:\n>>> %w", err)
		return m.err
	}
	request.Header.Set("Odoo-Access-Key", key)
	request.Header.Set("Cloudflare-Bypass-WAF", cfKey)
	response, err := client.Do(request)
	if err != nil {
		m.err = fmt.Errorf("error loading Odoo Master Data: request error:\n>>> %w", err)
		return m.err
	}
	defer response.Body.Close()

	rBody, err := io.ReadAll(response.Body)
	if err != nil {
		m.err = fmt.Errorf("error loading Odoo Master Data: error reading response:\n>>> %w", err)
		return m.err
	}

	if response.StatusCode != http.StatusOK {
		m.err = fmt.Errorf("error loading Odoo Master Data: non-200 response: [%s] %s", response.Status, string(rBody))
		return m.err
	}

	m.Data = &odooDataStruct{}
	decoder := json.NewDecoder(bytes.NewReader(rBody))
	if err := decoder.Decode(m.Data); err != nil {
		m.Data = nil
		m.err = fmt.Errorf("error loading Odoo Master Data: error decoding result:\n>>> %w", err)
		return m.err
	}

	m.err = nil
	return nil
}

func (m *odooDataManager) GetCountryAndStateIds(ctx context.Context, countryCode string, stateCode string) (int, int) {
	country, found := m.Data.Countries[countryCode]
	if found {
		state, found := country.States[stateCode]
		if found {
			return int(country.Id), int(state.Id)
		}
		return int(country.Id), 0
	}
	return fetchCountryAndStateIds(ctx, countryCode, stateCode)
}

func fetchCountryAndStateIds(ctx context.Context, countryCode string, stateCode string) (countryId int, stateId int) {
	countryCacheKey := []any{"res.country", countryCode}
	countryId, found := app.GetCacheValue(ctx, countryCacheKey, 0)
	if !found {
		countryId, _ = SearchId(ctx, "res.country", []any{[]any{"code", "=", countryCode}}, nil)
	}
	if countryId != 0 {
		app.SetCacheValue(ctx, countryCacheKey, countryId)
		stateCacheKey := []any{"res.country.state", countryId, stateCode}
		stateId, found = app.GetCacheValue(ctx, stateCacheKey, 0)
		if !found {
			stateId, _ = SearchId(ctx, "res.country.state", []any{
				[]any{"country_id", "=", countryId},
				[]any{"code", "=", stateCode},
			}, nil)
			if stateId != 0 {
				app.SetCacheValue(ctx, stateCacheKey, stateId)
			}
		}
	}
	return countryId, stateId
}

func (m *odooDataManager) GetTax(ctx context.Context, name string, percentage float64, companyId int) (int, error) {
	taxes := map[int][]odooDataTax{
		int(CompanyQF): OdooDataManager.Data.Taxes.QF,
		int(CompanyFM): OdooDataManager.Data.Taxes.FM,
	}[companyId]
	if taxes == nil {
		return 0, fmt.Errorf("invalid company ID %v", companyId)
	}
	for _, tax := range taxes {
		if tax.Description == name && tax.Amount == percentage {
			return int(tax.Id), nil
		}
	}
	return fetchTax(ctx, name, percentage, companyId)
}

func fetchTax(ctx context.Context, name string, percentage float64, companyId int) (int, error) {
	cacheKey := []any{"tax", name, companyId}
	taxId, found := app.GetCacheValue(ctx, cacheKey, 0)
	if found {
		return taxId, nil
	}
	taxData := map[string]any{
		"name":         name,
		"description":  name,
		"amount_type":  "percent",
		"type_tax_use": "sale",
		"amount":       percentage,
		"company_id":   companyId,
	}
	taxId, err := FindFirstOrCreate(ctx, "account.tax", MapToDomain(taxData), taxData, nil)
	if err != nil {
		return 0, err
	}
	app.SetCacheValue(ctx, cacheKey, taxId)
	return taxId, nil
}

func (m *odooDataManager) GetDeliveryCarrier(ctx context.Context, name string, deliveryType string, productId int) (int, error) {
	for _, carrier := range OdooDataManager.Data.DeliveryCarriers {
		if carrier.Name == name && carrier.DeliveryType == deliveryType && int(carrier.ProductId) == productId {
			return int(carrier.Id), nil
		}
	}
	return fetchDeliveryCarrier(ctx, name, deliveryType, productId)
}

func fetchDeliveryCarrier(ctx context.Context, name string, deliveryType string, productId int) (int, error) {
	cacheKey := []any{"carrier", name, deliveryType, productId}
	carrierId, found := app.GetCacheValue(ctx, cacheKey, 0)
	if found {
		return carrierId, nil
	}
	carrierData := map[string]any{
		"name":              name,
		"product_id":        productId,
		"delivery_type":     deliveryType,
		"company_id":        false,
		"integration_level": "rate",
	}
	carrierId, err := FindFirstOrCreate(ctx, "delivery.carrier", MapToDomain(carrierData), carrierData, nil)
	if err != nil {
		return 0, err
	}
	app.SetCacheValue(ctx, cacheKey, carrierId)
	return carrierId, nil
}

var OdooDataManager = &odooDataManager{}

func OdooDataMiddleware(function app.NetlifyFunction) app.NetlifyFunction {
	return func(ctx context.Context, request events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
		err := OdooDataManager.load(ctx)
		if err != nil {
			return app.NetlifyLogAndJsonResponse(500, "Error loading Odoo Master Data", err)
		}

		return function(ctx, request)
	}
}
