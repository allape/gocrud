package gocrud

import "testing"

// TestStartServer this will never pass
//
//goland:noinspection GoUnusedFunction
func _TestStartServer(t *testing.T) {
	router := startServer(t)
	static := router.Group("/static")
	err := NewHttpFileSystem(static, TestData, HttpFileSystemConfig{
		AllowOverwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = router.Run(HttpBinding)
}
