package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/metaleap/go-util-fs"
)

type GonadIrMeta struct {
	Imports           GIrMPkgRefs          `json:",omitempty"`
	ExtTypeAliases    []*GIrMNamedTypeRef  `json:",omitempty"`
	ExtTypeClasses    []*GIrMTypeClass     `json:",omitempty"`
	ExtTypeClassInsts []*GIrMTypeClassInst `json:",omitempty"`
	ExtTypeDataDecls  []*GIrMTypeDataDecl  `json:",omitempty"`
	ExtValDecls       []*GIrMNamedTypeRef  `json:",omitempty"`
	GoTypeDefs        GIrANamedTypeRefs    `json:",omitempty"`
	GoValDecls        GIrANamedTypeRefs    `json:",omitempty"`

	imports []*ModuleInfo

	mod  *ModuleInfo
	proj *PsBowerProject
	save bool
}

type GIrMPkgRefs []*GIrMPkgRef

func (me GIrMPkgRefs) Len() int           { return len(me) }
func (me GIrMPkgRefs) Less(i, j int) bool { return me[i].P < me[j].P }
func (me GIrMPkgRefs) Swap(i, j int)      { me[i], me[j] = me[j], me[i] }
func (me *GIrMPkgRefs) AddIfHasnt(lname, imppath, qname string) {
	if !me.Has(imppath) {
		*me = append(*me, &GIrMPkgRef{used: true, N: lname, P: imppath, Q: qname})
	}

}
func (me GIrMPkgRefs) Has(imppath string) bool {
	for _, imp := range me {
		if imp.P == imppath {
			return true
		}
	}
	return false
}

type GIrMPkgRef struct {
	N string
	Q string
	P string

	used bool
}

type GIrMNamedTypeRef struct {
	Name string       `json:"tnn"`
	Ref  *GIrMTypeRef `json:"tnr,omitempty"`
}

type GIrMTypeClass struct {
	Name        string               `json:"tcn"`
	Constraints []*GIrMTypeRefConstr `json:"tcc,omitempty"`
	TypeArgs    []string             `json:"tca,omitempty"`
	Members     []GIrMNamedTypeRef   `json:"tcm,omitempty"`
}

type GIrMTypeClassInst struct {
	Name        string               `json:"tcin"`
	ClassName   string               `json:"tcicn,omitempty"`
	Constraints []*GIrMTypeRefConstr `json:"tcic,omitempty"`
	Types       GIrMTypeRefs         `json:"tcit,omitempty"`
	Chain       []string             `json:"tcich,omitempty"`
	ChainIndex  int                  `json:"tcichi,omitempty"`
}

type GIrMTypeDataDecl struct {
	Name  string              `json:"tdn"`
	Ctors []*GIrMTypeDataCtor `json:"tdc,omitempty"`
	Args  []string            `json:"tda,omitempty"`
}

type GIrMTypeDataCtor struct {
	Name string       `json:"tdcn"`
	Args GIrMTypeRefs `json:"tdca,omitempty"`

	gtd     *GIrANamedTypeRef
	comment *GIrAComments
}

type GIrMTypeRefs []*GIrMTypeRef

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

	tmp_assoc *GIrANamedTypeRef
}

func (me *GIrMTypeRef) Eq(cmp *GIrMTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.TypeConstructor == cmp.TypeConstructor && me.TypeVar == cmp.TypeVar && me.REmpty == cmp.REmpty && me.TUnknown == cmp.TUnknown && me.TypeApp.Eq(cmp.TypeApp) && me.ConstrainedType.Eq(cmp.ConstrainedType) && me.RCons.Eq(cmp.RCons) && me.ForAll.Eq(cmp.ForAll) && me.Skolem.Eq(cmp.Skolem))
}

func (me GIrMTypeRefs) Eq(cmp GIrMTypeRefs) bool {
	if len(me) != len(cmp) {
		return false
	}
	for i, _ := range me {
		if !me[i].Eq(cmp[i]) {
			return false
		}
	}
	return true
}

type GIrMTypeRefAppl struct {
	Left  *GIrMTypeRef `json:"t1,omitempty"`
	Right *GIrMTypeRef `json:"t2,omitempty"`
}

func (me *GIrMTypeRefAppl) Eq(cmp *GIrMTypeRefAppl) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Left.Eq(cmp.Left) && me.Right.Eq(cmp.Right))
}

type GIrMTypeRefRow struct {
	Label string       `json:"rl,omitempty"`
	Left  *GIrMTypeRef `json:"r1,omitempty"`
	Right *GIrMTypeRef `json:"r2,omitempty"`
}

func (me *GIrMTypeRefRow) Eq(cmp *GIrMTypeRefRow) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Label == cmp.Label && me.Left.Eq(cmp.Left) && me.Right.Eq(cmp.Right))
}

type GIrMTypeRefConstr struct {
	Class string       `json:"cc,omitempty"`
	Data  interface{}  `json:"cd,omitempty"` // when needed: Data = [[Text]] Bool
	Args  GIrMTypeRefs `json:"ca,omitempty"`
	Ref   *GIrMTypeRef `json:"cr,omitempty"`
}

func (me *GIrMTypeRefConstr) Eq(cmp *GIrMTypeRefConstr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Class == cmp.Class && me.Data == cmp.Data && me.Ref.Eq(cmp.Ref) && me.Args.Eq(cmp.Args))
}

type GIrMTypeRefExist struct {
	Name        string       `json:"en,omitempty"`
	Ref         *GIrMTypeRef `json:"er,omitempty"`
	SkolemScope *int         `json:"es,omitempty"`
}

func (me *GIrMTypeRefExist) Eq(cmp *GIrMTypeRefExist) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Ref.Eq(cmp.Ref) && me.SkolemScope == cmp.SkolemScope)
}

type GIrMTypeRefSkolem struct {
	Name  string `json:"sn,omitempty"`
	Value int    `json:"sv,omitempty"`
	Scope int    `json:"ss,omitempty"`
}

func (me *GIrMTypeRefSkolem) Eq(cmp *GIrMTypeRefSkolem) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Value == cmp.Value && me.Scope == cmp.Scope)
}

func newModImp(impmod *ModuleInfo) *GIrMPkgRef {
	return &GIrMPkgRef{N: impmod.pName, Q: impmod.qName, P: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)}
}

func qNameFromExt(subArrAt0andStrAt1 []interface{}) (qname string) {
	for _, s := range subArrAt0andStrAt1[0].([]interface{}) {
		qname += (s.(string) + ".")
	}
	switch x := subArrAt0andStrAt1[1].(type) {
	case string:
		qname += x
	case map[string]string:
		qname += x["Ident"]
	case map[string]interface{}:
		qname += x["Ident"].(string)
	}
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

func (me *GonadIrMeta) populateExtFuncsAndVals() {
	for _, efdecl := range me.mod.ext.EfDecls {
		if edval := efdecl.EDValue; edval != nil {
			referstotypeclassmember := false
			for _, etc := range me.ExtTypeClasses {
				for _, etcm := range etc.Members {
					if etcm.Name == edval.Name.Ident {
						referstotypeclassmember = true
						break
					}
				}
				if referstotypeclassmember {
					break
				}
			}
			if !referstotypeclassmember {
				tr := me.newTypeRefFromExtTc(edval.Type)
				me.ExtValDecls = append(me.ExtValDecls, &GIrMNamedTypeRef{Name: edval.Name.Ident, Ref: tr})
			}
		}
	}
}

func (me *GonadIrMeta) populateExtTypeDataDecls() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDType != nil && d.EDType.DeclKind != nil {
			if m_edTypeDeclarationKind, ok := d.EDType.DeclKind.(map[string]interface{}); ok && m_edTypeDeclarationKind != nil {
				if m_DataType, ok := m_edTypeDeclarationKind["DataType"].(map[string]interface{}); ok && m_DataType != nil {
					datadecl := &GIrMTypeDataDecl{Name: d.EDType.Name}
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
							datadecl.Ctors = append(datadecl.Ctors, &ctor)
						}
					}
					me.ExtTypeDataDecls = append(me.ExtTypeDataDecls, datadecl)
				}
			} else if s_edTypeDeclarationKind, ok := d.EDType.DeclKind.(string); ok {
				switch s_edTypeDeclarationKind {
				case "TypeSynonym":
				//	type-aliases handled separately in populateExtTypeAliases already, nothing to do here
				case "ExternData":
					if ufs.FileExists(me.mod.srcFilePath[:len(me.mod.srcFilePath)-len(".purs")] + ".go") {
						//	type will be present to go build at compilation time --- the typical case
					} else {
						//	special case for official purescript core libs: alias to gonad's default ffi package
						ta := &GIrMNamedTypeRef{Name: d.EDType.Name, Ref: &GIrMTypeRef{TypeConstructor: nsPrefixDefaultFfiPkg + me.mod.qName + "." + d.EDType.Name}}
						me.ExtTypeAliases = append(me.ExtTypeAliases, ta)
					}
				default:
					panic(me.mod.extFilePath + ": unrecognized edTypeDeclarationKind value of '" + s_edTypeDeclarationKind + "', please report!")
				}
			}
		}
	}
}

func (me *GonadIrMeta) populateExtTypeAliases() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.mod.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := &GIrMNamedTypeRef{Name: d.EDTypeSynonym.Name}
			ta.Ref = me.newTypeRefFromExtTc(*d.EDTypeSynonym.Type)
			me.ExtTypeAliases = append(me.ExtTypeAliases, ta)
		}
	}
}

func (me *GonadIrMeta) populateExtTypeClasses() {
	populateconstraints := func(xconstrs []PsExtConstr) (mconstrs []*GIrMTypeRefConstr) {
		for _, constr := range xconstrs {
			trc := &GIrMTypeRefConstr{Class: qNameFromExt(constr.Class), Data: constr.Data}
			for _, carg := range constr.Args {
				trc.Args = append(trc.Args, me.newTypeRefFromExtTc(carg))
			}
			mconstrs = append(mconstrs, trc)
		}
		return
	}
	for _, efdecl := range me.mod.ext.EfDecls {
		if edc := efdecl.EDClass; edc != nil {
			tc := &GIrMTypeClass{Name: edc.Name}
			for _, edca := range edc.TypeArgs {
				tc.TypeArgs = append(tc.TypeArgs, edca[0].(string))
			}
			tc.Constraints = populateconstraints(edc.Constraints)
			for _, edcm := range edc.Members {
				mident := edcm[0].(map[string]interface{})["Ident"].(string)
				mtc := me.newTypeRefFromExtTc(newTaggedContents(edcm[1].(map[string]interface{})))
				tc.Members = append(tc.Members, GIrMNamedTypeRef{Name: mident, Ref: mtc})
			}
			me.ExtTypeClasses = append(me.ExtTypeClasses, tc)
		}
		if edi := efdecl.EDInstance; edi != nil {
			tci := &GIrMTypeClassInst{Name: edi.Name.Ident, ClassName: qNameFromExt(edi.ClassName), ChainIndex: edi.ChainIndex}
			for _, tc := range edi.Types {
				tci.Types = append(tci.Types, me.newTypeRefFromExtTc(tc))
			}
			tci.Constraints = populateconstraints(edi.Constraints)
			for _, nametuples := range edi.Chain {
				tci.Chain = append(tci.Chain, qNameFromExt(nametuples))
			}
			me.ExtTypeClassInsts = append(me.ExtTypeClassInsts, tci)
		}
	}
}

func (me *GonadIrMeta) PopulateFromCoreImp() {
	for _, impname := range me.mod.coreimp.Imports {
		if impname != "Prim" && impname != "Prelude" && impname != me.mod.qName {
			me.imports = append(me.imports, FindModuleByQName(impname))
		}
	}
	me.populateExtTypeAliases()
	me.populateExtTypeClasses()
	me.populateExtTypeDataDecls()
	me.populateExtFuncsAndVals()
	me.populateGoTypeDefs()
	me.populateGoValDecls()

	for _, impmod := range me.imports {
		me.Imports = append(me.Imports, newModImp(impmod))
	}
	return
}

func (me *GonadIrMeta) PopulateFromLoaded() error {
	me.imports = nil
	for _, imp := range me.Imports {
		if !strings.HasPrefix(imp.Q, nsPrefixDefaultFfiPkg) {
			if impmod := FindModuleByQName(imp.Q); impmod == nil {
				return errors.New("Bad import " + imp.Q)
			} else {
				me.imports = append(me.imports, impmod)
			}
		}
	}
	return nil
}

func (me *GonadIrMeta) populateGoValDecls() {
	mdict, m := map[string][]string{}, map[string]bool{}
	var tdict map[string][]string

	for _, evd := range me.ExtValDecls {
		tdict = map[string][]string{}
		gvd := &GIrANamedTypeRef{Export: true}
		gvd.setBothNamesFromPsName(evd.Name)
		for true {
			_, funcexists := m[gvd.NameGo]
			if gtd := me.GoTypeDefByGoName(gvd.NameGo); funcexists || gtd != nil {
				gvd.NameGo += "ˇ"
			} else {
				break
			}
		}
		m[gvd.NameGo] = true
		gvd.setRefFrom(me.toGIrATypeRef(mdict, tdict, evd.Ref))
		me.GoValDecls = append(me.GoValDecls, gvd)
	}
}

func (me *GonadIrMeta) GoValDeclByGoName(goname string) *GIrANamedTypeRef {
	for _, gvd := range me.GoValDecls {
		if gvd.NameGo == goname {
			return gvd
		}
	}
	return nil
}

func (me *GonadIrMeta) GoValDeclByPsName(psname string) *GIrANamedTypeRef {
	for _, gvd := range me.GoValDecls {
		if gvd.NamePs == psname {
			return gvd
		}
	}
	return nil
}

func (me *GonadIrMeta) GoTypeDefByGoName(goname string) *GIrANamedTypeRef {
	for _, gtd := range me.GoTypeDefs {
		if gtd.NameGo == goname {
			return gtd
		}
	}
	return nil
}

func (me *GonadIrMeta) GoTypeDefByPsName(psname string) *GIrANamedTypeRef {
	for _, gtd := range me.GoTypeDefs {
		if gtd.NamePs == psname {
			return gtd
		}
	}
	return nil
}

func (me *GonadIrMeta) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
