package main

import (
	"fmt"
	"strings"
)

type GIrMTypeAlias struct {
	Name string       `json:"tan"`
	Ref  *GIrMTypeRef `json:"tar,omitempty"`
}

type GIrMTypeRef struct {
	TypeConstructor string             `json:"tc,omitempty"`
	TypeVar         string             `json:"tv,omitempty"`
	REmpty          bool               `json:"re,omitempty"`
	TUnknown        int                `json:"tu,omitempty"`
	TypeApp         *GIrMTypeRefAppl   `json:"ta,omitempty"`
	ConstrainedType *GIrMTypeRefConstr `json:"ct,omitempty"`
	RCons           *GIrMTypeRefRow    `json:"rc,omitempty"`
	ForAll          *GIrMTypeRefExist  `json:"fa,omitempty"`
	Skolem          *GIrMTypeRefSkolem `json:"sk,omitempty"`
}

type GIrMTypeRefAppl struct {
	Left  *GIrMTypeRef `json:"t1,omitempty"`
	Right *GIrMTypeRef `json:"t2,omitempty"`
}
type GIrMTypeRefRow struct {
	Label string       `json:"rl,omitempty"`
	Left  *GIrMTypeRef `json:"r1,omitempty"`
	Right *GIrMTypeRef `json:"r2,omitempty"`
}
type GIrMTypeRefConstr struct {
	Class string         `json:"cc,omitempty"`
	Data  interface{}    `json:"cd,omitempty"` // when needed: Data = [[Text]] Bool
	Args  []*GIrMTypeRef `json:"ca,omitempty"`
	Ref   *GIrMTypeRef   `json:"cr,omitempty"`
}
type GIrMTypeRefExist struct {
	Name        string       `json:"en,omitempty"`
	Ref         *GIrMTypeRef `json:"er,omitempty"`
	SkolemScope *int         `json:"es,omitempty"`
}
type GIrMTypeRefSkolem struct {
	Name  string `json:"sn,omitempty"`
	Value int    `json:"sv,omitempty"`
	Scope int    `json:"ss,omitempty"`
}

func (me *GonadIrMeta) populateTypeAliases() {
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
			tref.RCons = &GIrMTypeRefRow{
				Label: disc[0].(string), Left: resolve(newTaggedContents(tcdis)), Right: resolve(newTaggedContents(tcsub))}
		case "ForAll":
			disc := tc.Contents.([]interface{})
			tcdis := disc[1].(map[string]interface{})
			tref.ForAll = &GIrMTypeRefExist{Name: disc[0].(string), Ref: resolve(newTaggedContents(tcdis))}
			if len(disc) > 2 && disc[2] != nil {
				if i, ok := disc[2].(int); ok {
					tref.ForAll.SkolemScope = &i
				}
			}
		case "Skolem":
			disc := tc.Contents.([]interface{})
			tref.Skolem = &GIrMTypeRefSkolem{
				Name: disc[0].(string), Value: disc[1].(int), Scope: disc[2].(int)}
		case "TypeApp":
			disc := tc.Contents.([]interface{})
			tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
			tref.TypeApp = &GIrMTypeRefAppl{
				Left: resolve(newTaggedContents(tcdis)), Right: resolve(newTaggedContents(tcsub))}
		case "ConstrainedType":
			disc := tc.Contents.([]interface{})
			tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
			tref.ConstrainedType = &GIrMTypeRefConstr{
				Data: tcdis["constraintData"], Ref: resolve(newTaggedContents(tcsub))}
			for _, tca := range tcdis["constraintArgs"].([]interface{}) {
				tref.ConstrainedType.Args = append(tref.ConstrainedType.Args, resolve(newTaggedContents(tca.(map[string]interface{}))))
			}
			for _, s := range tcdis["constraintClass"].([]interface{})[0].([]interface{}) {
				tref.ConstrainedType.Class += (s.(string) + ".")
			}
			tref.ConstrainedType.Class += tcdis["constraintClass"].([]interface{})[1].(string)
		default:
			fmt.Printf("\n%s\t%s?!\n\t%v\n", n, tc.Tag, tc.Contents)
		}
		return
	}
	for _, d := range me.mod.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.mod.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := GIrMTypeAlias{Name: d.EDTypeSynonym.Name}
			n = d.EDTypeSynonym.Name
			ta.Ref = resolve(*d.EDTypeSynonym.Type)
			me.TypeAliases = append(me.TypeAliases, ta)
		}
	}
}

func (me *GonadIrMeta) populateGoTypeDefs() {
	dict := map[string][]string{}
	for _, ta := range me.TypeAliases {
		gtd := &GIrATypeDef{Name: ta.Name, Ref: me.toGIrATypeRef(dict, ta.Ref)}
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}
}

func (me *GonadIrMeta) toGIrATypeRef(dict map[string][]string, tr *GIrMTypeRef) (gtr GIrATypeRef) {
	if len(tr.TypeConstructor) > 0 {
		gtr = GIrATypeRefNamed{Name: tr.TypeConstructor}
	} else if tr.REmpty {
		gtr = &GIrATypeRefVoid{}
	} else if tr.TUnknown > 0 {
		gtr = &GIrATypeRefUnknown{tr.TUnknown}
	} else if len(tr.TypeVar) > 0 {
		gtr = GIrATypeRefNamed{Name: strings.Join(dict[tr.TypeVar], "And")}
	} else if tr.ConstrainedType != nil {
		if len(tr.ConstrainedType.Args) == 0 || len(tr.ConstrainedType.Args[0].TypeVar) == 0 {
			panic(fmt.Errorf("%s: unexpected type-class/type-var association %v, please report!", me.mod.srcFilePath, tr.ConstrainedType))
		}
		dict[tr.ConstrainedType.Args[0].TypeVar] = append(dict[tr.ConstrainedType.Args[0].TypeVar], tr.ConstrainedType.Class)
		gtr = me.toGIrATypeRef(dict, tr.ConstrainedType.Ref)
	}
	return
}
