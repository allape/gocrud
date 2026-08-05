package gocrud

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
)

func Ternary[T any](condition bool, onTrue T, onFalse T) T {
	if condition {
		return onTrue
	}
	return onFalse
}

func TernaryFunc[T any](conditionFunc func() bool, onTrueFunc, onFalseFunc func() T) T {
	if conditionFunc() {
		return onTrueFunc()
	}
	return onFalseFunc()
}

func Pick[T any](arr []T, index int, defaultValue T) T {
	if index < 0 || index >= len(arr) {
		return defaultValue
	}
	return arr[index]
}

func PickFirstValuableString(array []string) (string, bool) {
	if len(array) > 0 && array[0] != "" {
		return array[0], true
	}
	return "", false
}

func NowString(patternOrEmpty string) string {
	return time.Now().Format(Ternary(patternOrEmpty == "", "2006-01-02 15:04:05.000", patternOrEmpty))
}

func MapFuncOverCommaSeparatedString(mapFunc func(string), css string) {
	for s := range strings.SplitSeq(css, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		mapFunc(s)
	}
}

func StringArrayFromCommaSeparatedString(css string) []string {
	var array []string
	MapFuncOverCommaSeparatedString(func(s string) {
		array = append(array, s)
	}, css)
	return array
}

func RemoveDuplication[T ~[]E, E comparable](array T) T {
	fresh := make(T, 0, len(array))
	for _, v := range array {
		if slices.Contains(fresh, v) {
			continue
		}
		fresh = append(fresh, v)
	}
	return fresh
}

func IsNotEmptyArray(v any) bool {
	k := reflect.ValueOf(v)

	if k.Kind() == reflect.Pointer {
		k = k.Elem()
	}

	if k.Kind() != reflect.Array && k.Kind() != reflect.Slice {
		return false
	}

	return k.Len() > 0
}

func GetJSONFieldNameOf[T any](fields ...string) ([]string, error) {
	reflected := reflect.TypeFor[T]()

	jsonFieldNames := make([]string, len(fields))

	for i, field := range fields {
		objectField, ok := reflected.FieldByName(field)
		if !ok {
			return nil, fmt.Errorf("field %s not found", field)
		}
		jsonTag := objectField.Tag.Get("json")

		frags := strings.Split(jsonTag, ",")
		if len(frags) == 0 {
			return nil, fmt.Errorf("field %s has no json tag", field)
		}
		jsonFieldNames[i] = strings.TrimSpace(frags[0])
	}

	return jsonFieldNames, nil
}

func IDsJoin(ids []ID, sep string) string {
	strIds := make([]string, len(ids))
	for i, id := range ids {
		strIds[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(strIds, sep)
}

func SHASum256(src []byte) []byte {
	mk := sha256.Sum256(src)
	return mk[:]
}

func SHASum256FromString(s string) []byte {
	return SHASum256([]byte(s))
}

func HexedSHASum256(src []byte) string {
	//return strings.ToLower(hex.EncodeToString(SHASum256(src)))
	return hex.EncodeToString(SHASum256(src))
}
