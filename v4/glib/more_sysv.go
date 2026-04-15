//go:build darwin || freebsd || (linux && (amd64 || arm64))

package glib

import (
	"fmt"
	"reflect"

	"github.com/bnema/purego"
)

func unrefCallback(fnPtr interface{}) error {
	val := reflect.ValueOf(fnPtr)
	if val.IsNil() {
		return fmt.Errorf("function pointer must not be nil")
	}
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Func {
		return fmt.Errorf("type must be a function pointer")
	}
	cbPtr := val.Pointer()
	if _, ok := GetCallback(cbPtr); !ok {
		return fmt.Errorf("callback not found in registry")
	}
	defer RemoveCallback(cbPtr)
	return purego.UnrefCallback(refPtr)
}
