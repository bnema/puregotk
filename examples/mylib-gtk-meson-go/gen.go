package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/bnema/puregotk/pkg/gir/pass"
	"github.com/bnema/puregotk/pkg/gir/util"
)

//go:generate go run gen.go

func main() {
	dir := "."
	os.RemoveAll(dir)
	var girs []string
	filepath.Walk("internal/gir/spec", func(path string, f os.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".gir") {
			return nil
		}
		girs = append(girs, path)
		return nil
	})

	// Locate puregotk dependency and collect its GIR files
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/bnema/puregotk")
	output, err := cmd.Output()
	if err != nil {
		panic("puregotk dependency not found: " + err.Error())
	}
	puregotk := strings.TrimSpace(string(output))

	var puregotkGirs []string
	filepath.Walk(filepath.Join(puregotk, "internal/gir/spec"), func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".gir") {
			puregotkGirs = append(puregotkGirs, path)
		}
		return nil
	})

	p, err := pass.New(girs, "github.com/bnema/puregotk/examples/mylib-gtk-meson-go",
		pass.Dependency{
			Module: "github.com/bnema/puregotk/v4",
			Files:  puregotkGirs,
		},
	)
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
		"propsset": util.PropertyScalarSet,
		"propsget": util.PropertyScalarGet,
		"propvset": util.PropertyVectorSet,
		"propvget": util.PropertyVectorGet,
	}).ParseFiles(filepath.Join(puregotk, "templates/go"))
	if err != nil {
		panic(err)
	}

	// Only generate code for local namespaces
	p.Parsed = p.Parsed[:len(girs)]

	// Write go files by making the second pass
	p.Second(dir, gotemp)
}
