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
		if findings := lazyGuardFindings(path, source); len(findings) != 0 {
			t.Errorf("%s has non-dominating lazy guards:\n%s", path, strings.Join(findings, "\n"))
		}
	}
}

func TestDisplayGetDefaultRetainsGeneratedBorrowedResult(t *testing.T) {
	source, err := os.ReadFile("../../../v4/gdk/gdkdisplay.go")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "func DisplayGetDefault() *Display {")
	if start < 0 {
		t.Fatal("generated DisplayGetDefault wrapper not found")
	}
	body := string(source[start:])
	end := strings.Index(body, "\n}\n")
	if end < 0 {
		t.Fatal("generated DisplayGetDefault wrapper is not terminated")
	}
	body = body[:end+3]

	ordered := []string{
		`core.LazyRegister(&xDisplayGetDefault, "GDK", "gdk_display_get_default", false)`,
		"cret := xDisplayGetDefault()",
		"if cret == 0",
		"gobject.IncreaseRef(cret)",
		"cls.Ptr = cret",
	}
	previous := -1
	for _, want := range ordered {
		if strings.Count(body, want) != 1 {
			t.Fatalf("DisplayGetDefault generated body has %d occurrences of %q, want 1:\n%s", strings.Count(body, want), want, body)
		}
		position := strings.Index(body, want)
		if position <= previous {
			t.Fatalf("DisplayGetDefault generated operations are out of order at %q:\n%s", want, body)
		}
		previous = position
	}
}

func TestLazyGuardValidationRejectsMisleadingGuards(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "guard after call",
			source: `package fixture
var xBad func()
func Bad() {
	xBad()
	core.LazyRegister(&xBad, "BAD", "bad", false)
}`,
		},
		{
			name: "guard in unrelated nested closure",
			source: `package fixture
var xBad func()
func Bad() {
	_ = func() { core.LazyRegister(&xBad, "BAD", "bad", false) }
	xBad()
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if findings := lazyGuardFindings(tt.name, []byte(tt.source)); len(findings) == 0 {
				t.Fatal("misleading guard was accepted")
			}
		})
	}
}

func TestLazyGuardValidationInspectsFunctionLiterals(t *testing.T) {
	source := []byte(`package fixture
var xGood func()
var guarded = func() {
	core.LazyRegister(&xGood, "GOOD", "good", false)
	xGood()
}`)
	if findings := lazyGuardFindings("function literal", source); len(findings) != 0 {
		t.Fatalf("guarded function literal rejected: %v", findings)
	}
}

func lazyGuardFindings(filename string, source []byte) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		return []string{err.Error()}
	}

	helperTargets := make(map[string]string)
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if target := soleDirectLazyTarget(decl.Body); target != "" {
				helperTargets[decl.Name.Name] = target
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
					continue
				}
				lit, ok := value.Values[0].(*ast.FuncLit)
				if ok {
					if target := soleDirectLazyTarget(lit.Body); target != "" {
						helperTargets[value.Names[0].Name] = target
					}
				}
			}
		}
	}

	var findings []string
	ast.Inspect(file, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}
		guarded := make(map[string]bool)
		for _, stmt := range block.List {
			for _, call := range directCalls(stmt) {
				if target := lazyTarget(call); target != "" {
					guarded[target] = true
					continue
				}
				name, ok := call.Fun.(*ast.Ident)
				if !ok {
					continue
				}
				if target := helperTargets[name.Name]; target != "" {
					guarded[target] = true
					continue
				}
				if isManualNativeTarget(name.Name) && !guarded[name.Name] {
					pos := fset.Position(call.Pos())
					findings = append(findings, pos.String()+": "+name.Name+" call lacks a preceding same-block lazy guard")
				}
			}
		}
		return true
	})
	return findings
}

func soleDirectLazyTarget(block *ast.BlockStmt) string {
	if block == nil || len(block.List) != 1 {
		return ""
	}
	statement, ok := block.List[0].(*ast.ExprStmt)
	if !ok {
		return ""
	}
	call, ok := statement.X.(*ast.CallExpr)
	if !ok {
		return ""
	}
	return lazyTarget(call)
}

func directCalls(node ast.Node) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(node, func(child ast.Node) bool {
		if child == nil {
			return false
		}
		if child != node {
			switch child.(type) {
			case *ast.BlockStmt, *ast.FuncLit:
				return false
			}
		}
		if call, ok := child.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func lazyTarget(call *ast.CallExpr) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "LazyRegister" {
		return ""
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "core" || len(call.Args) == 0 {
		return ""
	}
	address, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || address.Op != token.AND {
		return ""
	}
	target, ok := address.X.(*ast.Ident)
	if !ok || !isManualNativeTarget(target.Name) {
		return ""
	}
	return target.Name
}

func isManualNativeTarget(name string) bool {
	return len(name) > 1 && name[0] == 'x' && name[1] >= 'A' && name[1] <= 'Z'
}

func TestCanonicalManualFilesMatchGeneratedCopies(t *testing.T) {
	pairs := []struct {
		canonical string
		generated string
	}{
		{"../../../templates/gobject", "../../../v4/gobject/more.go"},
		{"../../../templates/gobject_signal_lifecycle_test", "../../../v4/gobject/signal_lifecycle_test.go"},
		{"../../../templates/gdk_dmabuf", "../../../v4/gdk/gdkdmabuftexturebuilder_extra.go"},
		{"../../../templates/gdk_dmabuf_test", "../../../v4/gdk/gdkdmabuftexturebuilder_extra_test.go"},
		{"../../../templates/webkit", "../../../v4/webkit/more.go"},
		{"../../../templates/webkit_test", "../../../v4/webkit/more_test.go"},
	}
	for _, pair := range pairs {
		t.Run(filepath.Base(pair.generated), func(t *testing.T) {
			canonical, err := os.ReadFile(pair.canonical)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := os.ReadFile(pair.generated)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canonical, generated) {
				t.Fatalf("generated copy %s differs from canonical template %s", pair.generated, pair.canonical)
			}
		})
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
