package gocrud

type Code string

type Coder interface {
	OK() Code
	InternalServerError() Code
	BadRequest() Code
	NotFound() Code
	Conflict() Code
}

type DefaultCoder struct {
	Coder
}

func (DefaultCoder) OK() Code {
	return "0"
}

func (DefaultCoder) InternalServerError() Code {
	return "500"
}

func (DefaultCoder) BadRequest() Code {
	return "400"
}

func (DefaultCoder) NotFound() Code {
	return "404"
}

func (DefaultCoder) Conflict() Code {
	return "409"
}
