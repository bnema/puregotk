//go:build linux

package sessionlock_test

import (
	"testing"

	"github.com/bnema/puregotk/v4/sessionlock"
)

func TestAvailable(t *testing.T) {
	avail := sessionlock.Available()
	t.Logf("sessionlock.Available() = %v", avail)
}
