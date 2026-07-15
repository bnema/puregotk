// Command firstuse checks that the blank-import set can resolve symbols on demand.
package main

import (
	"github.com/bnema/puregotk/v4/adw"
	"github.com/bnema/puregotk/v4/gdk"
	"github.com/bnema/puregotk/v4/gio"
	"github.com/bnema/puregotk/v4/glib"
	"github.com/bnema/puregotk/v4/gtk"
)

func main() {
	if gtk.GetMajorVersion() == 0 {
		panic("gtk returned major version zero")
	}
	if adw.GetMajorVersion() == 0 {
		panic("adwaita returned major version zero")
	}
	_ = glib.CheckVersion(0, 0, 0)
	// These accessors are safe without a display or application; their result
	// is environment-dependent, but calling them verifies first-use resolution.
	_ = gdk.DisplayGetDefault()
	_ = gio.ApplicationGetDefault()
}
