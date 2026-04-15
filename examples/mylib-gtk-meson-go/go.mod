module github.com/bnema/puregotk/examples/mylib-gtk-meson-go

go 1.25.0

replace github.com/bnema/puregotk v0.0.0-00010101000000-000000000000 => ../..

require (
	github.com/bnema/purego v0.11.0-bnema.2
	github.com/bnema/puregotk v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	mvdan.cc/gofumpt v0.9.2 // indirect
)
