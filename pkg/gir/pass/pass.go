package pass

import (
	"github.com/bnema/puregotk/internal/gir/pass"
	"github.com/bnema/puregotk/internal/gir/types"
)

type (
	Pass       = pass.Pass
	Repository = types.Repository
)

var (
	New = pass.New
)
