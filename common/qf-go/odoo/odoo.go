package odoo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"qf/go/app"
	"reflect"
	"strings"
	"time"
)

type companyId int

const CompanyQF = companyId(2)
const CompanyFM = companyId(3)
const ShippingSku = "WEBSHIP"
const TwoshipSku = "2SHIP_DELIVERY"
const DateFormat = "2006-01-02 15:04:05"

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

func jsonRpc(ctx context.Context, service string, method string, args []any) (any, error) {
	db := os.Getenv("ODOO_DB")
	uid := os.Getenv("ODOO_USER_ID")
	pwd := os.Getenv("ODOO_PASSWORD")
	domain := os.Getenv("ODOO_DOMAIN")
	cfKey := os.Getenv("CLOUDFLARE_BYPASS_WAF")
	if !(db != "" && domain != "" && uid != "" && pwd != "" && cfKey != "") {
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
	request, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyJson))
	if err != nil {
		return nil, fmt.Errorf("error creating request for Odoo JSON-RPC call:\n>>> %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cloudflare-Bypass-WAF", cfKey)
	response, err := client.Do(request)
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

func WithContext(ctx context.Context, odooContext map[string]any) func() {
	cacheKey := []any{"odoo", "context"}
	currentCtx, _ := app.GetCacheValue(ctx, cacheKey, map[string]any{})
	maps.Copy(odooContext, currentCtx)
	return app.SetCacheValue(ctx, cacheKey, odooContext)
}

func JsonRpcExecuteKw(ctx context.Context, model, method string, args []any, kwargs map[string]any) (any, error) {
	odooCtx, _ := app.GetCacheValue(ctx, []any{"odoo", "context"}, map[string]any{})
	if kwargs == nil {
		kwargs = map[string]any{}
	}
	kwargsCtx, kwargsCtxFound := kwargs["context"]
	if kwargsCtxFound && kwargsCtx != nil && reflect.ValueOf(kwargsCtx).Kind() == reflect.Map {
		maps.Copy(odooCtx, kwargsCtx.(map[string]any))
	}
	kwargs["context"] = odooCtx
	return jsonRpc(ctx, "object", "execute_kw", []any{model, method, args, kwargs})
}

func SearchRead(ctx context.Context, model string, domain []any, fields []string, limit int, odooContext map[string]any) ([]map[string]any, error) {
	records, err := JsonRpcExecuteKw(ctx, model, "search_read", []any{}, map[string]any{
		"domain":  domain,
		"fields":  fields,
		"limit":   limit,
		"context": odooContext,
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

func SearchCount(ctx context.Context, model string, domain []any, odooContext map[string]any) (int, error) {
	count, err := JsonRpcExecuteKw(ctx, model, "search_count", []any{
		domain,
	}, map[string]any{
		"context": odooContext,
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

func SearchReadOne(ctx context.Context, model string, domain []any, fields []string, odooContext map[string]any) (map[string]any, error) {
	records, err := SearchRead(ctx, model, domain, fields, 2, odooContext)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, fmt.Errorf("search expected exactly 1 result, %d received", len(records))
	}
	return records[0], nil
}

func SearchReadById(ctx context.Context, model string, id int, fields []string, odooContext map[string]any) (map[string]any, error) {
	return SearchReadOne(ctx, model, []any{[]any{"id", "=", id}}, fields, nil)
}

func ReadRecordByXID(ctx context.Context, model string, xid string, fields []string) (map[string]any, error) {
	imd, err := SearchIrModelData(ctx, model, xid)
	if err != nil {
		return nil, err
	}

	if !imd.Exists {
		// Nothing found, but it is not an error
		return nil, nil
	}

	recordData, err := SearchReadOne(
		ctx,
		model,
		[]any{
			[]any{"id", "=", imd.ResId},
		},
		fields,
		map[string]any{"active_test": false},
	)
	if err != nil {
		if strings.Contains(err.Error(), " 0 received") {
			// The XID (ir.model.data) remains in Odoo, but the related record was deleted
			// So we can safely delete the XID and return that no records were found
			Unlink(ctx, "ir.model.data", imd.Id, nil)
			imd.Exists = false
			xidDataCacheKey := []any{"xidData", model, xid}
			app.SetCacheValue(ctx, xidDataCacheKey, imd) // Set the value as "doesn't exist" in the cache
			return nil, nil
		}
		return nil, fmt.Errorf("error reading data for record %v of type %v with id %v\nERROR=%w", xid, model, imd.ResId, err)
	}

	return recordData, nil
}

func GetIDByXID(ctx context.Context, model string, xid string) (int, error) {
	imd, err := SearchIrModelData(ctx, model, xid)
	if err != nil {
		return 0, err
	}
	if !imd.Exists {
		// Nothing found, but it is not an error
		return 0, nil
	}
	return imd.ResId, nil
}

type IrModelData struct {
	Id     int
	Module string
	Name   string
	Model  string
	ResId  int
	Exists bool
	Xid    string
}

func PrefetchIrModelData(ctx context.Context, xids []IrModelData) error {
	domain := make([]any, 0, (len(xids)*3)+(len(xids)-1))
	for range len(xids) - 1 {
		domain = append(domain, "|")
	}
	for _, xid := range xids {
		domain = append(domain, "&")
		domain = append(domain, []any{"module", "=", xid.Module})
		domain = append(domain, []any{"name", "=", xid.Name})
	}
	res, err := SearchRead(ctx, "ir.model.data", domain, []string{"module", "name", "model", "res_id", "id"}, 0, nil)
	if err != nil {
		return err
	}
	foundMap := map[string]map[string]any{}
	for _, r := range res {
		foundMap[r["module"].(string)+"."+r["name"].(string)] = r
	}
	for _, xid := range xids {
		xidDataCacheKey := []any{"xidData", xid.Model, xid.Xid}
		foundXid, found := foundMap[xid.Xid]
		if !found {
			app.SetCacheValue(ctx, xidDataCacheKey, xid)
			continue
		}
		foundModel := foundXid["model"].(string)
		if foundModel != xid.Model {
			return fmt.Errorf("incorrect model for xid %v, expected %v, got %v", xid.Xid, xid.Model, foundModel)
		}
		xid.Exists = true
		xid.ResId = int(foundXid["res_id"].(float64))
		xid.Id = int(foundXid["id"].(float64))
		app.SetCacheValue(ctx, xidDataCacheKey, xid)
	}
	return nil
}

func SearchIrModelData(ctx context.Context, model string, xid string) (imd IrModelData, err error) {
	xidDataCacheKey := []any{"xidData", model, xid}
	if imd, found := app.GetCacheValue(ctx, xidDataCacheKey, IrModelData{}); found {
		return imd, nil
	}
	splitXid := strings.Split(xid, ".")
	if len(splitXid) != 2 {
		return imd, fmt.Errorf("invalid xid: %s", xid)
	}
	xidModule := splitXid[0]
	xidName := splitXid[1]
	imd.Module = xidModule
	imd.Name = xidName
	imd.Model = model
	imd.Xid = xid
	modelData, err := SearchRead(
		ctx,
		"ir.model.data",
		[]any{
			[]any{"module", "=", xidModule},
			[]any{"name", "=", xidName},
		},
		[]string{"id", "module", "name", "model", "res_id"},
		0, nil,
	)
	if err != nil {
		return imd, err
	}
	if len(modelData) > 1 {
		return imd, fmt.Errorf("expected at most 1 match for xid %v, got %v", xid, len(modelData))
	}
	if len(modelData) == 0 {
		// Nothing was found, but this is not an error
		app.SetCacheValue(ctx, xidDataCacheKey, imd) // Cache empty struct for the XID
		return imd, nil                              // Return empty struct with no error
	}
	foundModel := modelData[0]["model"].(string)
	if foundModel != imd.Model {
		return imd, fmt.Errorf("model mismatch for xid %v, expected %v, got %v", xid, imd.Model, foundModel)
	}
	imd.Id = int(modelData[0]["id"].(float64))
	imd.Module = modelData[0]["module"].(string)
	imd.Name = modelData[0]["name"].(string)
	imd.Xid = imd.Module + "." + imd.Name
	imd.ResId = int(modelData[0]["res_id"].(float64))
	imd.Exists = true
	app.SetCacheValue(ctx, xidDataCacheKey, imd)
	return imd, nil
}

func AssignRecordXID(ctx context.Context, model string, id int, xid string) error {
	imd, err := SearchIrModelData(ctx, model, xid)
	if err != nil {
		return err
	}

	if imd.Exists {
		// xid already assigned to correct record
		return nil
	}

	imdId, err := Create(ctx, "ir.model.data", map[string]any{
		"module": imd.Module,
		"name":   imd.Name,
		"model":  model,
		"res_id": id,
	}, nil)
	if err != nil {
		return fmt.Errorf("error assigning xid %v to record %v(%v): %w", xid, model, id, err)
	}

	imd.Id = imdId
	imd.ResId = id
	imd.Exists = true
	xidDataCacheKey := []any{"xidData", model, xid}
	app.SetCacheValue(ctx, xidDataCacheKey, imd) // Update cache to make sure it is found from this point on

	return nil
}

func CreateMulti(ctx context.Context, model string, data []map[string]any, odooContext map[string]any) ([]int, error) {
	result, err := JsonRpcExecuteKw(ctx, model, "create", []any{data}, map[string]any{"context": odooContext})
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

func Create(ctx context.Context, model string, data map[string]any, odooContext map[string]any) (int, error) {
	xid, hasXid := odooContext["xid"]
	delete(odooContext, "xid")
	idList, err := CreateMulti(ctx, model, []map[string]any{data}, odooContext)
	if err != nil {
		return 0, err
	}
	if hasXid {
		err := AssignRecordXID(ctx, model, idList[0], xid.(string))
		if err != nil {
			UnlinkMulti(ctx, model, idList, nil)
			return 0, fmt.Errorf("error assigning XID %v after creation of %v (%v)\nERROR=%w", xid, model, idList[0], err)
		}
	}
	return idList[0], nil
}

func WriteMulti(ctx context.Context, model string, ids []int, data map[string]any, odooContext map[string]any) error {
	result, err := JsonRpcExecuteKw(ctx, model, "write", []any{ids, data}, map[string]any{"context": odooContext})
	if err != nil {
		return err
	}

	_, ok := result.(bool)
	if !ok {
		return fmt.Errorf("invalid result from write, expected boolean, got %T", result)
	}

	return nil
}

func Write(ctx context.Context, model string, id int, data map[string]any, odooContext map[string]any) error {
	return WriteMulti(ctx, model, []int{id}, data, odooContext)
}

func SearchIds(ctx context.Context, model string, domain []any, odooContext map[string]any) ([]int, error) {
	result, err := SearchRead(ctx, model, domain, []string{"id"}, 0, odooContext)
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

func SearchId(ctx context.Context, model string, domain []any, odooContext map[string]any) (int, error) {
	ids, err := SearchIds(ctx, model, domain, odooContext)
	if err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, fmt.Errorf("search expected exactly 1 ID, got %v", len(ids))
	}
	return ids[0], nil
}

func SearchFirstId(ctx context.Context, model string, domain []any, odooContext map[string]any) (int, error) {
	ids, err := SearchIds(ctx, model, domain, odooContext)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return ids[0], nil
}

func SearchWrite(ctx context.Context, model string, domain []any, data map[string]any, odooContext map[string]any) error {
	ids, err := SearchIds(ctx, model, domain, odooContext)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		// Nothing to write to
		return nil
	}

	return WriteMulti(ctx, model, ids, data, odooContext)
}

func SearchWriteOne(ctx context.Context, model string, domain []any, data map[string]any, odooContext map[string]any) error {
	ids, err := SearchIds(ctx, model, domain, odooContext)
	if err != nil {
		return err
	}

	if len(ids) != 1 {
		return fmt.Errorf("search write expected exactly 1 match, got %d", len(ids))
	}

	return WriteMulti(ctx, model, ids, data, odooContext)
}

func FindOrCreate(ctx context.Context, model string, domain []any, createData map[string]any, odooContext map[string]any) (int, error) {
	ids, err := SearchIds(ctx, model, domain, odooContext)
	if err != nil {
		return 0, err
	}

	if len(ids) > 1 {
		return 0, fmt.Errorf("write or create expected at most 1 match, got %d", len(ids))
	}

	if len(ids) == 1 {
		return ids[0], nil
	}
	id, err := Create(ctx, model, createData, odooContext)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func FindFirstOrCreate(ctx context.Context, model string, domain []any, createData map[string]any, odooContext map[string]any) (int, error) {
	id, err := SearchFirstId(ctx, model, domain, odooContext)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		id, err = Create(ctx, model, createData, odooContext)
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func WriteOrCreate(ctx context.Context, model string, domain []any, data map[string]any, writeOnlyData map[string]any, createOnlyData map[string]any, odooContext map[string]any) (int, error) {
	createData := map[string]any{}
	maps.Copy(createData, data)
	maps.Copy(createData, createOnlyData)
	id, err := FindOrCreate(ctx, model, domain, createData, odooContext)
	if err != nil {
		return 0, err
	}
	if writeOnlyData != nil {
		maps.Copy(data, writeOnlyData)
	}
	err = Write(ctx, model, id, data, odooContext)
	if err != nil {
		return id, err
	}
	return id, nil
}

func UnlinkMulti(ctx context.Context, model string, ids []int, odooContext map[string]any) error {
	_, err := JsonRpcExecuteKw(ctx, model, "unlink", []any{ids}, map[string]any{"context": odooContext})
	if err != nil {
		return err
	}
	return nil
}

func Unlink(ctx context.Context, model string, id int, odooContext map[string]any) error {
	return UnlinkMulti(ctx, model, []int{id}, odooContext)
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
