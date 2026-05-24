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

func TestGetCallbackAcquiresReferenceUntilHandlerMappingRemoved(t *testing.T) {
	cb := func(uintptr) {}
	cbPtr := uintptr(0xace)
	refPtr := purego.NewCallbackFnPtr(&cb)
	SaveCallbackWithClosure(cbPtr, refPtr, cb)
	SaveHandlerMapping(1, cbPtr)

	acquiredRefPtr, ok := GetCallback(cbPtr)
	if !ok {
		t.Fatal("GetCallback returned ok=false")
	}
	if acquiredRefPtr != refPtr {
		t.Fatalf("GetCallback ref = %x, want %x", acquiredRefPtr, refPtr)
	}

	RemoveCallbackByHandler(1)
	callbacks.RLock()
	_, stillRegistered := callbacks.refs[cbPtr]
	callbacks.RUnlock()
	if !stillRegistered {
		t.Fatal("callback was released before acquired handler mapping was saved")
	}
	SaveHandlerMapping(2, cbPtr)
	RemoveCallbackByHandler(2)
	RemoveCallbackByHandler(3)
}
