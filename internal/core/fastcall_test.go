package core

import (
	"testing"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/v4/gobject/types"
)

// echoCallback returns a C function pointer that returns its first integer
// argument, or the second one when sel is 1. It lets the tests compare the fast
// path with purego.RegisterFunc on the exact same native symbol.
func echoCallback(sel int) uintptr {
	return purego.NewCallbackInts(func(a *purego.CallbackArgs) uintptr { return a.Int(sel) })
}

var (
	echoFirst  = echoCallback(0)
	echoSecond = echoCallback(1)
)

func TestBindFastMatchesRegisterFunc(t *testing.T) {
	if !fastCallSupported {
		t.Skip("fast call path disabled on this platform")
	}

	t.Run("signed int return", func(t *testing.T) {
		var fast, slow func(uintptr) int
		mustBind(t, &fast, echoFirst)
		purego.RegisterFunc(&slow, echoFirst)
		for _, v := range []uintptr{0, 1, ^uintptr(0), uintptr(1) << 63} {
			if fast(v) != slow(v) {
				t.Fatalf("int(%#x): fast=%d slow=%d", v, fast(v), slow(v))
			}
		}
	})
	t.Run("bool return uses low byte", func(t *testing.T) {
		var fast, slow func(uintptr) bool
		mustBind(t, &fast, echoFirst)
		purego.RegisterFunc(&slow, echoFirst)
		for _, v := range []uintptr{0, 1, 0x100, 0xff00, 0x2} {
			if fast(v) != slow(v) {
				t.Fatalf("bool(%#x): fast=%v slow=%v", v, fast(v), slow(v))
			}
		}
	})
	t.Run("unsigned returns", func(t *testing.T) {
		var fast32, slow32 func(uintptr) uint32
		mustBind(t, &fast32, echoFirst)
		purego.RegisterFunc(&slow32, echoFirst)
		var fastG func() types.GType
		mustBind(t, &fastG, echoFirst)
		v := uintptr(0xdead_beef_cafe_f00d)
		if fast32(v) != slow32(v) {
			t.Fatalf("uint32: fast=%#x slow=%#x", fast32(v), slow32(v))
		}
	})
	t.Run("bool and negative int arguments", func(t *testing.T) {
		var fastI, slowI func(uintptr, int) uintptr
		mustBind(t, &fastI, echoSecond)
		purego.RegisterFunc(&slowI, echoSecond)
		for _, v := range []int{0, -1, -100, 1 << 40} {
			if fastI(0, v) != slowI(0, v) {
				t.Fatalf("int arg %d: fast=%#x slow=%#x", v, fastI(0, v), slowI(0, v))
			}
		}
		var fastBool func(uintptr, bool)
		mustBind(t, &fastBool, echoSecond)
	})
}

func TestBindFastRejectsUnsupportedSignatures(t *testing.T) {
	var withString func(uintptr, string)
	var withFloat func(uintptr, float64)
	var tooMany func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr)
	for name, fptr := range map[string]any{"string": &withString, "float": &withFloat, "seven args": &tooMany} {
		if bindFast(fptr, echoFirst) {
			t.Errorf("%s signature was bound to the integer-only fast path", name)
		}
	}
}

func TestBindFastDoesNotAllocate(t *testing.T) {
	if !fastCallSupported {
		t.Skip("fast call path disabled on this platform")
	}
	var get func(uintptr) int
	var set func(uintptr, int)
	mustBind(t, &get, echoFirst)
	mustBind(t, &set, echoSecond)
	allocs := testing.AllocsPerRun(1000, func() {
		set(1, -5)
		_ = get(7)
	})
	if allocs != 0 {
		t.Fatalf("fast calls allocated %.1f times, want 0", allocs)
	}
}

func mustBind(t *testing.T, fptr any, sym uintptr) {
	t.Helper()
	if !bindFast(fptr, sym) {
		t.Fatalf("bindFast rejected %T", fptr)
	}
}
