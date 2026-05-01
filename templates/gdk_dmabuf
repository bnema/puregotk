package gdk

import "github.com/bnema/puregotk/v4/glib"

// BuildWithDestroyNotifyPointer builds a DMABUF texture using a raw native
// GDestroyNotify function pointer.
//
// The generated Build method is appropriate for Go callbacks. DMABUF file
// descriptors, however, often need a native destroy notify such as libc close(2)
// so the descriptor is closed exactly when GDK releases the texture, without a
// callback from GTK/GSK finalizer code into Go.
func (x *DmabufTextureBuilder) BuildWithDestroyNotifyPointer(destroy uintptr, data uintptr) (*Texture, error) {
	var cls *Texture
	var cerr *glib.Error

	cret := xDmabufTextureBuilderBuild(x.GoPointer(), destroy, data, &cerr)
	if cret == 0 {
		return nil, cerr
	}
	cls = &Texture{}
	cls.Ptr = cret
	if cerr == nil {
		return cls, nil
	}
	return cls, cerr
}
