module codeberg.org/puregotk/puregotk/examples/myapp-gnome-meson

go 1.25.0

tool github.com/dennwc/flatpak-go-mod

require (
	codeberg.org/puregotk/puregotk v0.0.0-00010101000000-000000000000
	codeberg.org/puregotk/puregotk/examples/mylib-gtk-meson-go v0.0.0-00010101000000-000000000000
	github.com/pojntfx/go-gettext v0.4.0
)

require (
	codeberg.org/puregotk/purego v0.0.0-20260224095105-2513c838cb80 // indirect
	github.com/dennwc/flatpak-go-mod v0.1.1-0.20251220152743-1642390bc050 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	golang.org/x/mod v0.31.0 // indirect
)

replace (
	codeberg.org/puregotk/puregotk v0.0.0-00010101000000-000000000000 => ../..
	codeberg.org/puregotk/puregotk/examples/mylib-gtk-meson-go v0.0.0-00010101000000-000000000000 => ../mylib-gtk-meson-go
)
