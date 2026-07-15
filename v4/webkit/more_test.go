package webkit

import "testing"

func TestNavigationActionFromPointerCopiesExactlyOnce(t *testing.T) {
	oldTarget := xNavigationActionCopy
	oldRegister := lazyRegisterNavigationActionCopy
	xNavigationActionCopy = nil
	defer func() {
		xNavigationActionCopy = oldTarget
		lazyRegisterNavigationActionCopy = oldRegister
	}()

	var registrations int
	var nativeCalls int
	var got uintptr
	lazyRegisterNavigationActionCopy = func() {
		registrations++
		xNavigationActionCopy = func(ptr uintptr) uintptr {
			nativeCalls++
			got = ptr
			return 0xcafe
		}
	}

	if action := NavigationActionFromPointer(0); action != nil {
		t.Fatalf("nil input returned %p", action)
	}
	if registrations != 0 || nativeCalls != 0 {
		t.Fatalf("nil input made %d registrations and %d native calls, want none", registrations, nativeCalls)
	}

	action := NavigationActionFromPointer(0xbeef)
	if registrations != 1 || nativeCalls != 1 {
		t.Fatalf("registrations/native calls = %d/%d, want 1/1", registrations, nativeCalls)
	}
	if got != 0xbeef {
		t.Fatalf("native argument = %#x, want %#x", got, uintptr(0xbeef))
	}
	if action == nil {
		t.Fatal("owned copy is nil")
	}
	if action.GoPointer() != 0xcafe {
		t.Fatalf("owned copy pointer = %#x, want %#x", action.GoPointer(), uintptr(0xcafe))
	}
}
