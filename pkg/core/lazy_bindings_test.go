package core_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bnema/puregotk/pkg/core"
)

func TestLazyResolverCachesLibraryAndSymbol(t *testing.T) {
	var opens, resolves int
	resolver := core.NewLazyResolver(
		func(string) ([]string, error) { return []string{"libdemo.so"}, nil },
		func(string) (uintptr, error) { opens++; return 42, nil },
		func(target any, libraries []uintptr, symbol string) error {
			resolves++
			if len(libraries) != 1 || libraries[0] != 42 || symbol != "demo_symbol" {
				t.Fatalf("unexpected resolution: libraries=%v symbol=%q", libraries, symbol)
			}
			return nil
		},
	)
	var target func()

	if err := resolver.Register(&target, "DEMO", "demo_symbol"); err != nil {
		t.Fatal(err)
	}
	if err := resolver.Register(&target, "DEMO", "demo_symbol"); err != nil {
		t.Fatal(err)
	}
	if opens != 1 || resolves != 1 {
		t.Fatalf("opens=%d resolves=%d, want one of each", opens, resolves)
	}
}

func TestLazyResolverCachesRequiredAndOptionalFailures(t *testing.T) {
	want := errors.New("library unavailable")
	var opens int
	resolver := core.NewLazyResolver(
		func(string) ([]string, error) { return []string{"missing.so"}, nil },
		func(string) (uintptr, error) { opens++; return 0, want },
		func(any, []uintptr, string) error { t.Fatal("resolver called after failed open"); return nil },
	)
	var required, optional func()

	for range 2 {
		if err := resolver.Register(&required, "MISSING", "required_symbol"); !errors.Is(err, want) {
			t.Fatalf("required error=%v, want %v", err, want)
		}
	}
	for range 2 {
		if resolver.RegisterOptional(&optional, "MISSING", "optional_symbol") {
			t.Fatal("optional registration succeeded for unavailable library")
		}
	}
	if opens != 1 {
		t.Fatalf("opens=%d, want cached single open", opens)
	}
}

func TestLazyResolverCachesMissingSymbol(t *testing.T) {
	want := errors.New("symbol missing")
	var resolves int
	resolver := core.NewLazyResolver(
		func(string) ([]string, error) { return []string{"libdemo.so"}, nil },
		func(string) (uintptr, error) { return 7, nil },
		func(any, []uintptr, string) error { resolves++; return want },
	)
	var target func()

	for range 2 {
		if err := resolver.Register(&target, "DEMO", "missing_symbol"); !errors.Is(err, want) {
			t.Fatalf("error=%v, want %v", err, want)
		}
	}
	if resolves != 1 {
		t.Fatalf("resolves=%d, want one cached symbol lookup", resolves)
	}
}

func TestLazyResolverPublishesConcurrentFirstResolutionOnce(t *testing.T) {
	const callers = 3
	started := make(chan struct{})
	release := make(chan struct{})
	invoked := make(chan struct{}, callers)
	var mu sync.Mutex
	opens, resolves := 0, 0
	resolver := core.NewLazyResolver(
		func(string) ([]string, error) { return []string{"libdemo.so"}, nil },
		func(string) (uintptr, error) {
			mu.Lock()
			opens++
			mu.Unlock()
			close(started)
			<-release
			return 99, nil
		},
		func(target any, libraries []uintptr, symbol string) error {
			mu.Lock()
			resolves++
			mu.Unlock()
			*(target.(*func())) = func() { invoked <- struct{}{} }
			return nil
		},
	)
	var target func()
	errs := make(chan error, callers)
	for range callers {
		go func() {
			if err := resolver.Register(&target, "DEMO", "demo_symbol"); err != nil {
				errs <- err
				return
			}
			target()
			errs <- nil
		}()
	}
	wait := func(name string, ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s", name)
		}
	}
	wait("first open", started)
	mu.Lock()
	gotOpens := opens
	mu.Unlock()
	if gotOpens != 1 {
		t.Fatalf("opens while blocked=%d, want one", gotOpens)
	}
	close(release)
	for range callers {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for resolver caller")
		}
	}
	for range callers {
		wait("published target invocation", invoked)
	}
	mu.Lock()
	defer mu.Unlock()
	if opens != 1 || resolves != 1 {
		t.Fatalf("opens=%d resolves=%d, want one of each", opens, resolves)
	}
}
