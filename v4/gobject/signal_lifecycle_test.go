package gobject

import (
	"testing"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/v4/glib"
)

func TestSignalHandlerDisconnectReleasesTrackedCallback(t *testing.T) {
	oldDisconnect := xSignalHandlerDisconnect
	var nativeCalls int
	var nativeInstance uintptr
	var nativeHandler uint
	xSignalHandlerDisconnect = func(instance uintptr, handler uint) {
		nativeCalls++
		nativeInstance = instance
		nativeHandler = handler
	}
	defer func() { xSignalHandlerDisconnect = oldDisconnect }()

	obj := &Object{Ptr: 1}
	for i := 0; i < 2100; i++ {
		handlerID := uint(i + 1)
		cb := func(uintptr) {}
		cbPtr := uintptr(i + 1)
		refPtr := purego.NewCallbackFnPtr(&cb)
		glib.SaveCallbackWithClosure(cbPtr, refPtr, cb)
		glib.SaveHandlerMapping(handlerID, cbPtr)

		SignalHandlerDisconnect(obj, handlerID)

		if nativeCalls != int(handlerID) || nativeInstance != obj.GoPointer() || nativeHandler != handlerID {
			t.Fatalf("native disconnect call = (calls %d, instance %x, handler %d), want (calls %d, instance %x, handler %d)", nativeCalls, nativeInstance, nativeHandler, handlerID, obj.GoPointer(), handlerID)
		}
		assertCallbackReleased(t, handlerID, cbPtr)
	}
}

func TestObjectDisconnectSignalReleasesTrackedCallback(t *testing.T) {
	oldDisconnect := xSignalHandlerDisconnect
	var nativeCalls int
	var nativeInstance uintptr
	var nativeHandler uint
	xSignalHandlerDisconnect = func(instance uintptr, handler uint) {
		nativeCalls++
		nativeInstance = instance
		nativeHandler = handler
	}
	defer func() { xSignalHandlerDisconnect = oldDisconnect }()

	obj := Object{Ptr: 1}
	for i := 0; i < 2100; i++ {
		handlerID := uint(i + 1)
		cb := func(uintptr) {}
		cbPtr := uintptr(i + 1)
		refPtr := purego.NewCallbackFnPtr(&cb)
		glib.SaveCallbackWithClosure(cbPtr, refPtr, cb)
		glib.SaveHandlerMapping(handlerID, cbPtr)

		obj.DisconnectSignal(handlerID)

		if nativeCalls != int(handlerID) || nativeInstance != obj.GoPointer() || nativeHandler != handlerID {
			t.Fatalf("native disconnect call = (calls %d, instance %x, handler %d), want (calls %d, instance %x, handler %d)", nativeCalls, nativeInstance, nativeHandler, handlerID, obj.GoPointer(), handlerID)
		}
		assertCallbackReleased(t, handlerID, cbPtr)
	}
}

func assertCallbackReleased(t *testing.T, handlerID uint, cbPtr uintptr) {
	t.Helper()
	if refPtr, ok := glib.GetCallback(cbPtr); ok {
		t.Fatalf("callback for handler %d and cbPtr %x is still tracked with refPtr %x", handlerID, cbPtr, refPtr)
	}
}
