package gocrud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
)

// M2MHandler
// M1: model 1, should have ID field, connected by objectFieldName1 of M2MHelper
// M2: model 2, should have ID field, connected by objectFieldName2 of M2MHelper
// M2M: Many to Many, Models to Models
type M2MHandler[M1 any, M2 any, M2M any] struct {
	baseURL             string
	httpClient          *http.Client
	okayHttpStatusRange *HttpStatusRange

	ObjectFieldName1 string
	ObjectFieldName2 string

	jsonFieldName1 string
	jsonFieldName2 string
}

func (d *M2MHandler[M1, M2, M2M]) GetAll(t1IDs, t2IDs []ID, params ...SearchParams) ([]M2M, error) {
	if len(t1IDs) == 0 && len(t2IDs) == 0 {
		return nil, errors.New("t1IDs and t2IDs can not be empty at the same time")
	}

	mergedParams := MergeMaps(SearchParams{}, params...)

	if len(t1IDs) > 0 {
		mergedParams["in_"+d.jsonFieldName1] = IDsJoin(t1IDs, ",")
	}
	if len(t2IDs) > 0 {
		mergedParams["in_"+d.jsonFieldName2] = IDsJoin(t2IDs, ",")
	}

	u, err := url.Parse(d.baseURL + "/all")
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(mergedParams)
	if err != nil {
		return nil, err
	}

	res := new(R[[]M2M])

	err = MakeJSONRequest[[]M2M](d.httpClient, d.okayHttpStatusRange, u, http.MethodPost, bytes.NewReader(body), res)
	if err != nil {
		return nil, err
	} else if res == nil {
		return nil, errors.New("response is nil")
	}

	return res.Data, nil
}

func (d *M2MHandler[M1, M2, M2M]) Save(records []M2M) (int64, error) {
	u, err := url.Parse(d.baseURL + "/save")
	if err != nil {
		return -1, err
	}

	body, err := json.Marshal(records)
	if err != nil {
		return -1, err
	}

	res := new(R[int64])
	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodPut, bytes.NewReader(body), res)
	if err != nil {
		return -1, err
	}

	return res.Data, nil
}

func (d *M2MHandler[M1, M2, M2M]) SaveAfterDelete(deleteByField string, idToDelete ID, records []M2M) (int64, error) {
	if deleteByField != d.ObjectFieldName1 && deleteByField != d.ObjectFieldName2 {
		return -1, fmt.Errorf("deleteByField must be %s or %s", d.ObjectFieldName1, d.ObjectFieldName2)
	}

	jsonField := ""
	switch deleteByField {
	case d.ObjectFieldName1:
		jsonField = d.jsonFieldName1
	case d.ObjectFieldName2:
		jsonField = d.jsonFieldName2
	}

	u, err := url.Parse(fmt.Sprintf("%s/save/%s/%d", d.baseURL, jsonField, idToDelete))
	if err != nil {
		return -1, err
	}

	for index, record := range records {
		reflected := reflect.ValueOf(record)
		idField := reflected.FieldByName(deleteByField)

		if idField.Uint() != uint64(idToDelete) {
			return -1, fmt.Errorf("%s must be %d at index of %d", deleteByField, idToDelete, index)
		}
	}

	body, err := json.Marshal(records)
	if err != nil {
		return -1, err
	}

	res := new(R[int64])
	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodPost, bytes.NewReader(body), res)
	if err != nil {
		return -1, err
	}
	return res.Data, nil
}

func (d *M2MHandler[M1, M2, M2M]) Delete(id1, id2 ID) (bool, error) {
	u, err := url.Parse(fmt.Sprintf("%s?%s=%d&%s=%d", d.baseURL, url.QueryEscape(d.jsonFieldName1), id1, url.QueryEscape(d.jsonFieldName2), id2))
	if err != nil {
		return false, err
	}

	res := new(R[bool])
	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodDelete, bytes.NewReader(nil), res)
	if err != nil {
		return false, err
	}
	return res.Data, nil
}

func NewM2MHandler[M1 any, M2 any, M2M any](
	baseURL string,
	httpClient *http.Client,
	okayHttpStatusRange *HttpStatusRange,
	objectFieldName1, objectFieldName2 string,
) (*M2MHandler[M1, M2, M2M], error) {
	if objectFieldName1 == "" {
		return nil, fmt.Errorf("objectFieldName1 is empty")
	}
	if objectFieldName2 == "" {
		return nil, fmt.Errorf("objectFieldName2 is empty")
	}

	{
		reflected := reflect.TypeFor[M2M]()

		field1, ok := reflected.FieldByName(objectFieldName1)
		if !ok {
			return nil, fmt.Errorf("field %s does NOT exist in %s", objectFieldName1, reflected.Name())
		} else if field1.Type.Kind() != IDKind {
			return nil, fmt.Errorf("field %s type MUST be %s, but got %s", objectFieldName1, IDKind, field1.Type.Kind())
		}

		field2, ok := reflected.FieldByName(objectFieldName2)
		if !ok {
			return nil, fmt.Errorf("field %s does NOT exist in %s", objectFieldName2, reflected.Name())
		} else if field2.Type.Kind() != IDKind {
			return nil, fmt.Errorf("field %s type MUST be %s, but got %s", objectFieldName2, IDKind, field2.Type.Kind())
		}
	}

	jsonFields, err := GetJSONFieldNameOf[M2M](objectFieldName1, objectFieldName2)
	if err != nil {
		return nil, err
	} else if len(jsonFields) != 2 {
		return nil, fmt.Errorf("expect 2 json fields, got %d", len(jsonFields))
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if okayHttpStatusRange == nil {
		okayHttpStatusRange = &DefaultOkayHttpStatusRange
	}

	handler := &M2MHandler[M1, M2, M2M]{
		baseURL:             baseURL,
		httpClient:          httpClient,
		okayHttpStatusRange: okayHttpStatusRange,
		ObjectFieldName1:    objectFieldName1,
		ObjectFieldName2:    objectFieldName2,

		jsonFieldName1: jsonFields[0],
		jsonFieldName2: jsonFields[1],
	}
	return handler, nil
}
