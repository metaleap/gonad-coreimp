package main

import (
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/metaleap/go-util-fs"
)

/*
Essentially the intermediate representation that we
place as gonadmeta.json next to the purs compiler's
outputs (coreimp.json and externs.json).

This is so all that info can be looked up when the
module/package doesn't need to be re-generated but
is referred to from one that does.

Represents "top-level declarations" (type-defs, plus
top-level consts/vars and funcs) both as the original
PureScript defs and the Golang equivalents.

Somehow it evolved that the former have names prefixed
with GIrM (meta) and the latter with GIrA (AST). Both
are held in the GonadIrMeta struct / gonadmeta.json.
(The former defined throughout ir-meta.go and the latter
mostly in ir-typestuff.go.)

This is all synthesized from the raw-JSON representations
we first load into ps-coreimp-*.go structures, but those
are unwieldy to operate on directly, hence we form this
sanitized "intermediate representation". When it's later
looked-up as another module/package is regenerated, the
format can be readily-deserialized without needing to
reprocess/reinterpret the original raw source coreimp.
*/

type GonadIrMeta struct {
	Exports           []string             `json:",omitempty"`
	Imports           GIrMPkgRefs          `json:",omitempty"`
	EnvTypeSyns       []*GIrMNamedTypeRef  `json:",omitempty"`
	EnvTypeClasses    []*GIrMTypeClass     `json:",omitempty"`
	EnvTypeClassInsts []*GIrMTypeClassInst `json:",omitempty"`
	EnvTypeDataDecls  []*GIrMTypeDataDecl  `json:",omitempty"`
	EnvValDecls       []*GIrMNamedTypeRef  `json:",omitempty"`
	GoTypeDefs        GIrANamedTypeRefs    `json:",omitempty"`
	GoValDecls        GIrANamedTypeRefs    `json:",omitempty"`
	ForeignImp        *GIrMPkgRef          `json:",omitempty"`

	imports []*ModuleInfo

	mod  *ModuleInfo
	proj *PsBowerProject
	save bool
}

type GIrMPkgRefs []*GIrMPkgRef

func (me GIrMPkgRefs) Len() int           { return len(me) }
func (me GIrMPkgRefs) Less(i, j int) bool { return me[i].P < me[j].P }
func (me GIrMPkgRefs) Swap(i, j int)      { me[i], me[j] = me[j], me[i] }

func (me *GIrMPkgRefs) AddIfHasnt(lname, imppath, qname string) (pkgref *GIrMPkgRef) {
	if pkgref = me.ByImpPath(imppath); pkgref == nil {
		pkgref = &GIrMPkgRef{used: true, N: lname, P: imppath, Q: qname}
		*me = append(*me, pkgref)
	}
	return
}

func (me GIrMPkgRefs) ByImpPath(imppath string) *GIrMPkgRef {
	for _, imp := range me {
		if imp.P == imppath {
			return imp
		}
	}
	return nil
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
	Args        []string             `json:"tca,omitempty"`
	Constraints []*GIrMTypeRefConstr `json:"tcc,omitempty"`
	Members     []GIrMNamedTypeRef   `json:"tcm,omitempty"`
}

type GIrMTypeClassInst struct {
	Name      string       `json:"tcin"`
	ClassName string       `json:"tcicn,omitempty"`
	InstTypes GIrMTypeRefs `json:"tcit,omitempty"`
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
	TypeApp         *GIrMTypeRefAppl   `json:"ta,omitempty"`
	ConstrainedType *GIrMTypeRefConstr `json:"ct,omitempty"`
	RCons           *GIrMTypeRefRow    `json:"rc,omitempty"`
	ForAll          *GIrMTypeRefExist  `json:"fa,omitempty"`
	Skolem          *GIrMTypeRefSkolem `json:"sk,omitempty"`

	tmp_assoc *GIrANamedTypeRef
}

func (me *GIrMTypeRef) Eq(cmp *GIrMTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.TypeConstructor == cmp.TypeConstructor && me.TypeVar == cmp.TypeVar && me.REmpty == cmp.REmpty && me.TypeApp.Eq(cmp.TypeApp) && me.ConstrainedType.Eq(cmp.ConstrainedType) && me.RCons.Eq(cmp.RCons) && me.ForAll.Eq(cmp.ForAll) && me.Skolem.Eq(cmp.Skolem))
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
	Args  GIrMTypeRefs `json:"ca,omitempty"`
	Ref   *GIrMTypeRef `json:"cr,omitempty"`
}

func (me *GIrMTypeRefConstr) Eq(cmp *GIrMTypeRefConstr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Class == cmp.Class && me.Ref.Eq(cmp.Ref) && me.Args.Eq(cmp.Args))
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

func (me *GonadIrMeta) newTypeRefFromEnvTag(tc *CoreImpEnvTagType) (tref *GIrMTypeRef) {
	tref = &GIrMTypeRef{}
	if tc.isTypeConstructor() {
		tref.TypeConstructor = tc.text
	} else if tc.isTypeVar() {
		tref.TypeVar = tc.text
	} else if tc.isREmpty() {
		tref.REmpty = true
	} else if tc.isRCons() {
		tref.RCons = &GIrMTypeRefRow{
			Label: tc.text, Left: me.newTypeRefFromEnvTag(tc.type0), Right: me.newTypeRefFromEnvTag(tc.type1)}
	} else if tc.isForAll() {
		tref.ForAll = &GIrMTypeRefExist{Name: tc.text, Ref: me.newTypeRefFromEnvTag(tc.type0)}
		if tc.skolem >= 0 {
			tref.ForAll.SkolemScope = &tc.skolem
		}
	} else if tc.isSkolem() {
		tref.Skolem = &GIrMTypeRefSkolem{Name: tc.text, Value: tc.num, Scope: tc.skolem}
	} else if tc.isTypeApp() {
		tref.TypeApp = &GIrMTypeRefAppl{Left: me.newTypeRefFromEnvTag(tc.type0), Right: me.newTypeRefFromEnvTag(tc.type1)}
	} else if tc.isConstrainedType() {
		tref.ConstrainedType = &GIrMTypeRefConstr{Ref: me.newTypeRefFromEnvTag(tc.type0), Class: tc.constr.Class}
		for _, tca := range tc.constr.Args {
			tref.ConstrainedType.Args = append(tref.ConstrainedType.Args, me.newTypeRefFromEnvTag(tca))
		}
	} else {
		panic(coreImpEnvErr("tagged-type", tc.Tag))
	}
	return
}

func (me *GonadIrMeta) populateEnvFuncsAndVals() {
	for fname, fdef := range me.mod.coreimp.DeclEnv.Functions {
		me.EnvValDecls = append(me.EnvValDecls, &GIrMNamedTypeRef{Name: fname, Ref: me.newTypeRefFromEnvTag(fdef.Type)})
	}
}

func (me *GonadIrMeta) populateEnvTypeDataDecls() {
	for tdefname, tdef := range me.mod.coreimp.DeclEnv.TypeDefs {
		if tdef.Decl.TypeSynonym {
			//	type-aliases handled separately in populateEnvTypeSyns already, nothing to do here
		} else if tdef.Decl.ExternData {
			if ffigofilepath := me.mod.srcFilePath[:len(me.mod.srcFilePath)-len(".purs")] + ".go"; ufs.FileExists(ffigofilepath) {
				panic("Time to handle FFI " + ffigofilepath)
			} else {
				//	special case for official purescript core libs: alias to applicable struct from gonad's default ffi packages
				ta := &GIrMNamedTypeRef{Name: tdefname, Ref: &GIrMTypeRef{TypeConstructor: nsPrefixDefaultFfiPkg + me.mod.qName + "." + tdefname}}
				me.EnvTypeSyns = append(me.EnvTypeSyns, ta)
			}
		} else {
			dt := &GIrMTypeDataDecl{Name: tdefname}
			for dtargname, _ := range tdef.Decl.DataType.Args {
				dt.Args = append(dt.Args, dtargname)
			}
			for dcname, dcargtypes := range tdef.Decl.DataType.Ctors {
				dtc := &GIrMTypeDataCtor{Name: dcname}
				for _, dcargtype := range dcargtypes {
					dtc.Args = append(dtc.Args, me.newTypeRefFromEnvTag(dcargtype))
				}
				dt.Ctors = append(dt.Ctors, dtc)
			}
			me.EnvTypeDataDecls = append(me.EnvTypeDataDecls, dt)
		}
	}
}

func (me *GonadIrMeta) populateEnvTypeSyns() {
	for tsname, tsdef := range me.mod.coreimp.DeclEnv.TypeSyns {
		if _, istc := me.mod.coreimp.DeclEnv.Classes[tsname]; !istc {
			ts := &GIrMNamedTypeRef{Name: tsname}
			ts.Ref = me.newTypeRefFromEnvTag(tsdef.Type)
			me.EnvTypeSyns = append(me.EnvTypeSyns, ts)
		}
	}
}

func (me *GonadIrMeta) populateEnvTypeClasses() {
	for tcname, tcdef := range me.mod.coreimp.DeclEnv.Classes {
		tc := &GIrMTypeClass{Name: tcname}
		for tcarg, _ := range tcdef.Args {
			tc.Args = append(tc.Args, tcarg)
		}
		for tcmname, tcmdef := range tcdef.Members {
			tref := me.newTypeRefFromEnvTag(tcmdef)
			tc.Members = append(tc.Members, GIrMNamedTypeRef{Name: tcmname, Ref: tref})
		}
		for _, tcsc := range tcdef.Superclasses {
			c := &GIrMTypeRefConstr{Class: tcsc.Class}
			for _, tcsca := range tcsc.Args {
				c.Args = append(c.Args, me.newTypeRefFromEnvTag(tcsca))
			}
			tc.Constraints = append(tc.Constraints, c)
		}
		me.EnvTypeClasses = append(me.EnvTypeClasses, tc)
	}
	for _, m := range me.mod.coreimp.DeclEnv.ClassDicts {
		for tciclass, tcinsts := range m {
			for tciname, tcidef := range tcinsts {
				tci := &GIrMTypeClassInst{Name: tciname, ClassName: tciclass}
				for _, tcit := range tcidef.InstanceTypes {
					tci.InstTypes = append(tci.InstTypes, me.newTypeRefFromEnvTag(tcit))
				}
				me.EnvTypeClassInsts = append(me.EnvTypeClassInsts, tci)
			}
		}
	}
}

func (me *GonadIrMeta) PopulateFromCoreImp() {
	me.mod.coreimp.prep()
	for _, exp := range me.mod.ext.EfExports {
		if len(exp.TypeRef) > 1 {
			me.Exports = append(me.Exports, exp.TypeRef[1].(string))
		} else if len(exp.TypeClassRef) > 1 {
			me.Exports = append(me.Exports, exp.TypeClassRef[1].(string))
		} else if len(exp.ValueRef) > 1 {
			me.Exports = append(me.Exports, exp.ValueRef[1].(map[string]interface{})["Ident"].(string))
		} else if len(exp.TypeInstanceRef) > 1 {
			me.Exports = append(me.Exports, exp.TypeInstanceRef[1].(map[string]interface{})["Ident"].(string))
		}
	}
	for _, imp := range me.mod.coreimp.Imps {
		if impname := strings.Join(imp, "."); impname != "Prim" && impname != "Prelude" && impname != me.mod.qName {
			me.imports = append(me.imports, FindModuleByQName(impname))
		}
	}
	me.populateEnvTypeSyns()
	me.populateEnvTypeClasses()
	me.populateEnvTypeDataDecls()
	me.populateEnvFuncsAndVals()
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

	for _, evd := range me.EnvValDecls {
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
