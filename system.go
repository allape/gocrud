package gocrud

import (
	"os"
	"os/signal"
	"syscall"
)

func Wait4CtrlC() os.Signal {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	return <-sigs
}
