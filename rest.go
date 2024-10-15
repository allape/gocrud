package gocrud

type R[T any] struct {
	Code    string `json:"c"`
	Message string `json:"m"`
	Data    T      `json:"d"`
}
