package types

import (
	"slices"
	"strings"
)

const (
	puregoModule = "github.com/bnema/purego"
	coreModule   = "github.com/bnema/puregotk"
)

type ImportSet struct {
	pkgName        string
	module         string
	packageImports map[string]string
	imports        map[string]struct{}
}

func NewImportSet(pkgName string, module string, packageImports map[string]string) *ImportSet {
	return &ImportSet{
		pkgName:        pkgName,
		module:         module,
		packageImports: packageImports,
		imports:        make(map[string]struct{}),
	}
}

func (s *ImportSet) AddPkg(pkg string) {
	pkg = strings.ToLower(pkg)
	if pkg == s.pkgName {
		return
	}

	if imp, ok := s.packageImports[pkg]; ok {
		s.imports[imp] = struct{}{}
	}
}

func (s *ImportSet) AddCore() {
	s.imports[coreModule+"/pkg/core"] = struct{}{}
}

func (s *ImportSet) AddPurego() {
	s.imports[puregoModule] = struct{}{}
}

func (s *ImportSet) AddUnsafe() {
	s.imports["unsafe"] = struct{}{}
}

func (s *ImportSet) AddStructs() {
	s.imports["structs"] = struct{}{}
}

func (s *ImportSet) AddTypes() {
	if imp, ok := s.packageImports["gobject"]; ok {
		s.imports[imp+"/types"] = struct{}{}
	}
}

func (s *ImportSet) TrackGoType(goType string) {
	parts := strings.Split(goType, ".") // Type without package
	if len(parts) < 2 {
		return
	}

	pkg := baseTypeName(parts[0])
	if pkg == "types" {
		s.AddTypes()
		return
	}

	s.AddPkg(pkg)
}

func (s *ImportSet) Ordered() []string {
	// gofumpt still needs the imports to be separate despite
	// the "std imports must be in a separate group at the top"
	// rule, else it inserts newlines between imports
	var stdlib, thirdParty []string
	for imp := range s.imports {
		if !strings.Contains(imp, ".") {
			stdlib = append(stdlib, imp)
		} else {
			thirdParty = append(thirdParty, imp)
		}
	}

	return slices.Concat(stdlib, thirdParty)
}
