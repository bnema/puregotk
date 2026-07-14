package pass

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/bnema/puregotk/internal/gir/types"
	"github.com/bnema/puregotk/internal/gir/util"
)

func TestFileHasBindingsIncludesTypeGetters(t *testing.T) {
	if !(&file{aliases: []types.AliasTemplate{{TypeGetter: "demo_type_get"}}}).hasBindings() {
		t.Fatal("type getter was omitted from lazy binding setup")
	}
	if (&file{}).hasBindings() {
		t.Fatal("empty generated file unexpectedly needs lazy binding setup")
	}
}

func TestThrowingCallbackAccessorsAdaptHiddenError(t *testing.T) {
	data, err := os.ReadFile("../../../templates/go")
	if err != nil {
		t.Fatal(err)
	}
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
	}).Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}

	p, err := New([]string{"../../../internal/gir/spec/GLib-2.0.gir"}, "github.com/bnema/puregotk/v4")
	if err != nil {
		t.Fatal(err)
	}
	p.First()
	out := t.TempDir()
	p.Second(out, gotemp)
	generated, err := os.ReadFile(filepath.Join(out, "glib", "giochannel.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	for _, want := range []string{
		"func (x *IOFuncs) OverrideIoRead(cb func(*IOChannel, string, uint, uint) IOStatus)",
		"purego.NewCallback(func(ChannelVarp *IOChannel, BufVarp string, CountVarp uint, BytesReadVarp uint, cerrp **Error) IOStatus",
		"return cb(ChannelVarp, BufVarp, CountVarp, BytesReadVarp)",
		"var rawCallback func(ChannelVarp *IOChannel, BufVarp string, CountVarp uint, BytesReadVarp uint, cerrp **Error) IOStatus",
		"var cerr *Error",
		"return rawCallback(ChannelVar, BufVar, CountVar, BytesReadVar, &cerr)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("throwing IOFuncs callback did not adapt its hidden GError parameter; missing %q\\n%s", want, source)
		}
	}
}

func TestManualNativeTemplateCallsAreLazyGuarded(t *testing.T) {
	paths := []string{
		"../../../templates/gobject",
		"../../../templates/gdk_dmabuf",
		"../../../templates/webkit",
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		functionSource := func(fn *ast.FuncDecl) string {
			return string(source[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset])
		}
		var functions []*ast.FuncDecl
		var targets = make(map[string]bool)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			functions = append(functions, fn)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if ok && len(name.Name) > 1 && name.Name[0] == 'x' && name.Name[1] >= 'A' && name.Name[1] <= 'Z' {
					targets[name.Name] = true
				}
				return true
			})
		}
		lazyHelper := make(map[string]string)
		for _, fn := range functions {
			body := functionSource(fn)
			for target := range targets {
				if strings.Contains(body, "core.LazyRegister(&"+target+",") {
					lazyHelper[fn.Name.Name] = target
				}
			}
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
					continue
				}
				fn, ok := value.Values[0].(*ast.FuncLit)
				if !ok {
					continue
				}
				body := string(source[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset])
				for target := range targets {
					if strings.Contains(body, "core.LazyRegister(&"+target+",") {
						lazyHelper[value.Names[0].Name] = target
					}
				}
			}
		}
		for _, fn := range functions {
			body := functionSource(fn)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if !ok || !targets[name.Name] {
					return true
				}
				guarded := strings.Contains(body, "core.LazyRegister(&"+name.Name+",")
				for helper, target := range lazyHelper {
					guarded = guarded || (target == name.Name && strings.Contains(body, helper+"("))
				}
				if !guarded {
					t.Errorf("manual native target %s in %s:%s is not guarded by lazy registration", name.Name, path, fn.Name.Name)
				}
				return true
			})
		}
	}
}

func TestGoTemplateEmitsLazySymbolRegistration(t *testing.T) {
	data, err := os.ReadFile("../../../templates/go")
	if err != nil {
		t.Fatal(err)
	}
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
	}).Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	err = gotemp.Execute(&output, types.TemplateArg{
		PkgName:       "demo",
		PkgEnv:        "DEMO",
		PkgConfigName: "demo-1.0",
		NeedsInit:     true,
		Functions: []types.FuncTemplate{{
			Name:  "Demo",
			CName: "demo_symbol",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated := output.String()
	if !strings.Contains(generated, `core.LazyRegister(&xDemo, "DEMO", "demo_symbol", false)`) {
		t.Fatalf("generated function does not lazily register its symbol:\n%s", generated)
	}
	if strings.Contains(generated, "purego.Dlopen") || strings.Contains(generated, "core.PuregoSafeRegister") {
		t.Fatalf("generated init still eagerly opens or registers symbols:\n%s", generated)
	}

	throwingImports := types.NewImportSet("gio", "example.test", nil)
	throwingArgs := (&types.Parameters{}).Template("glib", "", types.KindMap{}, true, types.ArgsFromGoToC, throwingImports)
	throwingRet := (&types.ReturnValue{}).Template("glib", "", types.KindMap{}, true, throwingImports)

	output.Reset()
	err = gotemp.Execute(&output, types.TemplateArg{
		PkgName:   "gio",
		PkgEnv:    "GIO",
		NeedsInit: true,
		Interfaces: []types.InterfaceTemplate{{
			Name: "Action",
			Methods: []types.InterfaceFuncTemplate{
				{
					FullName: "GActionActivate",
					FuncTemplate: types.FuncTemplate{
						Name:  "Activate",
						CName: "g_action_activate",
					},
				},
				{
					FullName: "GActionFail",
					FuncTemplate: types.FuncTemplate{
						Name:  "Fail",
						CName: "g_action_fail",
						Args:  throwingArgs,
						Ret:   throwingRet,
					},
				},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated = output.String()
	if !strings.Contains(generated, "var XGActionActivate func(uintptr)") ||
		!strings.Contains(generated, "= func(instance uintptr)") ||
		!strings.Contains(generated, `core.LazyRegister(&xXGActionActivate, "GIO", "g_action_activate", false)`) ||
		!strings.Contains(generated, "xXGActionActivate(instance)") ||
		!strings.Contains(generated, "func(instance uintptr, cerrp **Error)") ||
		!strings.Contains(generated, "xXGActionFail(instance, cerrp)") {
		t.Fatalf("exported interface binding is not a direct-callable lazy thunk:\n%s", generated)
	}
	if _, err := format.Source([]byte(generated)); err != nil {
		t.Fatalf("direct-callable interface thunk is not valid Go: %v\n%s", err, generated)
	}

	output.Reset()
	err = gotemp.Execute(&output, types.TemplateArg{
		PkgName:       "webkit",
		PkgEnv:        "WEBKIT",
		NeedsInit:     true,
		RegisterTypes: true,
		Classes: []types.ClassTemplate{{
			Name:       "WebView",
			TypeGetter: "webkit_web_view_get_type",
		}},
		Interfaces: []types.InterfaceTemplate{{
			Name:       "NavigationPolicyDecision",
			TypeGetter: "webkit_navigation_policy_decision_get_type",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	generated = output.String()
	if !strings.Contains(generated, "Manually register types") ||
		!strings.Contains(generated, "WebViewGLibType()") ||
		!strings.Contains(generated, "NavigationPolicyDecisionGLibType()") {
		t.Fatalf("WebKit type-registration workaround was omitted:\n%s", generated)
	}

	output.Reset()
	err = gotemp.Execute(&output, types.TemplateArg{
		PkgName:         "optional",
		PkgEnv:          "OPTIONAL",
		NeedsInit:       true,
		OptionalLibrary: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `func Available() bool`) ||
		!strings.Contains(output.String(), `core.LibraryAvailable("OPTIONAL")`) {
		t.Fatalf("optional package no longer preserves Available:\n%s", output.String())
	}
}
