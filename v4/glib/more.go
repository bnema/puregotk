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

// ---------------------------------------------------------------------------
// Source callback trampoline
//
// GLib source functions (IdleAdd, TimeoutAdd, etc.) previously allocated a new
// purego callback slot for every invocation. purego has a hard limit of 2000
// slots, so long-running programs that schedule many one-shot idle/timeout
// callbacks would exhaust the pool and panic.
//
// The trampoline uses a single purego callback that dispatches through a
// Go-side map keyed by an opaque ID passed as GLib's user_data pointer.
// This means all IdleAdd/TimeoutAdd calls share ONE purego slot regardless
// of how many are outstanding.
// ---------------------------------------------------------------------------

// sourceEntry holds a registered GLib source callback.
type sourceEntry struct {
	fn   SourceFunc
	once bool // if true, automatically remove after first call (SourceOnceFunc semantics)
}

var sourceTrampolines = struct {
	sync.Mutex
	nextID         uintptr
	funcs          map[uintptr]*sourceEntry
	sourceToDataID map[uint]uintptr // GLib source ID → trampoline data ID
}{
	funcs:          make(map[uintptr]*sourceEntry),
	sourceToDataID: make(map[uint]uintptr),
}

// sourceTrampolineCb is the single purego callback shared by all source functions.
// GLib calls it with the user_data we provided (our map key).
var sourceTrampolineCb uintptr

// sourceTrampolineOnceCb is the single purego callback for SourceOnceFunc sources
// (IdleAddOnce, TimeoutAddOnce). These have signature func(uintptr) with no return.
var sourceTrampolineOnceCb uintptr

func initSourceTrampoline() {
	fn := func(id uintptr) uintptr {
		sourceTrampolines.Lock()
		entry, ok := sourceTrampolines.funcs[id]
		if !ok {
			sourceTrampolines.Unlock()
			return 0 // SOURCE_REMOVE — entry already cleaned up (e.g. by SourceRemove)
		}
		cb := entry.fn
		sourceTrampolines.Unlock()

		result := cb(0)

		if !result {
			sourceTrampolines.Lock()
			delete(sourceTrampolines.funcs, id)
			// Also clean up the reverse mapping (source ID → data ID).
			for sid, did := range sourceTrampolines.sourceToDataID {
				if did == id {
					delete(sourceTrampolines.sourceToDataID, sid)
					break
				}
			}
			sourceTrampolines.Unlock()
		}
		if result {
			return 1
		}
		return 0
	}
	sourceTrampolineCb = purego.NewCallback(fn)

	onceFn := func(id uintptr) {
		sourceTrampolines.Lock()
		entry, ok := sourceTrampolines.funcs[id]
		if !ok {
			sourceTrampolines.Unlock()
			return
		}
		cb := entry.fn
		delete(sourceTrampolines.funcs, id)
		// Also clean up the reverse mapping.
		for sid, did := range sourceTrampolines.sourceToDataID {
			if did == id {
				delete(sourceTrampolines.sourceToDataID, sid)
				break
			}
		}
		sourceTrampolines.Unlock()

		cb(0)
	}
	sourceTrampolineOnceCb = purego.NewCallback(onceFn)
}

// registerSourceFunc stores a SourceFunc in the trampoline map and returns
// the trampoline callback pointer and the user_data key.
func registerSourceFunc(fn *SourceFunc, once bool) (trampolineCb uintptr, userData uintptr) {
	if fn == nil {
		return 0, 0
	}
	sourceTrampolines.Lock()
	sourceTrampolines.nextID++
	id := sourceTrampolines.nextID
	sourceTrampolines.funcs[id] = &sourceEntry{fn: *fn, once: once}
	sourceTrampolines.Unlock()
	if once {
		return sourceTrampolineOnceCb, id
	}
	return sourceTrampolineCb, id
}

// registerSourceOnceFunc stores a SourceOnceFunc in the trampoline map and
// returns the trampoline callback pointer and the user_data key.
func registerSourceOnceFunc(fn *SourceOnceFunc) (trampolineCb uintptr, userData uintptr) {
	if fn == nil {
		return 0, 0
	}
	// Wrap SourceOnceFunc as SourceFunc so the entry type is uniform.
	wrapped := SourceFunc(func(data uintptr) bool {
		(*fn)(data)
		return false
	})
	return registerSourceFunc(&wrapped, true)
}

// saveSourceTrampolineMapping records the GLib source ID → trampoline data ID
// mapping so that SourceRemove can clean up the trampoline entry.
func saveSourceTrampolineMapping(sourceID uint, dataID uintptr) {
	if sourceID == 0 {
		return
	}
	sourceTrampolines.Lock()
	sourceTrampolines.sourceToDataID[sourceID] = dataID
	sourceTrampolines.Unlock()
}

// removeSourceTrampolineBySourceID cleans up trampoline state when a GLib
// source is removed via SourceRemove (before the callback fires).
func removeSourceTrampolineBySourceID(sourceID uint) {
	sourceTrampolines.Lock()
	if dataID, ok := sourceTrampolines.sourceToDataID[sourceID]; ok {
		delete(sourceTrampolines.sourceToDataID, sourceID)
		delete(sourceTrampolines.funcs, dataID)
	}
	sourceTrampolines.Unlock()
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

func init() {
	initSourceTrampoline()
}

func (e *Error) Error() string {
	return fmt.Sprintf("Gtk reported an error with message: '%s', domain: '%v' and code: '%v'", e.MessageGo(), e.Domain, e.Code)
}

func (e *Error) MessageGo() string {
	return core.GoString(e.Message)
}
