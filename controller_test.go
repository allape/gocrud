package gocrud

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/allape/gogger"
)

func mustBeURL(urlStr string, searchParams ...SearchParams) *url.URL {
	u, err := url.Parse(urlStr)
	if err != nil {
		panic(err)
	}

	query := u.Query()
	for _, params := range searchParams {
		for key, value := range params {
			query.Add(key, value)
		}
	}
	u.RawQuery = query.Encode()

	return u
}

func mustBeReader[T any](record T) io.Reader {
	data, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(data)
}

func TestSetupDualPrimaryKeyModelController(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	err = SetupDualPrimaryKeyModelController[UserTag](
		engine.Group("/user-tag"), db, gogger.New("controller:user-tag"),
		"UserID", "TagID",
		"user_id", "tag_id",
	)
	if err != nil {
		t.Fatal(err)
	}

	var binding = "127.0.0.1:8010"
	var addr = "http://" + binding

	go func() {
		_ = engine.Run(binding)
	}()

	t.Logf("Server started on %s", binding)

	wait(t)

	// test basic save
	saveR := new(R[int64])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/save"), http.MethodPut,
		mustBeReader([]UserTag{{UserID: 123, TagID: 456}}),
		saveR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if saveR.Data != 1 {
		t.Errorf("expected %d, got %d", 1, saveR.Data)
	}

	// test "get all" of the saved records of above
	allR := new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all?in_userId=123"), http.MethodGet,
		nil,
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 1 {
		t.Fatalf("got %d, want 1", len(allR.Data))
	}
	if allR.Data[0].UserID != 123 {
		t.Fatalf("got %d, want 123", allR.Data[0].UserID)
	}
	if allR.Data[0].TagID != 456 {
		t.Fatalf("got %d, want 456", allR.Data[0].TagID)
	}

	// test basic delete
	deleteR := new(R[int64])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag?userId=234&tagId=567"), http.MethodDelete,
		nil,
		deleteR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if deleteR.Data != 0 {
		t.Fatalf("got %d, want 0", deleteR.Data)
	}

	deleteR = new(R[int64])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag?userId=123&tagId=456"), http.MethodDelete,
		nil,
		deleteR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if deleteR.Data != 1 {
		t.Fatalf("got %d, want 1", deleteR.Data)
	}

	batchSaveR := new(R[int64])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/save/userId/123"), http.MethodPost,
		mustBeReader([]UserTag{
			{UserID: 456, TagID: 10}, // UserID should be 123, therefore expecting an error
			{UserID: 123, TagID: 11},
		}),
		batchSaveR,
	)
	if err == nil {
		t.Fatalf("got nil, want error")
	}

	// test batch save
	batchSaveR = new(R[int64])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/save/userId/123"), http.MethodPost,
		mustBeReader([]UserTag{
			{UserID: 123, TagID: 1},
			{UserID: 123, TagID: 2},
			{UserID: 123, TagID: 3},
			{UserID: 123, TagID: 5},
		}),
		batchSaveR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if batchSaveR.Data != 4 {
		t.Fatalf("got %d, want 4", batchSaveR.Data)
	}

	// region

	// revalidate what has been done by above
	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all?in_userId=123"), http.MethodGet,
		nil,
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 4 {
		t.Fatalf("got %d, want 4", len(allR.Data))
	}
	if !slices.ContainsFunc(allR.Data, func(tag UserTag) bool {
		return tag.UserID == 123 && tag.TagID == 5
	}) {
		t.Fatalf("should contain TagID of 5")
	}
	if slices.ContainsFunc(allR.Data, func(tag UserTag) bool {
		return tag.UserID == 123 && tag.TagID == 4
	}) {
		t.Fatalf("should not contain TagID of 4")
	}

	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all?in_userId=456"), http.MethodGet,
		nil,
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 0 {
		t.Fatalf("got %d, want 0", len(allR.Data))
	}

	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all?in_tagId=5"), http.MethodGet,
		nil,
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 1 {
		t.Fatalf("got %d, want 1", len(allR.Data))
	}

	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all?in_tagId=4"), http.MethodGet,
		nil,
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 0 {
		t.Fatalf("got %d, want 0", len(allR.Data))
	}

	// test "get all" in POST
	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all"), http.MethodPost,
		mustBeReader(map[string]string{"in_userId": "123"}),
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 4 {
		t.Fatalf("got %d, want 4", len(allR.Data))
	}

	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all"), http.MethodPost,
		mustBeReader(map[string]string{"in_userId": "456"}),
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 0 {
		t.Fatalf("got %d, want 0", len(allR.Data))
	}

	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all"), http.MethodPost,
		mustBeReader(map[string]string{"in_tagId": "5"}),
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 1 {
		t.Fatalf("got %d, want 1", len(allR.Data))
	}

	// endregion

	// test delete old records before saving new records
	batchSaveR = new(R[int64])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/save/userId/123"), http.MethodPost,
		mustBeReader([]UserTag{
			{UserID: 123, TagID: 10},
			{UserID: 123, TagID: 11},
		}),
		batchSaveR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if batchSaveR.Data != 2 {
		t.Fatalf("got %d, want 2", batchSaveR.Data)
	}

	// revalidate what has been done by above
	allR = new(R[[]UserTag])
	err = MakeJSONRequest(
		http.DefaultClient, &DefaultOkayHttpStatusRange,
		mustBeURL(addr+"/user-tag/all?in_userId=123"), http.MethodGet,
		nil,
		allR,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(allR.Data) != 2 {
		t.Fatalf("got %d, want 2", len(allR.Data))
	}
}

func TestNewDualPrimaryKeyModelHandler(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	err = SetupDualPrimaryKeyModelController[UserTag](
		engine.Group("/user-tag"), db, gogger.New("controller:user-tag"),
		"UserID", "TagID",
		"user_id", "tag_id",
	)
	if err != nil {
		t.Fatal(err)
	}

	var binding = "127.0.0.1:8011"
	var addr = "http://" + binding

	go func() {
		_ = engine.Run(binding)
	}()

	t.Logf("Server started on %s", binding)

	wait(t)

	handler, err := NewDualPrimaryKeyModelHandler[User, Tag, UserTag](
		addr+"/user-tag", nil, nil,
		"UserID", "TagID",
	)
	if err != nil {
		t.Fatal(err)
	}

	count, err := handler.Save([]UserTag{
		{UserID: 123, TagID: 456},
	})
	if err != nil {
		t.Fatal(err)
	} else if count != 1 {
		t.Fatalf("got %d, want 1", count)
	}

	all, err := handler.GetAll([]ID{123}, nil)
	if err != nil {
		t.Fatal(err)
	} else if len(all) != 1 {
		t.Fatalf("got %d, want 1", len(all))
	} else if all[0].UserID != 123 {
		t.Fatalf("got %d, want 123", all[0].UserID)
	} else if all[0].TagID != 456 {
		t.Fatalf("got %d, want 456", all[0].TagID)
	}

	count, err = handler.Delete(123, 567)
	if err != nil {
		t.Fatal(err)
	} else if count != 0 {
		t.Fatalf("got %d, want 0", count)
	}

	count, err = handler.Delete(123, 456)
	if err != nil {
		t.Fatal(err)
	} else if count != 1 {
		t.Fatalf("got %d, want 1", count)
	}

	count, err = handler.SaveAfterDelete(handler.ObjectFieldName1, 123, []UserTag{
		{UserID: 456, TagID: 10},
		{UserID: 123, TagID: 11},
	})
	if err == nil {
		t.Fatalf("got nil, want error")
	}

	count, err = handler.SaveAfterDelete(handler.ObjectFieldName1, 123, []UserTag{
		{UserID: 123, TagID: 1},
		{UserID: 123, TagID: 2},
		{UserID: 123, TagID: 3},
		{UserID: 123, TagID: 5},
	})
	if err != nil {
		t.Fatal(err)
	} else if count != 4 {
		t.Fatalf("got %d, want 4", count)
	}

	all, err = handler.GetAll([]ID{123}, nil)
	if err != nil {
		t.Fatal(err)
	} else if len(all) != 4 {
		t.Fatalf("got %d, want 4", len(all))
	}
	if !slices.ContainsFunc(all, func(tag UserTag) bool {
		return tag.UserID == 123 && tag.TagID == 5
	}) {
		t.Fatalf("should contain TagID of 5")
	}
	if slices.ContainsFunc(all, func(tag UserTag) bool {
		return tag.UserID == 123 && tag.TagID == 4
	}) {
		t.Fatalf("should not contain TagID of 4")
	}

	all, err = handler.GetAll([]ID{456}, nil)
	if err != nil {
		t.Fatal(err)
	} else if len(all) != 0 {
		t.Fatalf("got %d, want 0", len(all))
	}

	all, err = handler.GetAll(nil, []ID{5})
	if err != nil {
		t.Fatal(err)
	} else if len(all) != 1 {
		t.Fatalf("got %d, want 1", len(all))
	}

	all, err = handler.GetAll(nil, []ID{4})
	if err != nil {
		t.Fatal(err)
	} else if len(all) != 0 {
		t.Fatalf("got %d, want 0", len(all))
	}

	count, err = handler.SaveAfterDelete(handler.ObjectFieldName1, 123, []UserTag{
		{UserID: 123, TagID: 10},
		{UserID: 123, TagID: 11},
	})
	if err != nil {
		t.Fatal(err)
	} else if count != 2 {
		t.Fatalf("got %d, want 2", count)
	}

	all, err = handler.GetAll([]ID{123}, nil)
	if err != nil {
		t.Fatal(err)
	} else if len(all) != 2 {
		t.Fatalf("got %d, want 2", len(all))
	}
}
