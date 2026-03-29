//go:build linux

package layershell_test

import (
	"testing"

	"github.com/bnema/puregotk/v4/layershell"
)

func TestAvailable(t *testing.T) {
	// Available() should not panic regardless of whether the library is installed
	avail := layershell.Available()
	t.Logf("layershell.Available() = %v", avail)
}

func TestVersionFunctions(t *testing.T) {
	if !layershell.Available() {
		t.Skip("libgtk4-layer-shell.so.0 not available")
	}
	major := layershell.GetMajorVersion()
	minor := layershell.GetMinorVersion()
	micro := layershell.GetMicroVersion()
	t.Logf("gtk4-layer-shell version: %d.%d.%d", major, minor, micro)
}
