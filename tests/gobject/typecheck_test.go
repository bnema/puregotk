//go:build linux

package gobject_test

import (
	"testing"

	"github.com/bnema/puregotk/v4/gobject"
)

func TestTypeCheckInstanceIsAPtrNil(t *testing.T) {
	if gobject.TypeCheckInstanceIsAPtr(0, gobject.ObjectGLibType()) {
		t.Fatal("TypeCheckInstanceIsAPtr should return false for a nil pointer")
	}

	var obj *gobject.Object
	if obj.IsA(gobject.ObjectGLibType()) {
		t.Fatal("(*Object).IsA should return false for a nil receiver")
	}
}

func TestTypeCheckInstanceIsAPtrObject(t *testing.T) {
	obj := gobject.NewObjectv(gobject.ObjectGLibType(), 0, nil)
	if obj == nil {
		t.Fatal("NewObjectv returned nil")
	}
	defer obj.Unref()

	if !gobject.TypeCheckInstanceIsAPtr(obj.GoPointer(), gobject.ObjectGLibType()) {
		t.Fatal("TypeCheckInstanceIsAPtr should recognize GObject instances")
	}

	if !obj.IsA(gobject.ObjectGLibType()) {
		t.Fatal("(*Object).IsA should recognize GObject instances")
	}

	if obj.IsA(gobject.TypeParamVal) {
		t.Fatal("(*Object).IsA should return false for unrelated GTypes")
	}
}
