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

func (me irANamedTypeRefs) eq(cmp irANamedTypeRefs) bool {
	if l := len(me); l != len(cmp) {
		return false
	} else {
		for i := 0; i < l; i++ {
			if !me[i].eq(cmp[i]) {
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

func (me *irANamedTypeRef) turnRefAliasIntoRefPtr() {
	me.RefPtr = &irATypeRefPtr{Of: &irANamedTypeRef{RefAlias: me.RefAlias}}
	me.RefAlias = ""
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

func (me *irANamedTypeRef) nameless() (copy *irANamedTypeRef) {
	copy = &irANamedTypeRef{}
	copy.copyFrom(me, false, true, false)
	return
}

func (me *irANamedTypeRef) eq(cmp *irANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.eq(cmp.RefInterface) && me.RefFunc.eq(cmp.RefFunc) && me.RefStruct.eq(cmp.RefStruct) && me.RefArray.eq(cmp.RefArray) && me.RefPtr.eq(cmp.RefPtr))
}

func (me *irANamedTypeRef) hasTypeInfo() bool {
	return len(me.RefAlias) > 0 || me.RefArray != nil || me.RefFunc != nil || me.RefInterface != nil || me.RefPtr != nil || me.RefStruct != nil || me.RefUnknown != 0
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

func (me *irATypeRefArray) eq(cmp *irATypeRefArray) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.eq(cmp.Of))
}

type irATypeRefPtr struct {
	Of *irANamedTypeRef
}

func (me *irATypeRefPtr) eq(cmp *irATypeRefPtr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.eq(cmp.Of))
}

type irATypeRefInterface struct {
	Embeds  []string         `json:",omitempty"`
	Methods irANamedTypeRefs `json:",omitempty"`

	xtc              *irMTypeClass
	xtd              *irMTypeDataDecl
	inheritedMethods irANamedTypeRefs
}

func (me *irATypeRefInterface) eq(cmp *irATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.eq(cmp.Methods))
}

func (me *irATypeRefInterface) allMethods() (allmethods irANamedTypeRefs) {
	allmethods = me.Methods
	if Proj.BowerJsonFile.Gonad.CodeGen.TypeClasses2Interfaces && (!areOverlappingInterfacesSupportedByGo) && len(me.Embeds) > 0 {
		// if len(me.inheritedMethods) == 0 {
		// 	m := map[string]*irANamedTypeRef{}
		// 	for _, embed := range me.Embeds {
		// 		if gtd := findGoTypeByPsQName(embed); gtd == nil || gtd.RefInterface == nil {
		// 			panic(notImplErr("reference to interface-type-class", embed, me.xtc.Name))
		// 		} else {
		// 			for _, method := range gtd.RefInterface.allMethods() {
		// 				if dupl := m[method.NameGo]; dupl == nil {
		// 					m[method.NameGo], me.inheritedMethods = method, append(me.inheritedMethods, method)
		// 				} else if !dupl.eq(method) {
		// 					panic("Interface (generated from type-class " + me.xtc.Name + ") would inherit multiple (but different-signature) methods named " + method.NameGo)
		// 				}
		// 			}
		// 		}
		// 	}
		// }
		// allmethods = append(me.inheritedMethods, allmethods...)
	}
	return
}

type irATypeRefFunc struct {
	Args irANamedTypeRefs `json:",omitempty"`
	Rets irANamedTypeRefs `json:",omitempty"`

	impl *irABlock
}

func (me *irATypeRefFunc) eq(cmp *irATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.eq(cmp.Args) && me.Rets.eq(cmp.Rets))
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

func (me *irATypeRefStruct) eq(cmp *irATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.eq(cmp.Fields))
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

func (me *irMeta) populateGoTypeDefs() {
	for _, ts := range me.EnvTypeSyns {
		tdict := map[string][]string{}
		gtd := &irANamedTypeRef{Export: me.hasExport(ts.Name)}
		gtd.setBothNamesFromPsName(ts.Name)
		gtd.setRefFrom(me.toIrATypeRef(tdict, ts.Ref))
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}
	if Proj.BowerJsonFile.Gonad.CodeGen.TypeClasses2Interfaces {
		// for _, tc := range me.EnvTypeClasses {
		// 	tdict := map[string][]string{}
		// 	gif := &irATypeRefInterface{xtc: tc}
		// 	for _, tcc := range tc.Constraints {
		// 		for _, tcca := range tcc.Args {
		// 			ensureIfaceForTvar(tdict, tcca.TypeVar, tcc.Class)
		// 		}
		// 		if legacyIfaceEmbeds && !uslice.StrHas(gif.Embeds, tcc.Class) {
		// 			gif.Embeds = append(gif.Embeds, tcc.Class)
		// 		}
		// 	}
		// 	for _, tcm := range tc.Members {
		// 		ifm := &irANamedTypeRef{NamePs: tcm.Name, NameGo: sanitizeSymbolForGo(tcm.Name, true)}
		// 		ifm.setRefFrom(me.toIrATypeRef(tdict, tcm.Ref))
		// 		if ifm.RefFunc == nil {
		// 			if ifm.RefInterface != nil {
		// 				ifm.RefFunc = &irATypeRefFunc{
		// 					Rets: irANamedTypeRefs{&irANamedTypeRef{}},
		// 				}
		// 				ifm.RefFunc.Rets[0].setRefFrom(ifm.RefInterface)
		// 				ifm.RefInterface = nil
		// 			} else if len(ifm.RefAlias) > 0 {
		// 				ifm.RefFunc = &irATypeRefFunc{
		// 					Rets: irANamedTypeRefs{&irANamedTypeRef{RefAlias: ifm.RefAlias}},
		// 				}
		// 				ifm.RefAlias = ""
		// 			} else if ifm.RefArray != nil || ifm.RefPtr != nil || ifm.RefStruct != nil || ifm.RefUnknown > 0 {
		// 				panic(notImplErr("RefType", "ifm", me.mod.srcFilePath))
		// 			} else {
		// 				ifm.RefFunc = &irATypeRefFunc{
		// 					Rets: irANamedTypeRefs{&irANamedTypeRef{}},
		// 				}
		// 			}
		// 		} else {
		// 			ifm.RefFunc.Args[0].setBothNamesFromPsName("v")
		// 		}
		// 		gif.Methods = append(gif.Methods, ifm)
		// 	}
		// 	tgif := &irANamedTypeRef{Export: me.hasExport(tc.Name)}
		// 	tgif.setBothNamesFromPsName(tc.Name)
		// 	tgif.setRefFrom(gif)
		// 	me.GoTypeDefs = append(me.GoTypeDefs, tgif)
		// }
	} else {
		for _, tc := range me.EnvTypeClasses {
			tdict, gtd := map[string][]string{}, &irANamedTypeRef{Export: me.hasExport(tc.Name)}
			gtd.setBothNamesFromPsName(tc.Name)
			gtd.NameGo += "ˇ"
			gtd.RefStruct = &irATypeRefStruct{PassByPtr: true}
			for _, tcm := range tc.Members {
				tcmfield := &irANamedTypeRef{Export: true}
				tcmfield.setBothNamesFromPsName(tcm.Name)
				tcmfield.setRefFrom(me.toIrATypeRef(tdict, tcm.Ref))
				gtd.RefStruct.Fields = append(gtd.RefStruct.Fields, tcmfield)
			}
			me.GoTypeDefs = append(me.GoTypeDefs, gtd)
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
				panic(notImplErr("Prim type", tname, me.mod.srcFilePath))
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
			isnewtype, hasselfref, hasctorargs := false, false, false
			gid := &irANamedTypeRef{RefInterface: &irATypeRefInterface{xtd: td}, Export: me.hasExport(td.Name)}
			gid.setBothNamesFromPsName(td.Name)
			for _, ctor := range td.Ctors {
				if numargs := len(ctor.Args); numargs > 0 {
					if hasctorargs = true; numargs == 1 && numctors == 1 {
						if ctor.Args[0].TypeConstructor != (me.mod.qName + "." + td.Name) {
							isnewtype = true
						} else {
							hasselfref = true
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
						RefStruct: &irATypeRefStruct{PassByPtr: hasselfref || (len(ctor.Args) > 1 && hasctorargs)}}
					ctor.gtd.setBothNamesFromPsName(gid.NamePs + "ˇ" + ctor.Name)
					ctor.gtd.NamePs = ctor.Name
					for ia, ctorarg := range ctor.Args {
						field := &irANamedTypeRef{}
						field.setRefFrom(me.toIrATypeRef(tdict, ctorarg))
						ctorarg.tmp_assoc = field
						field.NameGo = fmt.Sprintf("%s%d", sanitizeSymbolForGo(ctor.Name, true), ia)
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
	if len(tr.TypeConstructor) > 0 {
		return tr.TypeConstructor
	} else if tr.REmpty {
		return nil
	} else if len(tr.TypeVar) > 0 {
		embeds := tdict[tr.TypeVar]
		if len(embeds) == 1 {
			return embeds[0]
		}
		return &irATypeRefInterface{Embeds: embeds}
	} else if tr.ConstrainedType != nil {
		if Proj.BowerJsonFile.Gonad.CodeGen.TypeClasses2Interfaces {
			// if len(tr.ConstrainedType.Args) == 0 || len(tr.ConstrainedType.Args[0].TypeVar) == 0 {
			// 	ensureIfaceForTvar(tdict, "", tr.ConstrainedType.Class) // TODO deal with this properly
			// } else {
			// 	ensureIfaceForTvar(tdict, tr.ConstrainedType.Args[0].TypeVar, tr.ConstrainedType.Class)
			// }
		}
		return me.toIrATypeRef(tdict, tr.ConstrainedType.Ref)
	} else if tr.ForAll != nil {
		return me.toIrATypeRef(tdict, tr.ForAll.Ref)
	} else if tr.Skolem != nil {
		return fmt.Sprintf("Skolem_%s_scope%d_value%d", tr.Skolem.Name, tr.Skolem.Scope, tr.Skolem.Value)
	} else if tr.RCons != nil {
		rectype := &irATypeRefStruct{PassByPtr: true, Fields: irANamedTypeRefs{&irANamedTypeRef{Export: true}}}
		rectype.Fields[0].setBothNamesFromPsName(tr.RCons.Label)
		rectype.Fields[0].setRefFrom(me.toIrATypeRef(tdict, tr.RCons.Left))

		if nextrow := me.toIrATypeRef(tdict, tr.RCons.Right); nextrow != nil {
			rectype.Fields = append(rectype.Fields, nextrow.(*irATypeRefStruct).Fields...)
		}
		return rectype
	} else if tr.TypeApp != nil {
		if tr.TypeApp.Left.TypeConstructor == "Prim.Record" {
			return me.toIrATypeRef(tdict, tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor == "Prim.Array" {
			array := &irATypeRefArray{Of: &irANamedTypeRef{}}
			array.Of.setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Right))
			return array
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" {
			funtype := &irATypeRefFunc{}
			funtype.Args = irANamedTypeRefs{&irANamedTypeRef{}}
			funtype.Args[0].setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Left.TypeApp.Right))
			funtype.Rets = irANamedTypeRefs{&irANamedTypeRef{}}
			funtype.Rets[0].setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Right))
			return funtype
		} else if len(tr.TypeApp.Left.TypeConstructor) > 0 {
			return me.toIrATypeRef(tdict, tr.TypeApp.Left)
			// } else {
			//	Nested stuff ie. (Either foo) bar
		}
	}
	return nil
}
