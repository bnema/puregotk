package glib

import (
	"testing"
	"unsafe"

	"github.com/bnema/purego"
)

// These tests call the registered C function pointers through purego so the
// NewCallbackInts argument and return-value decoding is exercised end to end.

func TestSourceLifecycleCallbacksDecodeCArguments(t *testing.T) {
	const source, wantData, callback = uintptr(0x4004), uintptr(0xcafe), uintptr(0x5005)
	token := registerSourceCallback(source, sourceCallbackEntry{data: wantData, callback: callback})

	var ref, unref func(uintptr)
	var get func(token, source uintptr, callbackOut, dataOut *uintptr)
	purego.RegisterFunc(&ref, sourceCallbackLifecycleFuncs.xRef)
	purego.RegisterFunc(&unref, sourceCallbackLifecycleFuncs.xUnref)
	purego.RegisterFunc(&get, sourceCallbackLifecycleFuncs.xGet)

	ref(token)
	var gotCallback, gotData uintptr
	get(token, source, &gotCallback, &gotData)
	if gotCallback != callback || gotData != wantData {
		t.Fatalf("xGet = (%#x, %#x), want (%#x, %#x)", gotCallback, gotData, callback, wantData)
	}
	unref(token)
	unref(token)
	sourceCallbacks.RLock()
	_, alive := sourceCallbacks.byToken[token]
	sourceCallbacks.RUnlock()
	if alive {
		t.Fatal("token still registered after balanced ref/unref")
	}
}

func TestChildWatchCallbackPreservesSignedCInts(t *testing.T) {
	const source, data = uintptr(0x6006), uintptr(0xface)
	const pid, status = Pid(-7), int32(-1)
	called := false
	callback := ChildWatchFunc(func(gotPID Pid, gotStatus int, gotData uintptr) {
		called = true
		if gotPID != pid || gotStatus != int(status) || gotData != data {
			t.Fatalf("callback = (%d, %d, %#x), want (%d, %d, %#x)", gotPID, gotStatus, gotData, pid, status, data)
		}
	})
	token := registerSourceCallback(source, sourceCallbackEntry{childWatchFunc: callback})
	t.Cleanup(func() { unrefSourceCallback(token) })

	prev := xMainCurrentSource
	xMainCurrentSource = func() uintptr { return source }
	t.Cleanup(func() { xMainCurrentSource = prev })

	// C ABI: GPid and gint are 32-bit ints passed in 64-bit registers.
	var invoke func(pid int32, status int32, data uintptr)
	purego.RegisterFunc(&invoke, childWatchDispatchCb)
	invoke(int32(pid), status, data)
	if !called {
		t.Fatal("child-watch callback was not dispatched")
	}
}

func TestSourceDispatchCallbackReturnsCBool(t *testing.T) {
	const source = uintptr(0x7007)
	keep := SourceFunc(func(uintptr) bool { return true })
	token := registerSourceCallback(source, sourceCallbackEntry{sourceFunc: keep})
	t.Cleanup(func() { unrefSourceCallback(token) })

	prev := xMainCurrentSource
	xMainCurrentSource = func() uintptr { return source }
	t.Cleanup(func() { xMainCurrentSource = prev })

	var dispatch func(uintptr) bool
	purego.RegisterFunc(&dispatch, sourceDispatchCb)
	if !dispatch(0) {
		t.Fatal("dispatch returned false for a source that asked to continue")
	}

	xMainCurrentSource = func() uintptr { return 0 }
	if dispatch(0) {
		t.Fatal("dispatch returned true with no current source")
	}
}

// GLib calls these callbacks on every source attach and dispatch (IdleAdd,
// TimeoutAdd, ...). The C-to-Go path itself must not allocate.
func TestSourceLifecycleCallbacksDoNotAllocate(t *testing.T) {
	const source = uintptr(0x8008)
	keep := SourceFunc(func(uintptr) bool { return true })
	token := registerSourceCallback(source, sourceCallbackEntry{sourceFunc: keep})
	t.Cleanup(func() { unrefSourceCallback(token) })

	prev := xMainCurrentSource
	xMainCurrentSource = func() uintptr { return source }
	t.Cleanup(func() { xMainCurrentSource = prev })

	f := sourceCallbackLifecycleFuncs
	var callback, data uintptr
	allocs := testing.AllocsPerRun(1000, func() {
		purego.Syscall6(f.xRef, token, 0, 0, 0, 0, 0)
		purego.Syscall6(f.xGet, token, source, uintptr(unsafe.Pointer(&callback)), uintptr(unsafe.Pointer(&data)), 0, 0)
		purego.Syscall6(sourceDispatchCb, 0, 0, 0, 0, 0, 0)
		purego.Syscall6(f.xUnref, token, 0, 0, 0, 0, 0)
	})
	if allocs != 0 {
		t.Fatalf("source lifecycle callbacks allocated %.1f times per dispatch, want 0", allocs)
	}
}
