package pass

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"text/template"

	"github.com/bnema/puregotk/internal/gir/types"
	"github.com/bnema/puregotk/internal/gir/util"
)

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
}
