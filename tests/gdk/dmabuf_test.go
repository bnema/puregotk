//go:build linux

package gdk_test

import (
	"os"
	"testing"

	"github.com/bnema/puregotk/v4/gdk"
)

func TestDmabufTextureBuilderRawDestroyNotifyBuildMethodExists(t *testing.T) {
	var _ func(*gdk.DmabufTextureBuilder, uintptr, uintptr) (*gdk.Texture, error) = (*gdk.DmabufTextureBuilder).BuildWithDestroyNotifyPointer
}

func TestDmabufTextureBuilderSymbolsAndProperties(t *testing.T) {
	builder := gdk.NewDmabufTextureBuilder()
	if builder == nil {
		t.Fatal("NewDmabufTextureBuilder returned nil")
	}
	defer builder.Unref()

	builder.SetWidth(64)
	builder.SetHeight(32)
	builder.SetFourcc(0x34325241) // DRM_FORMAT_ARGB8888
	builder.SetModifier(0)
	file, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close test fd: %v", err)
		}
	}()

	builder.SetPremultiplied(true)
	builder.SetNPlanes(1)
	builder.SetFd(0, int(file.Fd()))
	builder.SetStride(0, 256)
	builder.SetOffset(0, 0)

	if got := builder.GetWidth(); got != 64 {
		t.Fatalf("GetWidth() = %d, want 64", got)
	}
	if got := builder.GetHeight(); got != 32 {
		t.Fatalf("GetHeight() = %d, want 32", got)
	}
	if got := builder.GetFourcc(); got != 0x34325241 {
		t.Fatalf("GetFourcc() = %#x, want DRM_FORMAT_ARGB8888", got)
	}
	if got := builder.GetModifier(); got != 0 {
		t.Fatalf("GetModifier() = %d, want 0", got)
	}
	if got := builder.GetNPlanes(); got != 1 {
		t.Fatalf("GetNPlanes() = %d, want 1", got)
	}
	if got := builder.GetPremultiplied(); got != true {
		t.Fatalf("GetPremultiplied() = %t, want true", got)
	}
	if got := builder.GetFd(0); got != int(file.Fd()) {
		t.Fatalf("GetFd(0) = %d, want %d", got, file.Fd())
	}
	if got := builder.GetStride(0); got != 256 {
		t.Fatalf("GetStride(0) = %d, want 256", got)
	}
	if got := builder.GetOffset(0); got != 0 {
		t.Fatalf("GetOffset(0) = %d, want 0", got)
	}

	var nilBuilder *gdk.DmabufTextureBuilder
	if got := nilBuilder.GoPointer(); got != 0 {
		t.Fatalf("nil DmabufTextureBuilder GoPointer() = %#x, want 0", got)
	}
}

func TestDmabufFormatsTypeRegistered(t *testing.T) {
	if got := gdk.DmabufFormatsGLibType(); got == 0 {
		t.Fatal("DmabufFormatsGLibType returned 0")
	}
}
