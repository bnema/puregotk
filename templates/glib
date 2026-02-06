package glib

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/jwijenbergh/purego"
	"github.com/jwijenbergh/puregotk/pkg/core"
)

var callbacks = struct {
	sync.RWMutex
	refs              map[uintptr]uintptr
	closures          map[uintptr]interface{}
	handlerToCallback map[uint]uintptr
}{
	refs:              make(map[uintptr]uintptr),
	closures:          make(map[uintptr]interface{}),
	handlerToCallback: make(map[uint]uintptr),
}

// GetCallback retrives a callback reference by value.
// Users should not need to call this.
func GetCallback(cbPtr uintptr) (uintptr, bool) {
	callbacks.RLock()
	defer callbacks.RUnlock()
	refPtr, ok := callbacks.refs[cbPtr]
	return refPtr, ok
}

// SaveCallback saves a reference to the callback value.
// Users should not need to call this.
func SaveCallback(cbPtr uintptr, refPtr uintptr) {
	callbacks.Lock()
	callbacks.refs[cbPtr] = refPtr
	callbacks.Unlock()
}

// SaveCallbackWithClosure saves a reference to the callback value and retains the
// provided closure to prevent it from being garbage collected.
// Users should not need to call this.
func SaveCallbackWithClosure(cbPtr uintptr, refPtr uintptr, closure interface{}) {
	callbacks.Lock()
	callbacks.refs[cbPtr] = refPtr
	callbacks.closures[cbPtr] = closure
	callbacks.Unlock()
}

// RemoveCallback removes a callback from the registry, allowing it to be garbage
// collected.
// Users should not need to call this.
func RemoveCallback(cbPtr uintptr) {
	callbacks.Lock()
	delete(callbacks.refs, cbPtr)
	delete(callbacks.closures, cbPtr)
	callbacks.Unlock()
}

// SaveHandlerMapping records a signal handler ID → callback pointer mapping
// so that DisconnectSignal can clean up the callback registry.
func SaveHandlerMapping(handlerID uint, cbPtr uintptr) {
	callbacks.Lock()
	callbacks.handlerToCallback[handlerID] = cbPtr
	callbacks.Unlock()
}

// RemoveCallbackByHandler removes a callback from the registry using a signal handler ID.
func RemoveCallbackByHandler(handlerID uint) {
	callbacks.Lock()
	if cbPtr, ok := callbacks.handlerToCallback[handlerID]; ok {
		delete(callbacks.refs, cbPtr)
		delete(callbacks.closures, cbPtr)
		delete(callbacks.handlerToCallback, handlerID)
	}
	callbacks.Unlock()
}

// UnrefCallbackValue unreferences the provided callback by reflect.value to free a purego slot
//
// NOTE: Windows does not support unreferencing callbacks, so on that platform this operation is
// a NOOP, callback memory is never freed, and there is a limit on maximum total callbacks.
// See the purego documentation for further details.
func UnrefCallback(fnPtr interface{}) error {
	return unrefCallback(fnPtr)
}

// NewCallback is an alias to purego.NewCallback
func NewCallback(fnPtr interface{}) uintptr {
	return purego.NewCallbackFnPtr(fnPtr)
}

// NewCallbackNullable is an alias to purego.NewCallback that returns a null pointer for null functions
func NewCallbackNullable(fn interface{}) uintptr {
	val := reflect.ValueOf(fn)
	if val.IsNil() {
		return 0
	}

	return NewCallback(fn)
}

func (e *Error) Error() string {
	return fmt.Sprintf("Gtk reported an error with message: '%s', domain: '%v' and code: '%v'", e.MessageGo(), e.Domain, e.Code)
}

func (e *Error) MessageGo() string {
	return core.GoString(e.Message)
}
