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

		signalCb := func(Object) {}
		signalData := glib.SaveSignalHandler(&signalCb)
		glib.SaveSignalHandlerMapping(handlerID, signalData)

		SignalHandlerDisconnect(obj, handlerID)

		if nativeCalls != int(handlerID) || nativeInstance != obj.GoPointer() || nativeHandler != handlerID {
			t.Fatalf("native disconnect call = (calls %d, instance %x, handler %d), want (calls %d, instance %x, handler %d)", nativeCalls, nativeInstance, nativeHandler, handlerID, obj.GoPointer(), handlerID)
		}
		assertCallbackReleased(t, handlerID, cbPtr)
		assertSignalHandlerReleased(t, handlerID, signalData)
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

		signalCb := func(Object) {}
		signalData := glib.SaveSignalHandler(&signalCb)
		glib.SaveSignalHandlerMapping(handlerID, signalData)

		obj.DisconnectSignal(handlerID)

		if nativeCalls != int(handlerID) || nativeInstance != obj.GoPointer() || nativeHandler != handlerID {
			t.Fatalf("native disconnect call = (calls %d, instance %x, handler %d), want (calls %d, instance %x, handler %d)", nativeCalls, nativeInstance, nativeHandler, handlerID, obj.GoPointer(), handlerID)
		}
		assertCallbackReleased(t, handlerID, cbPtr)
		assertSignalHandlerReleased(t, handlerID, signalData)
	}
}

func TestSaveSignalHandlerMappingReleasesRemappedData(t *testing.T) {
	handlerID := uint(4242)
	firstCb := func(Object) {}
	secondCb := func(Object) {}
	firstData := glib.SaveSignalHandler(&firstCb)
	secondData := glib.SaveSignalHandler(&secondCb)

	glib.SaveSignalHandlerMapping(handlerID, firstData)
	glib.SaveSignalHandlerMapping(handlerID, secondData)

	assertSignalHandlerReleased(t, handlerID, firstData)
	if _, ok := glib.GetSignalHandler(secondData); !ok {
		t.Fatalf("replacement signal handler for handler %d and data %x was not tracked", handlerID, secondData)
	}
	glib.RemoveCallbackByHandler(handlerID)
	assertSignalHandlerReleased(t, handlerID, secondData)
}

func TestGeneratedConnectSignalReleasesSignalHandler(t *testing.T) {
	oldConnect := xSignalConnectData
	oldDisconnect := xSignalHandlerDisconnect
	const handlerID = uint(99)
	var signalData uintptr
	xSignalConnectData = func(instance uintptr, detailedSignal string, handler uintptr, data uintptr, destroyData uintptr, flags ConnectFlags) uint {
		signalData = data
		return handlerID
	}
	xSignalHandlerDisconnect = func(uintptr, uint) {}
	defer func() {
		xSignalConnectData = oldConnect
		xSignalHandlerDisconnect = oldDisconnect
	}()

	group := &SignalGroup{Object: Object{Ptr: 1}}
	cb := func(SignalGroup, uintptr) {}
	if got := group.ConnectBind(&cb); got != handlerID {
		t.Fatalf("ConnectBind handler ID = %d, want %d", got, handlerID)
	}
	if signalData == 0 {
		t.Fatal("ConnectBind did not pass signal user data")
	}
	if _, ok := glib.GetSignalHandler(signalData); !ok {
		t.Fatalf("signal handler for generated ConnectBind data %x was not tracked", signalData)
	}

	SignalHandlerDisconnect(&group.Object, handlerID)
	assertSignalHandlerReleased(t, handlerID, signalData)
}

func assertCallbackReleased(t *testing.T, handlerID uint, cbPtr uintptr) {
	t.Helper()
	if refPtr, ok := glib.GetCallback(cbPtr); ok {
		t.Fatalf("callback for handler %d and cbPtr %x is still tracked with refPtr %x", handlerID, cbPtr, refPtr)
	}
}

func assertSignalHandlerReleased(t *testing.T, handlerID uint, data uintptr) {
	t.Helper()
	if _, ok := glib.GetSignalHandler(data); ok {
		t.Fatalf("signal handler for handler %d and data %x is still tracked", handlerID, data)
	}
}
