package glib

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/bnema/purego"
	"github.com/bnema/puregotk/pkg/core"
)

var callbacks = struct {
	sync.RWMutex
	refs              map[uintptr]uintptr
	closures          map[uintptr]interface{}
	handlerToCallback map[uint]uintptr
	callbackRefCount  map[uintptr]int
}{
	refs:              make(map[uintptr]uintptr),
	closures:          make(map[uintptr]interface{}),
	handlerToCallback: make(map[uint]uintptr),
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

// SaveCallbackWithClosure saves a reference to the callback value and retains
// the provided closure to prevent it from being garbage collected.
// Users should not need to call this.
func SaveCallbackWithClosure(cbPtr uintptr, refPtr uintptr, closure interface{}) {
	callbacks.Lock()
	callbacks.refs[cbPtr] = refPtr
	callbacks.closures[cbPtr] = closure
	callbacks.Unlock()
}

// RemoveCallback removes a callback from the registry.
// Users should not need to call this.
func RemoveCallback(cbPtr uintptr) {
	callbacks.Lock()
	for handlerID, mappedCbPtr := range callbacks.handlerToCallback {
		if mappedCbPtr == cbPtr {
			delete(callbacks.handlerToCallback, handlerID)
		}
	}
	delete(callbacks.refs, cbPtr)
	delete(callbacks.closures, cbPtr)
	delete(callbacks.callbackRefCount, cbPtr)
	callbacks.Unlock()
}

func retainCallback(cbPtr uintptr) {
	callbacks.callbackRefCount[cbPtr]++
}

func releaseCallback(cbPtr uintptr) {
	count := callbacks.callbackRefCount[cbPtr] - 1
	if count > 0 {
		callbacks.callbackRefCount[cbPtr] = count
		return
	}
	delete(callbacks.callbackRefCount, cbPtr)
	delete(callbacks.refs, cbPtr)
	delete(callbacks.closures, cbPtr)
}

// SaveHandlerMapping records a signal handler ID → callback pointer mapping so
// DisconnectSignal can clean up the callback registry.
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
		releaseCallback(prevCbPtr)
	}
	callbacks.handlerToCallback[handlerID] = cbPtr
	retainCallback(cbPtr)
}

// RemoveCallbackByHandler removes a callback from the registry using a signal handler ID.
func RemoveCallbackByHandler(handlerID uint) {
	callbacks.Lock()
	if cbPtr, ok := callbacks.handlerToCallback[handlerID]; ok {
		delete(callbacks.handlerToCallback, handlerID)
		releaseCallback(cbPtr)
	}
	callbacks.Unlock()
}

type trackedSourceEntry struct {
	data           uintptr
	sourceFunc     SourceFunc
	sourceOnceFunc SourceOnceFunc
	childWatchFunc ChildWatchFunc
}

var trackedSources = struct {
	sync.RWMutex
	entries map[uint]trackedSourceEntry
}{
	entries: make(map[uint]trackedSourceEntry),
}

var (
	sourceFuncTrampolineCb     uintptr
	sourceOnceFuncTrampolineCb uintptr
	childWatchFuncTrampolineCb uintptr
)

func currentTrackedSourceID() uint {
	src := MainCurrentSource()
	if src == nil {
		return 0
	}
	return src.GetId()
}

func getTrackedSourceEntry() (uint, trackedSourceEntry, bool) {
	sourceID := currentTrackedSourceID()
	if sourceID == 0 {
		return 0, trackedSourceEntry{}, false
	}
	trackedSources.RLock()
	entry, ok := trackedSources.entries[sourceID]
	trackedSources.RUnlock()
	return sourceID, entry, ok
}

func trackSourceFunc(sourceID uint, fn *SourceFunc, data uintptr) {
	if sourceID == 0 || fn == nil {
		return
	}
	trackedSources.Lock()
	trackedSources.entries[sourceID] = trackedSourceEntry{data: data, sourceFunc: *fn}
	trackedSources.Unlock()
}

func trackSourceOnceFunc(sourceID uint, fn *SourceOnceFunc, data uintptr) {
	if sourceID == 0 || fn == nil {
		return
	}
	trackedSources.Lock()
	trackedSources.entries[sourceID] = trackedSourceEntry{data: data, sourceOnceFunc: *fn}
	trackedSources.Unlock()
}

func trackChildWatchFunc(sourceID uint, fn *ChildWatchFunc, data uintptr) {
	if sourceID == 0 || fn == nil {
		return
	}
	trackedSources.Lock()
	trackedSources.entries[sourceID] = trackedSourceEntry{data: data, childWatchFunc: *fn}
	trackedSources.Unlock()
}

func removeTrackedSource(sourceID uint) {
	if sourceID == 0 {
		return
	}
	trackedSources.Lock()
	delete(trackedSources.entries, sourceID)
	trackedSources.Unlock()
}

func trackedSourceIDByUserData(data uintptr) uint {
	ctx := MainContextDefault()
	if ctx == nil {
		return 0
	}
	src := ctx.FindSourceByUserData(data)
	if src == nil {
		return 0
	}
	return src.GetId()
}

func initSourceTrampolines() {
	sourceFuncTrampolineCb = purego.NewCallback(func(data uintptr) bool {
		sourceID, entry, ok := getTrackedSourceEntry()
		if !ok || entry.sourceFunc == nil {
			return false
		}
		keep := entry.sourceFunc(data)
		if !keep {
			removeTrackedSource(sourceID)
		}
		return keep
	})

	sourceOnceFuncTrampolineCb = purego.NewCallback(func(data uintptr) {
		sourceID, entry, ok := getTrackedSourceEntry()
		if !ok || entry.sourceOnceFunc == nil {
			return
		}
		removeTrackedSource(sourceID)
		entry.sourceOnceFunc(data)
	})

	childWatchFuncTrampolineCb = purego.NewCallback(func(pid Pid, waitStatus int32, data uintptr) {
		sourceID, entry, ok := getTrackedSourceEntry()
		if !ok || entry.childWatchFunc == nil {
			return
		}
		removeTrackedSource(sourceID)
		entry.childWatchFunc(pid, int(waitStatus), data)
	})
}

func init() {
	initSourceTrampolines()
}

// UnrefCallbackValue unreferences the provided callback by reflect.value to free a purego slot
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

// UnrefCallback removes puregotk's bookkeeping for a callback created from
// a function pointer.
//
// NOTE: Upstream purego does not support releasing callback slots after
// purego.NewCallback, so this only clears puregotk's local registry state.
func UnrefCallback(fnPtr interface{}) error {
	return unrefCallback(fnPtr)
}

// NewCallback converts a pointer-to-function into a C callback pointer.
// It reuses an existing callback for the same function pointer via the
// local callback registry.
func NewCallback(fnPtr interface{}) uintptr {
	val := reflect.ValueOf(fnPtr)
	if val.IsNil() {
		panic("purego: function must not be nil")
	}
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Func {
		panic("purego: the type must be a function pointer but was not")
	}

	cbPtr := val.Pointer()
	if refPtr, ok := GetCallback(cbPtr); ok {
		return refPtr
	}

	refPtr := purego.NewCallback(val.Elem().Interface())
	SaveCallbackWithClosure(cbPtr, refPtr, fnPtr)
	return refPtr
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
