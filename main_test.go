package gocrud

import (
	"fmt"
)

type baseAddress struct {
	addr      string
	startPort int64
}

func (b baseAddress) NewAddress(portOffset int64) string {
	return fmt.Sprintf("%s:%d", b.addr, b.startPort+portOffset)
}

type testAddress struct {
	crud          baseAddress
	crudy         baseAddress
	fsDare        baseAddress
	fsObject      baseAddress
	fs            baseAddress
	helper        baseAddress
	index         baseAddress
	m2m           baseAddress
	model         baseAddress
	searchHandler baseAddress
}

var address = testAddress{
	crud:          baseAddress{"127.0.0.1", 8080},
	crudy:         baseAddress{"127.0.0.1", 8000},
	fsDare:        baseAddress{"127.0.0.1", 8010},
	fsObject:      baseAddress{"127.0.0.1", 8020},
	fs:            baseAddress{"127.0.0.1", 8030},
	helper:        baseAddress{"127.0.0.1", 8040},
	index:         baseAddress{"127.0.0.1", 8050},
	m2m:           baseAddress{"127.0.0.1", 8060},
	model:         baseAddress{"127.0.0.1", 8070},
	searchHandler: baseAddress{"127.0.0.1", 8090},
}
