// package pass implements the first and second pass to go from gir files to go files
// the first pass collects basic type information
// the second pass uses the basic type information to go over the gir files again and convert it to go files
package pass

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"iter"
	"os"
	"strings"
	"text/template"

	"github.com/bnema/puregotk/internal/gir/types"
	"github.com/bnema/puregotk/internal/gir/util"
	"mvdan.cc/gofumpt/format"
)

type Dependency struct {
	Module string
	Files  []string
}

// NamespaceConfig holds per-namespace generator overrides.
type NamespaceConfig struct {
	PackageName     string
	OptionalLibrary bool
	BuildConstraint string
}

var namespaceConfigs = map[string]NamespaceConfig{
	"Gtk4LayerShell":  {PackageName: "layershell", OptionalLibrary: true, BuildConstraint: "//go:build linux"},
	"Gtk4SessionLock": {PackageName: "sessionlock", OptionalLibrary: true, BuildConstraint: "//go:build linux"},
}

func packageNameForNamespace(namespace string) string {
	if cfg, ok := namespaceConfigs[namespace]; ok && cfg.PackageName != "" {
		return cfg.PackageName
	}
	return strings.ToLower(namespace)
}

// filterSentinelEnumMembers removes enum sentinel members such as *_ENTRY_NUMBER.
func filterSentinelEnumMembers(members []types.Member) []types.Member {
	filtered := make([]types.Member, 0, len(members))
	for _, m := range members {
		if strings.HasSuffix(m.CIdentifier, "_ENTRY_NUMBER") {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

type Pass struct {
	Parsed []types.Repository
	Types  types.KindMap

	impModule         string
	impPackageImports map[string]string
}

// New parses the given GIR files and associates their namespaces with module
// for import resolution. Additional deps can override the module for specific
// GIR files (e.g. files from a dependency). The returned Pass is then used
// across First and Second to collect type info and generate Go source files.
func New(files []string, module string, deps ...Dependency) (*Pass, error) {
	fileModule := make(map[string]string)
	allFiles := files
	for _, d := range deps {
		for _, f := range d.Files {
			fileModule[f] = d.Module
		}
		allFiles = append(allFiles, d.Files...)
	}
	p := Pass{
		Parsed: make([]types.Repository, len(allFiles)),
		Types:  make(types.KindMap),

		impModule:         module,
		impPackageImports: make(map[string]string),
	}
	for i, f := range allFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var r types.Repository
		err = xml.Unmarshal(b, &r)
		if err != nil {
			return nil, err
		}
		p.Parsed[i] = r

		namespace := r.Namespaces[0].Name
		pkgKey := strings.ToLower(namespace)
		pkgPath := packageNameForNamespace(namespace)
		m := module
		if override, ok := fileModule[f]; ok {
			m = override
		}
		p.impPackageImports[pkgKey] = m + "/" + pkgPath
	}
	return &p, nil
}

func (p *Pass) collectTypes(r types.Repository) {
	ns := r.Namespaces[0]
	for _, cls := range ns.Classes {
		p.Types.Add(ns.Name, cls.Name, types.ClassesType, cls)
	}
	for _, rec := range ns.Records {
		p.Types.Add(ns.Name, rec.Name, types.RecordsType, rec)
	}
	for _, en := range ns.Enums {
		// TODO: This probably shouldn't be aliastype, but we should make dedicated types
		p.Types.Add(ns.Name, en.Name, types.AliasType, en)
	}
	for _, cb := range ns.Callbacks {
		p.Types.Add(ns.Name, cb.Name, types.CallbackType, cb)
	}
	for _, b := range ns.Bitfields {
		// TODO: This probably shouldn't be aliastype, but we should make dedicated types
		p.Types.Add(ns.Name, b.Name, types.AliasType, b)
	}
	for _, inter := range ns.Interfaces {
		p.Types.Add(ns.Name, inter.Name, types.InterfacesType, inter)
	}
	for _, alias := range ns.Aliases {
		// Check what the alias points to and use the same type
		aliasTarget := alias.Type.Name
		if aliasTarget != "" {
			targetKind := p.Types.Kind(ns.Name, aliasTarget)
			if targetKind != types.UnknownType {
				p.Types.Add(ns.Name, alias.Name, targetKind, alias)
			} else {
				// If we don't know the target type yet, default to alias type
				p.Types.Add(ns.Name, alias.Name, types.AliasType, alias)
			}
		} else {
			p.Types.Add(ns.Name, alias.Name, types.AliasType, alias)
		}
	}
}

// First does a "first pass" meaning it collects basic type information for all the repositories
func (p *Pass) First() {
	for _, r := range p.Parsed {
		p.collectTypes(r)
	}
}

type file struct {
	imps *types.ImportSet

	aliases    []types.AliasTemplate
	enums      []types.EnumTemplate
	constants  []types.ConstantTemplate
	records    []types.RecordTemplate
	callbacks  []types.CallbackTemplate
	interfaces []types.InterfaceTemplate
	functions  []types.FuncTemplate
	classes    []types.ClassTemplate
}

func (f *file) allFuncs() iter.Seq[types.FuncTemplate] {
	return func(yield func(types.FuncTemplate) bool) {
		for _, fn := range f.functions {
			if !yield(fn) {
				return
			}
		}
		for _, r := range f.records {
			for _, fn := range r.Constructors {
				if !yield(fn) {
					return
				}
			}
			for _, fn := range r.Receivers {
				if !yield(fn) {
					return
				}
			}
		}
		for _, c := range f.classes {
			for _, fn := range c.Constructors {
				if !yield(fn) {
					return
				}
			}
			for _, fn := range c.Receivers {
				if !yield(fn) {
					return
				}
			}
			for _, fn := range c.Functions {
				if !yield(fn) {
					return
				}
			}
		}
		for _, i := range f.interfaces {
			for _, m := range i.Methods {
				if !yield(m.FuncTemplate) {
					return
				}
			}
		}
	}
}

func (p *Pass) writeGo(r types.Repository, gotemp *template.Template, dir string) {
	ns := r.Namespaces[0]
	nsCfg := namespaceConfigs[ns.Name]
	pkgName := packageNameForNamespace(ns.Name)

	files := make(map[string]*file)
	getFile := func(fn string) *file {
		pf, ok := files[fn]
		if !ok {
			pf = &file{
				imps: types.NewImportSet(pkgName, p.impModule, p.impPackageImports),
			}
			files[fn] = pf
		}

		return pf
	}

	for _, el := range ns.Bitfields {
		filtered := el
		filtered.Members = filterSentinelEnumMembers(el.Members)
		temp := filtered.Template(ns.Name, ns.CIdentifierPrefixes)
		pf := getFile(el.FilenameSafe())
		pf.enums = append(pf.enums, temp)
		if temp.TypeGetter != "" {
			pf.imps.AddTypes()
		}
	}

	for _, el := range ns.Enums {
		filtered := el
		filtered.Members = filterSentinelEnumMembers(el.Members)
		temp := filtered.Template(ns.Name, ns.CIdentifierPrefixes)
		pf := getFile(el.FilenameSafe())
		pf.enums = append(pf.enums, temp)
		if temp.TypeGetter != "" {
			pf.imps.AddTypes()
		}
	}

	for _, con := range ns.Constants {
		pf := getFile(con.FilenameSafe())
		ct := con.Template(ns.Name, p.Types)
		pf.constants = append(pf.constants, ct)
		pf.imps.TrackGoType(ct.Type)
	}

	callbackDocs := make(map[string]string)
	for _, cb := range ns.Callbacks {
		callbackDocs[cb.Name] = cb.Doc.StringSafe()
	}

	recordLookup := make(map[string]bool)
	for _, rec := range ns.Records {
		name := util.SnakeToCamel(rec.Name)
		pf := getFile(rec.FilenameSafe())
		imps := pf.imps
		constructors := make([]types.FuncTemplate, len(rec.Constructors))
		receivers := make([]types.FuncTemplate, 0, len(rec.Methods))
		fields := make([]types.RecordField, 0, len(rec.Fields))
		callbackAccessors := make([]types.CallbackAccessor, 0)
		if rec.GLibGetType != "" {
			imps.AddTypes()
		}
		for i, c := range rec.Constructors {
			constructors[i] = types.FuncTemplate{
				Name:  util.ConstructorName(c.Name, rec.Name),
				CName: c.CIdentifier,
				Doc:   c.Doc.StringSafe(),
				Args:  c.Parameters.Template(ns.Name, "", p.Types, c.Throws, types.ArgsFromGoToC, imps),
				Ret:   c.ReturnValue.Template(ns.Name, "", p.Types, c.Throws, imps),
			}
		}
		for _, f := range rec.Fields {
			var _type string
			var fieldName string

			// Check if this field is a callback
			if f.Callback != nil {
				_type = "uintptr"
				fieldName = "x" + util.SnakeToCamel(f.Name) // Prefix callback pointer fields with `x` to make them private

				callbackName := util.SnakeToCamel(f.Name)
				args := f.Callback.Parameters.Template(ns.Name, "", p.Types, f.Callback.Throws, types.ArgsFromCToGo, imps)
				ret := f.Callback.ReturnValue.Template(ns.Name, "", p.Types, f.Callback.Throws, imps)

				apiTypes := args.API.Types

				var doc string
				if f.Callback.Doc != nil && f.Callback.Doc.String != "" {
					doc = f.Callback.Doc.StringSafe()
				} else {
					baseClassName := strings.TrimSuffix(rec.Name, "Class")
					callbackName := baseClassName + util.SnakeToCamel(f.Name) + "Func"

					if callbackDoc, exists := callbackDocs[callbackName]; exists && callbackDoc != "" {
						doc = callbackDoc
					} else {
						doc = f.Doc.StringSafe()
					}
				}

				imps.AddPurego()
				imps.AddUnsafe()

				callbackAccessors = append(callbackAccessors, types.CallbackAccessor{
					Name:         callbackName,
					CName:        f.Name,
					Doc:          doc,
					CallbackType: "func(" + strings.Join(apiTypes, ", ") + ") " + ret.Value,
					Args:         args,
					Ret:          ret,
				})
			} else {
				_type = f.Translate(ns.Name, p.Types)
				if _type == "" {
					continue
				}
				// HACK: Handle the specific case where a gint is converted to an int
				// But for structs this needs to be an int32 as purego just gets the pointer to the struct
				// Instead of converting each field separately
				if f.AnyType.Type != nil && f.AnyType.Type.CType == "gint" {
					_type = "int32"
				}

				// HACK: in structs the strings should be uintptr as we convert it ourselves
				if _type == "string" {
					_type = "uintptr"
				}

				// HACK: Special handling for parent_class field - it should be embedded as a full struct
				// to match C's memory layout, not converted to uintptr
				// See https://docs.gtk.org/gobject/tutorial.html
				if f.Name == "parent_class" && f.AnyType.Type != nil {
					// Check if this is a Record type with no pointers (embedded struct)
					typeName := util.NormalizeNamespace(ns.Name, f.AnyType.Type.Name, true)
					kind := p.Types.Kind(ns.Name, typeName)
					if kind == types.RecordsType && !strings.Contains(f.AnyType.Type.CType, "*") {
						// Use the full struct type for embedding
						_type = typeName
					}
				}

				imps.TrackGoType(_type)
				fieldName = util.SnakeToCamel(f.Name)
			}

			fields = append(fields, types.RecordField{
				Name: fieldName,
				Type: _type,
			})
		}
		for _, f := range rec.Methods {
			name := types.SafeReceiverMethodName(util.SnakeToCamel(rec.Name), util.SnakeToCamel(f.Name))
			if name == "" {
				name = util.SnakeToCamel(f.CIdentifier)
			}
			for _, f := range fields {
				if f.Name == name {
					name = name + "Fn"
					break
				}
			}
			receivers = append(receivers, types.FuncTemplate{
				Doc:   f.Doc.StringSafe(),
				Name:  name,
				CName: f.CIdentifier,
				Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC, imps),
				Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws, imps),
			})
		}
		pf.records = append(pf.records, types.RecordTemplate{
			Name:              name,
			Doc:               rec.Doc.StringSafe(),
			Constructors:      constructors,
			Receivers:         receivers,
			Fields:            fields,
			CallbackAccessors: callbackAccessors,
			TypeGetter:        rec.GLibGetType,
		})
		recordLookup[name] = true
	}

	// set every callback equal to uintptr as well
	for _, cb := range ns.Callbacks {
		pf := getFile(cb.FilenameSafe())
		cbImps := types.NewImportSet(pkgName, p.impModule, p.impPackageImports)
		cbT := types.CallbackTemplate{
			Doc:  cb.Doc.StringSafe(),
			Name: cb.Name,
			Args: cb.Parameters.Template(ns.Name, "", p.Types, cb.Throws, types.ArgsFromCToGo, cbImps),
			Ret:  cb.ReturnValue.Template(ns.Name, "", p.Types, cb.Throws, cbImps),
		}
		for _, t := range cbT.Args.Pure.Types {
			pf.imps.TrackGoType(t)
		}
		pf.imps.TrackGoType(cbT.Ret.Raw)
		pf.callbacks = append(pf.callbacks, cbT)
	}

	for _, inter := range ns.Interfaces {
		pf := getFile(inter.FilenameSafe())
		pf.interfaces = append(pf.interfaces, types.ConvertInterface(ns.Name, "", inter, nil, p.Types, pf.imps))
	}

	for _, union := range ns.Unions {
		pf := getFile(union.FilenameSafe())
		name := util.SnakeToCamel(union.Name)
		interT := types.AliasTemplate{
			Doc:  union.Doc.StringSafe(),
			Name: name,
			// structs are not yet supported in CGO
			Value: "uintptr",
		}
		pf.aliases = append(pf.aliases, interT)
	}

	for _, alias := range ns.Aliases {
		pf := getFile(alias.FilenameSafe())
		typeName := alias.Template(ns.Name, p.Types)
		if typeName == "" {
			typeName = "uintptr"
		}
		name := util.SnakeToCamel(alias.Name)
		aliasT := types.AliasTemplate{
			Doc:  alias.Doc.StringSafe(),
			Name: name,
			// structs are not yet supported in CGO
			Value: typeName,
		}
		pf.aliases = append(pf.aliases, aliasT)
	}

	for _, f := range ns.Functions {
		name := util.SnakeToCamel(f.Name)
		if p.Types.Kind(ns.Name, name) != types.UnknownType {
			name = "New" + name
		}
		pf := getFile(f.FilenameSafe())
		pf.functions = append(pf.functions, types.FuncTemplate{
			Name:  name,
			CName: f.CIdentifier,
			Doc:   f.Doc.StringSafe(),
			Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC, pf.imps),
			Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws, pf.imps),
		})
	}

	for _, cls := range ns.Classes {
		implemented := make(map[string]bool)
		constructors := make([]types.FuncTemplate, len(cls.Constructors))
		functions := make([]types.FuncTemplate, len(cls.Functions))
		pf := getFile(cls.FilenameSafe())
		imps := pf.imps

		if cls.GLibGetType != "" {
			imps.AddTypes()
		}

		for i, c := range cls.Constructors {
			c.ReturnValue.AnyType.Type.Name = cls.Name
			constructors[i] = types.FuncTemplate{
				Name:  util.ConstructorName(c.Name, cls.Name),
				CName: c.CIdentifier,
				Doc:   c.Doc.StringSafe(),
				Args:  c.Parameters.Template(ns.Name, "", p.Types, c.Throws, types.ArgsFromGoToC, imps),
				Ret:   c.ReturnValue.Template(ns.Name, "", p.Types, c.Throws, imps),
			}
		}
		signals := make([]types.SignalsTemplate, len(cls.Signals))
		for i, s := range cls.Signals {
			imps.AddPkg("glib")
			imps.AddPkg("gobject")

			signals[i] = types.SignalsTemplate{
				Doc:   s.Doc.StringSafe(),
				Name:  util.DashToCamel(s.Name),
				CName: s.Name,
				Args:  s.Parameters.Template(ns.Name, "", p.Types, false, types.ArgsFromCToGo, imps),
				Ret:   s.ReturnValue.Template(ns.Name, "", p.Types, false, imps),
			}
		}
		receivers := make([]types.FuncTemplate, len(cls.Methods))
		for i, f := range cls.Methods {
			name := types.SafeReceiverMethodName(util.SnakeToCamel(cls.Name), util.SnakeToCamel(f.Name))
			implemented[name] = true
			receivers[i] = types.FuncTemplate{
				Doc:   f.Doc.StringSafe(),
				Name:  name,
				CName: f.CIdentifier,
				Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC, imps),
				Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws, imps),
			}
		}
		var interfaces []types.InterfaceTemplate
		for i, f := range cls.Functions {
			name := fmt.Sprintf("%s%s", util.SnakeToCamel(cls.Name), util.SnakeToCamel(f.Name))
			functions[i] = types.FuncTemplate{
				Name:  name,
				CName: f.CIdentifier,
				Doc:   f.Doc.StringSafe(),
				Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC, imps),
				Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws, imps),
			}
		}
		for _, impl := range cls.Implements {
			interfaces = append(interfaces, types.GetInterfaceFuncs(ns.Name, impl.Name, implemented, p.Types, imps))
		}
		properties := make([]types.PropertyTemplate, 0, len(cls.Properties))
		for _, prop := range cls.Properties {
			propTemp := prop.Template(ns.Name, p.Types, imps)

			// TODO: Implement non-primitive types, then remove this
			if propTemp.GValueType != "" {
				properties = append(properties, propTemp)
			}
		}

		parent := util.NormalizeNamespace(ns.Name, cls.Parent, true)
		imps.TrackGoType(parent)

		pf.classes = append(pf.classes, types.ClassTemplate{
			Doc:          cls.Doc.StringSafe(),
			Name:         cls.Name,
			Parent:       parent,
			Constructors: constructors,
			Receivers:    receivers,
			Interfaces:   interfaces,
			Functions:    functions,
			Properties:   properties,
			Signals:      signals,
			TypeGetter:   cls.GLibGetType,
		})
	}

	var pkgConfigName string
	if len(r.Packages) > 0 {
		pkgConfigName = r.Packages[0].Name
	}

	var sharedLibraries []string
	if ns.SharedLibrary != "" {
		for lib := range strings.SplitSeq(ns.SharedLibrary, ",") {
			if trimmed := strings.TrimSpace(lib); trimmed != "" {
				sharedLibraries = append(sharedLibraries, trimmed)
			}
		}
	}

	// Derive macOS .dylib equivalents from .so names
	var dylibNames []string
	for _, soName := range sharedLibraries {
		if strings.Contains(soName, ".so.") {
			dylibNames = append(dylibNames, strings.Replace(soName, ".so.", ".", 1)+".dylib")
		}
	}
	sharedLibraries = append(sharedLibraries, dylibNames...)

	for fn, pf := range files {
		methods := 0
		for _, i := range pf.interfaces {
			methods += len(i.Methods)
		}
		for _, i := range pf.records {
			methods += len(i.Constructors)
			methods += len(i.Receivers)
		}
		for _, i := range pf.classes {
			methods += len(i.Constructors)
			methods += len(i.Receivers)
			methods += len(i.Functions)
		}
		// we do not need to add the length of interfaces in here
		// as they should only be loaded when there are classes
		needsInit := (len(pf.functions) + methods) > 0
		if needsInit {
			pf.imps.AddPurego()
			pf.imps.AddCore()
		}

		if len(pf.records) > 0 {
			pf.imps.AddStructs()
			pf.imps.AddUnsafe()
		}

		for f := range pf.allFuncs() {
			for _, t := range f.Args.Pure.Types {
				pf.imps.TrackGoType(t)
			}

			pf.imps.TrackGoType(f.Ret.Raw)
		}

		// Packages that need to have their types manually registered
		// See https://bugs.webkit.org/show_bug.cgi?id=175937
		registerTypes := pkgName == "webkit"

		args := types.TemplateArg{
			PkgName:         pkgName,
			PkgEnv:          strings.ToUpper(pkgName),
			PkgConfigName:   pkgConfigName,
			SharedLibraries: sharedLibraries,
			NeedsInit:       needsInit,
			OptionalLibrary: nsCfg.OptionalLibrary,
			BuildConstraint: nsCfg.BuildConstraint,
			RegisterTypes:   registerTypes,
			Imports:         pf.imps.Ordered(),
			Aliases:         pf.aliases,
			Callbacks:       pf.callbacks,
			Records:         pf.records,
			Enums:           pf.enums,
			Constants:       pf.constants,
			Functions:       pf.functions,
			Interfaces:      pf.interfaces,
			Classes:         pf.classes,
		}

		var uf bytes.Buffer
		err := gotemp.Execute(&uf, args)
		if err != nil {
			panic(err)
		}

		f, err := format.Source(uf.Bytes(), format.Options{})
		if err != nil {
			panic(err)
		}

		os.MkdirAll(fmt.Sprintf(dir+"/%s", pkgName), 0o755)

		err = os.WriteFile(fmt.Sprintf(dir+"/%s/%s", pkgName, fn), f, 0666)
		if err != nil {
			panic(err)
		}
	}
}

func (p *Pass) Second(dir string, gotemp *template.Template) {
	for _, r := range p.Parsed {
		p.writeGo(r, gotemp, dir)
	}
}
