package main

import (
	"fmt"
)

type GIrMTypeAlias struct {
	Name string       `json:"tan"`
	Ref  *GIrMTypeRef `json:"tar,omitempty"`
}

type GIrMTypeDataDecl struct {
	Name  string             `json:"tdn"`
	Ctors []GIrMTypeDataCtor `json:"tdc,omitempty"`
}

type GIrMTypeDataCtor struct {
	Name string         `json:"tcn"`
	Args []*GIrMTypeRef `json:"tca,omitempty"`
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

func (me *GonadIrMeta) newTypeRefFromExtTc(tc TaggedContents) (tref *GIrMTypeRef) {
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
			Label: disc[0].(string), Left: me.newTypeRefFromExtTc(newTaggedContents(tcdis)), Right: me.newTypeRefFromExtTc(newTaggedContents(tcsub))}
	case "ForAll":
		disc := tc.Contents.([]interface{})
		tcdis := disc[1].(map[string]interface{})
		tref.ForAll = &GIrMTypeRefExist{Name: disc[0].(string), Ref: me.newTypeRefFromExtTc(newTaggedContents(tcdis))}
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
			Left: me.newTypeRefFromExtTc(newTaggedContents(tcdis)), Right: me.newTypeRefFromExtTc(newTaggedContents(tcsub))}
	case "ConstrainedType":
		disc := tc.Contents.([]interface{})
		tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
		tref.ConstrainedType = &GIrMTypeRefConstr{
			Data: tcdis["constraintData"], Ref: me.newTypeRefFromExtTc(newTaggedContents(tcsub))}
		for _, tca := range tcdis["constraintArgs"].([]interface{}) {
			tref.ConstrainedType.Args = append(tref.ConstrainedType.Args, me.newTypeRefFromExtTc(newTaggedContents(tca.(map[string]interface{}))))
		}
		for _, s := range tcdis["constraintClass"].([]interface{})[0].([]interface{}) {
			tref.ConstrainedType.Class += (s.(string) + ".")
		}
		tref.ConstrainedType.Class += tcdis["constraintClass"].([]interface{})[1].(string)
	default:
		fmt.Printf("\n%s?!\n\t%v\n", tc.Tag, tc.Contents)
	}
	return
}

func (me *GonadIrMeta) populateTypeDataDecls() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDType != nil && d.EDType.DeclKind != nil {
			if m_edTypeDeclarationKind, ok := d.EDType.DeclKind.(map[string]interface{}); ok && m_edTypeDeclarationKind != nil {
				if m_DataType, ok := m_edTypeDeclarationKind["DataType"].(map[string]interface{}); ok && m_DataType != nil {
					datadecl := GIrMTypeDataDecl{Name: d.EDType.Name}
					for _, ctorif := range m_DataType["ctors"].([]interface{}) {
						if ctorarr, ok := ctorif.([]interface{}); (!ok) || len(ctorarr) != 2 {
							panic(fmt.Errorf("%s: unexpected ctor array in %s, please report: %v", me.mod.srcFilePath, datadecl.Name, ctorif))
						} else {
							ctor := GIrMTypeDataCtor{Name: ctorarr[0].(string)}
							for _, ctorarg := range ctorarr[1].([]interface{}) {
								ctor.Args = append(ctor.Args, me.newTypeRefFromExtTc(newTaggedContents(ctorarg.(map[string]interface{}))))
							}
							datadecl.Ctors = append(datadecl.Ctors, ctor)
						}
					}
					if len(datadecl.Ctors) == 0 {
						panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, datadecl.Name, m_DataType))
					} else {
						me.TypeDataDecls = append(me.TypeDataDecls, datadecl)
					}
				}
			}
		}
	}
}

func (me *GonadIrMeta) populateTypeAliases() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.mod.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := GIrMTypeAlias{Name: d.EDTypeSynonym.Name}
			ta.Ref = me.newTypeRefFromExtTc(*d.EDTypeSynonym.Type)
			me.TypeAliases = append(me.TypeAliases, ta)
		}
	}
}

func (me *GonadIrMeta) populateGoTypeDefs() {
	mdict := map[string][]string{}
	for _, ta := range me.TypeAliases {
		tdict := map[string][]string{}
		gtd := &GIrATypeDef{Name: ta.Name}
		switch gtr := me.toGIrATypeRef(mdict, tdict, ta.Ref).(type) {
		case string:
			gtd.Alias = gtr
		case int:
			gtd.Unknown = gtr
		case *GIrATypeRefInterface:
			gtd.Interface = gtr
		case nil:
		}
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}
}

func (me *GonadIrMeta) toGIrATypeRef(mdict map[string][]string, tdict map[string][]string, tr *GIrMTypeRef) (gtr interface{}) {
	if len(tr.TypeConstructor) > 0 {
		gtr = tr.TypeConstructor
	} else if tr.REmpty {
		gtr = nil
	} else if tr.TUnknown > 0 {
		gtr = tr.TUnknown
	} else if len(tr.TypeVar) > 0 {
		gtr = &GIrATypeRefInterface{Embeds: tdict[tr.TypeVar]}
	} else if tr.ConstrainedType != nil {
		if len(tr.ConstrainedType.Args) == 0 || len(tr.ConstrainedType.Args[0].TypeVar) == 0 {
			panic(fmt.Errorf("%s: unexpected type-class/type-var association %v, please report!", me.mod.srcFilePath, tr.ConstrainedType))
		}
		tdict[tr.ConstrainedType.Args[0].TypeVar] = append(tdict[tr.ConstrainedType.Args[0].TypeVar], tr.ConstrainedType.Class)
		gtr = me.toGIrATypeRef(mdict, tdict, tr.ConstrainedType.Ref)
	}
	return
}
