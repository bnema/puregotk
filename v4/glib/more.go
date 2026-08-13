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
	refs                  map[uintptr]uintptr
	closures              map[uintptr]interface{}
	handlerToCallback     map[uint]uintptr
	callbackRefCount      map[uintptr]int
	sharedCallbacks       map[string]uintptr
	nextSignalHandlerData uintptr
	signalHandlers        map[uintptr]interface{}
	handlerToSignalData   map[uint]uintptr
}{
	refs:                make(map[uintptr]uintptr),
	closures:            make(map[uintptr]interface{}),
	handlerToCallback:   make(map[uint]uintptr),
	callbackRefCount:    make(map[uintptr]int),
	sharedCallbacks:     make(map[string]uintptr),
	signalHandlers:      make(map[uintptr]interface{}),
	handlerToSignalData: make(map[uint]uintptr),
}

// GetCallback retrieves and acquires a callback reference by value.
// The acquired reference is transferred to SaveHandlerMapping, which releases
// it when the signal handler is disconnected.
// Users should not need to call this.
func GetCallback(cbPtr uintptr) (uintptr, bool) {
	callbacks.Lock()
	defer callbacks.Unlock()
	refPtr, ok := callbacks.refs[cbPtr]
	if !ok {
		return 0, false
	}
	retainCallback(cbPtr)
	return refPtr, true
}

// SaveCallback saves and acquires a reference to the callback value.
// Users should not need to call this.
func SaveCallback(cbPtr uintptr, refPtr uintptr) {
	callbacks.Lock()
	callbacks.refs[cbPtr] = refPtr
	retainCallback(cbPtr)
	callbacks.Unlock()
}

// SaveCallbackWithClosure saves and acquires a reference to the callback value
// and retains the provided closure to prevent it from being garbage collected.
// Users should not need to call this.
func SaveCallbackWithClosure(cbPtr uintptr, refPtr uintptr, closure interface{}) {
	callbacks.Lock()
	callbacks.refs[cbPtr] = refPtr
	callbacks.closures[cbPtr] = closure
	retainCallback(cbPtr)
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
	if refPtr, ok := callbacks.refs[cbPtr]; ok {
		_ = purego.UnrefCallback(refPtr)
	}
	delete(callbacks.callbackRefCount, cbPtr)
	delete(callbacks.refs, cbPtr)
	delete(callbacks.closures, cbPtr)
}

// SaveHandlerMapping records a signal handler ID → callback pointer mapping so
// DisconnectSignal can clean up the callback registry. cbPtr must already have
// been acquired by GetCallback, SaveCallback, or SaveCallbackWithClosure.
func SaveHandlerMapping(handlerID uint, cbPtr uintptr) {
	callbacks.Lock()
	defer callbacks.Unlock()
	if handlerID == 0 {
		releaseCallback(cbPtr)
		return
	}
	if prevCbPtr, ok := callbacks.handlerToCallback[handlerID]; ok {
		if prevCbPtr == cbPtr {
			releaseCallback(cbPtr)
			return
		}
		releaseCallback(prevCbPtr)
	}
	callbacks.handlerToCallback[handlerID] = cbPtr
}

// RemoveCallbackByHandler removes a callback from the registry using a signal handler ID.
func RemoveCallbackByHandler(handlerID uint) {
	callbacks.Lock()
	if cbPtr, ok := callbacks.handlerToCallback[handlerID]; ok {
		delete(callbacks.handlerToCallback, handlerID)
		releaseCallback(cbPtr)
	}
	if data, ok := callbacks.handlerToSignalData[handlerID]; ok {
		delete(callbacks.handlerToSignalData, handlerID)
		delete(callbacks.signalHandlers, data)
	}
	callbacks.Unlock()
}

// SharedCallback returns a process-lifetime purego callback for key, creating it once.
// Generated signal bindings use this to avoid allocating one purego trampoline per
// signal connection.
func SharedCallback(key string, fn interface{}) uintptr {
	callbacks.Lock()
	defer callbacks.Unlock()
	if cb, ok := callbacks.sharedCallbacks[key]; ok {
		return cb
	}
	cb := purego.NewCallback(fn)
	callbacks.sharedCallbacks[key] = cb
	return cb
}

// SaveSignalHandler stores a per-signal-connection Go callback and returns a
// non-zero user data ID passed to g_signal_connect_data.
func SaveSignalHandler(callback interface{}) uintptr {
	callbacks.Lock()
	defer callbacks.Unlock()
	callbacks.nextSignalHandlerData++
	data := callbacks.nextSignalHandlerData
	if data == 0 {
		callbacks.nextSignalHandlerData++
		data = callbacks.nextSignalHandlerData
	}
	callbacks.signalHandlers[data] = callback
	return data
}

// GetSignalHandler retrieves a Go callback stored by SaveSignalHandler.
func GetSignalHandler(data uintptr) (interface{}, bool) {
	callbacks.RLock()
	defer callbacks.RUnlock()
	callback, ok := callbacks.signalHandlers[data]
	return callback, ok
}

// SaveSignalHandlerMapping records a signal handler ID → user data mapping so
// explicit disconnect can clean up signal callback state before destroy notify.
func SaveSignalHandlerMapping(handlerID uint, data uintptr) {
	callbacks.Lock()
	defer callbacks.Unlock()
	if handlerID == 0 {
		delete(callbacks.signalHandlers, data)
		return
	}
	if prevData := callbacks.handlerToSignalData[handlerID]; prevData != 0 && prevData != data {
		delete(callbacks.signalHandlers, prevData)
	}
	callbacks.handlerToSignalData[handlerID] = data
}

// ReleaseSignalHandler removes a signal callback by user data. It is safe to
// call multiple times; native destroy notify and explicit disconnect may race.
func ReleaseSignalHandler(data uintptr) {
	callbacks.Lock()
	defer callbacks.Unlock()
	delete(callbacks.signalHandlers, data)
	for handlerID, mappedData := range callbacks.handlerToSignalData {
		if mappedData == data {
			delete(callbacks.handlerToSignalData, handlerID)
		}
	}
}

var signalDestroyNotifyCallback uintptr

func initSignalDestroyNotify() {
	signalDestroyNotifyCallback = purego.NewCallback(func(data uintptr, _ uintptr) {
		ReleaseSignalHandler(data)
	})
}

// SignalDestroyNotify returns a shared GDestroyNotify callback for signal user data.
func SignalDestroyNotify() uintptr {
	return signalDestroyNotifyCallback
}

type sourceCallbackEntry struct {
	source         uintptr
	data           uintptr
	refs           uint
	callback       uintptr
	sourceFunc     SourceFunc
	sourceOnceFunc SourceOnceFunc
	childWatchFunc ChildWatchFunc
	notify         DestroyNotify
}

var sourceCallbacks = struct {
	sync.RWMutex
	nextToken uintptr
	byToken   map[uintptr]*sourceCallbackEntry
	bySource  map[uintptr]uintptr
}{
	byToken:  make(map[uintptr]*sourceCallbackEntry),
	bySource: make(map[uintptr]uintptr),
}

var (
	sourceDispatchCb             uintptr
	sourceOnceDispatchCb         uintptr
	childWatchDispatchCb         uintptr
	sourceCallbackLifecycleFuncs SourceCallbackFuncs
)

func registerSourceCallback(source uintptr, entry sourceCallbackEntry) uintptr {
	if source == 0 {
		return 0
	}
	entry.source = source
	entry.refs = 1

	sourceCallbacks.Lock()
	for {
		sourceCallbacks.nextToken++
		if sourceCallbacks.nextToken != 0 {
			break
		}
	}
	token := sourceCallbacks.nextToken
	sourceCallbacks.byToken[token] = &entry
	sourceCallbacks.bySource[source] = token
	sourceCallbacks.Unlock()
	return token
}

func refSourceCallback(token uintptr) {
	sourceCallbacks.Lock()
	if entry := sourceCallbacks.byToken[token]; entry != nil {
		entry.refs++
	}
	sourceCallbacks.Unlock()
}

func unrefSourceCallback(token uintptr) {
	var notify DestroyNotify
	var data uintptr

	sourceCallbacks.Lock()
	entry := sourceCallbacks.byToken[token]
	if entry == nil {
		sourceCallbacks.Unlock()
		return
	}
	if entry.refs > 1 {
		entry.refs--
		sourceCallbacks.Unlock()
		return
	}
	delete(sourceCallbacks.byToken, token)
	if sourceCallbacks.bySource[entry.source] == token {
		delete(sourceCallbacks.bySource, entry.source)
	}
	notify = entry.notify
	data = entry.data
	sourceCallbacks.Unlock()

	if notify != nil {
		notify(data)
	}
}

func getSourceCallback(token, source uintptr, callbackOut, dataOut *uintptr) {
	if callbackOut == nil || dataOut == nil {
		return
	}
	*callbackOut = 0
	*dataOut = 0

	sourceCallbacks.RLock()
	entry := sourceCallbacks.byToken[token]
	if entry != nil && entry.source == source {
		*callbackOut = entry.callback
		*dataOut = entry.data
	}
	sourceCallbacks.RUnlock()
}

func dispatchSource(source, data uintptr) bool {
	sourceCallbacks.RLock()
	token := sourceCallbacks.bySource[source]
	entry := sourceCallbacks.byToken[token]
	var fn SourceFunc
	if entry != nil {
		fn = entry.sourceFunc
	}
	sourceCallbacks.RUnlock()
	if fn == nil {
		return false
	}
	return fn(data)
}

func dispatchSourceOnce(source, data uintptr) bool {
	sourceCallbacks.RLock()
	token := sourceCallbacks.bySource[source]
	entry := sourceCallbacks.byToken[token]
	var fn SourceOnceFunc
	if entry != nil {
		fn = entry.sourceOnceFunc
	}
	sourceCallbacks.RUnlock()
	if fn != nil {
		fn(data)
	}
	return false
}

func dispatchChildWatch(source uintptr, pid Pid, waitStatus int32, data uintptr) {
	sourceCallbacks.RLock()
	token := sourceCallbacks.bySource[source]
	entry := sourceCallbacks.byToken[token]
	var fn ChildWatchFunc
	if entry != nil {
		fn = entry.childWatchFunc
	}
	sourceCallbacks.RUnlock()
	if fn != nil {
		fn(pid, int(waitStatus), data)
	}
}

func attachSourceCallback(source *Source, priority *int, entry sourceCallbackEntry) uint {
	if source == nil {
		return 0
	}
	token := registerSourceCallback(source.GoPointer(), entry)
	source.SetCallbackIndirect(token, &sourceCallbackLifecycleFuncs)
	if priority != nil {
		source.SetPriority(*priority)
	}
	sourceID := source.Attach(nil)
	source.Unref()
	return sourceID
}

func attachSourceFunc(source *Source, priority *int, fn *SourceFunc, data uintptr, notify *DestroyNotify) uint {
	if fn == nil {
		if source != nil {
			source.Unref()
		}
		return 0
	}
	entry := sourceCallbackEntry{data: data, callback: sourceDispatchCb}
	entry.sourceFunc = *fn
	if notify != nil {
		entry.notify = *notify
	}
	return attachSourceCallback(source, priority, entry)
}

func attachSourceOnceFunc(source *Source, fn *SourceOnceFunc, data uintptr) uint {
	if fn == nil {
		if source != nil {
			source.Unref()
		}
		return 0
	}
	entry := sourceCallbackEntry{
		data:           data,
		callback:       sourceOnceDispatchCb,
		sourceOnceFunc: *fn,
	}
	return attachSourceCallback(source, nil, entry)
}

func attachChildWatchFunc(source *Source, priority *int, fn *ChildWatchFunc, data uintptr, notify *DestroyNotify) uint {
	if fn == nil {
		if source != nil {
			source.Unref()
		}
		return 0
	}
	entry := sourceCallbackEntry{data: data, callback: childWatchDispatchCb, childWatchFunc: *fn}
	if notify != nil {
		entry.notify = *notify
	}
	return attachSourceCallback(source, priority, entry)
}

func initSourceCallbackLifecycle() {
	sourceDispatchCb = purego.NewCallback(func(data uintptr) bool {
		source := MainCurrentSource()
		if source == nil {
			return false
		}
		return dispatchSource(source.GoPointer(), data)
	})
	sourceOnceDispatchCb = purego.NewCallback(func(data uintptr) bool {
		source := MainCurrentSource()
		if source == nil {
			return false
		}
		return dispatchSourceOnce(source.GoPointer(), data)
	})
	childWatchDispatchCb = purego.NewCallback(func(pid Pid, waitStatus int32, data uintptr) {
		source := MainCurrentSource()
		if source == nil {
			return
		}
		dispatchChildWatch(source.GoPointer(), pid, waitStatus, data)
	})
	sourceCallbackLifecycleFuncs = SourceCallbackFuncs{
		xRef: purego.NewCallback(func(token uintptr) {
			refSourceCallback(token)
		}),
		xUnref: purego.NewCallback(func(token uintptr) {
			unrefSourceCallback(token)
		}),
		xGet: purego.NewCallback(func(token, source uintptr, callbackOut, dataOut *uintptr) {
			getSourceCallback(token, source, callbackOut, dataOut)
		}),
	}
}

func init() {
	initSourceCallbackLifecycle()
	initSignalDestroyNotify()
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
