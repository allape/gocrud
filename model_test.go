package gocrud

import (
	"slices"
	"testing"
)

func TestIDsFromCommaSplitString(t *testing.T) {
	ids := IDsFromCommaSeparatedString("1,           2, 3, 4, 5, , asd, w233, ---,,,,,6")
	if slices.Compare(ids, []ID{1, 2, 3, 4, 5, 6}) != 0 {
		t.Error("ids length should be 5")
	}
}
