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
	sourceToCallback  map[uint]uintptr
	callbackRefCount  map[uintptr]int
}{
	refs:              make(map[uintptr]uintptr),
	closures:          make(map[uintptr]interface{}),
	handlerToCallback: make(map[uint]uintptr),
	sourceToCallback:  make(map[uint]uintptr),
	callbackRefCount:  make(map[uintptr]int),
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
	if _, ok := callbacks.callbackRefCount[cbPtr]; !ok {
		callbacks.callbackRefCount[cbPtr] = 1
	}
	callbacks.Unlock()
}

// RemoveCallback removes a callback from the registry, allowing it to be garbage
// collected.
// Users should not need to call this.
func RemoveCallback(cbPtr uintptr) {
	callbacks.Lock()
	for handlerID, mappedCbPtr := range callbacks.handlerToCallback {
		if mappedCbPtr == cbPtr {
			delete(callbacks.handlerToCallback, handlerID)
		}
	}
	for sourceID, mappedCbPtr := range callbacks.sourceToCallback {
		if mappedCbPtr == cbPtr {
			delete(callbacks.sourceToCallback, sourceID)
		}
	}
	delete(callbacks.refs, cbPtr)
	delete(callbacks.closures, cbPtr)
	delete(callbacks.callbackRefCount, cbPtr)
	callbacks.Unlock()
}

// acquireCallbackRef increments callbackRefCount for cbPtr.
// Caller must hold callbacks.Lock().
func acquireCallbackRef(cbPtr uintptr) {
	callbacks.callbackRefCount[cbPtr]++
}

func hasCallbackMappings(cbPtr uintptr) bool {
	for _, mappedCbPtr := range callbacks.handlerToCallback {
		if mappedCbPtr == cbPtr {
			return true
		}
	}
	for _, mappedCbPtr := range callbacks.sourceToCallback {
		if mappedCbPtr == cbPtr {
			return true
		}
	}
	return false
}

// releaseCallbackRef decrements callbackRefCount for cbPtr and removes callback
// data when it reaches zero.
// Caller must hold callbacks.Lock().
// Handler/source mappings to cbPtr are expected to be removed or replaced by
// the caller (RemoveCallbackByHandler, RemoveCallbackBySource,
// SaveHandlerMapping, SaveSourceMapping).
func releaseCallbackRef(cbPtr uintptr) {
	count, ok := callbacks.callbackRefCount[cbPtr]
	if !ok {
		return
	}

	count--
	if count > 0 {
		callbacks.callbackRefCount[cbPtr] = count
		return
	}

	delete(callbacks.callbackRefCount, cbPtr)
	delete(callbacks.refs, cbPtr)
	delete(callbacks.closures, cbPtr)
}

// SaveHandlerMapping records a signal handler ID → callback pointer mapping
// so that DisconnectSignal can clean up the callback registry.
func SaveHandlerMapping(handlerID uint, cbPtr uintptr) {
	if handlerID == 0 {
		return
	}

	callbacks.Lock()
	defer callbacks.Unlock()
	if prevCbPtr, ok := callbacks.handlerToCallback[handlerID]; ok {
		if prevCbPtr == cbPtr {
			return
		}
		releaseCallbackRef(prevCbPtr)
		if !hasCallbackMappings(prevCbPtr) {
			releaseCallbackRef(prevCbPtr)
		}
	}
	callbacks.handlerToCallback[handlerID] = cbPtr
	acquireCallbackRef(cbPtr)
}

// RemoveCallbackByHandler removes a callback from the registry using a signal handler ID.
func RemoveCallbackByHandler(handlerID uint) {
	callbacks.Lock()
	if cbPtr, ok := callbacks.handlerToCallback[handlerID]; ok {
		delete(callbacks.handlerToCallback, handlerID)
		releaseCallbackRef(cbPtr)
		if !hasCallbackMappings(cbPtr) {
			releaseCallbackRef(cbPtr)
		}
	}
	callbacks.Unlock()
}

// SaveSourceMapping records a source ID -> callback pointer mapping.
func SaveSourceMapping(sourceID uint, cbPtr uintptr) {
	if sourceID == 0 {
		return
	}

	callbacks.Lock()
	defer callbacks.Unlock()
	if prevCbPtr, ok := callbacks.sourceToCallback[sourceID]; ok {
		if prevCbPtr == cbPtr {
			return
		}
		releaseCallbackRef(prevCbPtr)
		if !hasCallbackMappings(prevCbPtr) {
			releaseCallbackRef(prevCbPtr)
		}
	}
	callbacks.sourceToCallback[sourceID] = cbPtr
	acquireCallbackRef(cbPtr)
}

// RemoveCallbackBySource removes a callback mapping using a source ID.
func RemoveCallbackBySource(sourceID uint) {
	callbacks.Lock()
	if cbPtr, ok := callbacks.sourceToCallback[sourceID]; ok {
		delete(callbacks.sourceToCallback, sourceID)
		releaseCallbackRef(cbPtr)
		if !hasCallbackMappings(cbPtr) {
			releaseCallbackRef(cbPtr)
		}
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
