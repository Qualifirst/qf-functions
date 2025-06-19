package types

import (
	"strconv"
	"time"
)

type Edges[T any] struct {
	Edges []Edge[T] `json:"edges"`
}

func (e *Edges[T]) Length() int {
	return len(e.Edges)
}
func (e *Edges[T]) Get(i int) *T {
	return &e.Edges[i].Node
}
func (e *Edges[T]) GetCursor(i int) *string {
	return e.Edges[i].Cursor
}
func (e *Edges[T]) Iter(yield func(int, *T) bool) {
	for i, edge := range e.Edges {
		if !yield(i, &edge.Node) {
			return
		}
	}
}

type Edge[T any] struct {
	Cursor *string `json:"cursor"`
	Node   T       `json:"node"`
}

type Identifiable struct {
	Id *string `json:"id"`
}

type KeyVal struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Count struct {
	Count     float64 `json:"count"`
	Precision string  `json:"precision"`
}

type Money struct {
	AmountString string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

func (m *Money) Amount() float64 {
	if m.AmountString != "" {
		amount, _ := strconv.ParseFloat(m.AmountString, 64)
		return amount
	}
	return 0.0
}

type MoneyBag struct {
	ShopMoney        Money `json:"shopMoney"`
	PresentmentMoney Money `json:"presentmentMoney"`
}

func (m *MoneyBag) Amount() float64 {
	return m.ShopMoney.Amount()
}

type EmailAddress struct {
	EmailAddress string `json:"emailAddress"`
}

type PhoneNumber struct {
	PhoneNumber string `json:"phoneNumber"`
}

type Address struct {
	Id       *string `json:"id"`
	Phone    string  `json:"phone"`
	Address1 string  `json:"address1"`
	Address2 string  `json:"address2"`
	City     string  `json:"city"`
	Zip      string  `json:"zip"`

	// CustomerAddress fields
	Name                 string `json:"name,omitempty"`
	Company              string `json:"company,omitempty"`
	CustomerProvinceCode string `json:"provinceCode"`
	CustomerCountryCode  string `json:"countryCodeV2"`

	// CompanyLocationAddress fields
	FirstName            string `json:"firstName,omitempty"`
	LastName             string `json:"lastName,omitempty"`
	CompanyName          string `json:"companyName,omitempty"`
	Recipient            string `json:"recipient,omitempty"`
	LocationProvinceCode string `json:"zoneCode"`
	LocationCountryCode  string `json:"countryCode"`
}

func (a *Address) ProvinceCode() string {
	if a.CustomerProvinceCode != "" {
		return a.CustomerProvinceCode
	}
	return a.LocationProvinceCode
}
func (a *Address) CountryCode() string {
	if a.CustomerCountryCode != "" {
		return a.CustomerCountryCode
	}
	return a.LocationCountryCode
}

type CompanyContact struct {
	Id            *string      `json:"id"`
	Company       Identifiable `json:"company"`
	Customer      Identifiable `json:"customer"`
	Title         string       `json:"title"`
	IsMainContact bool         `json:"isMainContact"`
}

type Customer struct {
	Id                  *string          `json:"id"`
	DisplayName         string           `json:"displayName"`
	DefaultEmailAddress EmailAddress     `json:"defaultEmailAddress"`
	DefaultPhoneNumber  PhoneNumber      `json:"defaultPhoneNumber"`
	DefaultAddress      Address          `json:"defaultAddress"`
	CompanyContacts     []CompanyContact `json:"companyContactProfiles"`
}

type Company struct {
	Id             *string                `json:"id"`
	Name           string                 `json:"name"`
	Note           string                 `json:"note"`
	MainContact    CompanyContact         `json:"mainContact"`
	LocationsCount Count                  `json:"locationsCount"`
	Locations      Edges[CompanyLocation] `json:"locations"`
}

type CompanyLocation struct {
	Id              *string `json:"id"`
	Phone           string  `json:"phone"`
	Note            string  `json:"note"`
	BillingAddress  Address `json:"billingAddress"`
	ShippingAddress Address `json:"shippingAddress"`
}

type OrderTaxLine struct {
	Price          MoneyBag `json:"priceSet"`
	RatePercentage float64  `json:"ratePercentage"`
	Title          string   `json:"title"`
}

type OrderLine struct {
	Id        *string        `json:"id"`
	Name      string         `json:"name"`
	Sku       string         `json:"sku"`
	Quantity  int            `json:"currentQuantity"`
	UnitPrice MoneyBag       `json:"discountedUnitPriceSet"`
	TaxLines  []OrderTaxLine `json:"taxLines"`
}

type OrderShippingLine struct {
	Id                *string        `json:"id"`
	Title             string         `json:"title"`
	CarrierIdentifier string         `json:"carrierIdentifier"`
	Code              string         `json:"code"`
	DeliveryCategory  string         `json:"deliveryCategory"`
	Source            string         `json:"source"`
	Price             MoneyBag       `json:"discountedPriceSet"`
	TaxLines          []OrderTaxLine `json:"taxLines"`
}

type OrderTransactionInterface interface {
	GetId() *string
	GetKind() string
	GetStatus() string
	GetAmount() float64
	GetUnsettledAmount() float64
	GetAuthorizationExpiresAt() time.Time
}

type OrderParentTransaction struct {
	Id                     *string   `json:"id"`
	Kind                   string    `json:"kind"`
	Status                 string    `json:"status"`
	AmountSet              MoneyBag  `json:"amountSet"`
	TotalUnsettledSet      MoneyBag  `json:"totalUnsettledSet"`
	AuthorizationExpiresAt time.Time `json:"authorizationExpiresAt"`
}

func (t OrderParentTransaction) GetId() *string              { return t.Id }
func (t OrderParentTransaction) GetKind() string             { return t.Kind }
func (t OrderParentTransaction) GetStatus() string           { return t.Status }
func (t OrderParentTransaction) GetAmount() float64          { return t.AmountSet.Amount() }
func (t OrderParentTransaction) GetUnsettledAmount() float64 { return t.TotalUnsettledSet.Amount() }
func (t OrderParentTransaction) GetAuthorizationExpiresAt() time.Time {
	return t.AuthorizationExpiresAt
}

type OrderTransaction struct {
	Id                     *string                `json:"id"`
	Kind                   string                 `json:"kind"`
	Status                 string                 `json:"status"`
	ParentTransaction      OrderParentTransaction `json:"parentTransaction"`
	AmountSet              MoneyBag               `json:"amountSet"`
	TotalUnsettledSet      MoneyBag               `json:"totalUnsettledSet"`
	AuthorizationExpiresAt time.Time              `json:"authorizationExpiresAt"`
}

func (t OrderTransaction) GetId() *string              { return t.Id }
func (t OrderTransaction) GetKind() string             { return t.Kind }
func (t OrderTransaction) GetStatus() string           { return t.Status }
func (t OrderTransaction) GetAmount() float64          { return t.AmountSet.Amount() }
func (t OrderTransaction) GetUnsettledAmount() float64 { return t.TotalUnsettledSet.Amount() }
func (t OrderTransaction) GetAuthorizationExpiresAt() time.Time {
	return t.AuthorizationExpiresAt
}

type Order struct {
	Id                   *string            `json:"id"`
	Name                 string             `json:"name"`
	CreatedAt            time.Time          `json:"createdAt"`
	StatusPageURL        string             `json:"statusPageUrl"`
	DeliveryInstructions KeyVal             `json:"deliveryInstructions"`
	PurchaseOrderNumber  KeyVal             `json:"purchaseOrder"`
	Customer             Customer           `json:"customer"`
	CustomAttributes     []KeyVal           `json:"customAttributes"`
	BillingAddress       Address            `json:"billingAddress"`
	ShippingAddress      Address            `json:"shippingAddress"`
	Lines                Edges[OrderLine]   `json:"lineItems"`
	ShippingLine         OrderShippingLine  `json:"shippingLine"`
	Transactions         []OrderTransaction `json:"transactions"`
}

func (o *Order) CustomAttribute(key string) string {
	for _, attr := range o.CustomAttributes {
		if attr.Key == key {
			return attr.Value
		}
	}
	return ""
}
