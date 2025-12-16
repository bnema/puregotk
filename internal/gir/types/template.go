package types

import (
	"fmt"
	"strings"

	"github.com/jwijenbergh/puregotk/internal/gir/util"
)

type argsTemplate struct {
	// Names are the variables but just the names
	Names []string

	// Types are the variables but just the types
	Types []string

	// Call are the variables as given in a function call
	Call []string

	// CallWithRefs is like Call but with callback parameters using {name}Ref
	// for use in contexts that generate closure wrappers
	CallWithRefs []string

	Full []string
}

// CallbackParam holds metadata for callback parameters to enable proper closure generation
type CallbackParam struct {
	// Name is the parameter name (e.g., "CallbackVar")
	Name string
	// TypeName is the callback type name (e.g., "TickCallback")
	TypeName string
	// PureTypes are the pure argument types for the closure
	PureTypes []string
	// RetRaw is the return type for the closure (e.g., "bool")
	RetRaw string
	// Nullable indicates if the callback can be nil
	Nullable bool
}

type funcArgsTemplate struct {
	// Pure are the arguments as passed directly to PureGo
	// The pure Call is a special case that contains the arguments for a callback call
	Pure argsTemplate

	// API are the arguments as suitable for a Go API
	API argsTemplate

	// Callbacks tracks callback parameters for proper closure generation
	Callbacks []CallbackParam

	// UsesNullableHelper indicates nullable string handling that needs core import.
	UsesNullableHelper bool
}

// ArgContext indicates where the arguments are flowing so we can handle
// direction-sensitive cases (e.g. nullable strings) differently.
type ArgContext int

const (
	// ArgsFromGoToC covers regular functions/methods where Go calls into C.
	ArgsFromGoToC ArgContext = iota
	// ArgsFromCToGo covers callbacks/signals where C calls into Go.
	ArgsFromCToGo
)

func isStringType(t string) bool {
	if strings.HasPrefix(t, "[]") {
		return false
	}
	return strings.TrimLeft(t, "*") == "string"
}

// NeedsCore reports whether this argument set requires core helpers.
func (f funcArgsTemplate) NeedsCore() bool {
	return f.UsesNullableHelper
}

func qualifyCallbackType(t string, callbackNS string, currentNS string) string {
	if t == "" || callbackNS == "" || strings.EqualFold(callbackNS, currentNS) {
		return t
	}

	if strings.Contains(t, ".") {
		return t
	}

	ptrPrefix := ""
	base := t
	for strings.HasPrefix(base, "*") {
		ptrPrefix += "*"
		base = strings.TrimPrefix(base, "*")
	}

	slicePrefix := ""
	for strings.HasPrefix(base, "[]") {
		slicePrefix += "[]"
		base = strings.TrimPrefix(base, "[]")
	}

	arrayPrefix := ""
	for strings.HasPrefix(base, "[") {
		end := strings.Index(base, "]")
		if end == -1 {
			break
		}
		arrayPrefix += base[:end+1]
		base = base[end+1:]
	}

	switch base {
	case "bool", "byte", "complex64", "complex128", "error", "float32", "float64", "int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
		return t
	}

	if base == "" {
		return t
	}

	qualifiedBase := callbackNS + "." + base
	return ptrPrefix + slicePrefix + arrayPrefix + qualifiedBase
}

func qualifyCallbackTypes(types []string, callbackNS string, currentNS string) []string {
	qualified := make([]string, len(types))
	for i, t := range types {
		qualified[i] = qualifyCallbackType(t, callbackNS, currentNS)
	}
	return qualified
}

func (f *funcArgsTemplate) AddAPI(t string, n string, k Kind, ns string, nullable bool, isOut bool, ctx ArgContext) {
	c := n
	cRef := n // For CallWithRefs, defaults to same as Call
	stars := strings.Count(t, "*")
	gobjectNs := "gobject."
	glibNs := "glib."
	if strings.ToLower(ns) == "gobject" {
		gobjectNs = ""
	}
	if strings.ToLower(ns) == "glib" {
		glibNs = ""
	}

	if isOut {
		if stars == 0 {
			// For out parameters, the C type already has a pointer, and so do non-primitive Go types.
			// For primitive Go types we need to manually add the *
			t = "*" + t
		}
		c = n
		cRef = n
	} else {
		// Nullable strings differ based on direction. For Go->C we need a *string API type
		// and pass a pointer (or nil) to C. For C->Go we keep the string as-is.
		if ctx == ArgsFromGoToC && nullable && isStringType(t) {
			f.UsesNullableHelper = true
			t = "*string"
			c = fmt.Sprintf("core.NullableStringToPtr(%s)", n)
			cRef = c
		}

		switch k {
		case CallbackType:
			// Call uses glib.NewCallback for contexts like callback accessor getters
			if nullable {
				c = fmt.Sprintf("%sNewCallbackNullable(%s)", glibNs, n)
			} else {
				c = fmt.Sprintf("%sNewCallback(%s)", glibNs, n)
			}
			// For CallWithRefs, start with the same value as Call
			// It will be updated to {name}Ref in Add() if the callback lookup succeeds
			cRef = c
			t = "*" + t
		case ClassesType:
			if stars == 0 {
				c = n
				t = "uintptr"
			} else if stars > 1 {
				c = fmt.Sprintf("%sConvertPtr(%s)", gobjectNs, n)
			} else if stars == 1 {
				c = n + ".GoPointer()"
			}
			cRef = c
		case InterfacesType:
			t = strings.TrimPrefix(t, "*")
			if stars == 0 {
				c = n
				t = "uintptr"
			} else if stars > 1 {
				c = fmt.Sprintf("%sConvertPtr(%s)", gobjectNs, n)
			} else if stars == 1 {
				c = n + ".GoPointer()"
			}
			cRef = c
		default:
			cRef = c
		}

		// special case for varargs
		if n == "varArgs" {
			c = n + "..."
			cRef = c
		}
	}

	f.API.Names = append(f.API.Names, n)
	f.API.Types = append(f.API.Types, t)
	f.API.Call = append(f.API.Call, c)
	f.API.CallWithRefs = append(f.API.CallWithRefs, cRef)
	f.API.Full = append(f.API.Full, n+" "+t)
}

func (f *funcArgsTemplate) AddPure(t string, n string, k Kind, isOut bool, nullable bool, ctx ArgContext) {
	n += "p"
	c := n
	stars := strings.Count(t, "*")

	if isOut {
		// Out parameters are always pointers in C
		if stars == 0 {
			// For primitive Go types we need to manually add the *
			t = "*" + t
		}
		c = n
	} else {
		if ctx == ArgsFromGoToC && nullable && isStringType(t) {
			f.UsesNullableHelper = true
			t = "uintptr"
			c = fmt.Sprintf("core.NullableStringToPtr(%s)", strings.TrimSuffix(n, "p"))
		}

		switch k {
		case RecordsType:
			if stars == 0 {
				t = "uintptr"
			}
		case CallbackType:
			c = fmt.Sprintf("(*%s)(unsafe.Pointer(%s))", strings.TrimPrefix(t, "*"), n)
			t = "uintptr"
		case ClassesType:
			if stars == 0 {
				c = n
				t = "uintptr"
			} else {
				// Remove all dereference operators to get the base class name
				baseName := strings.TrimPrefix(t, strings.Repeat("*", stars))
				if stars > 1 {
					// For double pointers like **ParamSpec, we need to pass the double pointer directly
					c = fmt.Sprintf("(**%s)(unsafe.Pointer(%s))", baseName, n)
				} else {
					c = fmt.Sprintf("%sNewFromInternalPtr(%s)", baseName, n)
				}
				t = "uintptr"
			}
		case InterfacesType:
			if stars == 0 {
				c = n
				t = "uintptr"
			} else {
				c = fmt.Sprintf("%s{Ptr: %s}", t+"Base", n)
				t = strings.Repeat("*", stars-1) + "uintptr"
			}
		}
	}
	f.Pure.Names = append(f.Pure.Names, n)
	f.Pure.Types = append(f.Pure.Types, t)
	f.Pure.Call = append(f.Pure.Call, c)
	f.Pure.Full = append(f.Pure.Full, n+" "+t)
}

func (f *funcArgsTemplate) Add(p Parameter, ins string, ns string, kinds KindMap, ctx ArgContext) {
	// get the lookup namespace
	// as if the interface namespace is non-empty
	// means we can also lookup in the namespace of the interface
	lns := ns
	if ins != "" {
		lns = ins
	}
	goType := p.Translate(lns, kinds)
	kind := kinds.Kind(lns, goType)

	// Save the original type name before normalization for callback lookup
	originalType := goType

	stars := strings.Count(goType, "*")
	goType = util.NormalizeNamespace(ns, goType, true)

	if kind != OtherType && kind != UnknownType {
		goType = util.AddNamespace(goType, ins)
	}
	if stars > 0 {
		goType = util.StarsInFront(strings.ReplaceAll(goType, "*", ""), stars)
	}

	// Get a suitable variable name
	varName := p.VarName()

	isOut := p.Direction == "out"

	f.AddAPI(goType, varName, kind, ns, p.Nullable, isOut, ctx)
	f.AddPure(goType, varName, kind, isOut, p.Nullable, ctx)

	// For callback parameters (not out parameters), populate callback metadata
	// This enables the template to generate proper closure wrapping
	if kind == CallbackType && !isOut {
		if cb, ok := kinds.GetCallback(lns, originalType); ok {
			// Determine the callback's namespace from the original type name
			// e.g., "gio.AsyncReadyCallback" -> "gio", "AsyncReadyCallback" -> lns
			cbNs := lns
			if parts := strings.Split(originalType, "."); len(parts) > 1 {
				cbNs = parts[0]
			}

			// Get the callback's pure argument types and return type
			// Use cbNs (callback's namespace) for proper type lookups
			cbArgs := cb.Parameters.Template(cbNs, "", kinds, cb.Throws, ArgsFromCToGo)
			var retRaw string
			if cb.ReturnValue != nil {
				cbRet := cb.ReturnValue.Template(cbNs, "", kinds, cb.Throws)
				retRaw = cbRet.Raw
			}

			qualifiedPureTypes := qualifyCallbackTypes(cbArgs.Pure.Types, cbNs, ns)
			qualifiedRetRaw := qualifyCallbackType(retRaw, cbNs, ns)

			f.Callbacks = append(f.Callbacks, CallbackParam{
				Name:      varName,
				TypeName:  strings.TrimPrefix(goType, "*"),
				PureTypes: qualifiedPureTypes,
				RetRaw:    qualifiedRetRaw,
				Nullable:  p.Nullable,
			})

			// Update CallWithRefs to use {name}Ref since we have the callback info
			// for generating the closure wrapper
			lastIdx := len(f.API.CallWithRefs) - 1
			f.API.CallWithRefs[lastIdx] = varName + "Ref"
		}
	}
}

func (f *funcArgsTemplate) AddThrows(ns string) {
	f.API.Call = append(f.API.Call, "&cerr")
	f.API.CallWithRefs = append(f.API.CallWithRefs, "&cerr")
	if strings.ToLower(ns) != "glib" {
		f.Pure.Types = append(f.Pure.Types, "**glib.Error")
	} else {
		f.Pure.Types = append(f.Pure.Types, "**Error")
	}
}

type CallbackTemplate struct {
	Doc  string
	Name string
	Args funcArgsTemplate
	Ret  funcRetTemplate
}

type AliasTemplate struct {
	// Name is the name of the alias given to the Go type declaration
	Name string

	// Doc is the documentation of the alias
	Doc string

	// Value is the value for the alias as a Go type
	Value string

	// TypeGetter is the function to get the GLib type
	TypeGetter string
}

type RecordField struct {
	// Name is the Go name of the field
	Name string

	// Type is the Go type of the field
	Type string
}

type CallbackAccessor struct {
	// Name is the Go name of the callback field (without x prefix)
	Name string

	// CName is the raw c name
	CName string

	// Doc is the documentation for the callback
	Doc string

	// CallbackType is the name of the callback function type
	CallbackType string

	// Args are the callback function arguments template
	Args funcArgsTemplate

	// Ret is the callback function return template
	Ret funcRetTemplate
}

type RecordTemplate struct {
	// Name is the name of the record given to the Go type declaration
	Name string

	// Doc is the documentation of the alias
	Doc string

	// Constructors is the slice of functions that create the class struct
	Constructors []FuncTemplate

	// Receivers is the slice of functions that have value receivers to the struct
	Receivers []FuncTemplate

	// Fields is the list of record fields
	Fields []RecordField

	// CallbackAccessors are the setter/getter methods for callback fields
	CallbackAccessors []CallbackAccessor

	// TypeGetter is the function to get the GLib type
	TypeGetter string
}

type enumValues struct {
	// Doc is the documentation for the value
	Doc string
	// Name is the name of the enumeration value
	Name string
	// Value is the actual underlying value
	Value int
}

type EnumTemplate struct {
	// Name is the name of the enumeration declared as the Go type for the int
	Name string
	// Doc is the documentation for the enumeration
	Doc string
	// Values are the list of values for the enumeration
	Values []enumValues
	// TypeGetter is the function to get the GLib type
	TypeGetter string
}

type ConstantTemplate struct {
	// Name is the name of the constant
	Name string
	// Doc is the documentation for the constant
	Doc string
	// Type is the Go type for the constant
	Type string
	// Values are the list of values for the constant
	Value string
}

type funcRetTemplate struct {
	// Raw is the raw value for the underlying purego function
	Raw string
	// Value is the underlying return value as a Go type
	Value string
	// Class indicates whether or not the return value is a class
	Class bool
	// RefSink indicates whether or not we should increase the reference count using obj.RefSink()
	RefSink bool
	// Throws indicates whether or not this function throws
	Throws bool
}

func (fr *funcRetTemplate) Instance() string {
	val := fr.Value + "{}"
	if strings.HasPrefix(fr.Value, "*") {
		return "&" + val[1:]
	}
	return val
}

func (fr *funcRetTemplate) Return() string {
	if fr.Throws {
		if fr.Value == "" {
			return "error"
		}
		return fmt.Sprintf("(%s, error)", fr.Value)
	}
	return fr.Value
}

func (fr *funcRetTemplate) HasReturn() bool {
	return fr.Value != "" || fr.Throws
}

func (fr *funcRetTemplate) Preamble(nglib bool) string {
	preamb := strings.Builder{}
	if fr.Class {
		preamb.WriteString("var cls ")
		preamb.WriteString(fr.Value)
		preamb.WriteString("\n")
	}
	if fr.Throws {
		preamb.WriteString("var cerr *")
		if nglib {
			preamb.WriteString("glib.")
		}
		preamb.WriteString("Error\n")
	}
	return preamb.String()
}

func (fr *funcRetTemplate) Fmt(ngo bool) string {
	if !fr.HasReturn() {
		return ""
	}
	after := strings.Builder{}
	val := "cret"
	if fr.Class {
		if fr.Throws {
			after.WriteString(`
    if cret == 0 {
        return nil, cerr
    }
`)
		} else {
			after.WriteString(`
    if cret == 0 {
        return nil
    }
`)
		}
		if fr.RefSink {
			if ngo {
				after.WriteString("gobject.")
			}
			after.WriteString("IncreaseRef(cret)\n")
		}
		after.WriteString("cls = ")
		after.WriteString(fr.Instance())
		after.WriteString("\n")
		after.WriteString("cls.Ptr = cret\n")
		val = "cls"
	}
	if fr.Throws {
		after.WriteString("if cerr == nil {\n")
		after.WriteString("return ")
		if fr.Value != "" {
			after.WriteString(val)
			after.WriteString(",")
		}
		after.WriteString("nil\n")
		after.WriteString("}\n")
		after.WriteString("return ")
		if fr.Value != "" {
			after.WriteString(val)
			after.WriteString(",")
		}
		after.WriteString("cerr\n")
		return after.String()
	}
	after.WriteString("return ")
	after.WriteString(val)
	return after.String()
}

type FuncTemplate struct {
	// Name is the name of the function declared as the Go function variable and public exposed API
	Name string
	// CName is the raw c name to be passed to purego register
	CName string
	// Doc is the documentation for the function
	Doc string
	// Args are the arguments
	Args funcArgsTemplate
	// Ret is the return argument
	Ret funcRetTemplate
}

type InterfaceFuncTemplate struct {
	Namespace string
	FullName  string
	FuncTemplate
}

type SignalsTemplate struct {
	Doc      string
	Name     string
	CName    string
	Args     funcArgsTemplate
	Ret      funcRetTemplate
	Detailed bool
}

type PropertyTemplate struct {
	// Doc is the documentation for the property
	Doc string
	// Name is the Go name for the property
	Name string
	// CName is the raw c name
	CName string
	// GoType is the Go type for the property
	GoType string
	// GValueType is the GObject Type constant (e.g. "TypeBooleanVal")
	GValueType string
	// SetMethod is the Value setter method name (e.g. "SetBoolean")
	SetMethod string
	// GetMethod is the Value getter method name (e.g. "GetBoolean")
	GetMethod string
	// Readable indicates if this property can be read
	Readable bool
	// Writable indicates if this property can be written
	Writable bool
}

type ClassTemplate struct {
	// Doc is the documentation for the class
	Doc string
	// Name is the name of the class that is given to the Go struct
	Name string
	// Parent is a non-empty string of the embedded parent struct
	Parent string
	// Constructors is the slice of functions that create the class struct
	Constructors []FuncTemplate
	// Receivers is the slice of functions that have value receivers to the struct
	Receivers []FuncTemplate
	// Interfaces are receiver methods that are implemented because it needs to satisfy a certain interface
	Interfaces []InterfaceTemplate
	// Functions are the Go function declarations
	Functions []FuncTemplate
	// Properties are the property getters and setters
	Properties []PropertyTemplate
	// Signals are helpers for ConnectX receivers
	Signals []SignalsTemplate
	// TypeGetter is the function to get the GLib type
	TypeGetter string
}

type InterfaceTemplate struct {
	Doc  string
	Name string
	// Methods is the methods that this interface defines
	Methods []InterfaceFuncTemplate
	// Properties are the property getters and setters
	Properties []PropertyTemplate
	// TypeGetter is the function to get the GLib type
	TypeGetter string
}

type TemplateArg struct {
	// PkgName is the name of the package, declared at the top-level
	PkgName string
	// PkgEnv is the name of the package in the load environment variable
	PkgEnv string
	// PkgConfigName is the pkg-config package name from the GIR file
	PkgConfigName string
	// SharedLibraries is the list of shared library names from the GIR file
	SharedLibraries []string
	// NeedsInit declares whether or not this file needs an init code to register functions with purego
	NeedsInit bool
	// NeedsCore indicates if core helpers are required even without init.
	NeedsCore bool
	// HasReceiverCallbacks indicates if any receiver method has callback parameters
	// This is used to conditionally import unsafe and purego
	HasReceiverCallbacks bool
	// HasFunctionCallbacks indicates if any standalone function has callback parameters
	HasFunctionCallbacks bool
	// Imports defines the package imports that we need
	// This does not include purego
	// As the template already includes that if `NeedsInit` is set to true
	Imports []string
	// Aliases are type aliases declared as type ... = ...
	Aliases []AliasTemplate
	// Aliases are structs that are not classes
	Records []RecordTemplate
	// Callbacks are functions that will be converted with purego to uintptr
	Callbacks []CallbackTemplate
	// Enums are enumerations declared as const ... .... = ....
	Enums []EnumTemplate
	// Constants are declared as const ... .... = ....
	Constants []ConstantTemplate
	// Functions are the Go function declarations
	Functions []FuncTemplate
	// Interfaces is the list of interfaces that this package implements
	Interfaces []InterfaceTemplate
	// Classes are the Go struct with receiver declarations
	Classes []ClassTemplate
}
