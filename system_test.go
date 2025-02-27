package gocrud

// panic: unexpected call to os.Exit(0) during test
//func TestWait4CtrlC(t *testing.T) {
//	c := make(chan os.Signal, 1)
//
//	go func() {
//		time.Sleep(1 * time.Second)
//		os.Exit(0) // Simulate Ctrl-C
//	}()
//
//	select {
//	case c <- Wait4CtrlC():
//		t.Log("Ctrl-C received")
//		break
//	case <-time.After(3 * time.Second):
//		t.Error("Ctrl-C not received")
//		break
//	}
//}
