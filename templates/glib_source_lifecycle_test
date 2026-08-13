package glib

import (
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bnema/purego"
)

const (
	idleLifecycleHelperEnv    = "PUREGOTK_IDLE_LIFECYCLE_HELPER"
	timeoutLifecycleHelperEnv = "PUREGOTK_TIMEOUT_LIFECYCLE_HELPER"
	childLifecycleHelperEnv   = "PUREGOTK_CHILD_LIFECYCLE_HELPER"
)

func TestIdleAddOnceRegistersCallbackBeforeSourceCanDispatch(t *testing.T) {
	if os.Getenv(idleLifecycleHelperEnv) == "1" {
		runImmediateIdleDispatchHelper(t)
		return
	}
	runLifecycleSubprocess(t, idleLifecycleHelperEnv)
}

func TestTimeoutAddRegistersCallbackBeforeSourceCanDispatch(t *testing.T) {
	if os.Getenv(timeoutLifecycleHelperEnv) == "1" {
		runImmediateTimeoutDispatchHelper(t)
		return
	}
	runLifecycleSubprocess(t, timeoutLifecycleHelperEnv)
}

func TestChildWatchAddRegistersCallbackBeforeSourceCanDispatch(t *testing.T) {
	if os.Getenv(childLifecycleHelperEnv) == "1" {
		runImmediateChildWatchDispatchHelper(t)
		return
	}
	runLifecycleSubprocess(t, childLifecycleHelperEnv)
}

func runLifecycleSubprocess(t *testing.T, helperEnv string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("immediate source dispatch failed: %v\n%s", err, output)
	}
}

func runImmediateIdleDispatchHelper(t *testing.T) {
	t.Helper()
	source := IdleSourceNew()
	if source == nil {
		t.Fatal("IdleSourceNew returned nil")
	}
	const sourceID, wantData = uint(41), uintptr(0xcafe)
	xMainCurrentSource = func() uintptr { return source.GoPointer() }
	xSourceGetId = func(uintptr) uint { return sourceID }
	xIdleAddOnce = func(callback, data uintptr) uint {
		var invoke func(uintptr)
		purego.RegisterFunc(&invoke, callback)
		invoke(data)
		return sourceID
	}
	xIdleSourceNew = func() uintptr { return source.GoPointer() }
	var callbackData uintptr
	var callbackFuncs *SourceCallbackFuncs
	xSourceSetCallbackIndirect = func(_ uintptr, data uintptr, funcs *SourceCallbackFuncs) {
		callbackData, callbackFuncs = data, funcs
	}
	xSourceAttach = func(uintptr, *MainContext) uint {
		if callbackData == 0 || callbackFuncs == nil {
			t.Fatal("source became attachable before its callback lifecycle was registered")
		}
		if invokeIndirectSourceCallback(t, callbackData, callbackFuncs, source) {
			t.Fatal("one-shot source requested another dispatch")
		}
		return sourceID
	}
	xSourceUnref = func(uintptr) {}

	called := false
	callback := SourceOnceFunc(func(data uintptr) {
		called = true
		if data != wantData {
			t.Fatalf("callback data = %#x, want %#x", data, wantData)
		}
	})
	if got := IdleAddOnce(&callback, wantData); got != sourceID {
		t.Fatalf("source ID = %d, want %d", got, sourceID)
	}
	if !called {
		t.Fatal("callback dispatched before IdleAddOnce returned was lost")
	}
}

func runImmediateTimeoutDispatchHelper(t *testing.T) {
	t.Helper()
	source := IdleSourceNew()
	if source == nil {
		t.Fatal("IdleSourceNew returned nil")
	}
	const sourceID, interval, wantData = uint(42), uint(25), uintptr(0xbeef)
	xMainCurrentSource = func() uintptr { return source.GoPointer() }
	xSourceGetId = func(uintptr) uint { return sourceID }
	xTimeoutAdd = func(gotInterval uint, callback, data uintptr) uint {
		if gotInterval != interval {
			t.Fatalf("interval = %d, want %d", gotInterval, interval)
		}
		var invoke func(uintptr) bool
		purego.RegisterFunc(&invoke, callback)
		invoke(data)
		return sourceID
	}
	xTimeoutSourceNew = func(gotInterval uint) uintptr {
		if gotInterval != interval {
			t.Fatalf("source interval = %d, want %d", gotInterval, interval)
		}
		return source.GoPointer()
	}
	var callbackData uintptr
	var callbackFuncs *SourceCallbackFuncs
	xSourceSetCallbackIndirect = func(_ uintptr, data uintptr, funcs *SourceCallbackFuncs) {
		callbackData, callbackFuncs = data, funcs
	}
	xSourceAttach = func(uintptr, *MainContext) uint {
		if !invokeIndirectSourceCallback(t, callbackData, callbackFuncs, source) {
			t.Fatal("recurring timeout requested removal")
		}
		return sourceID
	}
	xSourceUnref = func(uintptr) {}

	called := false
	callback := SourceFunc(func(data uintptr) bool {
		called = true
		if data != wantData {
			t.Fatalf("callback data = %#x, want %#x", data, wantData)
		}
		return true
	})
	if got := TimeoutAdd(interval, &callback, wantData); got != sourceID {
		t.Fatalf("source ID = %d, want %d", got, sourceID)
	}
	if !called {
		t.Fatal("callback dispatched before TimeoutAdd returned was lost")
	}
}

func runImmediateChildWatchDispatchHelper(t *testing.T) {
	t.Helper()
	source := IdleSourceNew()
	if source == nil {
		t.Fatal("IdleSourceNew returned nil")
	}
	const sourceID, pid, status, wantData = uint(43), Pid(1234), int32(17), uintptr(0xface)
	xMainCurrentSource = func() uintptr { return source.GoPointer() }
	xSourceGetId = func(uintptr) uint { return sourceID }
	xChildWatchAdd = func(gotPID Pid, callback, data uintptr) uint {
		if gotPID != pid {
			t.Fatalf("pid = %d, want %d", gotPID, pid)
		}
		var invoke func(Pid, int32, uintptr)
		purego.RegisterFunc(&invoke, callback)
		invoke(gotPID, status, data)
		return sourceID
	}
	xChildWatchSourceNew = func(gotPID Pid) uintptr {
		if gotPID != pid {
			t.Fatalf("source pid = %d, want %d", gotPID, pid)
		}
		return source.GoPointer()
	}
	var callbackData uintptr
	var callbackFuncs *SourceCallbackFuncs
	xSourceSetCallbackIndirect = func(_ uintptr, data uintptr, funcs *SourceCallbackFuncs) {
		callbackData, callbackFuncs = data, funcs
	}
	xSourceAttach = func(uintptr, *MainContext) uint {
		callback, data := indirectSourceCallback(t, callbackData, callbackFuncs, source)
		var invoke func(Pid, int32, uintptr)
		purego.RegisterFunc(&invoke, callback)
		invoke(pid, status, data)
		return sourceID
	}
	xSourceUnref = func(uintptr) {}

	called := false
	callback := ChildWatchFunc(func(gotPID Pid, gotStatus int, data uintptr) {
		called = true
		if gotPID != pid || gotStatus != int(status) || data != wantData {
			t.Fatalf("callback = (%d, %d, %#x), want (%d, %d, %#x)", gotPID, gotStatus, data, pid, status, wantData)
		}
	})
	if got := ChildWatchAdd(pid, &callback, wantData); got != sourceID {
		t.Fatalf("source ID = %d, want %d", got, sourceID)
	}
	if !called {
		t.Fatal("callback dispatched before ChildWatchAdd returned was lost")
	}
}

func invokeIndirectSourceCallback(t *testing.T, callbackData uintptr, callbackFuncs *SourceCallbackFuncs, source *Source) bool {
	t.Helper()
	callback, data := indirectSourceCallback(t, callbackData, callbackFuncs, source)
	var invoke func(uintptr) bool
	purego.RegisterFunc(&invoke, callback)
	return invoke(data)
}

func indirectSourceCallback(t *testing.T, callbackData uintptr, callbackFuncs *SourceCallbackFuncs, source *Source) (uintptr, uintptr) {
	t.Helper()
	if callbackData == 0 || callbackFuncs == nil || callbackFuncs.xGet == 0 {
		t.Fatal("source callback lifecycle is not configured")
	}
	var get func(uintptr, uintptr, *uintptr, *uintptr)
	purego.RegisterFunc(&get, callbackFuncs.xGet)
	var callback, data uintptr
	get(callbackData, source.GoPointer(), &callback, &data)
	if callback == 0 {
		t.Fatal("source callback is nil")
	}
	return callback, data
}

func TestSourceCallbackRegistryPreservesUserData(t *testing.T) {
	const source, wantData, callback = uintptr(0x1001), uintptr(0xcafe), uintptr(0x2002)
	token := registerSourceCallback(source, sourceCallbackEntry{data: wantData, callback: callback})
	t.Cleanup(func() { unrefSourceCallback(token) })
	var gotCallback, gotData uintptr
	getSourceCallback(token, source, &gotCallback, &gotData)
	if gotCallback != callback || gotData != wantData {
		t.Fatalf("indirect callback = (%#x, %#x), want (%#x, %#x)", gotCallback, gotData, callback, wantData)
	}
}

func TestChildWatchDispatchPreservesArguments(t *testing.T) {
	const source, data = uintptr(0x2112), uintptr(0xface)
	const pid, status = Pid(1234), int32(17)
	called := false
	callback := ChildWatchFunc(func(gotPID Pid, gotStatus int, gotData uintptr) {
		called = true
		if gotPID != pid || gotStatus != int(status) || gotData != data {
			t.Fatalf("callback = (%d, %d, %#x), want (%d, %d, %#x)", gotPID, gotStatus, gotData, pid, status, data)
		}
	})
	token := registerSourceCallback(source, sourceCallbackEntry{childWatchFunc: callback})
	dispatchChildWatch(source, pid, status, data)
	unrefSourceCallback(token)
	if !called {
		t.Fatal("child-watch callback was not dispatched")
	}
}

func TestSourceCallbackRegistrySurvivesSourceAddressReuse(t *testing.T) {
	const source = uintptr(0x3003)
	var firstNotified, secondNotified atomic.Int32
	first := SourceFunc(func(uintptr) bool {
		t.Fatal("stale callback dispatched after source address reuse")
		return false
	})
	secondCalled := false
	second := SourceFunc(func(uintptr) bool { secondCalled = true; return true })
	firstToken := registerSourceCallback(source, sourceCallbackEntry{
		sourceFunc: first, notify: func(uintptr) { firstNotified.Add(1) },
	})
	secondToken := registerSourceCallback(source, sourceCallbackEntry{
		sourceFunc: second, notify: func(uintptr) { secondNotified.Add(1) },
	})
	unrefSourceCallback(firstToken)
	if !dispatchSource(source, 2) || !secondCalled {
		t.Fatal("replacement callback was lost when the old source was finalized")
	}
	unrefSourceCallback(secondToken)
	if firstNotified.Load() != 1 || secondNotified.Load() != 1 {
		t.Fatalf("destroy notify counts = (%d, %d), want (1, 1)", firstNotified.Load(), secondNotified.Load())
	}
}

func TestSourceCallbackDestroyNotifyRunsOnceAfterLastReference(t *testing.T) {
	var notified atomic.Int32
	token := registerSourceCallback(0x4004, sourceCallbackEntry{
		notify: func(uintptr) { notified.Add(1) },
	})
	refSourceCallback(token)
	unrefSourceCallback(token)
	if notified.Load() != 0 {
		t.Fatal("destroy notify ran with an outstanding reference")
	}
	unrefSourceCallback(token)
	unrefSourceCallback(token)
	if got := notified.Load(); got != 1 {
		t.Fatalf("destroy notify count = %d, want 1", got)
	}
}

func TestSourceCallbackRegistryConcurrentLifecycle(t *testing.T) {
	const count = 1000
	var notified atomic.Int32
	token := registerSourceCallback(0x5000, sourceCallbackEntry{
		notify: func(uintptr) { notified.Add(1) },
	})
	for range count {
		refSourceCallback(token)
	}
	var wg sync.WaitGroup
	wg.Add(count)
	for range count {
		go func() {
			defer wg.Done()
			unrefSourceCallback(token)
		}()
	}
	wg.Wait()
	if got := notified.Load(); got != 0 {
		t.Fatalf("destroy notify ran with the initial reference alive: %d", got)
	}
	unrefSourceCallback(token)
	if got := notified.Load(); got != 1 {
		t.Fatalf("destroy notify count = %d, want 1", got)
	}
}

func TestSourceAddWrappersRejectNilCallbacks(t *testing.T) {
	var notified atomic.Int32
	notify := DestroyNotify(func(uintptr) { notified.Add(1) })
	tests := map[string]func() uint{
		"ChildWatchAdd":         func() uint { return ChildWatchAdd(1, nil, 0) },
		"ChildWatchAddFull":     func() uint { return ChildWatchAddFull(PRIORITY_DEFAULT, 1, nil, 0, &notify) },
		"IdleAdd":               func() uint { return IdleAdd(nil, 0) },
		"IdleAddFull":           func() uint { return IdleAddFull(PRIORITY_DEFAULT_IDLE, nil, 0, &notify) },
		"IdleAddOnce":           func() uint { return IdleAddOnce(nil, 0) },
		"TimeoutAdd":            func() uint { return TimeoutAdd(1, nil, 0) },
		"TimeoutAddFull":        func() uint { return TimeoutAddFull(PRIORITY_DEFAULT, 1, nil, 0, &notify) },
		"TimeoutAddOnce":        func() uint { return TimeoutAddOnce(1, nil, 0) },
		"TimeoutAddSeconds":     func() uint { return TimeoutAddSeconds(1, nil, 0) },
		"TimeoutAddSecondsFull": func() uint { return TimeoutAddSecondsFull(PRIORITY_DEFAULT, 1, nil, 0, &notify) },
		"TimeoutAddSecondsOnce": func() uint { return TimeoutAddSecondsOnce(1, nil, 0) },
	}
	for name, add := range tests {
		t.Run(name, func(t *testing.T) {
			if sourceID := add(); sourceID != 0 {
				t.Fatalf("nil callback returned source ID %d, want 0", sourceID)
			}
		})
	}
	if got := notified.Load(); got != 0 {
		t.Fatalf("destroy notify count = %d, want 0", got)
	}
}

func TestIdleAddFullPreservesNativeLookupAndRemovalSemantics(t *testing.T) {
	const data = uintptr(0x6a17e)
	var called, notified atomic.Int32
	callback := SourceFunc(func(gotData uintptr) bool {
		called.Add(1)
		if gotData != data {
			t.Errorf("callback data = %#x, want %#x", gotData, data)
		}
		return false
	})
	notify := DestroyNotify(func(gotData uintptr) {
		if gotData != data {
			t.Errorf("destroy notify data = %#x, want %#x", gotData, data)
		}
		notified.Add(1)
	})
	sourceID := IdleAddFull(PRIORITY_DEFAULT_IDLE, &callback, data, &notify)
	if sourceID == 0 {
		t.Fatal("IdleAddFull returned a zero source ID")
	}
	context := MainContextDefault()
	t.Cleanup(func() {
		if context.FindSourceById(sourceID) != nil {
			SourceRemove(sourceID)
		}
	})
	source := context.FindSourceByUserData(data)
	if source == nil || source.GetId() != sourceID {
		t.Fatalf("FindSourceByUserData did not find source %d", sourceID)
	}
	deadline := time.Now().Add(time.Second)
	for called.Load() == 0 && time.Now().Before(deadline) {
		context.Iteration(false)
	}
	if called.Load() != 1 || notified.Load() != 1 {
		t.Fatalf("callback/notify counts = (%d, %d), want (1, 1)", called.Load(), notified.Load())
	}
	if context.FindSourceByUserData(data) != nil {
		t.Fatal("completed source is still discoverable by user data")
	}
}

func TestRemovedSourcesReleaseRegistryWithoutCallbackSlotGrowth(t *testing.T) {
	const count = 2100
	var called, notified atomic.Int32
	callback := SourceFunc(func(uintptr) bool { called.Add(1); return true })
	notify := DestroyNotify(func(uintptr) { notified.Add(1) })

	sourceCallbacks.RLock()
	entriesBefore := len(sourceCallbacks.byToken)
	sourceCallbacks.RUnlock()
	for i := 1; i <= count; i++ {
		sourceID := TimeoutAddFull(PRIORITY_DEFAULT, 60_000, &callback, uintptr(i), &notify)
		if sourceID == 0 {
			t.Fatalf("could not add source %d", i)
		}
		if !SourceRemove(sourceID) {
			if source := MainContextDefault().FindSourceById(sourceID); source != nil {
				source.Destroy()
			}
			t.Fatalf("could not remove source %d (ID %d)", i, sourceID)
		}
	}
	if got := called.Load(); got != 0 {
		t.Fatalf("callback count = %d, want 0", got)
	}
	if got := notified.Load(); got != count {
		t.Fatalf("destroy notify count = %d, want %d", got, count)
	}
	sourceCallbacks.RLock()
	entriesAfter := len(sourceCallbacks.byToken)
	sourceCallbacks.RUnlock()
	if entriesAfter != entriesBefore {
		t.Fatalf("source registry grew from %d to %d entries", entriesBefore, entriesAfter)
	}
}
