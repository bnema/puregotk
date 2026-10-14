package glib

import (
	"testing"
	"unsafe"

	"github.com/bnema/purego"
)

// These tests call the registered C function pointers through purego so the
// NewCallbackInts argument and return-value decoding is exercised end to end.

// currentTestSource returns a real GLib source and makes it the current
// source seen by the dispatch callbacks for the duration of the test.
func currentTestSource(t *testing.T) uintptr {
	t.Helper()
	src := IdleSourceNew()
	if src == nil {
		t.Fatal("IdleSourceNew returned nil")
	}
	source := src.GoPointer()
	prev := xMainCurrentSource
	xMainCurrentSource = func() uintptr { return source }
	t.Cleanup(func() {
		xMainCurrentSource = prev
		src.Unref()
	})
	return source
}

func TestSourceLifecycleCallbacksDecodeCArguments(t *testing.T) {
	source := currentTestSource(t)
	const wantData, callback = uintptr(0xcafe), uintptr(0x5005)
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

// GPid and gint are 32-bit C ints: only the low 32 bits of their 64-bit
// registers are defined. Feed raw register values with explicit upper bits
// so the decoding must ignore them and sign-extend the low half.
func TestChildWatchCallbackDecodesLow32BitsOfCInts(t *testing.T) {
	source := currentTestSource(t)
	const data = uintptr(0xface)

	cases := []struct {
		name              string
		pidReg, statusReg uint64
		wantPID           Pid
		wantStatus        int
	}{
		{"negative, clean upper bits", 0x0000_0000_ffff_fff9, 0x0000_0000_ffff_ffff, -7, -1},
		{"positive, garbage upper bits", 0xdead_beef_0000_04d2, 0xffff_ffff_0000_0011, 1234, 17},
		{"negative, garbage upper bits", 0x1234_5678_8000_0000, 0xdead_beef_ffff_fffe, -2147483648, -2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			callback := ChildWatchFunc(func(gotPID Pid, gotStatus int, gotData uintptr) {
				called = true
				if gotPID != tc.wantPID || gotStatus != tc.wantStatus || gotData != data {
					t.Fatalf("callback = (%d, %d, %#x), want (%d, %d, %#x)", gotPID, gotStatus, gotData, tc.wantPID, tc.wantStatus, data)
				}
			})
			token := registerSourceCallback(source, sourceCallbackEntry{childWatchFunc: callback})
			defer unrefSourceCallback(token)

			purego.Syscall6(childWatchDispatchCb, uintptr(tc.pidReg), uintptr(tc.statusReg), data, 0, 0, 0)
			if !called {
				t.Fatal("child-watch callback was not dispatched")
			}
		})
	}
}

func TestSourceDispatchCallbackReturnsCBool(t *testing.T) {
	source := currentTestSource(t)
	keep := SourceFunc(func(uintptr) bool { return true })
	token := registerSourceCallback(source, sourceCallbackEntry{sourceFunc: keep})
	t.Cleanup(func() { unrefSourceCallback(token) })

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
	source := currentTestSource(t)
	keep := SourceFunc(func(uintptr) bool { return true })
	token := registerSourceCallback(source, sourceCallbackEntry{sourceFunc: keep})
	t.Cleanup(func() { unrefSourceCallback(token) })

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
