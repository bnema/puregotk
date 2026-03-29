// package pass implements the first and second pass to go from gir files to go files
// the first pass collects basic type information
// the second pass uses the basic type information to go over the gir files again and convert it to go files
package pass

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/bnema/puregotk/internal/gir/types"
	"github.com/bnema/puregotk/internal/gir/util"
)

type Pass struct {
	Parsed []types.Repository
	Types  types.KindMap
}

// NamespaceConfig holds per-namespace overrides for the code generator.
type NamespaceConfig struct {
	PackageName     string // override auto-derived package name (empty = use default)
	OptionalLibrary bool   // continue on dlopen failure instead of panic
	BuildConstraint string // e.g. "//go:build linux"
}

var namespaceConfigs = map[string]NamespaceConfig{
	"Gtk4LayerShell":  {PackageName: "layershell", OptionalLibrary: true, BuildConstraint: "//go:build linux"},
	"Gtk4SessionLock": {PackageName: "sessionlock", OptionalLibrary: true, BuildConstraint: "//go:build linux"},
}

// New creates a new pass struct by parsing gir files in the string slice
// This pass object will then be used to go over these files multiple times up until we have the full info to convert it to go files
func New(files []string) (*Pass, error) {
	p := Pass{
		Parsed: make([]types.Repository, len(files)),
		Types:  make(types.KindMap),
	}
	for i, f := range files {
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

// filterSentinelEnumMembers returns a copy of the enum with sentinel members
// (like *_ENTRY_NUMBER) removed. These are documented as "should not be used".
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

func (p *Pass) writeGo(r types.Repository, gotemp *template.Template, dir string) {
	ns := r.Namespaces[0]

	aliases := make(map[string][]types.AliasTemplate)
	enums := make(map[string][]types.EnumTemplate)
	var files []string
	for _, el := range ns.Bitfields {
		el.Members = filterSentinelEnumMembers(el.Members)
		temp := el.Template(ns.Name, ns.CIdentifierPrefixes)
		fn := el.FilenameSafe()
		files = append(files, fn)
		enums[fn] = append(enums[fn], temp)
	}

	for _, el := range ns.Enums {
		el.Members = filterSentinelEnumMembers(el.Members)
		temp := el.Template(ns.Name, ns.CIdentifierPrefixes)
		fn := el.FilenameSafe()
		files = append(files, fn)
		enums[fn] = append(enums[fn], temp)
	}

	constants := make(map[string][]types.ConstantTemplate)
	for _, con := range ns.Constants {
		fn := con.FilenameSafe()
		files = append(files, fn)
		constants[fn] = append(constants[fn], con.Template(ns.Name, p.Types))
	}

	callbackDocs := make(map[string]string)
	for _, cb := range ns.Callbacks {
		callbackDocs[cb.Name] = cb.Doc.StringSafe()
	}

	records := make(map[string][]types.RecordTemplate)
	recordLookup := make(map[string]bool)
	for _, rec := range ns.Records {
		name := util.SnakeToCamel(rec.Name)
		constructors := make([]types.FuncTemplate, len(rec.Constructors))
		receivers := make([]types.FuncTemplate, 0, len(rec.Methods))
		fields := make([]types.RecordField, 0, len(rec.Fields))
		callbackAccessors := make([]types.CallbackAccessor, 0)
		fn := rec.FilenameSafe()
		files = append(files, fn)
		for i, c := range rec.Constructors {
			constructors[i] = types.FuncTemplate{
				Name:  util.ConstructorName(c.Name, rec.Name),
				CName: c.CIdentifier,
				Doc:   c.Doc.StringSafe(),
				Args:  c.Parameters.Template(ns.Name, "", p.Types, c.Throws, types.ArgsFromGoToC),
				Ret:   c.ReturnValue.Template(ns.Name, "", p.Types, c.Throws),
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
				args := f.Callback.Parameters.Template(ns.Name, "", p.Types, f.Callback.Throws, types.ArgsFromCToGo)
				ret := f.Callback.ReturnValue.Template(ns.Name, "", p.Types, f.Callback.Throws)

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

				fieldName = util.SnakeToCamel(f.Name)
			}

			fields = append(fields, types.RecordField{
				Name: fieldName,
				Type: _type,
			})
		}
		for _, f := range rec.Methods {
			name := util.SnakeToCamel(f.Name)
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
				Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC),
				Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws),
			})
		}
		records[fn] = append(records[fn], types.RecordTemplate{
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

	callbacks := make(map[string][]types.CallbackTemplate)
	// set every callback equal to uintptr as well
	for _, cb := range ns.Callbacks {
		fn := cb.FilenameSafe()
		files = append(files, fn)
		cbT := types.CallbackTemplate{
			Doc:  cb.Doc.StringSafe(),
			Name: cb.Name,
			Args: cb.Parameters.Template(ns.Name, "", p.Types, cb.Throws, types.ArgsFromCToGo),
			Ret:  cb.ReturnValue.Template(ns.Name, "", p.Types, cb.Throws),
		}
		callbacks[fn] = append(callbacks[fn], cbT)
	}

	interfaces := make(map[string][]types.InterfaceTemplate)
	for _, inter := range ns.Interfaces {
		fn := inter.FilenameSafe()
		files = append(files, fn)
		interfaces[fn] = append(interfaces[fn], types.ConvertInterface(ns.Name, "", inter, nil, p.Types))
	}

	for _, union := range ns.Unions {
		fn := union.FilenameSafe()
		files = append(files, fn)
		name := util.SnakeToCamel(union.Name)
		interT := types.AliasTemplate{
			Doc:  union.Doc.StringSafe(),
			Name: name,
			// structs are not yet supported in CGO
			Value: "uintptr",
		}
		aliases[fn] = append(aliases[fn], interT)
	}

	for _, alias := range ns.Aliases {
		fn := alias.FilenameSafe()
		files = append(files, fn)
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
		aliases[fn] = append(aliases[fn], aliasT)
	}

	functions := make(map[string][]types.FuncTemplate)
	for _, f := range ns.Functions {
		name := util.SnakeToCamel(f.Name)
		if p.Types.Kind(ns.Name, name) != types.UnknownType {
			name = "New" + name
		}
		fn := f.FilenameSafe()
		files = append(files, fn)
		functions[fn] = append(functions[fn], types.FuncTemplate{
			Name:  name,
			CName: f.CIdentifier,
			Doc:   f.Doc.StringSafe(),
			Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC),
			Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws),
		})
	}

	classes := make(map[string][]types.ClassTemplate)
	for _, cls := range ns.Classes {
		implemented := make(map[string]bool)
		constructors := make([]types.FuncTemplate, len(cls.Constructors))
		functions := make([]types.FuncTemplate, len(cls.Functions))
		fn := cls.FilenameSafe()
		files = append(files, fn)

		for i, c := range cls.Constructors {
			c.ReturnValue.AnyType.Type.Name = cls.Name
			constructors[i] = types.FuncTemplate{
				Name:  util.ConstructorName(c.Name, cls.Name),
				CName: c.CIdentifier,
				Doc:   c.Doc.StringSafe(),
				Args:  c.Parameters.Template(ns.Name, "", p.Types, c.Throws, types.ArgsFromGoToC),
				Ret:   c.ReturnValue.Template(ns.Name, "", p.Types, c.Throws),
			}
		}
		signals := make([]types.SignalsTemplate, len(cls.Signals))
		for i, s := range cls.Signals {
			signals[i] = types.SignalsTemplate{
				Doc:      s.Doc.StringSafe(),
				Name:     util.DashToCamel(s.Name),
				CName:    s.Name,
				Args:     s.Parameters.Template(ns.Name, "", p.Types, false, types.ArgsFromCToGo),
				Ret:      s.ReturnValue.Template(ns.Name, "", p.Types, false),
				Detailed: s.Detailed,
			}
		}
		receivers := make([]types.FuncTemplate, len(cls.Methods))
		for i, f := range cls.Methods {
			name := util.SnakeToCamel(f.Name)
			implemented[name] = true
			receivers[i] = types.FuncTemplate{
				Doc:   f.Doc.StringSafe(),
				Name:  name,
				CName: f.CIdentifier,
				Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC),
				Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws),
			}
		}
		var interfaces []types.InterfaceTemplate
		for i, f := range cls.Functions {
			name := fmt.Sprintf("%s%s", util.SnakeToCamel(cls.Name), util.SnakeToCamel(f.Name))
			functions[i] = types.FuncTemplate{
				Name:  name,
				CName: f.CIdentifier,
				Doc:   f.Doc.StringSafe(),
				Args:  f.Parameters.Template(ns.Name, "", p.Types, f.Throws, types.ArgsFromGoToC),
				Ret:   f.ReturnValue.Template(ns.Name, "", p.Types, f.Throws),
			}
		}
		for _, impl := range cls.Implements {
			interfaces = append(interfaces, types.GetInterfaceFuncs(ns.Name, impl.Name, implemented, p.Types))
		}
		properties := make([]types.PropertyTemplate, 0, len(cls.Properties))
		for _, prop := range cls.Properties {
			propTemp := prop.Template(ns.Name, p.Types)

			// TODO: Implement non-primitive types, then remove this
			if propTemp.GValueType != "" {
				properties = append(properties, propTemp)
			}
		}
		classes[fn] = append(classes[fn], types.ClassTemplate{
			Doc:          cls.Doc.StringSafe(),
			Name:         cls.Name,
			Parent:       util.NormalizeNamespace(ns.Name, cls.Parent, true),
			Constructors: constructors,
			Receivers:    receivers,
			Interfaces:   interfaces,
			Functions:    functions,
			Properties:   properties,
			Signals:      signals,
			TypeGetter:   cls.GLibGetType,
		})
	}

	pkgName := strings.ToLower(ns.Name)
	nsCfg := namespaceConfigs[ns.Name] // zero value if not configured
	if nsCfg.PackageName != "" {
		pkgName = nsCfg.PackageName
	}

	var pkgConfigName string
	if len(r.Packages) > 0 {
		pkgConfigName = r.Packages[0].Name
	}

	var sharedLibraries []string
	if ns.SharedLibrary != "" {
		for _, lib := range strings.Split(ns.SharedLibrary, ",") {
			if trimmed := strings.TrimSpace(lib); trimmed != "" {
				sharedLibraries = append(sharedLibraries, trimmed)
			}
		}
	}

	for _, fn := range files {
		methods := 0
		for _, i := range interfaces[fn] {
			methods += len(i.Methods)
		}
		for _, i := range records[fn] {
			methods += len(i.Constructors)
			methods += len(i.Receivers)
		}
		for _, i := range classes[fn] {
			methods += len(i.Constructors)
			methods += len(i.Receivers)
			methods += len(i.Functions)
		}
		// we do not need to add the length of interfaces in here
		// as they should only be loaded when there are classes
		needsInit := (len(functions[fn]) + methods) > 0

		// Check if any receiver method has callback parameters
		// This is used to conditionally import unsafe and purego
		hasReceiverCallbacks := false
		for _, rec := range records[fn] {
			for _, r := range rec.Receivers {
				if len(r.Args.Callbacks) > 0 {
					hasReceiverCallbacks = true
					break
				}
			}
			if hasReceiverCallbacks {
				break
			}
		}
		if !hasReceiverCallbacks {
			for _, cls := range classes[fn] {
				for _, r := range cls.Receivers {
					if len(r.Args.Callbacks) > 0 {
						hasReceiverCallbacks = true
						break
					}
				}
				if hasReceiverCallbacks {
					break
				}
			}
		}

		// Check if any standalone function has callback parameters
		hasFunctionCallbacks := false
		for _, f := range functions[fn] {
			if len(f.Args.Callbacks) > 0 {
				hasFunctionCallbacks = true
				break
			}
		}

		needsCoreHelpers := false
		checkFuncArgs := func(funcs []types.FuncTemplate) {
			if needsCoreHelpers {
				return
			}
			for _, f := range funcs {
				if f.Args.NeedsCore() {
					needsCoreHelpers = true
					return
				}
			}
		}
		checkInterfaceArgs := func(funcs []types.InterfaceFuncTemplate) {
			if needsCoreHelpers {
				return
			}
			for _, f := range funcs {
				if f.Args.NeedsCore() {
					needsCoreHelpers = true
					return
				}
			}
		}

		for _, rec := range records[fn] {
			checkFuncArgs(rec.Constructors)
			checkFuncArgs(rec.Receivers)
		}
		for _, cls := range classes[fn] {
			checkFuncArgs(cls.Constructors)
			checkFuncArgs(cls.Receivers)
			checkFuncArgs(cls.Functions)
		}
		checkFuncArgs(functions[fn])
		for _, inter := range interfaces[fn] {
			checkInterfaceArgs(inter.Methods)
		}

		// Check if any signal has string parameters that need core.GoString()
		hasSignalStrings := false
		for _, cls := range classes[fn] {
			for _, sig := range cls.Signals {
				if sig.Args.HasPureStrings() {
					hasSignalStrings = true
					break
				}
			}
			if hasSignalStrings {
				break
			}
		}

		// Scan all types for cross-package references (glib.*, gobject.*, types.*).
		needsGLib, needsGObject, hasTypeGetters, crossPkgImports := scanCrossPackageRefs(
			pkgName, functions[fn], records[fn], classes[fn], interfaces[fn])
		// TypeGetters are emitted by the template when .TypeGetter != "", but
		// the string "types.GType" only appears in template output — not in the
		// Go data structures that scanCrossPackageRefs inspects. Check explicitly.
		if !hasTypeGetters {
			for _, r := range records[fn] {
				if r.TypeGetter != "" {
					hasTypeGetters = true
					break
				}
			}
		}
		if !hasTypeGetters {
			for _, c := range classes[fn] {
				if c.TypeGetter != "" {
					hasTypeGetters = true
					break
				}
			}
		}
		if !hasTypeGetters {
			for _, i := range interfaces[fn] {
				if i.TypeGetter != "" {
					hasTypeGetters = true
					break
				}
			}
		}
		if !hasTypeGetters {
			for _, a := range aliases[fn] {
				if a.TypeGetter != "" {
					hasTypeGetters = true
					break
				}
			}
		}
		if !hasTypeGetters {
			for _, e := range enums[fn] {
				if e.TypeGetter != "" {
					hasTypeGetters = true
					break
				}
			}
		}

		args := types.TemplateArg{
			PkgName:              pkgName,
			PkgEnv:               strings.ToUpper(pkgName),
			PkgConfigName:        pkgConfigName,
			SharedLibraries:      sharedLibraries,
			NeedsInit:            needsInit,
			NeedsCore:            needsCoreHelpers,
			HasReceiverCallbacks: hasReceiverCallbacks,
			HasFunctionCallbacks: hasFunctionCallbacks,
			HasSignalStrings:     hasSignalStrings,
			HasTypeGetters:       hasTypeGetters,
			NeedsGLib:            needsGLib,
			NeedsGObject:         needsGObject,
			CrossPkgImports:      crossPkgImports,
			OptionalLibrary:      nsCfg.OptionalLibrary,
			BuildConstraint:      nsCfg.BuildConstraint,
			Aliases:              aliases[fn],
			Callbacks:            callbacks[fn],
			Records:              records[fn],
			Enums:                enums[fn],
			Constants:            constants[fn],
			Functions:            functions[fn],
			Interfaces:           interfaces[fn],
			Classes:              classes[fn],
		}

		os.MkdirAll(fmt.Sprintf(dir+"/%s", pkgName), 0o755)

		f, err := os.Create(fmt.Sprintf(dir+"/%s/%s", pkgName, fn))
		if err != nil {
			panic(err)
		}
		err = gotemp.Execute(f, args)
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

// scanCrossPackageRefs checks if any generated code in the file references
// glib.* or gobject.* symbols in pure types, return types, record fields,
// class parents, signal args, or callback accessor types.
func scanCrossPackageRefs(
	pkgName string,
	funcs []types.FuncTemplate,
	recs []types.RecordTemplate,
	classes []types.ClassTemplate,
	ifaces []types.InterfaceTemplate,
) (needsGLib, needsGObject, needsTypes bool, crossPkgImports []string) {
	// Track all cross-package references (e.g. "gio.", "cairo.", "pango.")
	crossPkgs := make(map[string]bool)
	check := func(s string) {
		if !needsGLib && strings.Contains(s, "glib.") {
			needsGLib = true
		}
		if !needsGObject && strings.Contains(s, "gobject.") {
			needsGObject = true
		}
		if !needsTypes && strings.Contains(s, "types.") {
			needsTypes = true
		}
		// Detect any "pkg." references for cross-package imports
		for _, pkg := range []string{"gio", "cairo", "pango", "pangocairo", "graphene", "gsk", "gdk", "gtk"} {
			if strings.Contains(s, pkg+".") {
				crossPkgs[pkg] = true
			}
		}
	}
	checkFuncTypes := func(f types.FuncTemplate) {
		check(f.Ret.Raw)
		check(f.Ret.Value)
		// RefSink return types trigger gobject.IncreaseRef in template Fmt()
		if f.Ret.RefSink {
			needsGObject = true
		}
		// Throws trigger glib.Error in template Preamble()
		if f.Ret.Throws {
			needsGLib = true
		}
		for _, t := range f.Args.Pure.Types {
			check(t)
		}
		for _, t := range f.Args.API.Types {
			check(t)
		}
	}

	for _, f := range funcs {
		checkFuncTypes(f)
		// Callback parameters generate glib.GetCallback/SaveCallbackWithClosure
		// and gobject.SignalConnect calls in the template output.
		if len(f.Args.Callbacks) > 0 {
			needsGLib = true
		}
	}
	for _, r := range recs {
		for _, f := range r.Fields {
			check(f.Type)
		}
		for _, c := range r.Constructors {
			checkFuncTypes(c)
		}
		for _, m := range r.Receivers {
			checkFuncTypes(m)
			if len(m.Args.Callbacks) > 0 {
				needsGLib = true
			}
		}
		for _, ca := range r.CallbackAccessors {
			check(ca.CallbackType)
		}
	}
	for _, c := range classes {
		check(c.Parent)
		// Signals use glib helpers + gobject.SignalConnect
		if len(c.Signals) > 0 {
			needsGLib = true
			needsGObject = true
		}
		for _, con := range c.Constructors {
			checkFuncTypes(con)
		}
		for _, m := range c.Receivers {
			checkFuncTypes(m)
		}
		for _, f := range c.Functions {
			checkFuncTypes(f)
		}
		for _, s := range c.Signals {
			for _, t := range s.Args.Pure.Types {
				check(t)
			}
			for _, t := range s.Args.API.Types {
				check(t)
			}
		}
		// Class properties use gobject.Value
		if len(c.Properties) > 0 {
			needsGObject = true
		}
		// Class interface implementations (e.g. gio.ListModel methods on pango.FontFamily)
		for _, iface := range c.Interfaces {
			for _, m := range iface.Methods {
				checkFuncTypes(m.FuncTemplate)
				if m.Namespace != "" {
					check(m.Namespace)
				}
			}
			if len(iface.Properties) > 0 {
				needsGObject = true
			}
		}
	}
	for _, i := range ifaces {
		for _, m := range i.Methods {
			checkFuncTypes(m.FuncTemplate)
			// Interface methods have a Namespace prefix (e.g. "gio.")
			if m.Namespace != "" {
				check(m.Namespace)
			}
		}
		// Interface properties use gobject.Value/Object for get/set
		if len(i.Properties) > 0 {
			needsGObject = true
			needsGLib = true // glib.StrvGetType etc. used in property vector types
		}
		for _, p := range i.Properties {
			check(p.GoType)
		}
		// Interface methods with callbacks need glib for GetCallback/SaveCallbackWithClosure
		for _, m := range i.Methods {
			if len(m.FuncTemplate.Args.Callbacks) > 0 {
				needsGLib = true
			}
		}
	}
	// A package never needs to import itself, and glib/gobject cannot
	// import each other (circular dependency).
	if strings.EqualFold(pkgName, "glib") {
		needsGLib = false
		needsGObject = false // glib cannot import gobject (cycle)
	}
	if strings.EqualFold(pkgName, "gobject") {
		needsGObject = false
		// gobject CAN import glib (no cycle: glib does not import gobject)
	}
	delete(crossPkgs, strings.ToLower(pkgName))
	// glib, gobject, and types are handled by dedicated template conditions
	delete(crossPkgs, "glib")
	delete(crossPkgs, "gobject")
	// Build import paths for detected cross-package references
	const moduleBase = "github.com/bnema/puregotk/v4/"
	for pkg := range crossPkgs {
		crossPkgImports = append(crossPkgImports, moduleBase+pkg)
	}
	return
}
