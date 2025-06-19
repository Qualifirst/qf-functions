package shopifyodoo

import (
	"qf/go/shopify/adminapi/types"
	"testing"
	"time"
)

func TestShopifyIdToOdooXid_OK(t *testing.T) {
	testCases := []struct {
		Title    string
		Xid      string
		Expected string
	}{
		{
			Title:    "Full",
			Xid:      "gid://shopify/CompanyContact/123123123",
			Expected: "__export__.shopify_companycontact_123123123",
		},
		{
			Title:    "With additional params",
			Xid:      "gid://shopify/CompanyContact/123123123?SomethingHere=SomethingElse",
			Expected: "__export__.shopify_companycontact_123123123",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Title, func(t *testing.T) {
			res, err := ShopifyIdToOdooXid(tc.Xid)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res != tc.Expected {
				t.Fatalf("resulting xid does not match the expected value (expected) %s != (result) %s", tc.Expected, res)
			}
		})
	}
}

func TestShopifyIdToOdooXid_KO(t *testing.T) {
	testCases := []struct {
		Title string
		Xid   string
	}{
		{
			Title: "No number part",
			Xid:   "gid://shopify/CompanyContact/",
		},
		{
			Title: "No type part",
			Xid:   "gid://shopify//12321321",
		},
		{
			Title: "No both parts",
			Xid:   "gid://shopify//",
		},
		{
			Title: "No enough parts",
			Xid:   "gid://shopify/",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.Title, func(t *testing.T) {
			res, err := ShopifyIdToOdooXid(tc.Xid)
			if err == nil {
				t.Fatalf("expected error, but returned: %s", res)
			}
		})
	}
}

func TestComputeScheduleDate(t *testing.T) {
	testCases := []struct {
		Title        string
		OrderDate    string
		InTown       bool
		CompanyId    int
		ExpectedDate string
	}{
		// QF, In Town
		{Title: "Sunday, Before 5, In town, QF", OrderDate: "2025-06-15T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Sunday, After 5, In town, QF", OrderDate: "2025-06-15T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Saturday, Before 5, In town, QF", OrderDate: "2025-06-14T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Saturday, After 5, In town, QF", OrderDate: "2025-06-14T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Friday, Before 5, In town, QF", OrderDate: "2025-06-13T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Friday, After 5, In town, QF", OrderDate: "2025-06-13T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Thursday, Before 5, In town, QF", OrderDate: "2025-06-12T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Thursday, After 5, In town, QF", OrderDate: "2025-06-12T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Wednesday, Before 5, In town, QF", OrderDate: "2025-06-11T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Wednesday, After 5, In town, QF", OrderDate: "2025-06-11T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Tuesday, Before 5, In town, QF", OrderDate: "2025-06-10T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-11T12:00:00Z"},
		{Title: "Tuesday, After 5, In town, QF", OrderDate: "2025-06-10T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Monday, Before 5, In town, QF", OrderDate: "2025-06-09T16:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-10T12:00:00Z"},
		{Title: "Monday, After 5, In town, QF", OrderDate: "2025-06-09T21:26:59Z", InTown: true, CompanyId: 2, ExpectedDate: "2025-06-11T12:00:00Z"},
		// QF, Out of Town
		{Title: "Sunday, Before 5, Out of town, QF", OrderDate: "2025-06-15T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Sunday, After 5, Out of town, QF", OrderDate: "2025-06-15T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Saturday, Before 5, Out of town, QF", OrderDate: "2025-06-14T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Saturday, After 5, Out of town, QF", OrderDate: "2025-06-14T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Friday, Before 5, Out of town, QF", OrderDate: "2025-06-13T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Friday, After 5, Out of town, QF", OrderDate: "2025-06-13T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Thursday, Before 5, Out of town, QF", OrderDate: "2025-06-12T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Thursday, After 5, Out of town, QF", OrderDate: "2025-06-12T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Wednesday, Before 5, Out of town, QF", OrderDate: "2025-06-11T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-11T12:00:00Z"},
		{Title: "Wednesday, After 5, Out of town, QF", OrderDate: "2025-06-11T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Tuesday, Before 5, Out of town, QF", OrderDate: "2025-06-10T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-10T12:00:00Z"},
		{Title: "Tuesday, After 5, Out of town, QF", OrderDate: "2025-06-10T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-11T12:00:00Z"},
		{Title: "Monday, Before 5, Out of town, QF", OrderDate: "2025-06-09T16:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-09T12:00:00Z"},
		{Title: "Monday, After 5, Out of town, QF", OrderDate: "2025-06-09T21:26:59Z", InTown: false, CompanyId: 2, ExpectedDate: "2025-06-10T12:00:00Z"},
		// FM, In Town
		{Title: "Sunday, Before 5, In town, FM", OrderDate: "2025-06-15T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Sunday, After 5, In town, FM", OrderDate: "2025-06-15T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Saturday, Before 5, In town, FM", OrderDate: "2025-06-14T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Saturday, After 5, In town, FM", OrderDate: "2025-06-14T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Friday, Before 5, In town, FM", OrderDate: "2025-06-13T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Friday, After 5, In town, FM", OrderDate: "2025-06-13T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-17T12:00:00Z"},
		{Title: "Thursday, Before 5, In town, FM", OrderDate: "2025-06-12T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Thursday, After 5, In town, FM", OrderDate: "2025-06-12T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Wednesday, Before 5, In town, FM", OrderDate: "2025-06-11T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Wednesday, After 5, In town, FM", OrderDate: "2025-06-11T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Tuesday, Before 5, In town, FM", OrderDate: "2025-06-10T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-11T12:00:00Z"},
		{Title: "Tuesday, After 5, In town, FM", OrderDate: "2025-06-10T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Monday, Before 5, In town, FM", OrderDate: "2025-06-09T16:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-10T12:00:00Z"},
		{Title: "Monday, After 5, In town, FM", OrderDate: "2025-06-09T21:26:59Z", InTown: true, CompanyId: 3, ExpectedDate: "2025-06-11T12:00:00Z"},
		// FM, Out of Town
		{Title: "Sunday, Before 5, Out of town, FM", OrderDate: "2025-06-15T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Sunday, After 5, Out of town, FM", OrderDate: "2025-06-15T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Saturday, Before 5, Out of town, FM", OrderDate: "2025-06-14T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Saturday, After 5, Out of town, FM", OrderDate: "2025-06-14T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Friday, Before 5, Out of town, FM", OrderDate: "2025-06-13T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Friday, After 5, Out of town, FM", OrderDate: "2025-06-13T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-16T12:00:00Z"},
		{Title: "Thursday, Before 5, Out of town, FM", OrderDate: "2025-06-12T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Thursday, After 5, Out of town, FM", OrderDate: "2025-06-12T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-13T12:00:00Z"},
		{Title: "Wednesday, Before 5, Out of town, FM", OrderDate: "2025-06-11T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-11T12:00:00Z"},
		{Title: "Wednesday, After 5, Out of town, FM", OrderDate: "2025-06-11T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-12T12:00:00Z"},
		{Title: "Tuesday, Before 5, Out of town, FM", OrderDate: "2025-06-10T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-10T12:00:00Z"},
		{Title: "Tuesday, After 5, Out of town, FM", OrderDate: "2025-06-10T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-11T12:00:00Z"},
		{Title: "Monday, Before 5, Out of town, FM", OrderDate: "2025-06-09T16:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-09T12:00:00Z"},
		{Title: "Monday, After 5, Out of town, FM", OrderDate: "2025-06-09T21:26:59Z", InTown: false, CompanyId: 3, ExpectedDate: "2025-06-10T12:00:00Z"},
	}
	for _, tc := range testCases {
		t.Run(tc.Title, func(t *testing.T) {
			a := types.Address{City: "Some city"}
			var tz *time.Location
			if tc.CompanyId == 2 {
				a.CustomerProvinceCode = "ON"
				if tc.InTown {
					a.City = "Toronto"
				}
				tz, _ = time.LoadLocation("Canada/Eastern")
			} else {
				a.CustomerProvinceCode = "BC"
				if tc.InTown {
					a.City = "Vancouver"
				}
				tz, _ = time.LoadLocation("Canada/Pacific")
			}
			orderDate, err := time.Parse(time.RFC3339, tc.OrderDate)
			if err != nil {
				t.Fatalf("Unexpected error parsing date %v: %v", tc.OrderDate, err)
			}
			orderDate = time.Date(orderDate.Year(), orderDate.Month(), orderDate.Day(), orderDate.Hour(), orderDate.Minute(), orderDate.Second(), orderDate.Nanosecond(), tz).UTC()
			res, err := computeScheduledDate(orderDate, tc.CompanyId, &a)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			expected, err := time.Parse(time.RFC3339, tc.ExpectedDate)
			if err != nil {
				t.Fatalf("Unexpected error parsing date %v: %v", tc.ExpectedDate, err)
			}
			expected = time.Date(expected.Year(), expected.Month(), expected.Day(), 12, 0, 0, 0, tz).UTC()
			if res != expected {
				t.Fatalf("Incorrect resulting date. Expected=%v, Got=%v", expected, res)
			}
		})
	}
}
