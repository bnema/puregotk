package glib

import (
	"testing"

	"github.com/bnema/purego"
)

func TestRemoveCallbackByHandlerUnrefsPuregoCallbackSlot(t *testing.T) {
	for i := 0; i < 2100; i++ {
		cb := func(uintptr) {}
		cbPtr := uintptr(i + 1)
		refPtr := purego.NewCallbackFnPtr(&cb)
		SaveCallbackWithClosure(cbPtr, refPtr, cb)
		SaveHandlerMapping(uint(i+1), cbPtr)
		RemoveCallbackByHandler(uint(i + 1))
	}
}
