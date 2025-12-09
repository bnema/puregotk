package pass

import (
	"github.com/jwijenbergh/puregotk/internal/gir/pass"
	"github.com/jwijenbergh/puregotk/internal/gir/types"
)

type (
	Pass       = pass.Pass
	Repository = types.Repository
)

var (
	New = pass.New
)
