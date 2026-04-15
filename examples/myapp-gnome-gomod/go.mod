module github.com/bnema/puregotk/examples/myapp-gnome-gomod

go 1.25.0

tool github.com/dennwc/flatpak-go-mod

require (
	github.com/bnema/puregotk v0.0.0-00010101000000-000000000000
	github.com/bnema/puregotk/examples/mylib-gtk-meson-go v0.0.0-00010101000000-000000000000
	github.com/pojntfx/go-gettext v0.4.1
)

require (
	github.com/bnema/purego v0.0.0-20260224095105-2513c838cb80 // indirect
	github.com/dennwc/flatpak-go-mod v0.1.1-0.20251220152743-1642390bc050 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	golang.org/x/mod v0.31.0 // indirect
)

replace (
	github.com/bnema/puregotk v0.0.0-00010101000000-000000000000 => ../..
	github.com/bnema/puregotk/examples/mylib-gtk-meson-go v0.0.0-00010101000000-000000000000 => ../mylib-gtk-meson-go
)
