package gocrud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
)

type (
	SearchParams    map[string]string
	HttpStatusRange [2]int
)

var (
	DefaultOkayHttpStatusRange = HttpStatusRange{http.StatusOK, http.StatusMultipleChoices}
	ErrorBaseURLRequired       = errors.New("BaseURL is required")
)

type CrudyOption[T any] interface {
	Apply(*Crudy[T]) error
}

type CrudyBasicOptions[T any] struct {
	CrudyOption[T]
	BaseURL             string
	HttpClient          *http.Client
	OkayHttpStatusRange *HttpStatusRange
}

func (b CrudyBasicOptions[T]) Apply(crudy *Crudy[T]) error {
	if b.BaseURL != "" {
		crudy.baseURL = b.BaseURL
	}
	if b.HttpClient != nil {
		crudy.httpClient = b.HttpClient
	}
	if b.OkayHttpStatusRange != nil {
		crudy.okayHttpStatusRange = b.OkayHttpStatusRange
	}
	return nil
}

type CrudyPageOptions[T any] struct {
	CrudyOption[T]
	DefaultSize uint64
}

func (b CrudyPageOptions[T]) Apply(crudy *Crudy[T]) error {
	crudy.defaultPageSize = b.DefaultSize
	return nil
}

func NewCrudy[T any](baseURL string, options ...CrudyOption[T]) (*Crudy[T], error) {
	crudy := &Crudy[T]{
		baseURL: baseURL,
	}

	for _, option := range options {
		err := option.Apply(crudy)
		if err != nil {
			return nil, err
		}
	}

	if crudy.baseURL == "" {
		return nil, ErrorBaseURLRequired
	}

	_, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	if crudy.httpClient == nil {
		crudy.httpClient = http.DefaultClient
	}
	if crudy.okayHttpStatusRange == nil {
		crudy.okayHttpStatusRange = &DefaultOkayHttpStatusRange
	}

	if crudy.defaultPageSize == 0 {
		crudy.defaultPageSize = DefaultPageSize
	}

	return crudy, nil
}

func MakeJSONRequest[T any](
	httpClient *http.Client, okayHttpStatusRange *HttpStatusRange,
	u *url.URL, method string,
	body io.Reader, res *R[T],
) error {
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if okayHttpStatusRange != nil {
		if resp.StatusCode < okayHttpStatusRange[0] || resp.StatusCode >= okayHttpStatusRange[1] {
			return fmt.Errorf("status code: %d", resp.StatusCode)
		}
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(content, res)
	if err != nil {
		var anyRes R[any]
		err = json.Unmarshal(content, &anyRes)
		if err != nil {
			return err
		}

		if anyRes.Code != "0" {
			return errors.New(anyRes.Message)
		}

		reflected := reflect.TypeOf(new(T)).Elem()
		return fmt.Errorf("unable to convert from %v to %s", anyRes.Data, reflected.Name())
	}

	if res.Code != "0" {
		return errors.New(res.Message)
	}

	return nil
}

type Crudy[T any] struct {
	baseURL             string
	httpClient          *http.Client
	okayHttpStatusRange *HttpStatusRange // okayHttpStatusRange[0] <= status code < okayHttpStatusRange[1]

	defaultPageSize uint64
}

func (c *Crudy[T]) BuildURL(uri string, searchParams SearchParams) (*url.URL, error) {
	u, err := url.Parse(c.baseURL + uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for k, v := range searchParams {
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()

	return u, nil
}

func (c *Crudy[T]) Page(current, size uint64, searchParams SearchParams) ([]T, error) {
	if current <= 0 {
		current = 1
	}
	if size <= 0 {
		size = c.defaultPageSize
	}

	u, err := c.BuildURL(fmt.Sprintf("/page/%d/%d", current, size), nil)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(searchParams)
	if err != nil {
		return nil, err
	}

	var res R[[]T]
	err = MakeJSONRequest(c.httpClient, c.okayHttpStatusRange, u, http.MethodPost, bytes.NewReader(body), &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Crudy[T]) All(searchParams SearchParams) ([]T, error) {
	u, err := c.BuildURL("/all", nil)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(searchParams)
	if err != nil {
		return nil, err
	}

	var res R[[]T]
	err = MakeJSONRequest(c.httpClient, c.okayHttpStatusRange, u, http.MethodPost, bytes.NewReader(body), &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Crudy[T]) Count(searchParams SearchParams) (uint64, error) {
	u, err := c.BuildURL("/count", searchParams)
	if err != nil {
		return 0, err
	}

	var res R[uint64]
	err = MakeJSONRequest(c.httpClient, c.okayHttpStatusRange, u, http.MethodGet, nil, &res)
	if err != nil {
		return 0, err
	}

	return res.Data, nil
}

func (c *Crudy[T]) One(id ID) (*T, error) {
	u, err := c.BuildURL(fmt.Sprintf("/one/%d", id), nil)
	if err != nil {
		return nil, err
	}

	var res R[T]
	err = MakeJSONRequest(c.httpClient, c.okayHttpStatusRange, u, http.MethodGet, nil, &res)
	if err != nil {
		return nil, err
	}

	return &res.Data, nil
}

func (c *Crudy[T]) Save(t *T) (*T, error) {
	u, err := c.BuildURL("", nil)
	if err != nil {
		return nil, err
	}

	content, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	var res R[T]
	err = MakeJSONRequest(c.httpClient, c.okayHttpStatusRange, u, http.MethodPut, bytes.NewReader(content), &res)
	if err != nil {
		return nil, err
	}

	return &res.Data, nil
}

func (c *Crudy[T]) Delete(id ID) (bool, error) {
	u, err := c.BuildURL(fmt.Sprintf("/%d", id), nil)
	if err != nil {
		return false, err
	}

	var res R[bool]
	err = MakeJSONRequest(c.httpClient, c.okayHttpStatusRange, u, http.MethodDelete, nil, &res)
	if err != nil {
		return false, err
	}

	return res.Data, nil
}
