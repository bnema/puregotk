package gobject

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/pkg/core"
	"github.com/bnema/puregotk/v4/glib"
	"github.com/bnema/puregotk/v4/gobject/types"
)

type Ptr interface {
	GoPointer() uintptr
	SetGoPointer(uintptr)
}

func ConvertPtr(a interface{}) *uintptr {
	if a == nil || (reflect.ValueOf(a).Kind() == reflect.Ptr && reflect.ValueOf(a).IsNil()) {
		return nil
	}
	ptr := reflect.ValueOf(a).Elem()
	v, ok := ptr.Interface().(Ptr)
	if !ok {
		panic("not valid")
	}
	g := v.GoPointer()
	return &g
}

func IncreaseRef(a uintptr) {
	xObjectRefSink(a)
}

func SignalConnect(a uintptr, b string, c uintptr) uint {
	return xSignalConnectData(a, b, c, 0, 0, 0)
}

func (o Object) Cast(v Ptr) {
	v.SetGoPointer(o.GoPointer())
}

func (o Object) ConnectSignal(signal string, cb *func()) uint {
	cbPtr := uintptr(unsafe.Pointer(cb))
	if cbRefPtr, ok := glib.GetCallback(cbPtr); ok {
		handlerID := SignalConnect(o.GoPointer(), signal, cbRefPtr)
		glib.SaveHandlerMapping(handlerID, cbPtr)
		return handlerID
	}

	cbRefPtr := glib.NewCallback(cb)
	glib.SaveCallbackWithClosure(cbPtr, cbRefPtr, cb)
	handlerID := SignalConnect(o.GoPointer(), signal, cbRefPtr)
	glib.SaveHandlerMapping(handlerID, cbPtr)
	return handlerID
}

func (o Object) DisconnectSignal(handler uint) {
	SignalHandlerDisconnect(&o, handler)
	glib.RemoveCallbackByHandler(handler)
}

// ConnectNotifyWithDetail connects to the "notify" signal with a detail string.
func (x *Object) ConnectNotifyWithDetail(detail string, cb *func(Object, *ParamSpec)) uint {
	cbPtr := uintptr(unsafe.Pointer(cb))
	signalName := fmt.Sprintf("notify::%s", detail)
	if cbRefPtr, ok := glib.GetCallback(cbPtr); ok {
		handlerID := SignalConnect(x.GoPointer(), signalName, cbRefPtr)
		glib.SaveHandlerMapping(handlerID, cbPtr)
		return handlerID
	}

	fcb := func(clsPtr uintptr, PspecVarp uintptr) {
		fa := Object{}
		fa.Ptr = clsPtr
		cbFn := *cb
		cbFn(fa, func() *ParamSpec { cls := &ParamSpec{}; cls.Ptr = PspecVarp; return cls }())
	}
	cbRefPtr := glib.NewCallback(&fcb)
	glib.SaveCallbackWithClosure(cbPtr, cbRefPtr, cb)
	handlerID := SignalConnect(x.GoPointer(), signalName, cbRefPtr)
	glib.SaveHandlerMapping(handlerID, cbPtr)
	return handlerID
}

var xTypeCheckInstanceIsAPtr func(uintptr, types.GType) bool

// TypeCheckInstanceIsAPtr is like TypeCheckInstanceIsA but accepts a GoPointer().
func TypeCheckInstanceIsAPtr(ptr uintptr, ifaceType types.GType) bool {
	if ptr == 0 {
		return false
	}
	return xTypeCheckInstanceIsAPtr(ptr, ifaceType)
}

// IsA reports whether o is an instance of t, one of its subtypes, or an implementation of t.
func (o *Object) IsA(t types.GType) bool {
	return TypeCheckInstanceIsAPtr(o.GoPointer(), t)
}

func init() {
	core.SetPackageName("GOBJECT", "gobject-2.0")
	core.SetSharedLibraries("GOBJECT", []string{"libgobject-2.0.so.0", "libgobject-2.0.0.dylib"})
	var libs []uintptr
	for _, libPath := range core.GetPaths("GOBJECT") {
		lib, err := purego.Dlopen(libPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			panic(err)
		}
		libs = append(libs, lib)
	}
	core.PuregoSafeRegister(&xTypeCheckInstanceIsAPtr, libs, "g_type_check_instance_is_a")
}

// types
const (
	TypeInvalidVal           Type = 0
	TypeNoneVal                   = 1 << 2
	TypeInterfaceVal              = 2 << 2
	TypeCharVal                   = 3 << 2
	TypeUcharVal                  = 4 << 2
	TypeBooleanVal                = 5 << 2
	TypeIntVal                    = 6 << 2
	TypeUintVal                   = 7 << 2
	TypeLongVal                   = 8 << 2
	TypeUlongVal                  = 9 << 2
	TypeInt64Val                  = 10 << 2
	TypeUint64Val                 = 11 << 2
	TypeEnumVal                   = 12 << 2
	TypeFlagsVal                  = 13 << 2
	TypeFloatVal                  = 14 << 2
	TypeDoubleVal                 = 15 << 2
	TypeStringVal                 = 16 << 2
	TypePointerVal                = 17 << 2
	TypeBoxedVal                  = 18 << 2
	TypeParamVal                  = 19 << 2
	TypeObjectVal                 = 20 << 2
	TypeReservedGLibLastVal       = 31 << 2
	TypeReservedBseFirstVal       = 32 << 2
	TypeReservedBseLastVal        = 48 << 2
	TypeReservedUserFirstVal      = 49 << 2
)
