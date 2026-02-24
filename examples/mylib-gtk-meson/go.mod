module codeberg.org/puregotk/puregotk/examples/mylib-gtk-meson

go 1.25.0

tool github.com/dennwc/flatpak-go-mod

require (
	codeberg.org/puregotk/puregotk v0.0.0-20260224100813-799416f97c3f
	github.com/pojntfx/go-gettext v0.3.0
)

require (
	codeberg.org/puregotk/purego v0.0.0-20260224095105-2513c838cb80 // indirect
	github.com/dennwc/flatpak-go-mod v0.1.1-0.20251127123506-956509dd96ba // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/jwijenbergh/purego v0.0.0-20251017112123-b71757b9ba42 // indirect
	golang.org/x/mod v0.30.0 // indirect
)

replace codeberg.org/puregotk/puregotk v0.0.0-00010101000000-000000000000 => ../..
