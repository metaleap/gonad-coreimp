package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/metaleap/go-util-fs"
	"github.com/metaleap/go-util-slice"
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
with gIrM (meta) and the latter with gIrA (AST). Both
are held in the gonadIrMeta struct / gonadmeta.json.
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

type gonadIrMeta struct {
	Exports           []string             `json:",omitempty"`
	Imports           gIrMPkgRefs          `json:",omitempty"`
	EnvTypeSyns       []*gIrMNamedTypeRef  `json:",omitempty"`
	EnvTypeClasses    []*gIrMTypeClass     `json:",omitempty"`
	EnvTypeClassInsts []*gIrMTypeClassInst `json:",omitempty"`
	EnvTypeDataDecls  []*gIrMTypeDataDecl  `json:",omitempty"`
	EnvValDecls       []*gIrMNamedTypeRef  `json:",omitempty"`
	GoTypeDefs        gIrANamedTypeRefs    `json:",omitempty"`
	GoValDecls        gIrANamedTypeRefs    `json:",omitempty"`
	ForeignImp        *gIrMPkgRef          `json:",omitempty"`

	imports []*modPkg

	mod  *modPkg
	proj *psBowerProject
	save bool
}

type gIrMPkgRefs []*gIrMPkgRef

func (me gIrMPkgRefs) Len() int           { return len(me) }
func (me gIrMPkgRefs) Less(i, j int) bool { return me[i].P < me[j].P }
func (me gIrMPkgRefs) Swap(i, j int)      { me[i], me[j] = me[j], me[i] }

func (me *gIrMPkgRefs) addIfHasnt(lname, imppath, qname string) (pkgref *gIrMPkgRef) {
	if pkgref = me.byImpPath(imppath); pkgref == nil {
		pkgref = &gIrMPkgRef{N: lname, P: imppath, Q: qname}
		*me = append(*me, pkgref)
	}
	return
}

func (me gIrMPkgRefs) byImpPath(imppath string) *gIrMPkgRef {
	for _, imp := range me {
		if imp.P == imppath {
			return imp
		}
	}
	return nil
}

func (me gIrMPkgRefs) byImpName(pkgname string) *gIrMPkgRef {
	for _, imp := range me {
		if imp.N == pkgname || (imp.N == "" && imp.P == pkgname) {
			return imp
		}
	}
	return nil
}

type gIrMPkgRef struct {
	N string
	Q string
	P string

	emitted bool
}

type gIrMNamedTypeRef struct {
	Name string       `json:"tnn,omitempty"`
	Ref  *gIrMTypeRef `json:"tnr,omitempty"`
}

type gIrMTypeClass struct {
	Name        string               `json:"tcn,omitempty"`
	Args        []string             `json:"tca,omitempty"`
	Constraints []*gIrMTypeRefConstr `json:"tcc,omitempty"`
	Members     []*gIrMNamedTypeRef  `json:"tcm,omitempty"`
}

type gIrMTypeClassInst struct {
	Name      string       `json:"tcin,omitempty"`
	ClassName string       `json:"tcicn,omitempty"`
	InstTypes gIrMTypeRefs `json:"tcit,omitempty"`
}

type gIrMTypeDataDecl struct {
	Name  string              `json:"tdn,omitempty"`
	Ctors []*gIrMTypeDataCtor `json:"tdc,omitempty"`
	Args  []string            `json:"tda,omitempty"`
}

type gIrMTypeDataCtor struct {
	Name string       `json:"tdcn,omitempty"`
	Args gIrMTypeRefs `json:"tdca,omitempty"`

	gtd *gIrANamedTypeRef
}

type gIrMTypeRefs []*gIrMTypeRef

type gIrMTypeRef struct {
	TypeConstructor string             `json:"tc,omitempty"`
	TypeVar         string             `json:"tv,omitempty"`
	REmpty          bool               `json:"re,omitempty"`
	TypeApp         *gIrMTypeRefAppl   `json:"ta,omitempty"`
	ConstrainedType *gIrMTypeRefConstr `json:"ct,omitempty"`
	RCons           *gIrMTypeRefRow    `json:"rc,omitempty"`
	ForAll          *gIrMTypeRefExist  `json:"fa,omitempty"`
	Skolem          *gIrMTypeRefSkolem `json:"sk,omitempty"`

	tmp_assoc *gIrANamedTypeRef
}

func (me *gIrMTypeRef) eq(cmp *gIrMTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.TypeConstructor == cmp.TypeConstructor && me.TypeVar == cmp.TypeVar && me.REmpty == cmp.REmpty && me.TypeApp.eq(cmp.TypeApp) && me.ConstrainedType.eq(cmp.ConstrainedType) && me.RCons.eq(cmp.RCons) && me.ForAll.eq(cmp.ForAll) && me.Skolem.eq(cmp.Skolem))
}

func (me gIrMTypeRefs) eq(cmp gIrMTypeRefs) bool {
	if len(me) != len(cmp) {
		return false
	}
	for i, _ := range me {
		if !me[i].eq(cmp[i]) {
			return false
		}
	}
	return true
}

type gIrMTypeRefAppl struct {
	Left  *gIrMTypeRef `json:"t1,omitempty"`
	Right *gIrMTypeRef `json:"t2,omitempty"`
}

func (me *gIrMTypeRefAppl) eq(cmp *gIrMTypeRefAppl) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Left.eq(cmp.Left) && me.Right.eq(cmp.Right))
}

type gIrMTypeRefRow struct {
	Label string       `json:"rl,omitempty"`
	Left  *gIrMTypeRef `json:"r1,omitempty"`
	Right *gIrMTypeRef `json:"r2,omitempty"`
}

func (me *gIrMTypeRefRow) eq(cmp *gIrMTypeRefRow) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Label == cmp.Label && me.Left.eq(cmp.Left) && me.Right.eq(cmp.Right))
}

type gIrMTypeRefConstr struct {
	Class string       `json:"cc,omitempty"`
	Args  gIrMTypeRefs `json:"ca,omitempty"`
	Ref   *gIrMTypeRef `json:"cr,omitempty"`
}

func (me *gIrMTypeRefConstr) eq(cmp *gIrMTypeRefConstr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Class == cmp.Class && me.Ref.eq(cmp.Ref) && me.Args.eq(cmp.Args))
}

type gIrMTypeRefExist struct {
	Name        string       `json:"en,omitempty"`
	Ref         *gIrMTypeRef `json:"er,omitempty"`
	SkolemScope *int         `json:"es,omitempty"`
}

func (me *gIrMTypeRefExist) eq(cmp *gIrMTypeRefExist) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Ref.eq(cmp.Ref) && me.SkolemScope == cmp.SkolemScope)
}

type gIrMTypeRefSkolem struct {
	Name  string `json:"sn,omitempty"`
	Value int    `json:"sv,omitempty"`
	Scope int    `json:"ss,omitempty"`
}

func (me *gIrMTypeRefSkolem) eq(cmp *gIrMTypeRefSkolem) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Value == cmp.Value && me.Scope == cmp.Scope)
}

func newModImp(impmod *modPkg) *gIrMPkgRef {
	return &gIrMPkgRef{N: impmod.pName, Q: impmod.qName, P: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)}
}

func (me *gonadIrMeta) hasExport(name string) bool {
	return uslice.StrHas(me.Exports, name)
}

func (me *gonadIrMeta) tcMember(name string) *gIrMNamedTypeRef {
	for _, tc := range me.EnvTypeClasses {
		for _, tcm := range tc.Members {
			if tcm.Name == name {
				return tcm
			}
		}
	}
	return nil
}

func (me *gonadIrMeta) tcInst(name string) *gIrMTypeClassInst {
	for _, tci := range me.EnvTypeClassInsts {
		if tci.Name == name {
			return tci
		}
	}
	return nil
}

func (me *gonadIrMeta) newTypeRefFromEnvTag(tc *coreImpEnvTagType) (tref *gIrMTypeRef) {
	tref = &gIrMTypeRef{}
	if tc.isTypeConstructor() {
		tref.TypeConstructor = tc.text
	} else if tc.isTypeVar() {
		tref.TypeVar = tc.text
	} else if tc.isREmpty() {
		tref.REmpty = true
	} else if tc.isRCons() {
		tref.RCons = &gIrMTypeRefRow{
			Label: tc.text, Left: me.newTypeRefFromEnvTag(tc.type0), Right: me.newTypeRefFromEnvTag(tc.type1)}
	} else if tc.isForAll() {
		tref.ForAll = &gIrMTypeRefExist{Name: tc.text, Ref: me.newTypeRefFromEnvTag(tc.type0)}
		if tc.skolem >= 0 {
			tref.ForAll.SkolemScope = &tc.skolem
		}
	} else if tc.isSkolem() {
		tref.Skolem = &gIrMTypeRefSkolem{Name: tc.text, Value: tc.num, Scope: tc.skolem}
	} else if tc.isTypeApp() {
		tref.TypeApp = &gIrMTypeRefAppl{Left: me.newTypeRefFromEnvTag(tc.type0), Right: me.newTypeRefFromEnvTag(tc.type1)}
	} else if tc.isConstrainedType() {
		tref.ConstrainedType = &gIrMTypeRefConstr{Ref: me.newTypeRefFromEnvTag(tc.type0), Class: tc.constr.Class}
		for _, tca := range tc.constr.Args {
			tref.ConstrainedType.Args = append(tref.ConstrainedType.Args, me.newTypeRefFromEnvTag(tca))
		}
	} else {
		panic(notImplErr("tagged-type", tc.Tag, me.mod.srcFilePath))
	}
	return
}

func (me *gonadIrMeta) populateEnvFuncsAndVals() {
	for fname, fdef := range me.mod.coreimp.DeclEnv.Functions {
		me.EnvValDecls = append(me.EnvValDecls, &gIrMNamedTypeRef{Name: fname, Ref: me.newTypeRefFromEnvTag(fdef.Type)})
	}
}

func (me *gonadIrMeta) populateEnvTypeDataDecls() {
	for tdefname, tdef := range me.mod.coreimp.DeclEnv.TypeDefs {
		if tdef.Decl.TypeSynonym {
			//	type-aliases handled separately in populateEnvTypeSyns already, nothing to do here
		} else if tdef.Decl.ExternData {
			if ffigofilepath := me.mod.srcFilePath[:len(me.mod.srcFilePath)-len(".purs")] + ".go"; ufs.FileExists(ffigofilepath) {
				panic(me.mod.srcFilePath + ": time to handle FFI " + ffigofilepath)
			} else {
				//	special case for official purescript core libs: alias to applicable struct from gonad's default ffi packages
				ta := &gIrMNamedTypeRef{Name: tdefname, Ref: &gIrMTypeRef{TypeConstructor: nsPrefixDefaultFfiPkg + me.mod.qName + "." + tdefname}}
				me.EnvTypeSyns = append(me.EnvTypeSyns, ta)
			}
		} else {
			dt := &gIrMTypeDataDecl{Name: tdefname}
			for dtargname, _ := range tdef.Decl.DataType.Args {
				dt.Args = append(dt.Args, dtargname)
			}
			for dcname, dcargtypes := range tdef.Decl.DataType.Ctors {
				dtc := &gIrMTypeDataCtor{Name: dcname}
				for _, dcargtype := range dcargtypes {
					dtc.Args = append(dtc.Args, me.newTypeRefFromEnvTag(dcargtype))
				}
				dt.Ctors = append(dt.Ctors, dtc)
			}
			me.EnvTypeDataDecls = append(me.EnvTypeDataDecls, dt)
		}
	}
}

func (me *gonadIrMeta) populateEnvTypeSyns() {
	for tsname, tsdef := range me.mod.coreimp.DeclEnv.TypeSyns {
		if _, istc := me.mod.coreimp.DeclEnv.Classes[tsname]; !istc {
			ts := &gIrMNamedTypeRef{Name: tsname}
			ts.Ref = me.newTypeRefFromEnvTag(tsdef.Type)
			me.EnvTypeSyns = append(me.EnvTypeSyns, ts)
		}
	}
}

func (me *gonadIrMeta) populateEnvTypeClasses() {
	for tcname, tcdef := range me.mod.coreimp.DeclEnv.Classes {
		tc := &gIrMTypeClass{Name: tcname}
		for tcarg, _ := range tcdef.Args {
			tc.Args = append(tc.Args, tcarg)
		}
		for tcmname, tcmdef := range tcdef.Members {
			tref := me.newTypeRefFromEnvTag(tcmdef)
			tc.Members = append(tc.Members, &gIrMNamedTypeRef{Name: tcmname, Ref: tref})
		}
		for _, tcsc := range tcdef.Superclasses {
			c := &gIrMTypeRefConstr{Class: tcsc.Class}
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
				tci := &gIrMTypeClassInst{Name: tciname, ClassName: tciclass}
				for _, tcit := range tcidef.InstanceTypes {
					tci.InstTypes = append(tci.InstTypes, me.newTypeRefFromEnvTag(tcit))
				}
				me.EnvTypeClassInsts = append(me.EnvTypeClassInsts, tci)
			}
		}
	}
}

func (me *gonadIrMeta) populateFromCoreImp() {
	me.mod.coreimp.prep()
	// discover and store exports
	for _, exp := range me.mod.ext.EfExports {
		if len(exp.TypeRef) > 1 {
			tname := exp.TypeRef[1].(string)
			me.Exports = append(me.Exports, tname)
			if len(exp.TypeRef) > 2 {
				if ctornames, _ := exp.TypeRef[2].([]interface{}); len(ctornames) > 0 {
					for _, ctorname := range ctornames {
						if cn, _ := ctorname.(string); len(cn) > 0 && !me.hasExport(cn) {
							me.Exports = append(me.Exports, tname+"ĸ"+cn)
						}
					}
				} else {
					if td := me.mod.coreimp.DeclEnv.TypeDefs[tname]; td != nil && td.Decl.DataType != nil {
						for ctorname, _ := range td.Decl.DataType.Ctors {
							me.Exports = append(me.Exports, tname+"ĸ"+ctorname)
						}
					}
				}
			}
		} else if len(exp.TypeClassRef) > 1 {
			me.Exports = append(me.Exports, exp.TypeClassRef[1].(string))
		} else if len(exp.ValueRef) > 1 {
			me.Exports = append(me.Exports, exp.ValueRef[1].(map[string]interface{})["Ident"].(string))
		} else if len(exp.TypeInstanceRef) > 1 {
			me.Exports = append(me.Exports, exp.TypeInstanceRef[1].(map[string]interface{})["Ident"].(string))
		}
	}
	// discover and store imports
	for _, imp := range me.mod.coreimp.Imps {
		if impname := strings.Join(imp, "."); impname != "Prim" && impname != "Prelude" && impname != me.mod.qName {
			me.imports = append(me.imports, findModuleByQName(impname))
		}
	}
	for _, impmod := range me.imports {
		me.Imports = append(me.Imports, newModImp(impmod))
	}
	// transform 100% complete coreimp structures
	// into lean, only-what-we-use girMeta structures (still representing PS-not-Go decls)
	me.populateEnvTypeSyns()
	me.populateEnvTypeClasses()
	me.populateEnvTypeDataDecls()
	me.populateEnvFuncsAndVals()
	// then transform those into Go decls
	me.populateGoTypeDefs()
	me.populateGoValDecls()
}

func (me *gonadIrMeta) populateFromLoaded() {
	me.imports = nil
	for _, imp := range me.Imports {
		if !strings.HasPrefix(imp.Q, nsPrefixDefaultFfiPkg) {
			if impmod := findModuleByQName(imp.Q); impmod != nil {
				me.imports = append(me.imports, impmod)
			} else if len(imp.Q) > 0 {
				panic(fmt.Errorf("%s: bad import %s", me.mod.srcFilePath, imp.Q))
			}
		}
	}
}

func (me *gonadIrMeta) populateGoValDecls() {
	for _, evd := range me.EnvValDecls {
		tdict := map[string][]string{}
		gvd := &gIrANamedTypeRef{Export: me.hasExport(evd.Name)}
		gvd.setBothNamesFromPsName(evd.Name)
		for gtd := me.goTypeDefByGoName(gvd.NameGo); gtd != nil; gtd = me.goTypeDefByGoName(gvd.NameGo) {
			gvd.NameGo += "º"
		}
		for gvd2 := me.goValDeclByGoName(gvd.NameGo); gvd2 != nil; gvd2 = me.goValDeclByGoName(gvd.NameGo) {
			gvd.NameGo += "ª"
		}
		gvd.setRefFrom(me.toGIrATypeRef(tdict, evd.Ref))
		me.GoValDecls = append(me.GoValDecls, gvd)
	}
}

func (me *gonadIrMeta) goValDeclByGoName(goname string) *gIrANamedTypeRef {
	for _, gvd := range me.GoValDecls {
		if gvd.NameGo == goname {
			return gvd
		}
	}
	return nil
}

func (me *gonadIrMeta) goValDeclByPsName(psname string) *gIrANamedTypeRef {
	for _, gvd := range me.GoValDecls {
		if gvd.NamePs == psname {
			return gvd
		}
	}
	return nil
}

func (me *gonadIrMeta) goTypeDefByGoName(goname string) *gIrANamedTypeRef {
	for _, gtd := range me.GoTypeDefs {
		if gtd.NameGo == goname {
			return gtd
		}
	}
	return nil
}

func (me *gonadIrMeta) goTypeDefByPsName(psname string) *gIrANamedTypeRef {
	for _, gtd := range me.GoTypeDefs {
		if gtd.NamePs == psname {
			return gtd
		}
	}
	return nil
}

func (me *gonadIrMeta) writeAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
