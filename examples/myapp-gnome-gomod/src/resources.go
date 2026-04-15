package main

import (
	_ "embed"
	"path"
)

const (
	AppID      = "page.codeberg.puregotk.MyAppGnomeGomod"
	AppVersion = "0.1.0"
)

//go:generate sh -c "if command -v blueprint-compiler >/dev/null 2>&1; then blueprint-compiler batch-compile . . *.blp; fi; glib-compile-resources *.gresource.xml"
//go:embed myapp-gnome-gomod.gresource
var ResourceContents []byte

var (
	AppPath = path.Join("/page", "codeberg", "puregotk", "MyAppGnomeGomod")

	ResourceWindowUIPath = path.Join(AppPath, "window.ui")
)
