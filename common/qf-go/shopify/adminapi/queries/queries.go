package queries

// STRUCTS

type ShopifyQuery struct {
	ResultKey string
	Query     string
}

// FRAGMENTS

var mailingAddressFragment = `
fragment MailingAddressFields on MailingAddress {
	id
	name
	phone
	address1
	address2
	city
	provinceCode
	countryCodeV2
	zip
}
`

var companyContactFragment = `
fragment CompanyContactFields on CompanyContact {
	id
	company {
		id
	}
	customer {
		id
	}
	title
	isMainContact
}
`

var customerFragment = companyContactFragment + mailingAddressFragment + `
fragment CustomerFields on Customer {
	id
	displayName
	defaultEmailAddress {
		emailAddress
	}
	defaultPhoneNumber {
		phoneNumber
	}
	defaultAddress {
		...MailingAddressFields
	}
	companyContactProfiles {
		...CompanyContactFields
	}
}
`

var companyAddressFields = `
fragment CompanyAddressFields on CompanyAddress {
	id
	firstName
	lastName
	companyName
	recipient
	address1
	address2
	city
	zoneCode
	countryCode
	zip
	phone
}
`

var companyLocationFragment = companyAddressFields + `
fragment CompanyLocationFields on CompanyLocation {
	id
	phone
	note
	billingAddress {
		...CompanyAddressFields
	}
	shippingAddress {
		...CompanyAddressFields
	}
}
`

var companyFragment = companyContactFragment + companyLocationFragment + `
fragment CompanyFields on Company {
	id
	name
	note
	mainContact {
		...CompanyContactFields
	}
	locationsCount {
		count
		precision
	}
	locations(first:1) {
		edges {
			cursor
			node {
				...CompanyLocationFields
			}
		}
	}
}
`

var moneyFragment = `
fragment MoneyFields on MoneyV2 {
	amount
	currencyCode
}
`

var moneyBagFragment = moneyFragment + `
fragment MoneyBagFields on MoneyBag {
    shopMoney {
        ...MoneyFields
    }
    presentmentMoney {
        ...MoneyFields
    }
}
`

var orderMinFragment = `
fragment OrderMinFields on Order {
	id
	name
	customer {
		id
	}
	customAttributes {
		key
		value
	}
}
`

var orderFragment = orderMinFragment + mailingAddressFragment + moneyBagFragment + `
fragment OrderFields on Order {
	...OrderMinFields
	createdAt
    statusPageUrl
	billingAddress {
		...MailingAddressFields
	}
	shippingAddress {
		...MailingAddressFields
	}
	deliveryInstructions: metafield(namespace: "checkoutblocks", key: "delivery_instructions") {
		value
	}
	purchaseOrder: metafield(namespace: "checkoutblocks", key: "purchase_order") {
		value
	}
	lineItems(first: 250) {
		edges {
			node {
				id
				name
				sku
				currentQuantity
				discountedUnitPriceSet {
					...MoneyBagFields
				}
				taxLines {
					priceSet {
						...MoneyBagFields
					}
					ratePercentage
					title
				}
			}
		}
	}
	shippingLine {
		id
		title
		carrierIdentifier
		code
		deliveryCategory
		source
		discountedPriceSet {
			...MoneyBagFields
		}
		taxLines {
			priceSet {
				...MoneyBagFields
			}
			ratePercentage
			title
		}
	}
}
`

var orderTransactionFragment = moneyBagFragment + `
fragment OrderTransactionFields on OrderTransaction {
	id
	kind
	status
	amountSet {
		...MoneyBagFields
	}
	totalUnsettledSet {
		...MoneyBagFields
	}
	authorizationExpiresAt
}
`

var orderWithTransactionsFragment = orderTransactionFragment + `
fragment OrderWithTransactionsFields on Order {
	id
	transactions {
		...OrderTransactionFields
		parentTransaction {
			...OrderTransactionFields
		}
	}
}
`

// QUERIES

// Unmarshall to: types.Customer
var Customer = ShopifyQuery{
	ResultKey: "customer",
	Query: customerFragment + `
query ($id: ID!) {
	customer(id: $id) {
		...CustomerFields
	}
}
`,
}

// Unmarshall to: types.Company
var Company = ShopifyQuery{
	ResultKey: "company",
	Query: companyFragment + `
query ($id: ID!) {
	company(id: $id) {
		...CompanyFields
	}
}
`,
}

// Unmarshall to: types.Order
var OrderMinimal = ShopifyQuery{
	ResultKey: "order",
	Query: orderMinFragment + `
query ($id: ID!) {
	order(id: $id) {
		...OrderMinFields
	}
}
`,
}

// Unmarshall to: types.Order
var Order = ShopifyQuery{
	ResultKey: "order",
	Query: orderFragment + `
query ($id: ID!) {
	order(id: $id) {
		...OrderFields
	}
}
`,
}

// Unmarshall to: types.Order
var OrderWithTransactions = ShopifyQuery{
	ResultKey: "order",
	Query: orderWithTransactionsFragment + `
query ($id: ID!) {
	order(id: $id) {
		...OrderWithTransactionsFields
	}
}
`,
}
