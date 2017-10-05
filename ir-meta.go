package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
)

type GonadIrMeta struct {
	Imports          GIrMPkgRefs         `json:",omitempty"`
	ExtTypeAliases   []GIrMNamedTypeRef  `json:",omitempty"`
	ExtTypeClasses   []GIrMTypeClass     `json:",omitempty"`
	ExtTypeDataDecls []GIrMTypeDataDecl  `json:",omitempty"`
	GoTypeDefs       []*GIrANamedTypeRef `json:",omitempty"`

	imports []*ModuleInfo

	mod  *ModuleInfo
	proj *BowerProject
	save bool
}

type GIrMPkgRefs []*GIrMPkgRef

func (me GIrMPkgRefs) Len() int           { return len(me) }
func (me GIrMPkgRefs) Less(i, j int) bool { return me[i].P < me[j].P }
func (me GIrMPkgRefs) Swap(i, j int)      { me[i], me[j] = me[j], me[i] }

type GIrMPkgRef struct {
	N string
	Q string
	P string

	used bool
}

func newModImp(impmod *ModuleInfo) *GIrMPkgRef {
	return &GIrMPkgRef{N: impmod.pName, Q: impmod.qName, P: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)}
}

func qNameFromExt(subArrAt0andStrAt1 []interface{}) (qname string) {
	for _, s := range subArrAt0andStrAt1[0].([]interface{}) {
		qname += (s.(string) + ".")
	}
	qname += subArrAt0andStrAt1[1].(string)
	return
}

func (me *GonadIrMeta) newTypeRefFromExtTc(tc TaggedContents) (tref *GIrMTypeRef) {
	tref = &GIrMTypeRef{}
	switch tc.Tag {
	case "TypeConstructor":
		tref.TypeConstructor = qNameFromExt(tc.Contents.([]interface{}))
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
		tref.ConstrainedType.Class = qNameFromExt(tcdis["constraintClass"].([]interface{}))
	default:
		fmt.Printf("\n%s?!\n\t%v\n", tc.Tag, tc.Contents)
	}
	return
}

func (me *GonadIrMeta) populateExtTypeDataDecls() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDType != nil && d.EDType.DeclKind != nil {
			if m_edTypeDeclarationKind, ok := d.EDType.DeclKind.(map[string]interface{}); ok && m_edTypeDeclarationKind != nil {
				if m_DataType, ok := m_edTypeDeclarationKind["DataType"].(map[string]interface{}); ok && m_DataType != nil {
					datadecl := GIrMTypeDataDecl{Name: d.EDType.Name}
					for _, argif := range m_DataType["args"].([]interface{}) {
						datadecl.Args = append(datadecl.Args, argif.([]interface{})[0].(string))
					}
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
					me.ExtTypeDataDecls = append(me.ExtTypeDataDecls, datadecl)
				}
			}
		}
	}
}

func (me *GonadIrMeta) populateExtTypeAliases() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.mod.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := GIrMNamedTypeRef{Name: d.EDTypeSynonym.Name}
			ta.Ref = me.newTypeRefFromExtTc(*d.EDTypeSynonym.Type)
			me.ExtTypeAliases = append(me.ExtTypeAliases, ta)
		}
	}
}

func (me *GonadIrMeta) populateExtTypeClasses() {
	for _, efdecl := range me.mod.ext.EfDecls {
		if edc := efdecl.EDClass; edc != nil {
			tc := GIrMTypeClass{Name: edc.Name}
			for _, edca := range edc.TypeArgs {
				tc.TypeArgs = append(tc.TypeArgs, edca[0].(string))
			}
			for _, edcc := range edc.Constraints {
				tcc := &GIrMTypeRefConstr{Class: qNameFromExt(edcc.Class), Data: edcc.Data}
				for _, edcca := range edcc.Args {
					tcc.Args = append(tcc.Args, me.newTypeRefFromExtTc(edcca))
				}
				tc.Constraints = append(tc.Constraints, tcc)
			}
			for _, edcm := range edc.Members {
				mident := edcm[0].(map[string]interface{})["Ident"].(string)
				mtc := me.newTypeRefFromExtTc(newTaggedContents(edcm[1].(map[string]interface{})))
				tc.Members = append(tc.Members, GIrMNamedTypeRef{Name: mident, Ref: mtc})
			}
			me.ExtTypeClasses = append(me.ExtTypeClasses, tc)
		}
	}
}

func (me *GonadIrMeta) PopulateFromCoreImp() (err error) {
	for _, impname := range me.mod.coreimp.Imports {
		if impname != "Prim" && impname != "Prelude" && impname != me.mod.qName {
			me.imports = append(me.imports, FindModuleByQName(impname))
		}
	}
	me.populateExtTypeAliases()
	me.populateExtTypeClasses()
	me.populateExtTypeDataDecls()
	me.populateGoTypeDefs()

	if err == nil {
		for _, impmod := range me.imports {
			me.Imports = append(me.Imports, newModImp(impmod))
		}
	}
	return
}

func (me *GonadIrMeta) PopulateFromLoaded() error {
	me.imports = nil
	for _, imp := range me.Imports {
		if impmod := FindModuleByQName(imp.Q); impmod == nil {
			return errors.New("Bad import " + imp.Q)
		} else {
			me.imports = append(me.imports, impmod)
		}
	}
	return nil
}

func (me *GonadIrMeta) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
