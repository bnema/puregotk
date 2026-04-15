package main

//go:generate sh -c "if [ -z \"$FLATPAK_ID\" ]; then cd .. && GOWORK=off go tool github.com/dennwc/flatpak-go-mod --json . && GOWORK=off go tool github.com/dennwc/flatpak-go-mod --json --module-name mylibgtkmeson ../mylib-gtk-meson; fi"

import (
	"os"

	"github.com/bnema/puregotk/v4/gio"
	"github.com/bnema/puregotk/v4/glib"
)

func init() {
	resource, err := gio.NewResourceFromData(glib.NewBytes(ResourceContents, uint(len(ResourceContents))))
	if err != nil {
		panic(err)
	}
	gio.ResourcesRegister(resource)
}

func main() {
	app := NewApplication(
		"application_id", AppID,
		"flags", gio.GApplicationDefaultFlagsValue,
	)

	os.Exit(int(app.Run(len(os.Args), os.Args)))
}
