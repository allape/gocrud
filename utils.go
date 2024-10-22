package gocrud

import "time"

func Ternary[T any](condition bool, onTrue T, onFalse T) T {
	if condition {
		return onTrue
	}
	return onFalse
}

func Pick[T any](arr []T, index int, defaultValue T) T {
	if index < 0 || index >= len(arr) {
		return defaultValue
	}
	return arr[index]
}

func ValuableString(str *string, ifEmptyValue string) string {
	if str == nil || *str == "" {
		return ifEmptyValue
	}
	return *str
}

func ValuableStringFromArray(array []string) (bool, string) {
	if len(array) > 0 && array[0] != "" {
		return true, array[0]
	}
	return false, ""
}

func Pointer[T any](t T) *T {
	return &t
}

func NowString(pattern *string) string {
	return time.Now().Format(ValuableString(pattern, "2006-01-02 15:04:05.000"))
}
