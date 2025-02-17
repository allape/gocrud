package gocrud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
)

var (
	DefaultOkCodes       = []int{200}
	ErrorBaseURLRequired = errors.New("BaseURL is required")
)

type CrudyOption[T any] interface {
	Apply(*Crudy[T]) error
}

type CrudyBasicOptions[T any] struct {
	CrudyOption[T]
	BaseURL    string
	HttpClient *http.Client
	OkCodes    *[]int
}

func (b CrudyBasicOptions[T]) Apply(crudy *Crudy[T]) error {
	crudy.baseURL = b.BaseURL
	crudy.httpClient = b.HttpClient
	crudy.okCodes = b.OkCodes
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

func NewCrudy[T any](options ...CrudyOption[T]) (*Crudy[T], error) {
	crudy := &Crudy[T]{}

	for _, option := range options {
		err := option.Apply(crudy)
		if err != nil {
			return nil, err
		}
	}

	if crudy.baseURL == "" {
		return nil, ErrorBaseURLRequired
	}

	if crudy.httpClient == nil {
		crudy.httpClient = http.DefaultClient
	}
	if crudy.okCodes == nil {
		crudy.okCodes = &DefaultOkCodes
	}

	if crudy.defaultPageSize == 0 {
		crudy.defaultPageSize = uint64(DefaultPageSize)
	}

	return crudy, nil
}

func MakeJSONRequest[T any, RR any](crudy *Crudy[T], u *url.URL, method string, body io.Reader, res *R[RR]) error {
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := crudy.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if !slices.Contains(*crudy.okCodes, resp.StatusCode) {
		return fmt.Errorf("status code: %d", resp.StatusCode)
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
		} else {
			return errors.New("golang type of R is mismatched")
		}
	}

	if res.Code != "0" {
		return errors.New(res.Message)
	}

	return nil
}

type Crudy[T any] struct {
	baseURL    string
	httpClient *http.Client
	okCodes    *[]int

	defaultPageSize uint64
}

func (c *Crudy[T]) BuildURL(uri string, searchParams map[string]string) (*url.URL, error) {
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

func (c *Crudy[T]) Page(current, size uint64, searchParams map[string]string) ([]T, error) {
	if current <= 0 {
		current = 1
	}
	if size <= 0 {
		size = c.defaultPageSize
	}

	u, err := c.BuildURL(fmt.Sprintf("/page/%d/%d", current, size), searchParams)
	if err != nil {
		return nil, err
	}

	var res R[[]T]
	err = MakeJSONRequest(c, u, http.MethodGet, nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Crudy[T]) All(searchParams map[string]string) ([]T, error) {
	u, err := c.BuildURL("/all", searchParams)
	if err != nil {
		return nil, err
	}

	var res R[[]T]
	err = MakeJSONRequest(c, u, http.MethodGet, nil, &res)
	if err != nil {
		return nil, err
	}

	return res.Data, nil
}

func (c *Crudy[T]) Count(searchParams map[string]string) (uint64, error) {
	u, err := c.BuildURL("/count", searchParams)
	if err != nil {
		return 0, err
	}

	var res R[uint64]
	err = MakeJSONRequest(c, u, http.MethodGet, nil, &res)
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
	err = MakeJSONRequest(c, u, http.MethodGet, nil, &res)
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
	err = MakeJSONRequest(c, u, http.MethodPut, bytes.NewReader(content), &res)
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
	err = MakeJSONRequest(c, u, http.MethodDelete, nil, &res)
	if err != nil {
		return false, err
	}

	return res.Data, nil
}
