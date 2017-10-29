package main

import (
	"fmt"
	"strings"

	"github.com/metaleap/go-util/slice"
)

/*
Intermediate-representation:
of Golang named-and-typed declarations. This irANamedTypeRef
is embedded both in declarations that have a name and/or a type
(such as vars, args, consts, funcs etc) and in actual type-defs
(covered by the 'irATypeRefFoo' types in this it-typestuff.go).

More details in ir-meta.go.
*/

const (
	areOverlappingInterfacesSupportedByGo = true // technically would be false, see https://github.com/golang/go/issues/6977 --- in practice keep true until it's an actual issue in generated code
	legacyIfaceEmbeds                     = false
)

type irANamedTypeRefs []*irANamedTypeRef

func (me irANamedTypeRefs) Len() int { return len(me) }
func (me irANamedTypeRefs) Less(i, j int) bool {
	if me[i].sortIndex != me[j].sortIndex {
		return me[i].sortIndex < me[j].sortIndex
	}
	return strings.ToLower(me[i].NameGo) < strings.ToLower(me[j].NameGo)
}
func (me irANamedTypeRefs) Swap(i, j int) { me[i], me[j] = me[j], me[i] }

func (me irANamedTypeRefs) byPsName(psname string) *irANamedTypeRef {
	for _, gntr := range me {
		if gntr.NamePs == psname {
			return gntr
		}
	}
	return nil
}

func (me irANamedTypeRefs) equiv(cmp irANamedTypeRefs) bool {
	if l := len(me); l != len(cmp) {
		return false
	} else {
		for i := 0; i < l; i++ {
			if !me[i].equiv(cmp[i]) {
				return false
			}
		}
	}
	return true
}

type irANamedTypeRef struct {
	NamePs string `json:",omitempty"`
	NameGo string `json:",omitempty"`

	RefAlias     string               `json:",omitempty"`
	RefUnknown   int                  `json:",omitempty"`
	RefInterface *irATypeRefInterface `json:",omitempty"`
	RefFunc      *irATypeRefFunc      `json:",omitempty"`
	RefStruct    *irATypeRefStruct    `json:",omitempty"`
	RefArray     *irATypeRefArray     `json:",omitempty"`
	RefPtr       *irATypeRefPtr       `json:",omitempty"`

	Export bool `json:",omitempty"`

	sortIndex int
}

func (me *irANamedTypeRef) turnRefIntoRefPtr() {
	refptr := &irATypeRefPtr{Of: &irANamedTypeRef{}}
	refptr.Of.copyTypeInfoFrom(me)
	me.RefAlias, me.RefArray, me.RefFunc, me.RefInterface, me.RefPtr, me.RefStruct, me.RefUnknown = "", nil, nil, nil, refptr, nil, 0
}

func (me *irANamedTypeRef) clearTypeInfo() {
	me.RefAlias, me.RefUnknown, me.RefInterface, me.RefFunc, me.RefStruct, me.RefArray, me.RefPtr = "", 0, nil, nil, nil, nil, nil
}

func (me *irANamedTypeRef) copyFrom(from *irANamedTypeRef, names bool, trefs bool, export bool) {
	if names {
		me.NameGo, me.NamePs = from.NameGo, from.NamePs
	}
	if trefs {
		me.RefAlias, me.RefUnknown, me.RefInterface, me.RefFunc, me.RefStruct, me.RefArray, me.RefPtr = from.RefAlias, from.RefUnknown, from.RefInterface, from.RefFunc, from.RefStruct, from.RefArray, from.RefPtr
	}
	if export {
		me.Export = from.Export
	}
}

func (me *irANamedTypeRef) copyTypeInfoFrom(from *irANamedTypeRef) {
	me.copyFrom(from, false, true, false)
}

func (me *irANamedTypeRef) nameless() (copy *irANamedTypeRef) {
	copy = &irANamedTypeRef{}
	copy.copyTypeInfoFrom(me)
	return
}

func (me *irANamedTypeRef) equiv(cmp *irANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.equiv(cmp.RefInterface) && me.RefFunc.equiv(cmp.RefFunc) && me.RefStruct.equiv(cmp.RefStruct) && me.RefArray.equiv(cmp.RefArray) && me.RefPtr.equiv(cmp.RefPtr))
}

func (me *irANamedTypeRef) hasTypeInfoBeyondEmptyIface() (welltyped bool) {
	if welltyped = me.hasTypeInfo(); welltyped && me.RefInterface != nil {
		welltyped = len(me.RefInterface.Embeds) > 0 || len(me.RefInterface.Methods) > 0
	}
	return
}

func (me *irANamedTypeRef) hasTypeInfo() bool {
	return me != nil && me.RefAlias != "" || me.RefArray != nil || me.RefFunc != nil || me.RefInterface != nil || me.RefPtr != nil || me.RefStruct != nil || me.RefUnknown != 0
}

func (me *irANamedTypeRef) setBothNamesFromPsName(psname string) {
	me.NamePs = psname
	me.NameGo = sanitizeSymbolForGo(psname, me.Export)
}

func (me *irANamedTypeRef) setRefFrom(tref interface{}) {
	switch tr := tref.(type) {
	case *irANamedTypeRef:
		me.RefAlias = tr.RefAlias
		me.RefArray = tr.RefArray
		me.RefFunc = tr.RefFunc
		me.RefInterface = tr.RefInterface
		me.RefPtr = tr.RefPtr
		me.RefStruct = tr.RefStruct
		me.RefUnknown = tr.RefUnknown
	case *irATypeRefInterface:
		me.RefInterface = tr
	case *irATypeRefFunc:
		me.RefFunc = tr
	case *irATypeRefStruct:
		me.RefStruct = tr
	case *irATypeRefArray:
		me.RefArray = tr
	case *irATypeRefPtr:
		me.RefPtr = tr
	case int:
		me.RefUnknown = tr
	case string:
		me.RefAlias = tr
	case nil:
	default:
		panicWithType("setRefFrom", tref, "tref")
	}
}

type irATypeRefArray struct {
	Of *irANamedTypeRef
}

func (me *irATypeRefArray) equiv(cmp *irATypeRefArray) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.equiv(cmp.Of))
}

type irATypeRefPtr struct {
	Of *irANamedTypeRef
}

func (me *irATypeRefPtr) equiv(cmp *irATypeRefPtr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.equiv(cmp.Of))
}

type irATypeRefInterface struct {
	Embeds  []string         `json:",omitempty"`
	Methods irANamedTypeRefs `json:",omitempty"`

	isTypeVar        bool
	xtc              *irMTypeClass
	xtd              *irMTypeDataDecl
	inheritedMethods irANamedTypeRefs
}

func (me *irATypeRefInterface) equiv(cmp *irATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.isTypeVar == cmp.isTypeVar && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.equiv(cmp.Methods))
}

type irATypeRefFunc struct {
	Args irANamedTypeRefs `json:",omitempty"`
	Rets irANamedTypeRefs `json:",omitempty"`

	impl *irABlock
}

func (me *irATypeRefFunc) copyArgTypesOnlyFrom(namesIfMeNil bool, from *irATypeRefFunc) {
	copyargs := func(meargs irANamedTypeRefs, fromargs irANamedTypeRefs) irANamedTypeRefs {
		if numargsme := len(meargs); numargsme == 0 {
			for _, arg := range fromargs {
				mearg := &irANamedTypeRef{}
				mearg.copyFrom(arg, namesIfMeNil, true, false)
				meargs = append(meargs, mearg)
			}
		} else if numargsfrom := len(fromargs); numargsme != numargsfrom {
			panic(notImplErr("args-num mismatch", fmt.Sprintf("%v vs %v", numargsme, numargsfrom), "copyArgTypesFrom"))
		} else {
			for i := 0; i < numargsme; i++ {
				meargs[i].copyTypeInfoFrom(fromargs[i])
			}
		}
		return meargs
	}
	me.Args = copyargs(me.Args, from.Args)
	me.Rets = copyargs(me.Rets, from.Rets)
}

func (me *irATypeRefFunc) equiv(cmp *irATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.equiv(cmp.Args) && me.Rets.equiv(cmp.Rets))
}

func (me *irATypeRefFunc) haveAllArgsTypeInfo() bool {
	for _, arg := range me.Args {
		if !arg.hasTypeInfo() {
			return false
		}
	}
	for _, ret := range me.Rets {
		if !ret.hasTypeInfo() {
			return false
		}
	}
	return true
}

func (me *irATypeRefFunc) haveAnyArgsTypeInfo() bool {
	for _, arg := range me.Args {
		if arg.hasTypeInfo() {
			return true
		}
	}
	for _, ret := range me.Rets {
		if ret.hasTypeInfo() {
			return true
		}
	}
	return false
}

func (me *irATypeRefFunc) toSig(forceretarg bool) (rf *irATypeRefFunc) {
	rf = &irATypeRefFunc{}
	for _, arg := range me.Args {
		rf.Args = append(rf.Args, arg.nameless())
	}
	if len(me.Rets) == 0 && forceretarg {
		rf.Rets = append(rf.Rets, &irANamedTypeRef{})
	} else {
		for _, ret := range me.Rets {
			rf.Rets = append(rf.Rets, ret.nameless())
		}
	}
	return
}

type irATypeRefStruct struct {
	Embeds    []string         `json:",omitempty"`
	Fields    irANamedTypeRefs `json:",omitempty"`
	PassByPtr bool             `json:",omitempty"`
	Methods   irANamedTypeRefs `json:",omitempty"`
}

func (me *irATypeRefStruct) equiv(cmp *irATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.equiv(cmp.Fields))
}

func (me *irATypeRefStruct) memberByPsName(nameps string) (mem *irANamedTypeRef) {
	if mem = me.Fields.byPsName(nameps); mem == nil {
		mem = me.Methods.byPsName(nameps)
	}
	return
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
	var gtdi *irANamedTypeRef
	for _, gtd := range me.GoTypeDefs {
		if gtd.NamePs == psname {
			if gtd.RefInterface != nil {
				gtdi = gtd
			} else {
				return gtd
			}
		}
	}
	return gtdi
}

func (me *irMeta) populateGoTypeDefs() {
	for _, ts := range me.EnvTypeSyns {
		tc, gtd, tdict := me.tc(ts.Name), &irANamedTypeRef{Export: me.hasExport(ts.Name)}, map[string][]string{}
		gtd.setBothNamesFromPsName(ts.Name)
		gtd.setRefFrom(me.toIrATypeRef(tdict, ts.Ref))
		if tc != nil {
			if gtd.NameGo += "ˇ"; gtd.RefStruct != nil {
				gtd.RefStruct.PassByPtr = true
				for _, gtdf := range gtd.RefStruct.Fields {
					if gtdf.Export != gtd.Export {
						gtdf.Export = gtd.Export
						gtdf.setBothNamesFromPsName(gtdf.NamePs)
					}
					if tcm := tc.memberBy(gtdf.NamePs); tcm == nil {
						if rfn := gtdf.RefFunc; rfn == nil {
							panic(notImplErr("non-func super-class-referencing-struct-field type for", gtdf.NamePs, me.mod.srcFilePath))
						} else {
							for retfunc := rfn.Rets[0].RefFunc; retfunc != nil; retfunc = rfn.Rets[0].RefFunc {
								rfn = retfunc
							}
							if rfn.Rets[0].RefAlias == "" {
								panic(notImplErr("ultimate-return type in super-class-referencing-struct-field for", gtdf.NamePs, me.mod.srcFilePath))
							} else {
								refptr := &irATypeRefPtr{Of: &irANamedTypeRef{RefAlias: rfn.Rets[0].RefAlias}}
								rfn.Rets[0].RefAlias, rfn.Rets[0].RefPtr = "", refptr
							}
						}
					}
				}
			}
		}
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}
	for _, tc := range me.EnvTypeClasses {
		tsynfound := false
		for _, ts := range me.EnvTypeSyns {
			if tsynfound = (ts.Name == tc.Name); tsynfound {
				break
			}
		}
		if !tsynfound {
			panic(notImplErr("lack of pre-formed type-synonym for type-class", tc.Name, me.mod.srcFilePath))
			// tdict, gtd := map[string][]string{}, &irANamedTypeRef{Export: me.hasExport(tc.Name)}
			// gtd.setBothNamesFromPsName(tc.Name)
			// gtd.NameGo += "ˇ"
			// gtd.RefStruct = &irATypeRefStruct{PassByPtr: true}
			// for _, tcm := range tc.Members {
			// 	tcmfield := &irANamedTypeRef{Export: true}
			// 	tcmfield.setBothNamesFromPsName(tcm.Name)
			// 	tcmfield.setRefFrom(me.toIrATypeRef(tdict, tcm.Ref))
			// 	gtd.RefStruct.Fields = append(gtd.RefStruct.Fields, tcmfield)
			// }
			// me.GoTypeDefs = append(me.GoTypeDefs, gtd)
		}
	}
	me.GoTypeDefs = append(me.GoTypeDefs, me.toIrADataTypeDefs(me.EnvTypeDataDecls)...)
}

func (me *irAst) resolveGoTypeRefFromQName(tref string) (pname string, tname string) {
	var mod *modPkg
	wasprim := false
	i := strings.LastIndex(tref, ".")
	if tname = tref[i+1:]; i > 0 {
		pname = tref[:i]
		if pname == me.mod.qName {
			pname = ""
			mod = me.mod
		} else if wasprim = (pname == "Prim"); wasprim {
			pname = ""
			switch tname {
			case "Char":
				tname = "rune"
			case "String":
				tname = "string"
			case "Boolean":
				tname = "bool"
			case "Number":
				tname = "float64"
			case "Int":
				tname = "int"
			default:
				panic(notImplErr("Prim type '"+tname+"' for", tref, me.mod.srcFilePath))
			}
		} else {
			qn, foundimport, isffi := pname, false, strings.HasPrefix(pname, nsPrefixDefaultFfiPkg)
			if isffi {
				pname = strReplDot2Underscore.Replace(pname)
			} else {
				if mod = findModuleByQName(qn); mod == nil {
					if mod = findModuleByPName(qn); mod == nil {
						panic(notImplErr("module qname", qn, me.mod.srcFilePath))
					}
				}
				pname = mod.pName
			}
			for _, imp := range me.irM.Imports {
				if imp.PsModQName == qn {
					foundimport = true
					break
				}
			}
			if !foundimport {
				var imp *irMPkgRef
				if isffi {
					imp = &irMPkgRef{ImpPath: "github.com/metaleap/gonad/" + strReplDot2Slash.Replace(qn), PsModQName: qn, GoName: pname}
				} else {
					imp = mod.newModImp()
				}
				me.irM.imports, me.irM.Imports = append(me.irM.imports, mod), append(me.irM.Imports, imp)
			}
		}
	} else {
		mod = me.mod
	}
	if (!wasprim) && mod != nil {
		if gtd := mod.irMeta.goTypeDefByPsName(tname); gtd != nil {
			tname = gtd.NameGo
		}
	}
	return
}

func (me *irMeta) toIrADataTypeDefs(typedatadecls []*irMTypeDataDecl) (gtds irANamedTypeRefs) {
	for _, td := range typedatadecls {
		tdict := map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(notImplErr(me.mod.srcFilePath+": unexpected ctor absence for", td.Name, td))
		} else {
			isnewtype, hasctorargs := false, false
			gid := &irANamedTypeRef{RefInterface: &irATypeRefInterface{xtd: td}, Export: me.hasExport(td.Name)}
			gid.setBothNamesFromPsName(td.Name)
			for _, ctor := range td.Ctors {
				if numargs := len(ctor.Args); numargs > 0 {
					if hasctorargs = true; numargs == 1 && numctors == 1 {
						if ctor.Args[0].TypeConstructor != (me.mod.qName + "." + td.Name) {
							isnewtype = true
						}
					}
				}
			}
			if isnewtype {
				gid.RefInterface = nil
				gid.setRefFrom(me.toIrATypeRef(tdict, td.Ctors[0].Args[0]))
			} else {
				for _, ctor := range td.Ctors {
					ctor.gtd = &irANamedTypeRef{Export: me.hasExport(gid.NamePs + "ĸ" + ctor.Name),
						RefStruct: &irATypeRefStruct{PassByPtr: (hasctorargs && len(ctor.Args) >= Proj.BowerJsonFile.Gonad.CodeGen.PtrStructMinFieldCount)}}
					ctor.gtd.setBothNamesFromPsName(gid.NamePs + "ˇ" + ctor.Name)
					ctor.gtd.NamePs = ctor.Name
					for ia, ctorarg := range ctor.Args {
						field := &irANamedTypeRef{}
						if field.setRefFrom(me.toIrATypeRef(tdict, ctorarg)); field.RefAlias == (me.mod.qName + "." + ctor.Name) {
							//	an inconstructable self-recursive type, aka Data.Void
							field.turnRefIntoRefPtr()
						}
						ctorarg.tmp_assoc = field
						field.NameGo = fmt.Sprintf("%s%d", sanitizeSymbolForGo(ctor.Name, true), ia)
						field.NamePs = fmt.Sprintf("value%d", ia)
						ctor.gtd.RefStruct.Fields = append(ctor.gtd.RefStruct.Fields, field)
					}
					gtds = append(gtds, ctor.gtd)
				}
			}
			gtds = append(gtds, gid)
		}
	}
	return
}

func (me *irMeta) toIrATypeRef(tdict map[string][]string, tr *irMTypeRef) interface{} {
	funcyhackery := func(ret *irMTypeRef) interface{} {
		funtype := &irATypeRefFunc{}
		funtype.Args = irANamedTypeRefs{&irANamedTypeRef{}}
		funtype.Args[0].setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Left.TypeApp.Right))
		funtype.Rets = irANamedTypeRefs{&irANamedTypeRef{}}
		funtype.Rets[0].setRefFrom(me.toIrATypeRef(tdict, ret))
		return funtype
	}

	if tr.TypeConstructor != "" {
		return tr.TypeConstructor
	} else if tr.REmpty {
		return nil
	} else if tr.TypeVar != "" {
		// embeds := tdict[tr.TypeVar]
		// if len(embeds) == 1 {
		// 	return embeds[0]
		// }
		return &irATypeRefInterface{isTypeVar: true /*, Embeds: embeds*/}
	} else if tr.ConstrainedType != nil {
		/*	a whacky case from Semigroupoid.composeFlipped:
			ForAll(d).ForAll(c).ForAll(b).ForAll(a).ConstrT(Semibla).TApp {
				TApp { TCtor(Prim.Func), TApp { TApp { TVar(a),TVar(b) }, TVar(c) } },
				TApp {
					TApp { TCtor(Prim.Func), TApp { TApp { TVar(a), TVar(c) }, TVar(d) } },
					TApp { TApp{ TVar(a), TVar(b) }, TVar(d) }
				}
			}
		*/
		if tr.ConstrainedType.Ref.TypeApp != nil && tr.ConstrainedType.Ref.TypeApp.Left.TypeApp != nil && tr.ConstrainedType.Ref.TypeApp.Right.TypeApp != nil {
			funtype := &irATypeRefFunc{}
			funtype.Args = irANamedTypeRefs{&irANamedTypeRef{}}
			funtype.Args[0].setRefFrom(tr.ConstrainedType.Class)
			funtype.Args[0].turnRefIntoRefPtr()
			funtype.Rets = irANamedTypeRefs{&irANamedTypeRef{}}
			funtype.Rets[0].setRefFrom(me.toIrATypeRef(tdict, tr.ConstrainedType.Ref))
			return funtype
		}
		return me.toIrATypeRef(tdict, tr.ConstrainedType.Ref)
	} else if tr.ForAll != nil {
		return me.toIrATypeRef(tdict, tr.ForAll.Ref)
	} else if tr.Skolem != nil {
		return fmt.Sprintf("Skolem_%s_scope%d_value%d", tr.Skolem.Name, tr.Skolem.Scope, tr.Skolem.Value)
	} else if tr.RCons != nil {
		rectype := &irATypeRefStruct{}
		myfield := &irANamedTypeRef{Export: true}
		myfield.setBothNamesFromPsName(tr.RCons.Label)
		myfield.setRefFrom(me.toIrATypeRef(tdict, tr.RCons.Left))
		rectype.Fields = append(rectype.Fields, myfield)
		if nextrow := me.toIrATypeRef(tdict, tr.RCons.Right); nextrow != nil {
			rectype.Fields = append(rectype.Fields, nextrow.(*irATypeRefStruct).Fields...)
		}
		rectype.PassByPtr = len(rectype.Fields) >= Proj.BowerJsonFile.Gonad.CodeGen.PtrStructMinFieldCount
		return rectype
	} else if tr.TypeApp != nil {
		if tr.TypeApp.Left.TypeConstructor == "Prim.Record" {
			return me.toIrATypeRef(tdict, tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor == "Prim.Array" {
			array := &irATypeRefArray{Of: &irANamedTypeRef{}}
			array.Of.setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Right))
			return array
		} else if strings.HasPrefix(tr.TypeApp.Left.TypeConstructor, "Prim.") {
			panic(notImplErr("type-app left-hand primitive", tr.TypeApp.Left.TypeConstructor, me.mod.srcFilePath))
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" && tr.TypeApp.Left.TypeApp.Right.TypeApp != nil && tr.TypeApp.Left.TypeApp.Right.TypeApp.Left.TypeConstructor == "Prim.Record" && tr.TypeApp.Right.TypeApp != nil && tr.TypeApp.Right.TypeApp.Left != nil {
			return funcyhackery(tr.TypeApp.Right.TypeApp.Left)
		} else if tr.TypeApp.Left.TypeApp != nil && (tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" || /*insanely hacky*/ tr.TypeApp.Right.TypeVar != "") {
			return funcyhackery(tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor != "" {
			return me.toIrATypeRef(tdict, tr.TypeApp.Left)
		}
	}
	return nil
}
