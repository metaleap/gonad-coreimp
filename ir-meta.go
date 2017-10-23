package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/metaleap/go-util/fs"
	"github.com/metaleap/go-util/slice"
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
with irM (meta) and the latter with irA (AST). Both
are held in the irMeta struct / gonadmeta.json.
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

type irMeta struct {
	Exports           []string            `json:",omitempty"`
	Imports           irMPkgRefs          `json:",omitempty"`
	EnvTypeSyns       []*irMNamedTypeRef  `json:",omitempty"`
	EnvTypeClasses    []*irMTypeClass     `json:",omitempty"`
	EnvTypeClassInsts []*irMTypeClassInst `json:",omitempty"`
	EnvTypeDataDecls  []*irMTypeDataDecl  `json:",omitempty"`
	EnvValDecls       []*irMNamedTypeRef  `json:",omitempty"`
	GoTypeDefs        irANamedTypeRefs    `json:",omitempty"`
	GoValDecls        irANamedTypeRefs    `json:",omitempty"`
	ForeignImp        *irMPkgRef          `json:",omitempty"`

	imports []*modPkg

	mod  *modPkg
	proj *psBowerProject
	save bool
}

type irMPkgRefs []*irMPkgRef

func (me irMPkgRefs) Len() int           { return len(me) }
func (me irMPkgRefs) Less(i, j int) bool { return me[i].ImpPath < me[j].ImpPath }
func (me irMPkgRefs) Swap(i, j int)      { me[i], me[j] = me[j], me[i] }

func (me *irMPkgRefs) addIfHasnt(lname, imppath, qname string) (pkgref *irMPkgRef) {
	if pkgref = me.byImpPath(imppath); pkgref == nil {
		pkgref = &irMPkgRef{GoName: lname, ImpPath: imppath, PsModQName: qname}
		*me = append(*me, pkgref)
	}
	return
}

func (me irMPkgRefs) byImpPath(imppath string) *irMPkgRef {
	for _, imp := range me {
		if imp.ImpPath == imppath {
			return imp
		}
	}
	return nil
}

func (me irMPkgRefs) byImpName(pkgname string) *irMPkgRef {
	for _, imp := range me {
		if imp.GoName == pkgname || (imp.GoName == "" && imp.ImpPath == pkgname) {
			return imp
		}
	}
	return nil
}

type irMPkgRef struct {
	GoName     string
	PsModQName string
	ImpPath    string

	emitted bool
}

type irMNamedTypeRef struct {
	Name string      `json:"tnn,omitempty"`
	Ref  *irMTypeRef `json:"tnr,omitempty"`
}

type irMTypeClass struct {
	Name        string              `json:"tcn,omitempty"`
	Args        []string            `json:"tca,omitempty"`
	Constraints []*irMTypeRefConstr `json:"tcc,omitempty"`
	Members     []*irMNamedTypeRef  `json:"tcm,omitempty"`
}

type irMTypeClassInst struct {
	Name      string      `json:"tcin,omitempty"`
	ClassName string      `json:"tcicn,omitempty"`
	InstTypes irMTypeRefs `json:"tcit,omitempty"`
}

type irMTypeDataDecl struct {
	Name  string             `json:"tdn,omitempty"`
	Ctors []*irMTypeDataCtor `json:"tdc,omitempty"`
	Args  []string           `json:"tda,omitempty"`
}

type irMTypeDataCtor struct {
	Name string      `json:"tdcn,omitempty"`
	Args irMTypeRefs `json:"tdca,omitempty"`

	gtd *irANamedTypeRef
}

type irMTypeRefs []*irMTypeRef

type irMTypeRef struct {
	TypeConstructor string            `json:"tc,omitempty"`
	TypeVar         string            `json:"tv,omitempty"`
	REmpty          bool              `json:"re,omitempty"`
	TypeApp         *irMTypeRefAppl   `json:"ta,omitempty"`
	ConstrainedType *irMTypeRefConstr `json:"ct,omitempty"`
	RCons           *irMTypeRefRow    `json:"rc,omitempty"`
	ForAll          *irMTypeRefExist  `json:"fa,omitempty"`
	Skolem          *irMTypeRefSkolem `json:"sk,omitempty"`

	tmp_assoc *irANamedTypeRef
}

func (me *irMTypeRef) eq(cmp *irMTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.TypeConstructor == cmp.TypeConstructor && me.TypeVar == cmp.TypeVar && me.REmpty == cmp.REmpty && me.TypeApp.eq(cmp.TypeApp) && me.ConstrainedType.eq(cmp.ConstrainedType) && me.RCons.eq(cmp.RCons) && me.ForAll.eq(cmp.ForAll) && me.Skolem.eq(cmp.Skolem))
}

func (me irMTypeRefs) eq(cmp irMTypeRefs) bool {
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

type irMTypeRefAppl struct {
	Left  *irMTypeRef `json:"t1,omitempty"`
	Right *irMTypeRef `json:"t2,omitempty"`
}

func (me *irMTypeRefAppl) eq(cmp *irMTypeRefAppl) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Left.eq(cmp.Left) && me.Right.eq(cmp.Right))
}

type irMTypeRefRow struct {
	Label string      `json:"rl,omitempty"`
	Left  *irMTypeRef `json:"r1,omitempty"`
	Right *irMTypeRef `json:"r2,omitempty"`
}

func (me *irMTypeRefRow) eq(cmp *irMTypeRefRow) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Label == cmp.Label && me.Left.eq(cmp.Left) && me.Right.eq(cmp.Right))
}

type irMTypeRefConstr struct {
	Class string      `json:"cc,omitempty"`
	Args  irMTypeRefs `json:"ca,omitempty"`
	Ref   *irMTypeRef `json:"cr,omitempty"`
}

func (me *irMTypeRefConstr) eq(cmp *irMTypeRefConstr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Class == cmp.Class && me.Ref.eq(cmp.Ref) && me.Args.eq(cmp.Args))
}

type irMTypeRefExist struct {
	Name        string      `json:"en,omitempty"`
	Ref         *irMTypeRef `json:"er,omitempty"`
	SkolemScope *int        `json:"es,omitempty"`
}

func (me *irMTypeRefExist) eq(cmp *irMTypeRefExist) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Ref.eq(cmp.Ref) && me.SkolemScope == cmp.SkolemScope)
}

type irMTypeRefSkolem struct {
	Name  string `json:"sn,omitempty"`
	Value int    `json:"sv,omitempty"`
	Scope int    `json:"ss,omitempty"`
}

func (me *irMTypeRefSkolem) eq(cmp *irMTypeRefSkolem) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Value == cmp.Value && me.Scope == cmp.Scope)
}

func newModImp(impmod *modPkg) *irMPkgRef {
	return &irMPkgRef{GoName: impmod.pName, PsModQName: impmod.qName, ImpPath: path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)}
}

func (me *irMeta) hasExport(name string) bool {
	return uslice.StrHas(me.Exports, name)
}

func (me *irMeta) tcMember(name string) *irMNamedTypeRef {
	for _, tc := range me.EnvTypeClasses {
		for _, tcm := range tc.Members {
			if tcm.Name == name {
				return tcm
			}
		}
	}
	return nil
}

func (me *irMeta) tcInst(name string) *irMTypeClassInst {
	for _, tci := range me.EnvTypeClassInsts {
		if tci.Name == name {
			return tci
		}
	}
	return nil
}

func (me *irMeta) newTypeRefFromEnvTag(tc *coreImpEnvTagType) (tref *irMTypeRef) {
	tref = &irMTypeRef{}
	if tc.isTypeConstructor() {
		tref.TypeConstructor = tc.text
	} else if tc.isTypeVar() {
		tref.TypeVar = tc.text
	} else if tc.isREmpty() {
		tref.REmpty = true
	} else if tc.isRCons() {
		tref.RCons = &irMTypeRefRow{
			Label: tc.text, Left: me.newTypeRefFromEnvTag(tc.type0), Right: me.newTypeRefFromEnvTag(tc.type1)}
	} else if tc.isForAll() {
		tref.ForAll = &irMTypeRefExist{Name: tc.text, Ref: me.newTypeRefFromEnvTag(tc.type0)}
		if tc.skolem >= 0 {
			tref.ForAll.SkolemScope = &tc.skolem
		}
	} else if tc.isSkolem() {
		tref.Skolem = &irMTypeRefSkolem{Name: tc.text, Value: tc.num, Scope: tc.skolem}
	} else if tc.isTypeApp() {
		tref.TypeApp = &irMTypeRefAppl{Left: me.newTypeRefFromEnvTag(tc.type0), Right: me.newTypeRefFromEnvTag(tc.type1)}
	} else if tc.isConstrainedType() {
		tref.ConstrainedType = &irMTypeRefConstr{Ref: me.newTypeRefFromEnvTag(tc.type0), Class: tc.constr.Class}
		for _, tca := range tc.constr.Args {
			tref.ConstrainedType.Args = append(tref.ConstrainedType.Args, me.newTypeRefFromEnvTag(tca))
		}
	} else {
		panic(notImplErr("tagged-type", tc.Tag, me.mod.srcFilePath))
	}
	return
}

func (me *irMeta) populateEnvFuncsAndVals() {
	for fname, fdef := range me.mod.coreimp.DeclEnv.Functions {
		me.EnvValDecls = append(me.EnvValDecls, &irMNamedTypeRef{Name: fname, Ref: me.newTypeRefFromEnvTag(fdef.Type)})
	}
}

func (me *irMeta) populateEnvTypeDataDecls() {
	for tdefname, tdef := range me.mod.coreimp.DeclEnv.TypeDefs {
		if tdef.Decl.TypeSynonym {
			//	type-aliases handled separately in populateEnvTypeSyns already, nothing to do here
		} else if tdef.Decl.ExternData {
			if ffigofilepath := me.mod.srcFilePath[:len(me.mod.srcFilePath)-len(".purs")] + ".go"; ufs.FileExists(ffigofilepath) {
				panic(me.mod.srcFilePath + ": time to handle FFI " + ffigofilepath)
			} else {
				//	special case for official purescript core libs: alias to applicable struct from gonad's default ffi packages
				ta := &irMNamedTypeRef{Name: tdefname, Ref: &irMTypeRef{TypeConstructor: nsPrefixDefaultFfiPkg + me.mod.qName + "." + tdefname}}
				me.EnvTypeSyns = append(me.EnvTypeSyns, ta)
			}
		} else {
			dt := &irMTypeDataDecl{Name: tdefname}
			for dtargname, _ := range tdef.Decl.DataType.Args {
				dt.Args = append(dt.Args, dtargname)
			}
			for dcname, dcargtypes := range tdef.Decl.DataType.Ctors {
				dtc := &irMTypeDataCtor{Name: dcname}
				for _, dcargtype := range dcargtypes {
					dtc.Args = append(dtc.Args, me.newTypeRefFromEnvTag(dcargtype))
				}
				dt.Ctors = append(dt.Ctors, dtc)
			}
			me.EnvTypeDataDecls = append(me.EnvTypeDataDecls, dt)
		}
	}
}

func (me *irMeta) populateEnvTypeSyns() {
	for tsname, tsdef := range me.mod.coreimp.DeclEnv.TypeSyns {
		if _, istc := me.mod.coreimp.DeclEnv.Classes[tsname]; !istc {
			ts := &irMNamedTypeRef{Name: tsname}
			ts.Ref = me.newTypeRefFromEnvTag(tsdef.Type)
			me.EnvTypeSyns = append(me.EnvTypeSyns, ts)
		}
	}
}

func (me *irMeta) populateEnvTypeClasses() {
	for tcname, tcdef := range me.mod.coreimp.DeclEnv.Classes {
		tc := &irMTypeClass{Name: tcname}
		for tcarg, _ := range tcdef.Args {
			tc.Args = append(tc.Args, tcarg)
		}
		for tcmname, tcmdef := range tcdef.Members {
			tref := me.newTypeRefFromEnvTag(tcmdef)
			tc.Members = append(tc.Members, &irMNamedTypeRef{Name: tcmname, Ref: tref})
		}
		for _, tcsc := range tcdef.Superclasses {
			c := &irMTypeRefConstr{Class: tcsc.Class}
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
				tci := &irMTypeClassInst{Name: tciname, ClassName: tciclass}
				for _, tcit := range tcidef.InstanceTypes {
					tci.InstTypes = append(tci.InstTypes, me.newTypeRefFromEnvTag(tcit))
				}
				me.EnvTypeClassInsts = append(me.EnvTypeClassInsts, tci)
			}
		}
	}
}

func (me *irMeta) populateFromCoreImp() {
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
	// into lean, only-what-we-use irMeta structures (still representing PS-not-Go decls)
	me.populateEnvTypeSyns()
	me.populateEnvTypeClasses()
	me.populateEnvTypeDataDecls()
	me.populateEnvFuncsAndVals()
	// then transform those into Go decls
	me.populateGoTypeDefs()
	me.populateGoValDecls()
}

func (me *irMeta) populateFromLoaded() {
	me.imports = nil
	for _, imp := range me.Imports {
		if !strings.HasPrefix(imp.PsModQName, nsPrefixDefaultFfiPkg) {
			if impmod := findModuleByQName(imp.PsModQName); impmod != nil {
				me.imports = append(me.imports, impmod)
			} else if len(imp.PsModQName) > 0 {
				panic(fmt.Errorf("%s: bad import %s", me.mod.srcFilePath, imp.PsModQName))
			}
		}
	}
}

func (me *irMeta) populateGoValDecls() {
	for _, evd := range me.EnvValDecls {
		tdict := map[string][]string{}
		gvd := &irANamedTypeRef{Export: me.hasExport(evd.Name)}
		gvd.setBothNamesFromPsName(evd.Name)
		for gtd := me.goTypeDefByGoName(gvd.NameGo); gtd != nil; gtd = me.goTypeDefByGoName(gvd.NameGo) {
			gvd.NameGo += "º"
		}
		for gvd2 := me.goValDeclByGoName(gvd.NameGo); gvd2 != nil; gvd2 = me.goValDeclByGoName(gvd.NameGo) {
			gvd.NameGo += "ª"
		}
		gvd.setRefFrom(me.toIrATypeRef(tdict, evd.Ref))
		me.GoValDecls = append(me.GoValDecls, gvd)
	}
}

func (me *irMeta) goValDeclByGoName(goname string) *irANamedTypeRef {
	for _, gvd := range me.GoValDecls {
		if gvd.NameGo == goname {
			return gvd
		}
	}
	return nil
}

func (me *irMeta) goValDeclByPsName(psname string) *irANamedTypeRef {
	for _, gvd := range me.GoValDecls {
		if gvd.NamePs == psname {
			return gvd
		}
	}
	return nil
}

func (me *irMeta) goTypeDefByGoName(goname string) *irANamedTypeRef {
	for _, gtd := range me.GoTypeDefs {
		if gtd.NameGo == goname {
			return gtd
		}
	}
	return nil
}

func (me *irMeta) goTypeDefByPsName(psname string) *irANamedTypeRef {
	for _, gtd := range me.GoTypeDefs {
		if gtd.NamePs == psname {
			return gtd
		}
	}
	return nil
}

func (me *irMeta) writeAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}
