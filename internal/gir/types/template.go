package types

import (
	"fmt"
	"strings"

	"github.com/bnema/puregotk/internal/gir/util"
)

type argsTemplate struct {
	// Names are the variables but just the names
	Names []string

	// Types are the variables but just the types
	Types []string

	// Call are the variables as given in a function call
	Call []string

	Full []string
}

// GErrorParam tracks a signal parameter that is exposed as *glib.Error in the
// public API but still crosses the purego callback ABI as a raw pointer.
type GErrorParam struct {
	Index  int
	GoType string
}

type funcArgsTemplate struct {
	// Pure are the arguments as passed directly to PureGo
	// The pure Call is a special case that contains the arguments for a callback call
	Pure argsTemplate

	// API are the arguments as suitable for a Go API
	API argsTemplate

	// GErrors tracks signal parameters that should be surfaced as *glib.Error.
	GErrors []GErrorParam
}

func (f *funcArgsTemplate) AddAPI(t string, n string, k Kind, ns string, nullable bool, isOut bool, imps *ImportSet) {
	c := n
	stars := strings.Count(t, "*")
	gobjectNs := "gobject."
	if strings.ToLower(ns) == "gobject" {
		gobjectNs = ""
	}
	glibNs := "glib."
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
	} else {
		switch k {
		case CallbackType:
			// Destroy/notify callbacks are effectively optional even when GIR omits
			// the nullable annotation.
			lowerName := strings.ToLower(n)
			isDestroyNotify := strings.Contains(lowerName, "destroy") ||
				strings.Contains(lowerName, "notify") ||
				strings.Contains(lowerName, "dnotify")
			if nullable || isDestroyNotify {
				c = fmt.Sprintf("%sNewCallbackNullable(%s)", glibNs, n)
			} else {
				c = fmt.Sprintf("%sNewCallback(%s)", glibNs, n)
			}
			imps.AddPkg("glib")
			t = "*" + t
		case ClassesType:
			if stars == 0 {
				c = n
				t = "uintptr"
			} else if stars > 1 {
				c = fmt.Sprintf("%sConvertPtr(%s)", gobjectNs, n)
				imps.AddPkg("gobject")
			} else if stars == 1 {
				c = n + ".GoPointer()"
			}
		case InterfacesType:
			t = strings.TrimPrefix(t, "*")
			if stars == 0 {
				c = n
				t = "uintptr"
			} else if stars > 1 {
				c = fmt.Sprintf("%sConvertPtr(%s)", gobjectNs, n)
				imps.AddPkg("gobject")
			} else if stars == 1 {
				c = n + ".GoPointer()"
			}
		}

		// special case for varargs
		if n == "varArgs" {
			c = n + "..."
		}
	}

	f.API.Names = append(f.API.Names, n)
	f.API.Types = append(f.API.Types, t)
	f.API.Call = append(f.API.Call, c)
	f.API.Full = append(f.API.Full, n+" "+t)
	imps.TrackGoType(t)
}

func (f *funcArgsTemplate) AddPure(t string, n string, k Kind, isOut bool) {
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

func isGErrorType(girName string) bool {
	return girName == "GLib.Error" || girName == "Error"
}

// PuregoSignalFull returns the callback parameter list for generated signal
// trampolines. GError values stay pointer-shaped but are typed as
// unsafe.Pointer to keep go vet happy.
func (f funcArgsTemplate) PuregoSignalFull() []string {
	out := make([]string, len(f.Pure.Full))
	copy(out, f.Pure.Full)
	for _, ge := range f.GErrors {
		out[ge.Index] = f.Pure.Names[ge.Index] + " unsafe.Pointer"
	}
	return out
}

// PuregoSignalCall returns the public signal callback arguments, casting any
// tracked GError parameters back to their Go types.
func (f funcArgsTemplate) PuregoSignalCall() []string {
	out := make([]string, len(f.Pure.Call))
	copy(out, f.Pure.Call)
	for _, ge := range f.GErrors {
		out[ge.Index] = "(*" + ge.GoType + ")(" + f.Pure.Names[ge.Index] + ")"
	}
	return out
}

// baseType returns strips prefixes from a Go type (e.g. `*glib.Error` → `glib`, `[4]gdk.RGBA` → `gdk`).
func baseTypeName(typeName string) string {
	return strings.TrimLeftFunc(typeName, func(r rune) bool {
		return r == '*' || r == '[' || r == ']' || (r >= '0' && r <= '9')
	})
}

// isGoPrimitive checks if a scalar or vector type name is a Go primitive type
func isGoPrimitive(typeName string) bool {
	baseType := baseTypeName(typeName)

	for _, goType := range convList {
		if goType == baseType {
			return true
		}
	}

	return false
}

func (f *funcArgsTemplate) Add(p Parameter, ins string, ns string, kinds KindMap, imps *ImportSet) {
	// get the lookup namespace
	// as if the interface namespace is non-empty
	// means we can also lookup in the namespace of the interface
	lns := ns
	if ins != "" {
		lns = ins
	}
	goType := p.Translate(lns, kinds)
	kind := kinds.Kind(lns, goType)

	stars := strings.Count(goType, "*")
	goType = util.NormalizeNamespace(ns, goType, true)

	if kind != OtherType && kind != UnknownType && !isGoPrimitive(goType) { // Only add namespace for non-primitive types
		goType = util.AddNamespace(goType, ins)
	}
	if stars > 0 {
		goType = util.StarsInFront(strings.ReplaceAll(goType, "*", ""), stars)
	}

	// Get a suitable variable name
	varName := p.VarName()

	// GIR "inout" parameters are also pointer-bearing at the ABI/API level.
	isOut := p.Direction == "out" || p.Direction == "inout"

	f.AddAPI(goType, varName, kind, ns, p.Nullable, isOut, imps)
	f.AddPure(goType, varName, kind, isOut)

	// Signal callbacks with GError* parameters should expose *glib.Error in the
	// public API even though the raw callback ABI still uses a pointer value.
	if goType == "uintptr" && p.Type != nil && isGErrorType(p.Type.Name) {
		gerrorGoType := "glib.Error"
		if strings.ToLower(ns) == "glib" {
			gerrorGoType = "Error"
		}
		apiIdx := len(f.API.Types) - 1
		f.API.Types[apiIdx] = "*" + gerrorGoType
		f.API.Full[apiIdx] = varName + " *" + gerrorGoType
		f.GErrors = append(f.GErrors, GErrorParam{
			Index:  len(f.Pure.Types) - 1,
			GoType: gerrorGoType,
		})
	}
}

func (f *funcArgsTemplate) AddThrows(ns string, imps *ImportSet) {
	f.API.Call = append(f.API.Call, "&cerr")
	if strings.ToLower(ns) != "glib" {
		f.Pure.Types = append(f.Pure.Types, "**glib.Error")
		imps.AddPkg("glib")
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
	// Record indicates whether or not the return value is a record/boxed pointer type
	Record bool
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
	if fr.Record {
		baseType := strings.TrimPrefix(fr.Value, "*")
		if fr.Throws {
			after.WriteString("if cerr != nil {\n")
			after.WriteString("return nil, cerr\n")
			after.WriteString("}\n")
			after.WriteString("if cret == 0 {\n")
			after.WriteString("return nil, nil\n")
			after.WriteString("}\n")
			after.WriteString(fmt.Sprintf("return (*%s)(unsafe.Pointer(cret)), nil\n", baseType))
			return after.String()
		}
		after.WriteString("if cret == 0 {\n")
		after.WriteString("return nil\n")
		after.WriteString("}\n")
		after.WriteString(fmt.Sprintf("return (*%s)(unsafe.Pointer(cret))\n", baseType))
		return after.String()
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
	Doc   string
	Name  string
	CName string
	Args  funcArgsTemplate
	Ret   funcRetTemplate
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
	// OptionalLibrary declares whether init should tolerate a missing shared library.
	OptionalLibrary bool
	// BuildConstraint is an optional file-level build tag like //go:build linux.
	BuildConstraint string
	// RegisterTypes declares whether the types in the GIR file need to be manually registered
	// See https://bugs.webkit.org/show_bug.cgi?id=175937
	RegisterTypes bool
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
