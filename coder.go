package gocrud

type Code string

type Coder interface {
	OK() Code
	InternalServerError() Code
	BadRequest() Code
	NotFound() Code
	Conflict() Code
	New(code string) Code
}

type DefaultCoder struct {
	Coder
}

func (d *DefaultCoder) OK() Code {
	return d.New("0")
}

func (d *DefaultCoder) InternalServerError() Code {
	return d.New("500")
}

func (d *DefaultCoder) BadRequest() Code {
	return d.New("400")
}

func (d *DefaultCoder) NotFound() Code {
	return d.New("404")
}

func (d *DefaultCoder) Conflict() Code {
	return d.New("409")
}

func (d *DefaultCoder) New(code string) Code {
	return Code(code)
}
