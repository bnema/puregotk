//go:build linux

package gdk_test

import (
	"testing"
	"unsafe"

	"github.com/bnema/puregotk/v4/gdk"
)

func TestRectangleMatchesCLayout(t *testing.T) {
	if got := unsafe.Sizeof(gdk.Rectangle{}); got != 16 {
		t.Fatalf("sizeof(gdk.Rectangle) = %d, want 16 (four C ints)", got)
	}

	a := gdk.Rectangle{X: 10, Y: 20, Width: 30, Height: 40}
	b := gdk.Rectangle{X: 0, Y: 0, Width: 5, Height: 5}
	var dest gdk.Rectangle
	a.Union(&b, &dest)

	want := gdk.Rectangle{X: 0, Y: 0, Width: 40, Height: 60}
	if dest != want {
		t.Fatalf("Union() = %+v, want %+v", dest, want)
	}
}
