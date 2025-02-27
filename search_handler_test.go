package gocrud

import "testing"

func TestOverflowedArrayTrimmerFilter(t *testing.T) {
	maximum := 3
	array := []string{"a", "b"}

	trimmed := OverflowedArrayTrimmerFilter[string](maximum)(array)
	if len(trimmed) != len(array) {
		t.Errorf("Expected %d, got %d", len(array), len(trimmed))
	}

	array = []string{"a", "b", "c", "d", "e"}
	trimmed = OverflowedArrayTrimmerFilter[string](maximum)(array)
	if len(trimmed) != maximum {
		t.Errorf("Expected %d, got %d", maximum, len(trimmed))
	}
}
