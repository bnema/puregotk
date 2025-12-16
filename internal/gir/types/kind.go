package types

import (
	"strings"

	"github.com/jwijenbergh/puregotk/internal/gir/util"
)

type Kind int8

const (
	UnknownType Kind = iota
	AliasType
	CallbackType
	ClassesType
	InterfacesType
	RecordsType
	SliceType
	OtherType
)

type KindMap map[string]KindPair

func (km KindMap) key(ns string, name string) string {
	return util.NormalizeNamespace(ns, name, false)
}

func (km KindMap) Add(ns string, name string, t Kind, v interface{}) {
	k := km.key(ns, name)
	km[k] = KindPair{
		K:     t,
		Value: v,
	}
}

func (km KindMap) pair(ns string, name string) KindPair {
	f := util.NormalizeNamespace(ns, name, false)
	return km[f]
}

func (km KindMap) Kind(ns string, name string) Kind {
	if strings.HasPrefix(name, "[") {
		return SliceType
	}
	return km.pair(ns, name).K
}

func (km KindMap) MustInterface(ns string, name string) Interface {
	p := km.pair(ns, name)
	if p.K != InterfacesType {
		panic("value is not an interface")
	}
	return p.Value.(Interface)
}

// GetCallback retrieves a callback definition by namespace and name.
// Returns the Callback and true if found, otherwise nil and false.
func (km KindMap) GetCallback(ns string, name string) (Callback, bool) {
	p := km.pair(ns, name)
	if p.K != CallbackType {
		return Callback{}, false
	}
	cb, ok := p.Value.(Callback)
	return cb, ok
}

type KindPair struct {
	K     Kind
	Value interface{}
}
