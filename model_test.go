package gocrud

import (
	"slices"
	"testing"
)

func TestIDsFromCommaSplitString(t *testing.T) {
	ids := IDsFromCommaSplitString("1,           2, 3, 4, 5, , asd, w233, ---,,,,,6")
	if slices.Compare(ids, []ID{1, 2, 3, 4, 5, 6}) != 0 {
		t.Error("ids length should be 5")
	}
}

func TestModelContains(t *testing.T) {
	users := []User{
		{Base: Base{ID: 1}},
		{Base: Base{ID: 2}},
		{Base: Base{ID: 3}},
	}

	if ContainsByBase(users, User{Base: Base{ID: 1}}) != true {
		t.Error("should contain")
	}

	if ContainsByBase(users, User{Base: Base{ID: 4}}) != false {
		t.Error("should not contain")
	}

	if ContainsByID(users, 1) != true {
		t.Error("should contain")
	}

	if ContainsByID(users, 4) != false {
		t.Error("should not contain")
	}
}
