package core

import (
	"runtime"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/v4/gobject/types"
)

// fastCallSupported limits the reflection-free path to 64-bit targets where the
// first six integer-class C arguments travel in registers and every Go integer
// fits in one register, so purego.Syscall6 matches purego.RegisterFunc exactly.
var fastCallSupported = (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") &&
	(runtime.GOOS == "linux" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd")

// cbool mirrors purego's bool argument encoding.
func cbool(v bool) uintptr {
	if v {
		return 1
	}
	return 0
}

// gobool mirrors purego's bool return decoding (low byte only).
func gobool(r uintptr) bool {
	return byte(r) != 0
}

// bindFast publishes an allocation-free implementation of fptr when its
// signature only uses integer-class arguments and results. Conversions mirror
// purego.RegisterFunc: integers are passed as uintptr(v), bools as 1/0, and
// results are converted from the integer return register. It reports whether
// fptr was bound.
func bindFast(fptr any, sym uintptr) bool {
	if !fastCallSupported || sym == 0 {
		return false
	}
	s := func(a1, a2, a3, a4, a5, a6 uintptr) uintptr {
		r, _, _ := purego.Syscall6(sym, a1, a2, a3, a4, a5, a6)
		return r
	}
	switch f := fptr.(type) {
	case *func():
		*f = func() { s(0, 0, 0, 0, 0, 0) }
	case *func() uintptr:
		*f = func() uintptr { return s(0, 0, 0, 0, 0, 0) }
	case *func() uint:
		*f = func() uint { return uint(s(0, 0, 0, 0, 0, 0)) }
	case *func() types.GType:
		*f = func() types.GType { return types.GType(s(0, 0, 0, 0, 0, 0)) }

	case *func(uintptr):
		*f = func(a uintptr) { s(a, 0, 0, 0, 0, 0) }
	case *func(uintptr) uintptr:
		*f = func(a uintptr) uintptr { return s(a, 0, 0, 0, 0, 0) }
	case *func(uintptr) bool:
		*f = func(a uintptr) bool { return gobool(s(a, 0, 0, 0, 0, 0)) }
	case *func(uintptr) int:
		*f = func(a uintptr) int { return int(s(a, 0, 0, 0, 0, 0)) }
	case *func(uintptr) uint:
		*f = func(a uintptr) uint { return uint(s(a, 0, 0, 0, 0, 0)) }
	case *func(uintptr) uint32:
		*f = func(a uintptr) uint32 { return uint32(s(a, 0, 0, 0, 0, 0)) }
	case *func(uintptr) uint64:
		*f = func(a uintptr) uint64 { return uint64(s(a, 0, 0, 0, 0, 0)) }
	case *func(uintptr) int64:
		*f = func(a uintptr) int64 { return int64(s(a, 0, 0, 0, 0, 0)) }

	case *func(uintptr, uintptr):
		*f = func(a, b uintptr) { s(a, b, 0, 0, 0, 0) }
	case *func(uintptr, uintptr) uintptr:
		*f = func(a, b uintptr) uintptr { return s(a, b, 0, 0, 0, 0) }
	case *func(uintptr, uintptr) bool:
		*f = func(a, b uintptr) bool { return gobool(s(a, b, 0, 0, 0, 0)) }
	case *func(uintptr, bool):
		*f = func(a uintptr, b bool) { s(a, cbool(b), 0, 0, 0, 0) }
	case *func(uintptr, int):
		*f = func(a uintptr, b int) { s(a, uintptr(b), 0, 0, 0, 0) }
	case *func(uintptr, int) uintptr:
		*f = func(a uintptr, b int) uintptr { return s(a, uintptr(b), 0, 0, 0, 0) }
	case *func(uintptr, uint):
		*f = func(a uintptr, b uint) { s(a, uintptr(b), 0, 0, 0, 0) }
	case *func(uintptr, uint) uintptr:
		*f = func(a uintptr, b uint) uintptr { return s(a, uintptr(b), 0, 0, 0, 0) }
	case *func(uintptr, uint32):
		*f = func(a uintptr, b uint32) { s(a, uintptr(b), 0, 0, 0, 0) }
	case *func(uintptr, uint64):
		*f = func(a uintptr, b uint64) { s(a, uintptr(b), 0, 0, 0, 0) }
	case *func(uintptr, types.GType) bool:
		*f = func(a uintptr, b types.GType) bool { return gobool(s(a, uintptr(b), 0, 0, 0, 0)) }

	case *func(uintptr, uintptr, uintptr):
		*f = func(a, b, c uintptr) { s(a, b, c, 0, 0, 0) }
	case *func(uintptr, uintptr, uintptr) uintptr:
		*f = func(a, b, c uintptr) uintptr { return s(a, b, c, 0, 0, 0) }
	case *func(uintptr, uint, int):
		*f = func(a uintptr, b uint, c int) { s(a, uintptr(b), uintptr(c), 0, 0, 0) }
	case *func(uintptr, uint, uint):
		*f = func(a uintptr, b, c uint) { s(a, uintptr(b), uintptr(c), 0, 0, 0) }

	case *func(uintptr, uintptr, uintptr, uintptr):
		*f = func(a, b, c, d uintptr) { s(a, b, c, d, 0, 0) }
	case *func(uintptr, uintptr, uintptr, uintptr, uintptr):
		*f = func(a, b, c, d, e uintptr) { s(a, b, c, d, e, 0) }
	default:
		return false
	}
	return true
}
