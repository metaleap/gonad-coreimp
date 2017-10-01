package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
)

type GonadIrMeta struct {
	Imports     []GIrMPkgRef
	TypeAliases []GIrMTypeAlias

	imports []*ModuleInfo

	modinfo *ModuleInfo
	proj    *BowerProject
}

type GIrMPkgRef struct {
	N string
	Q string
	P string
}

type GIrMTypeAlias struct {
	Name string       `json:"n"`
	Ref  *GIrMTypeRef `json:"r,omitempty"`
}

type GIrMTypeRef struct {
	TypeConstructor string             `json:"tc,omitempty"`
	TypeVar         string             `json:"tv,omitempty"`
	REmpty          bool               `json:"re,omitempty"`
	TUnknown        int                `json:"tu,omitempty"`
	TypeApp         *gIrMTypeRefAppl   `json:"ta,omitempty"`
	ConstrainedType *gIrMTypeRefConstr `json:"ct,omitempty"`
	RCons           *gIrMTypeRefRow    `json:"rc,omitempty"`
	ForAll          *gIrMTypeRefExist  `json:"fa,omitempty"`
	Skolem          *gIrMTypeRefSkolem `json:"sk,omitempty"`
}
type gIrMTypeRefAppl struct {
	Left  *GIrMTypeRef `json:"tl,omitempty"`
	Right *GIrMTypeRef `json:"tr,omitempty"`
}
type gIrMTypeRefRow struct {
	Label string       `json:"rl,omitempty"`
	Left  *GIrMTypeRef `json:"tl,omitempty"`
	Right *GIrMTypeRef `json:"tr,omitempty"`
}
type gIrMTypeRefConstr struct {
	Class string         `json:"cc,omitempty"`
	Data  interface{}    `json:"cd,omitempty"` // when needed: Data = [[Text]] Bool
	Args  []*GIrMTypeRef `json:"ca,omitempty"`
	Ref   *GIrMTypeRef   `json:"r,omitempty"`
}
type gIrMTypeRefExist struct {
	Name        string       `json:"n,omitempty"`
	Ref         *GIrMTypeRef `json:"r,omitempty"`
	SkolemScope *int         `json:"s,omitempty"`
}
type gIrMTypeRefSkolem struct {
	Name  string `json:"n,omitempty"`
	Value int    `json:"v,omitempty"`
	Scope int    `json:"s,omitempty"`
}

func (me *GonadIrMeta) PopulateFromCoreImp() (err error) {
	//	IMPORTS
	for _, impname := range me.modinfo.coreimp.Imports {
		if impname != "Prim" && impname != "Prelude" && impname != me.modinfo.qName {
			me.imports = append(me.imports, FindModuleByQName(impname))
		}
	}

	//	TYPE ALIASES
	var resolve func(TaggedContents) *GIrMTypeRef
	var n string
	resolve = func(tc TaggedContents) (tref *GIrMTypeRef) {
		tref = &GIrMTypeRef{}
		switch tc.Tag {
		case "TypeConstructor":
			for _, s := range tc.Contents.([]interface{})[0].([]interface{}) {
				tref.TypeConstructor += (s.(string) + ".")
			}
			tref.TypeConstructor += tc.Contents.([]interface{})[1].(string)
		case "TypeVar":
			tref.TypeVar = tc.Contents.(string)
		case "TUnknown":
			tref.TUnknown = tc.Contents.(int)
		case "REmpty":
			tref.REmpty = true
		case "RCons":
			disc := tc.Contents.([]interface{})
			tcdis, tcsub := disc[1].(map[string]interface{}), disc[2].(map[string]interface{})
			tref.RCons = &gIrMTypeRefRow{
				Label: disc[0].(string), Left: resolve(newTaggedContents(tcdis)), Right: resolve(newTaggedContents(tcsub))}
		case "ForAll":
			disc := tc.Contents.([]interface{})
			tcdis := disc[1].(map[string]interface{})
			tref.ForAll = &gIrMTypeRefExist{Name: disc[0].(string), Ref: resolve(newTaggedContents(tcdis))}
			if len(disc) > 2 && disc[2] != nil {
				if i, ok := disc[2].(int); ok {
					tref.ForAll.SkolemScope = &i
				}
			}
		case "Skolem":
			disc := tc.Contents.([]interface{})
			tref.Skolem = &gIrMTypeRefSkolem{
				Name: disc[0].(string), Value: disc[1].(int), Scope: disc[2].(int)}
		case "TypeApp":
			disc := tc.Contents.([]interface{})
			tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
			tref.TypeApp = &gIrMTypeRefAppl{
				Left: resolve(newTaggedContents(tcdis)), Right: resolve(newTaggedContents(tcsub))}
		case "ConstrainedType":
			disc := tc.Contents.([]interface{})
			tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
			tref.ConstrainedType = &gIrMTypeRefConstr{
				Data: tcdis["constraintData"], Ref: resolve(newTaggedContents(tcsub))}
			for _, s := range tcdis["constraintClass"].([]interface{})[0].([]interface{}) {
				tref.ConstrainedType.Class += (s.(string) + ".")
			}
			tref.ConstrainedType.Class += tcdis["constraintClass"].([]interface{})[1].(string)
		default:
			fmt.Printf("\n%s\t%s?!\n\t%v\n", n, tc.Tag, tc.Contents)
		}
		return
	}
	for _, d := range me.modinfo.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.modinfo.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := GIrMTypeAlias{Name: d.EDTypeSynonym.Name}
			n = d.EDTypeSynonym.Name
			ta.Ref = resolve(*d.EDTypeSynonym.Type)
			me.TypeAliases = append(me.TypeAliases, ta)
		}
	}

	//	SET PUBLIC FIELDS USED FOR JSON SERIALIZATION
	if err == nil {
		for _, impmod := range me.imports {
			me.Imports = append(me.Imports, GIrMPkgRef{N: impmod.pName, Q: impmod.qName, P: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)})
		}
	}
	return
}

func (me *GonadIrMeta) PopulateFromLoaded() (err error) {
	me.imports = nil
	for _, imp := range me.Imports {
		if impmod := FindModuleByQName(imp.Q); impmod == nil {
			err = errors.New("Bad import " + imp.Q)
		} else {
			me.imports = append(me.imports, impmod)
		}
	}

	return
}

func (me *GonadIrMeta) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
