package odoo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"
)

var CompanyQF = 2
var CompanyFM = 3
var ShippingSku = "WEBSHIP"
var TwoshipSku = "2SHIP_DELIVERY"
var DateFormat = "2006-01-02 15:04:05"

type command struct{}

func (c *command) Create(values map[string]any) []any {
	return []any{0, 0, values}
}
func (c *command) Update(id int, values map[string]any) []any {
	return []any{1, id, values}
}
func (c *command) Delete(id int) []any {
	return []any{2, id, 0}
}
func (c *command) Unlink(id int) []any {
	return []any{3, id, 0}
}
func (c *command) Link(id int) []any {
	return []any{4, id, 0}
}
func (c *command) Clear() []any {
	return []any{5, 0, 0}
}
func (c *command) Set(ids []int) []any {
	return []any{6, 0, ids}
}

var Command command

func jsonRpc(service string, method string, args []any) (any, error) {
	db := os.Getenv("ODOO_DB")
	uid := os.Getenv("ODOO_USER_ID")
	pwd := os.Getenv("ODOO_PASSWORD")
	domain := os.Getenv("ODOO_DOMAIN")
	if !(db != "" && domain != "" && uid != "" && pwd != "") {
		return nil, fmt.Errorf("invalid or incomplete Odoo environment variables")
	}

	url := fmt.Sprintf("https://%s/jsonrpc", domain)
	body := map[string]any{
		"jsonrpc": "2.0",
		"method":  "call",
		"id":      uid,
		"params": map[string]any{
			"service": service,
			"method":  method,
			"args":    append([]any{db, uid, pwd}, args...),
		},
	}

	bodyJson, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("could not serialize json for Odoo JSON-RPC call:\n>>> %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Post(url, "application/json", bytes.NewBuffer(bodyJson))
	if err != nil {
		return nil, fmt.Errorf("request error during Odoo JSON-RPC call:\n>>> %w", err)
	}
	defer response.Body.Close()

	rBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from Odoo JSON-RPC call:\n>>> %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from Odoo JSON-RPC call: [%s] %s", response.Status, string(rBody))
	}

	rJson := map[string]any{}
	err = json.Unmarshal(rBody, &rJson)
	if err != nil {
		return nil, fmt.Errorf("invalid json response from Odoo JSON-RPC call: %s\n>>> %w", string(rBody), err)
	}

	rError, rErrorExist := rJson["error"]
	if rErrorExist {
		errMsg := "error received from Odoo JSON-RPC call: %s"
		rErrorJson, err := json.MarshalIndent(rError, "", "  ")
		if err != nil {
			return nil, fmt.Errorf(errMsg, rError)
		}
		return nil, fmt.Errorf(errMsg, rErrorJson)
	}

	result, resultOk := rJson["result"]
	if !resultOk {
		return nil, fmt.Errorf("result not found in response from Odoo JSON-RPC call: %s", string(rBody))
	}

	return result, nil
}

var globalContext = &map[string]any{}

func GlobalContext(context map[string]any) (reset func()) {
	currentContext := globalContext
	maps.Copy(context, *globalContext)
	globalContext = &context
	return func() {
		globalContext = currentContext
	}
}

func JsonRpcExecuteKw(model, method string, args []any, kwargs map[string]any) (any, error) {
	if kwargs == nil {
		kwargs = map[string]any{}
	}
	ctx := *globalContext
	kwargsContext, kwargsContextFound := kwargs["context"]
	if kwargsContextFound && kwargsContext != nil && reflect.ValueOf(kwargsContext).Kind() == reflect.Map {
		maps.Copy(ctx, kwargsContext.(map[string]any))
	}
	kwargs["context"] = ctx
	return jsonRpc("object", "execute_kw", []any{model, method, args, kwargs})
}

func SearchRead(model string, domain []any, fields []string, limit int, context map[string]any) ([]map[string]any, error) {
	ctx := *globalContext
	if context != nil {
		maps.Copy(ctx, context)
	}
	records, err := JsonRpcExecuteKw(model, "search_read", []any{}, map[string]any{
		"domain":  domain,
		"fields":  fields,
		"limit":   limit,
		"context": ctx,
	})
	if err != nil {
		return nil, err
	}

	recordsListAny, ok := records.([]any)
	if !ok {
		return nil, fmt.Errorf("search_read result is not valid")
	}

	recordsListMap := make([]map[string]any, len(recordsListAny))
	for i, record := range recordsListAny {
		recordMap, ok := record.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("search_read result values are not valid")
		}
		recordsListMap[i] = recordMap
	}

	return recordsListMap, nil
}

func SearchCount(model string, domain []any, context map[string]any) (int, error) {
	count, err := JsonRpcExecuteKw(model, "search_count", []any{
		domain,
	}, map[string]any{
		"context": context,
	})
	if err != nil {
		return 0, err
	}

	countFloat, ok := count.(float64)
	if !ok {
		return 0, fmt.Errorf("search_count result is not valid")
	}

	return int(countFloat), nil
}

func SearchReadOne(model string, domain []any, fields []string, context map[string]any) (map[string]any, error) {
	count, err := SearchCount(model, domain, context)
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("search expected exactly 1 result, %d received", count)
	}
	records, err := SearchRead(model, domain, fields, 1, context)
	if err != nil {
		return nil, err
	}
	return records[0], nil
}

func SearchReadById(model string, id int, fields []string, context map[string]any) (map[string]any, error) {
	return SearchReadOne(model, []any{[]any{"id", "=", id}}, fields, nil)
}

func ReadRecordByXID(model string, xid string, fields []string) (map[string]any, error) {
	splitXid := strings.Split(xid, ".")
	if len(splitXid) != 2 {
		return nil, fmt.Errorf("invalid xid: %s", xid)
	}
	xidModule := splitXid[0]
	xidName := splitXid[1]

	modelData, err := SearchRead(
		"ir.model.data",
		[]any{
			[]any{"module", "=", xidModule},
			[]any{"name", "=", xidName},
		},
		[]string{"id", "module", "name", "model", "res_id"},
		1, nil,
	)
	if err != nil {
		return nil, err
	}

	if len(modelData) > 1 {
		return nil, fmt.Errorf("non-unique results for XID %s", xid)
	}

	if len(modelData) == 0 {
		// Nothing found, but it is not an error
		return nil, nil
	}

	foundModel := modelData[0]["model"].(string)
	if model != foundModel {
		return nil, fmt.Errorf("model mismatch for XID %s: %s != %s", xid, model, foundModel)
	}

	recordId := int(modelData[0]["res_id"].(float64))
	recordData, err := SearchReadOne(
		model,
		[]any{
			[]any{"id", "=", recordId},
		},
		fields,
		map[string]any{"active_test": false},
	)
	if err != nil {
		if strings.Contains(err.Error(), " 0 received") {
			// The XID (ir.model.data) remains in Odoo, but the related record was deleted
			// So we can safely delete the XID and return that no records were found
			Unlink("ir.model.data", int(modelData[0]["id"].(float64)), nil)
			return nil, nil
		}
		return nil, fmt.Errorf("error reading data for record %v of type %v with id %v\nERROR=%w", xid, model, recordId, err)
	}

	return recordData, nil
}

func GetIDByXID(model string, xid string) (int, error) {
	rec, err := ReadRecordByXID(model, xid, []string{"id"})
	if err != nil || rec == nil {
		return 0, err
	}
	return int(rec["id"].(float64)), nil
}

func AssignRecordXID(model string, id int, xid string) error {
	splitXid := strings.Split(xid, ".")
	if len(splitXid) != 2 {
		return fmt.Errorf("invalid xid: %s", xid)
	}
	xidModule := splitXid[0]
	xidName := splitXid[1]

	modelData, err := SearchRead(
		"ir.model.data",
		[]any{
			[]any{"module", "=", xidModule},
			[]any{"name", "=", xidName},
		},
		[]string{"module", "name", "model", "res_id"},
		1,
		nil,
	)
	if err != nil {
		return err
	}

	if len(modelData) == 1 {
		foundModel := modelData[0]["model"].(string)
		foundId := int(modelData[0]["res_id"].(float64))
		if foundModel != model || foundId != id {
			return fmt.Errorf("xid already assigned to different record %s(%d)", foundModel, foundId)
		}

		// xid already assigned to correct record
		return nil
	}

	_, err = Create("ir.model.data", map[string]any{
		"module": xidModule,
		"name":   xidName,
		"model":  model,
		"res_id": id,
	}, nil)
	if err != nil {
		return fmt.Errorf("error assigning xid %v to record %v(%v): %w", xid, model, id, err)
	}

	return nil
}

func CreateMulti(model string, data []map[string]any, context map[string]any) ([]int, error) {
	result, err := JsonRpcExecuteKw(model, "create", []any{data}, map[string]any{"context": context})
	if err != nil {
		return []int{}, err
	}

	idListAny, ok := result.([]any)
	if !ok {
		return []int{}, fmt.Errorf("invalid result from create, expected slice of numbers, got %T", result)
	}

	idListInt := make([]int, len(idListAny))
	for i, idAny := range idListAny {
		id, ok := idAny.(float64)
		if !ok {
			return []int{}, fmt.Errorf("invalid result from create, expected slice of numbers, but element %d is %T", i, idAny)
		}
		idListInt[i] = int(id)
	}

	return idListInt, nil
}

func Create(model string, data map[string]any, context map[string]any) (int, error) {
	xid, hasXid := context["xid"]
	delete(context, "xid")
	idList, err := CreateMulti(model, []map[string]any{data}, context)
	if err != nil {
		return 0, err
	}
	if hasXid {
		err := AssignRecordXID(model, idList[0], xid.(string))
		if err != nil {
			UnlinkMulti(model, idList, nil)
			return 0, fmt.Errorf("error assigning XID %v after creation of %v (%v)\nERROR=%w", xid, model, idList[0], err)
		}
	}
	return idList[0], nil
}

func WriteMulti(model string, ids []int, data map[string]any, context map[string]any) error {
	result, err := JsonRpcExecuteKw(model, "write", []any{ids, data}, map[string]any{"context": context})
	if err != nil {
		return err
	}

	_, ok := result.(bool)
	if !ok {
		return fmt.Errorf("invalid result from write, expected boolean, got %T", result)
	}

	return nil
}

func Write(model string, id int, data map[string]any, context map[string]any) error {
	return WriteMulti(model, []int{id}, data, context)
}

func SearchIds(model string, domain []any, context map[string]any) ([]int, error) {
	result, err := SearchRead(model, domain, []string{"id"}, 0, context)
	if err != nil {
		return []int{}, err
	}

	ids := make([]int, len(result))
	for i, record := range result {
		id, ok := record["id"].(float64)
		if !ok {
			return []int{}, fmt.Errorf("invalid result from search, expected slice of numbers, but element %d is %T", i, record["id"])
		}
		ids[i] = int(id)
	}

	return ids, nil
}

func SearchId(model string, domain []any, context map[string]any) (int, error) {
	ids, err := SearchIds(model, domain, context)
	if err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, fmt.Errorf("search expected exactly 1 ID, got %v", len(ids))
	}
	return ids[0], nil
}

func SearchFirstId(model string, domain []any, context map[string]any) (int, error) {
	ids, err := SearchIds(model, domain, context)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return ids[0], nil
}

func SearchWrite(model string, domain []any, data map[string]any, context map[string]any) error {
	ids, err := SearchIds(model, domain, context)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		// Nothing to write to
		return nil
	}

	return WriteMulti(model, ids, data, context)
}

func SearchWriteOne(model string, domain []any, data map[string]any, context map[string]any) error {
	ids, err := SearchIds(model, domain, context)
	if err != nil {
		return err
	}

	if len(ids) != 1 {
		return fmt.Errorf("search write expected exactly 1 match, got %d", len(ids))
	}

	return WriteMulti(model, ids, data, context)
}

func FindOrCreate(model string, domain []any, createData map[string]any, context map[string]any) (int, error) {
	ids, err := SearchIds(model, domain, context)
	if err != nil {
		return 0, err
	}

	if len(ids) > 1 {
		return 0, fmt.Errorf("write or create expected at most 1 match, got %d", len(ids))
	}

	if len(ids) == 1 {
		return ids[0], nil
	}
	id, err := Create(model, createData, context)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func FindFirstOrCreate(model string, domain []any, createData map[string]any, context map[string]any) (int, error) {
	id, err := SearchFirstId(model, domain, context)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		id, err = Create(model, createData, context)
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func WriteOrCreate(model string, domain []any, data map[string]any, writeOnlyData map[string]any, createOnlyData map[string]any, context map[string]any) (int, error) {
	createData := map[string]any{}
	maps.Copy(createData, data)
	maps.Copy(createData, createOnlyData)
	id, err := FindOrCreate(model, domain, createData, context)
	if err != nil {
		return 0, err
	}
	if writeOnlyData != nil {
		maps.Copy(data, writeOnlyData)
	}
	err = Write(model, id, data, context)
	if err != nil {
		return id, err
	}
	return id, nil
}

func UnlinkMulti(model string, ids []int, context map[string]any) error {
	_, err := JsonRpcExecuteKw(model, "unlink", []any{ids}, map[string]any{"context": context})
	if err != nil {
		return err
	}
	return nil
}

func Unlink(model string, id int, context map[string]any) error {
	return UnlinkMulti(model, []int{id}, context)
}

func GetCountryAndStateIds(countryCode string, stateCode string) (int, int) {
	var countryId, stateId int
	country, err := SearchReadOne("res.country", []any{
		[]any{"code", "=", countryCode},
	}, []string{"id"}, nil)
	if err == nil {
		countryId = int(country["id"].(float64))
		state, err := SearchReadOne("res.country.state", []any{
			[]any{"country_id", "=", countryId},
			[]any{"code", "=", stateCode},
		}, []string{"id"}, nil)
		if err == nil {
			stateId = int(state["id"].(float64))
		}
	}
	return countryId, stateId
}

func MapToDomain(dataMap map[string]any) []any {
	count := 0
	domainMap := map[int][]any{}
	for key, val := range dataMap {
		rVal := reflect.ValueOf(val)
		switch rVal.Kind() {
		case reflect.Int, reflect.Float32, reflect.Float64, reflect.Bool:
			domainMap[count] = []any{key, "=", val}
			count++
		case reflect.String:
			domainMap[count] = []any{key, "=ilike", val}
			count++
		}
	}
	domain := make([]any, count)
	for i := range count {
		domain[i] = domainMap[i]
	}
	return domain
}
