package gobject

import (
	"testing"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/v4/glib"
	"github.com/bnema/puregotk/v4/gobject/types"
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

func TestSignalConnectDataRawRegistersLazyTarget(t *testing.T) {
	oldTarget := xSignalConnectData
	oldRegister := lazyRegisterSignalConnectData
	xSignalConnectData = nil
	defer func() {
		xSignalConnectData = oldTarget
		lazyRegisterSignalConnectData = oldRegister
	}()

	var registrations int
	var gotInstance, gotHandler, gotData, gotDestroy uintptr
	var gotSignal string
	var gotFlags ConnectFlags
	lazyRegisterSignalConnectData = func() {
		registrations++
		if xSignalConnectData != nil {
			t.Fatal("lazy registration target was already populated")
		}
		xSignalConnectData = func(instance uintptr, detailedSignal string, handler uintptr, data uintptr, destroyData uintptr, flags ConnectFlags) uint {
			gotInstance, gotSignal, gotHandler = instance, detailedSignal, handler
			gotData, gotDestroy, gotFlags = data, destroyData, flags
			return 73
		}
	}

	const (
		instance = uintptr(0x10)
		handler  = uintptr(0x20)
		data     = uintptr(0x30)
		destroy  = uintptr(0x40)
	)
	if got := SignalConnectDataRaw(instance, "pressed", handler, data, destroy, GConnectAfterValue); got != 73 {
		t.Fatalf("handler ID = %d, want 73", got)
	}
	if registrations != 1 {
		t.Fatalf("lazy registrations = %d, want 1", registrations)
	}
	if gotInstance != instance || gotSignal != "pressed" || gotHandler != handler || gotData != data || gotDestroy != destroy || gotFlags != GConnectAfterValue {
		t.Fatalf("native arguments = (%#x, %q, %#x, %#x, %#x, %#x), want (%#x, %q, %#x, %#x, %#x, %#x)", gotInstance, gotSignal, gotHandler, gotData, gotDestroy, gotFlags, instance, "pressed", handler, data, destroy, GConnectAfterValue)
	}
}

func TestSignalConnectUsesLazyRawPath(t *testing.T) {
	oldTarget := xSignalConnectData
	oldRegister := lazyRegisterSignalConnectData
	xSignalConnectData = nil
	defer func() {
		xSignalConnectData = oldTarget
		lazyRegisterSignalConnectData = oldRegister
	}()

	var registrations int
	lazyRegisterSignalConnectData = func() {
		registrations++
		xSignalConnectData = func(instance uintptr, detailedSignal string, handler uintptr, data uintptr, destroyData uintptr, flags ConnectFlags) uint {
			if instance != 1 || detailedSignal != "legacy" || handler != 2 || data != 0 || destroyData != 0 || flags != GConnectDefaultValue {
				t.Fatalf("legacy SignalConnect arguments changed")
			}
			return 74
		}
	}

	if got := SignalConnect(1, "legacy", 2); got != 74 {
		t.Fatalf("handler ID = %d, want 74", got)
	}
	if registrations != 1 {
		t.Fatalf("lazy registrations = %d, want 1", registrations)
	}
}

func TestIncreaseRefRegistersLazyTarget(t *testing.T) {
	oldTarget := xObjectRefSink
	oldRegister := lazyRegisterObjectRefSink
	xObjectRefSink = nil
	defer func() {
		xObjectRefSink = oldTarget
		lazyRegisterObjectRefSink = oldRegister
	}()

	var registrations int
	var nativeCalls int
	var got uintptr
	lazyRegisterObjectRefSink = func() {
		registrations++
		if xObjectRefSink != nil {
			t.Fatal("lazy registration target was already populated")
		}
		xObjectRefSink = func(arg uintptr) uintptr {
			nativeCalls++
			got = arg
			return arg
		}
	}

	const arg = uintptr(0xfeed)
	IncreaseRef(arg)
	if registrations != 1 {
		t.Fatalf("lazy registrations = %d, want 1", registrations)
	}
	if nativeCalls != 1 {
		t.Fatalf("native calls = %d, want 1", nativeCalls)
	}
	if got != arg {
		t.Fatalf("native argument = %#x, want %#x", got, arg)
	}
}

func TestTypeCheckInstanceIsAPtrBehavior(t *testing.T) {
	oldTarget := xTypeCheckInstanceIsAPtr
	oldRegister := lazyRegisterTypeCheckInstanceIsAPtr
	xTypeCheckInstanceIsAPtr = nil
	defer func() {
		xTypeCheckInstanceIsAPtr = oldTarget
		lazyRegisterTypeCheckInstanceIsAPtr = oldRegister
	}()

	var registrations int
	var nativeCalls int
	var gotPtr uintptr
	var gotType types.GType
	lazyRegisterTypeCheckInstanceIsAPtr = func() {
		registrations++
		xTypeCheckInstanceIsAPtr = func(ptr uintptr, ifaceType types.GType) bool {
			nativeCalls++
			gotPtr = ptr
			gotType = ifaceType
			return true
		}
	}

	const ifaceType = types.GType(0x42)
	if TypeCheckInstanceIsAPtr(0, ifaceType) {
		t.Fatal("nil pointer returned true")
	}
	if registrations != 0 || nativeCalls != 0 {
		t.Fatalf("nil pointer made %d registrations and %d native calls, want none", registrations, nativeCalls)
	}

	const ptr = uintptr(0xfeed)
	if !TypeCheckInstanceIsAPtr(ptr, ifaceType) {
		t.Fatal("native true result was not forwarded")
	}
	if registrations != 1 || nativeCalls != 1 {
		t.Fatalf("non-nil pointer made %d registrations and %d native calls, want one each", registrations, nativeCalls)
	}
	if gotPtr != ptr || gotType != ifaceType {
		t.Fatalf("native arguments = (%#x, %#x), want (%#x, %#x)", gotPtr, gotType, ptr, ifaceType)
	}
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
