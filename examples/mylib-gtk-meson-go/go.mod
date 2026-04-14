module codeberg.org/puregotk/puregotk/examples/mylib-gtk-meson-go

go 1.25.0

replace codeberg.org/puregotk/puregotk v0.0.0-00010101000000-000000000000 => ../..

require (
	codeberg.org/puregotk/purego v0.0.0-20260224095105-2513c838cb80
	codeberg.org/puregotk/puregotk v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	mvdan.cc/gofumpt v0.9.2 // indirect
)
