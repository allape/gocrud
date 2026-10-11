package gocrud

import (
	"fmt"
)

type M2MCrudy[T any] struct {
	*Crudy[T]

	JSONFieldName1 string
	JSONFieldName2 string
}

func (c *M2MCrudy[T]) Save(record *T) (*T, error) {
	return c.Crudy.Save(record)
}

func (c *M2MCrudy[T]) Page(current, size uint64, searchParams ...SearchParams) ([]T, error) {
	return c.Crudy.Page(current, size, searchParams...)
}

func (c *M2MCrudy[T]) All(searchParams ...SearchParams) ([]T, error) {
	return c.Crudy.All(searchParams...)
}

func (c *M2MCrudy[T]) Count(searchParams ...SearchParams) (uint64, error) {
	return c.Crudy.Count(searchParams...)
}

func (c *M2MCrudy[T]) One(id1 ID, id2 ID, searchParams ...SearchParams) (*T, error) {
	return c.Crudy.One(0, MergeMaps(SearchParams{
		c.JSONFieldName1: fmt.Sprintf("%d", id1),
		c.JSONFieldName2: fmt.Sprintf("%d", id2),
	}, searchParams...))
}

func (c *M2MCrudy[T]) Delete(id1 ID, id2 ID, searchParams ...SearchParams) (bool, error) {
	return c.Crudy.Delete(0, MergeMaps(SearchParams{
		c.JSONFieldName1: fmt.Sprintf("%d", id1),
		c.JSONFieldName2: fmt.Sprintf("%d", id2),
	}, searchParams...))
}

func NewM2MCrudy[T any](
	JSONFieldName1, JSONFieldName2,
	baseURL string, options ...CrudyOption[T],
) (*M2MCrudy[T], error) {
	crudy, err := NewCrudy[T](baseURL, options...)
	if err != nil {
		return nil, err
	}
	return &M2MCrudy[T]{
		Crudy: crudy,

		JSONFieldName1: JSONFieldName1,
		JSONFieldName2: JSONFieldName2,
	}, nil
}
