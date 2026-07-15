// Command blank measures pre-main initialization for the v4 import set.
package main

import (
	_ "github.com/bnema/puregotk/v4/adw"
	_ "github.com/bnema/puregotk/v4/gdk"
	_ "github.com/bnema/puregotk/v4/gio"
	_ "github.com/bnema/puregotk/v4/glib"
	_ "github.com/bnema/puregotk/v4/gtk"
)

func main() {}
