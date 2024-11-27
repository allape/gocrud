package gocrud

import (
	"fmt"
	"net/http"
)

type Code string

type Coder interface {
	OK() Code
	InternalServerError() Code
	BadRequest() Code
	NotFound() Code
	MethodNotAllowed() Code
	Conflict() Code

	From(code string) Code
	FromStatus(status int) Code
}

type DefaultCoder struct {
	Coder
}

func (d *DefaultCoder) OK() Code {
	return d.From("0")
}

func (d *DefaultCoder) InternalServerError() Code {
	return d.FromStatus(http.StatusInternalServerError)
}

func (d *DefaultCoder) BadRequest() Code {
	return d.FromStatus(http.StatusBadRequest)
}

func (d *DefaultCoder) NotFound() Code {
	return d.FromStatus(http.StatusNotFound)
}

func (d *DefaultCoder) MethodNotAllowed() Code {
	return d.FromStatus(http.StatusMethodNotAllowed)
}

func (d *DefaultCoder) Conflict() Code {
	return d.FromStatus(http.StatusConflict)
}

func (d *DefaultCoder) From(code string) Code {
	return Code(code)
}

func (d *DefaultCoder) FromStatus(status int) Code {
	return Code(fmt.Sprintf("%d", status))
}

func NewDefaultCoder() Coder {
	return &DefaultCoder{}
}
