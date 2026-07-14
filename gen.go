package main

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bnema/puregotk/pkg/gir/pass"
	"github.com/bnema/puregotk/pkg/gir/util"
)

//go:generate go run gen.go

func main() {
	dir := "v4"
	os.RemoveAll(dir)
	var girs []string
	filepath.Walk("internal/gir/spec", func(path string, f os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".gir") {
			return nil
		}
		girs = append(girs, path)
		return nil
	})

	p, err := pass.New(girs, "github.com/bnema/puregotk/v4")
	if err != nil {
		panic(err)
	}
	// collect basic type info
	p.First()

	// Create the template
	gotemp, err := template.New("go").Funcs(template.FuncMap{
		"conv":     util.ConvertArgs,
		"convc":    util.ConvertArgsComma,
		"convcb":   util.ConvertCallbackArgs,
		"convcd":   util.ConvertArgsCommaDeref,
		"convd":    util.ConvertArgsDeref,
		"convcbne": util.ConvertCallbackArgsNoErr,
		"convcbe":  util.ConvertCallbackArgsWithErr,
		"propsset": util.PropertyScalarSet,
		"propsget": util.PropertyScalarGet,
		"propvset": util.PropertyVectorSet,
		"propvget": util.PropertyVectorGet,
	}).ParseFiles("templates/go")
	if err != nil {
		panic(err)
	}

	// Write go files by making the second pass
	p.Second(dir, gotemp)

	// Finally copy some extra code that we want in the API
	data, err := os.ReadFile("templates/gobject")
	if err == nil {
		os.WriteFile("v4/gobject/more.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/gtype")
	if err == nil {
		mkerr := os.MkdirAll("v4/gobject/types", 0o755)
		if mkerr != nil {
			panic(mkerr)
		}
		os.WriteFile("v4/gobject/types/types.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/glib")
	if err == nil {
		os.WriteFile("v4/glib/more.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/glib_sysv")
	if err == nil {
		os.WriteFile("v4/glib/more_sysv.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/glib_windows")
	if err == nil {
		os.WriteFile("v4/glib/more_windows.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/glib_other")
	if err == nil {
		os.WriteFile("v4/glib/more_other.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/glib_callbacks_test")
	if err == nil {
		os.WriteFile("v4/glib/callbacks_test.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/gobject_signal_lifecycle_test")
	if err == nil {
		if werr := os.WriteFile("v4/gobject/signal_lifecycle_test.go", data, 0o644); werr != nil {
			panic(werr)
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}
	data, err = os.ReadFile("templates/webkit")
	if err == nil {
		os.WriteFile("v4/webkit/more.go", data, 0o644)
	}
	data, err = os.ReadFile("templates/gdk_dmabuf")
	if err == nil {
		if mkerr := os.MkdirAll("v4/gdk", 0o755); mkerr != nil {
			panic(mkerr)
		}
		if werr := os.WriteFile("v4/gdk/gdkdmabuftexturebuilder_extra.go", data, 0o644); werr != nil {
			panic(werr)
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}
}
