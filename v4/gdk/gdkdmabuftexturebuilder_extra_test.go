package gdk

import (
	"testing"

	"github.com/bnema/puregotk/v4/glib"
)

func TestBuildWithDestroyNotifyPointerForwardsTransferredTexture(t *testing.T) {
	oldTarget := xDmabufTextureBuilderBuild
	oldRegister := lazyRegisterDmabufTextureBuilderBuild
	xDmabufTextureBuilderBuild = nil
	defer func() {
		xDmabufTextureBuilderBuild = oldTarget
		lazyRegisterDmabufTextureBuilderBuild = oldRegister
	}()

	var registrations int
	var nativeCalls int
	var gotBuilder, gotDestroy, gotData uintptr
	lazyRegisterDmabufTextureBuilderBuild = func() {
		registrations++
		xDmabufTextureBuilderBuild = func(builder, destroy, data uintptr, cerr **glib.Error) uintptr {
			nativeCalls++
			gotBuilder, gotDestroy, gotData = builder, destroy, data
			if cerr == nil || *cerr != nil {
				t.Fatal("native error argument does not point to an empty error")
			}
			return 0xcafe
		}
	}

	builder := &DmabufTextureBuilder{}
	builder.Ptr = 0xbeef
	texture, err := builder.BuildWithDestroyNotifyPointer(0xdead, 0xf00d)
	if err != nil {
		t.Fatalf("BuildWithDestroyNotifyPointer error = %v", err)
	}
	if registrations != 1 || nativeCalls != 1 {
		t.Fatalf("registrations/native calls = %d/%d, want 1/1", registrations, nativeCalls)
	}
	if gotBuilder != builder.GoPointer() || gotDestroy != 0xdead || gotData != 0xf00d {
		t.Fatalf("native arguments = (%#x, %#x, %#x), want (%#x, %#x, %#x)", gotBuilder, gotDestroy, gotData, builder.GoPointer(), uintptr(0xdead), uintptr(0xf00d))
	}
	if texture == nil {
		t.Fatal("transferred texture is nil")
	}
	if texture.GoPointer() != 0xcafe {
		t.Fatalf("transferred texture pointer = %#x, want %#x", texture.GoPointer(), uintptr(0xcafe))
	}
}
