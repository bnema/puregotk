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
	handlerToCallback map[uint32]uintptr
	callbackRefCount  map[uintptr]int
}{
	refs:              make(map[uintptr]uintptr),
	closures:          make(map[uintptr]interface{}),
	handlerToCallback: make(map[uint32]uintptr),
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
func SaveHandlerMapping(handlerID uint32, cbPtr uintptr) {
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
func RemoveCallbackByHandler(handlerID uint32) {
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
	entries map[uint32]trackedSourceEntry
}{
	entries: make(map[uint32]trackedSourceEntry),
}

var (
	sourceFuncTrampolineCb     uintptr
	sourceOnceFuncTrampolineCb uintptr
	childWatchFuncTrampolineCb uintptr
)

func currentTrackedSourceID() uint32 {
	src := MainCurrentSource()
	if src == nil {
		return 0
	}
	return src.GetId()
}

func getTrackedSourceEntry() (uint32, trackedSourceEntry, bool) {
	sourceID := currentTrackedSourceID()
	if sourceID == 0 {
		return 0, trackedSourceEntry{}, false
	}
	trackedSources.RLock()
	entry, ok := trackedSources.entries[sourceID]
	trackedSources.RUnlock()
	return sourceID, entry, ok
}

func trackSourceFunc(sourceID uint32, fn *SourceFunc, data uintptr) {
	if sourceID == 0 || fn == nil {
		return
	}
	trackedSources.Lock()
	trackedSources.entries[sourceID] = trackedSourceEntry{data: data, sourceFunc: *fn}
	trackedSources.Unlock()
}

func trackSourceOnceFunc(sourceID uint32, fn *SourceOnceFunc, data uintptr) {
	if sourceID == 0 || fn == nil {
		return
	}
	trackedSources.Lock()
	trackedSources.entries[sourceID] = trackedSourceEntry{data: data, sourceOnceFunc: *fn}
	trackedSources.Unlock()
}

func trackChildWatchFunc(sourceID uint32, fn *ChildWatchFunc, data uintptr) {
	if sourceID == 0 || fn == nil {
		return
	}
	trackedSources.Lock()
	trackedSources.entries[sourceID] = trackedSourceEntry{data: data, childWatchFunc: *fn}
	trackedSources.Unlock()
}

func removeTrackedSource(sourceID uint32) {
	if sourceID == 0 {
		return
	}
	trackedSources.Lock()
	delete(trackedSources.entries, sourceID)
	trackedSources.Unlock()
}

func trackedSourceIDByUserData(data uintptr) uint32 {
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
		entry.childWatchFunc(pid, waitStatus, data)
	})
}

func init() {
	initSourceTrampolines()
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
