package main

import (
	_ "embed"
	"path"
)

const (
	dataKeyGoInstance = "go_instance"

	propertyIdTestButtonSensitive = 1
)

var (
	appPath = path.Join("/page", "codeberg", "puregotk", "MyLibGtkMeson")

	resourceWindowUIPath = path.Join(appPath, "window.ui")
)

//go:generate sh -c "if command -v blueprint-compiler >/dev/null 2>&1; then blueprint-compiler batch-compile . . *.blp; fi; glib-compile-resources *.gresource.xml"
//go:embed mylib-gtk-meson.gresource
var ResourceContents []byte
