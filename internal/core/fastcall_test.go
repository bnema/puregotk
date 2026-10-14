//go:build (amd64 || arm64) && (linux || darwin || freebsd)

package core

import (
	"testing"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/v4/gobject/types"
)

// recorded holds the integer arguments of the last call into recorder.
var recorded [6]uintptr

// recorder is a C function pointer that stores its six integer-class
// arguments and returns the first one. It lets the tests compare the fast path
// with purego.RegisterFunc on the same native symbol.
var recorder = purego.NewCallbackInts(func(a *purego.CallbackArgs) uintptr {
	for i := range recorded {
		recorded[i] = a.Int(i)
	}
	return a.Int(0)
})

// both binds fptr through the fast path and slow through RegisterFunc.
func both(t *testing.T, fast, slow any) {
	t.Helper()
	if !bindFast(fast, recorder) {
		t.Fatalf("bindFast rejected %T", fast)
	}
	purego.RegisterFunc(slow, recorder)
}

// sameArgs runs call through each implementation and checks the C side saw
// identical argument registers.
func sameArgs(t *testing.T, name string, fast, slow func()) {
	t.Helper()
	recorded = [6]uintptr{}
	slow()
	want := recorded
	recorded = [6]uintptr{}
	fast()
	if recorded != want {
		t.Fatalf("%s: fast args %#x, RegisterFunc args %#x", name, recorded, want)
	}
}

func TestBindFastReturnsMatchRegisterFunc(t *testing.T) {
	values := []uintptr{0, 1, 2, 0x100, 0xff00, ^uintptr(0), 1 << 63, 0xdead_beef_cafe_f00d}

	var fi, si func(uintptr) int
	var fu, su func(uintptr) uint
	var f32, s32 func(uintptr) uint32
	var f64, s64 func(uintptr) uint64
	var fi64, si64 func(uintptr) int64
	var fb, sb func(uintptr) bool
	both(t, &fi, &si)
	both(t, &fu, &su)
	both(t, &f32, &s32)
	both(t, &f64, &s64)
	both(t, &fi64, &si64)
	both(t, &fb, &sb)
	for _, v := range values {
		if fi(v) != si(v) || fu(v) != su(v) || f32(v) != s32(v) || f64(v) != s64(v) || fi64(v) != si64(v) || fb(v) != sb(v) {
			t.Fatalf("return decoding differs for %#x", v)
		}
	}

	// func() types.GType has no argument to echo: the recorder returns its
	// first argument register, which both paths leave as 0.
	var fg, sg func() types.GType
	both(t, &fg, &sg)
	if fg() != sg() {
		t.Fatalf("GType return differs: fast=%#x slow=%#x", fg(), sg())
	}
	var fgt, sgt func(uintptr, types.GType) bool
	both(t, &fgt, &sgt)
	if fgt(0x100, 7) != sgt(0x100, 7) || fgt(1, 7) != sgt(1, 7) {
		t.Fatal("GType-argument bool return differs")
	}
}

func TestBindFastArgumentsMatchRegisterFunc(t *testing.T) {
	var fb, sb func(uintptr, bool)
	both(t, &fb, &sb)
	for _, v := range []bool{true, false} {
		sameArgs(t, "bool arg", func() { fb(9, v) }, func() { sb(9, v) })
	}

	var fi, si func(uintptr, int)
	both(t, &fi, &si)
	for _, v := range []int{0, -1, -100, 1 << 40} {
		sameArgs(t, "int arg", func() { fi(9, v) }, func() { si(9, v) })
	}

	var fui, sui func(uintptr, uint, int)
	both(t, &fui, &sui)
	sameArgs(t, "uint,int args", func() { fui(1, 2, -3) }, func() { sui(1, 2, -3) })

	var fu32, su32 func(uintptr, uint32)
	both(t, &fu32, &su32)
	sameArgs(t, "uint32 arg", func() { fu32(1, 0xffff_ffff) }, func() { su32(1, 0xffff_ffff) })

	var fu64, su64 func(uintptr, uint64)
	both(t, &fu64, &su64)
	sameArgs(t, "uint64 arg", func() { fu64(1, 1<<63) }, func() { su64(1, 1<<63) })

	var f5, s5 func(a, b, c, d, e uintptr)
	both(t, &f5, &s5)
	sameArgs(t, "five args", func() { f5(1, 2, 3, 4, 5) }, func() { s5(1, 2, 3, 4, 5) })
}

func TestBindFastRejectsUnsupportedSignatures(t *testing.T) {
	var withString func(uintptr, string)
	var withFloat func(uintptr, float64)
	var tooMany func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr)
	for name, fptr := range map[string]any{"string": &withString, "float": &withFloat, "seven args": &tooMany} {
		if bindFast(fptr, recorder) {
			t.Errorf("%s signature was bound to the integer-only fast path", name)
		}
	}
}

func TestBindFastDoesNotAllocate(t *testing.T) {
	var get func(uintptr) int
	var set func(uintptr, int)
	if !bindFast(&get, recorder) || !bindFast(&set, recorder) {
		t.Fatal("bindFast rejected a supported signature")
	}
	allocs := testing.AllocsPerRun(1000, func() {
		set(1, -5)
		_ = get(7)
	})
	if allocs != 0 {
		t.Fatalf("fast calls allocated %.1f times, want 0", allocs)
	}
}
