package types

import (
	"strings"
	"testing"
)

func TestTransferNoneClassReturnEmitsIncreaseRef(t *testing.T) {
	kinds := KindMap{}
	kinds.Add("Gdk", "Display", ClassesType, nil)
	imports := NewImportSet("gdk", "github.com/bnema/puregotk/v4", nil)
	ret := (&ReturnValue{
		TransferOwnership: TransferOwnership{TransferOwnership: "none"},
		AnyType: AnyType{Type: &Type{Name: "Display", CType: "GdkDisplay*"}},
	}).Template("Gdk", "", kinds, false, imports)

	if !ret.Class || !ret.RefSink || ret.Raw != "uintptr" || ret.Value != "*Display" {
		t.Fatalf("transfer-none class return = %+v, want uintptr *Display with RefSink", ret)
	}

	output := ret.Fmt(true)
	const ref = "gobject.IncreaseRef(cret)"
	if !strings.Contains(output, ref) {
		t.Fatalf("transfer-none class return omitted borrowed-reference retention:\n%s", output)
	}
	if strings.Index(output, ref) > strings.Index(output, "cls.Ptr = cret") {
		t.Fatalf("borrowed reference was retained after publishing the pointer:\n%s", output)
	}
}
