package main

import (
	"sort"
	"strings"
)

/*
Golang intermediate-representation AST:
various transforms and operations on the AST,
"prep" ops are called from PrepFromCoreImp
and "post" ops are called from FinalizePostPrep.
*/

func (me *irAst) prepAddOrCull(a irA) {
	if a != nil {
		culled := false
		if ctor, _ := a.(*irACtor); ctor != nil {
			// PureScript CoreImp contains constructor functions for each ADT "sub-type", we drop those
			culled, me.culled.typeCtorFuncs = true, append(me.culled.typeCtorFuncs, ctor)
		} else if ab := a.Base(); ab != nil {
			// check if helper function related to type-classes / type-class instances:
			if culled = me.irM.tcInst(ab.NamePs) != nil; culled {
				// func instname(..)
				if afn, _ := a.(*irAFunc); afn != nil {
					p := afn.parent
					av := ªLet(afn.NameGo, afn.NamePs, afn)
					av.parent = p
					a = av
				}
				me.culled.tcInstDecls = append(me.culled.tcInstDecls, a)
			} else if culled = me.irM.tcMember(ab.NamePs) != nil; culled {
				me.culled.tcDictDecls = append(me.culled.tcDictDecls, a)
			}
		}
		if !culled {
			me.Add(a)
		}
	}
}

func (me *irAst) prepAddEnumishAdtGlobals() (nuglobalsmap map[string]*irALet) {
	//	add private globals to represent all arg-less ctors (ie. "one const per enum-value")
	nuglobals := []irA{}
	nuglobalsmap = map[string]*irALet{}
	for _, gtd := range me.irM.GoTypeDefs {
		if gtd.RefInterface != nil && gtd.RefInterface.xtd != nil {
			for _, ctor := range gtd.RefInterface.xtd.Ctors {
				if ctor.gtd != nil && len(ctor.Args) == 0 {
					nuvar := ªLet("º"+ctor.Name, "", ªO(&irANamedTypeRef{RefAlias: ctor.gtd.NameGo}))
					nuglobalsmap[ctor.Name] = nuvar
					nuglobals = append(nuglobals, nuvar)
				}
			}
		}
	}
	me.Add(nuglobals...)
	return
}

func (me *irAst) prepAddNewExtraTypes() {
	var newextratypes irANamedTypeRefs
	//	turn type-class instances into unexported 0-byte structs providing the corresponding interface-implementing method(s)
	for _, tci := range me.irM.EnvTypeClassInsts {
		if gid := findGoTypeByPsQName(tci.ClassName); gid == nil {
			panic(me.mod.srcFilePath + ": type-class '" + tci.ClassName + "' not found for instance '" + tci.Name + "'")
		} else {
			gtd := newextratypes.byPsName(tci.Name)
			if gtd == nil {
				gtd = &irANamedTypeRef{Export: false, RefStruct: &irATypeRefStruct{instOf: tci.ClassName}}
				gtd.setBothNamesFromPsName(tci.Name)
				gtd.NameGo = "ı" + gtd.NameGo
				newextratypes = append(newextratypes, gtd)
			}
			for _, method := range gid.RefInterface.Methods {
				mcopy := *method
				gtd.RefStruct.Methods = append(gtd.RefStruct.Methods, &mcopy)
			}
		}
	}
	if len(newextratypes) > 0 {
		sort.Sort(newextratypes)
		for i, gtd := range newextratypes {
			gtd.sortIndex = i + len(me.irM.GoTypeDefs)
		}
		me.irM.GoTypeDefs = append(me.irM.GoTypeDefs, newextratypes...)
	}
}

func (me *irAst) prepFixupExportedNames() {
	ensure := func(isfunc bool, gntr *irANamedTypeRef) *irANamedTypeRef {
		if gvd := me.irM.goValDeclByPsName(gntr.NamePs); gvd != nil {
			gntr.copyFrom(gvd, true, !isfunc, true)
			return gvd
		}
		return nil
	}
	me.topLevelDefs(func(a irA) bool {
		if av, _ := a.(*irALet); av != nil {
			ensure(false, &av.irANamedTypeRef)
		} else if af, _ := a.(*irAFunc); af != nil {
			if gvd := ensure(true, &af.irANamedTypeRef); gvd != nil {
				if gvd.RefFunc == nil {
					panic(notImplErr("NIL RefFunc for", gvd.NamePs, me.mod.srcFilePath))
				} else {
					for i, gvdfuncarg := range gvd.RefFunc.Args {
						af.RefFunc.Args[i].copyFrom(gvdfuncarg, false, true, false)
					}
					if len(af.RefFunc.Rets) > 0 {
						panic(notImplErr("RET values for", gvd.NamePs, me.mod.srcFilePath))
					}
					for i, gvdfuncret := range gvd.RefFunc.Rets {
						af.RefFunc.Rets = append(af.RefFunc.Rets, &irANamedTypeRef{})
						af.RefFunc.Rets[i].copyFrom(gvdfuncret, true, true, false)
					}
				}
			}

		}
		return false
	})
}

func (me *irAst) prepForeigns() {
	if reqforeign := me.mod.coreimp.namedRequires["$foreign"]; len(reqforeign) > 0 {
		qn := nsPrefixDefaultFfiPkg + me.mod.qName
		me.irM.ForeignImp = me.irM.Imports.addIfHasnt(strReplDot2Underscore.Replace(qn), "github.com/metaleap/gonad/"+strReplDot2Slash.Replace(qn), qn)
		me.irM.save = true
	}
}

func (me *irAst) prepMiscFixups(nuglobalsmap map[string]*irALet) {
	me.walk(func(ast irA) irA {
		if ast != nil {
			switch a := ast.(type) {
			case *irAOp2: // coreimp represents Ints JS-like as: expr|0 --- we ditch the |0 part
				if opright, _ := a.Right.(*irALitInt); opright != nil && a.Op2 == "|" && opright.LitInt == 0 {
					return a.Left
				}
			case *irADot:
				if dl, _ := a.DotLeft.(*irASym); dl != nil {
					if dr, _ := a.DotRight.(*irASym); dr != nil {
						//	find all CtorName.value references and change them to the new globals created in AddEnumishAdtGlobals
						if dr.NameGo == "value" {
							if nuglobalvar := nuglobalsmap[dl.NamePs]; nuglobalvar != nil {
								sym4nuvar := ªSymGo(nuglobalvar.NameGo)
								sym4nuvar.irANamedTypeRef = nuglobalvar.irANamedTypeRef
								return sym4nuvar
							}
						} else {
							//	if the dot's LHS refers to a package, ensure the import is marked as in-use and switch out dot for pkgsym
							for _, imp := range me.irM.Imports {
								if imp.GoName == dl.NameGo || (dl.NamePs == "$foreign" && imp == me.irM.ForeignImp) {
									dr.Export = true
									dr.NameGo = sanitizeSymbolForGo(dr.NameGo, dr.Export)
									return ªPkgSym(imp.GoName, dr.NameGo)
								}
							}
						}
					}
				}
			}
		}
		return ast
	})
}

func (me *irAst) postFixupAmpCtor(a *irAOp1, oc *irACall) irA {
	//	restore data-ctors from calls like (&CtorName(1, '2', "3")) to turn into DataNameˇCtorName{1, '2', "3"}
	var gtd *irANamedTypeRef
	if ocdot, _ := oc.Callee.(*irADot); ocdot != nil {
		if ocdot1, _ := ocdot.DotLeft.(*irASym); ocdot1 != nil {
			if mod := findModuleByPName(ocdot1.NamePs); mod != nil {
				if ocdot2, _ := ocdot.DotRight.(*irASym); ocdot2 != nil {
					gtd = mod.irMeta.goTypeDefByPsName(ocdot2.NamePs)
				}
			}
		}
	}
	ocv, _ := oc.Callee.(*irASym)
	if gtd == nil && ocv != nil {
		gtd = me.irM.goTypeDefByPsName(ocv.NamePs)
	}
	if gtd != nil {
		o := ªO(&irANamedTypeRef{RefAlias: gtd.NameGo})
		for _, ctorarg := range oc.CallArgs {
			of := ªOFld(ctorarg)
			of.parent = o
			o.ObjFields = append(o.ObjFields, of)
		}
		return o
	} else if ocv != nil && ocv.NamePs == "Error" {
		if len(oc.CallArgs) == 1 {
			if op2, _ := oc.CallArgs[0].(*irAOp2); op2 != nil && op2.Op2 == "+" {
				oc.CallArgs[0] = op2.Left
				op2.Left.Base().parent = oc
				if oparr := op2.Right.(*irALitArr); oparr != nil {
					for _, oparrelem := range oparr.ArrVals {
						nucallarg := oparrelem
						if oaedot, _ := oparrelem.(*irADot); oaedot != nil {
							if oaedot2, _ := oaedot.DotLeft.(*irADot); oaedot2 != nil {
								nucallarg = oaedot2.DotLeft
							} else {
								nucallarg = oaedot
							}
						}
						oc.CallArgs = append(oc.CallArgs, ªCall(ªDotNamed("reflect", "TypeOf"), nucallarg))
						oc.CallArgs = append(oc.CallArgs, nucallarg)
					}
				}
				if len(oc.CallArgs) > 1 {
					me.irM.Imports.addIfHasnt("reflect", "reflect", "")
					me.irM.save = true
					oc.CallArgs[0].(*irALitStr).LitStr += strings.Repeat(", ‹%v› %v", (len(oc.CallArgs)-1)/2)[2:]
				}
			}
		}
		me.irM.Imports.addIfHasnt("fmt", "fmt", "")
		me.irM.save = true
		call := ªCall(ªPkgSym("fmt", "Errorf"), oc.CallArgs...)
		return call
	} else if ocv != nil {
		println("TODO:\t" + me.mod.srcFilePath + "\t" + ocv.NamePs)
	}
	return a
}

func (me *irAst) postLinkTcInstFuncsToImplStructs() {
	for _, ifx := range me.culled.tcInstDecls {
		instvar, _ := ifx.(*irALet)
		instvar.Export = me.irM.hasExport(instvar.NamePs)
		instvar.setBothNamesFromPsName(instvar.NamePs)
		gtdinst := me.irM.goTypeDefByPsName(instvar.NamePs) // the private implementer struct-type
		tcmod, tcx := findPsTypeByQName(gtdinst.RefStruct.instOf)
		tc := tcx.(*irMTypeClass)

		if tcctor := tcmod.irAst.typeCtorFunc(tc.Name); tcctor == nil {
			panic(me.mod.srcFilePath + ": type-class ctor func not found for type-class '" + tc.Name + "' / '" + gtdinst.RefStruct.instOf + "' of instance '" + instvar.NamePs + "', please report")
		} else {
			// for i, instfuncarg := range tcctor.RefFunc.Args {
			// 	for _, gtdinst := range gtdinst.RefStruct.Methods {
			// 		if gtdinst.NamePs == instfuncarg.NamePs {
			// 			switch instvarx := instvar.LetVal.(type) {
			// 			case *irALitObj:
			// 				panic("This again?!")
			// 				ifofv := instvarx.ObjFields[i].FieldVal
			// 				switch ifa := ifofv.(type) {
			// 				case *irAFunc:
			// 					gtdinst.RefFunc.impl = ifa.FuncImpl
			// 				default:
			// 					oldp := ifofv.Parent()
			// 					gtdinst.RefFunc.impl = ªBlock(ªRet(ifofv))
			// 					gtdinst.RefFunc.impl.parent = oldp
			// 				}
			// 			case *irADot:
			// 				callargs := []irA{}
			// 				for _, gma := range gtdinst.RefFunc.Args {
			// 					callargs = append(callargs, ªSymGo(gma.NameGo))
			// 				}
			// 				gtdinst.RefFunc.impl = ªBlock(ªRet(ªCall(instvarx, callargs...)))
			// 			case *irAOp1:
			// 				// println("OP1\t" + me.mod.srcFilePath + "\t\t" + instvar.NamePs)
			// 			case *irAFunc:
			// 				// println("FUN\t" + me.mod.srcFilePath + "\t\t" + instvar.NamePs)
			// 			default:
			// 				println(instvar.LetVal.(*irALitObj).NamePs)
			// 			}
			// 			break
			// 		}
			// 	}
			// }
		}
		nuctor := ªO(&irANamedTypeRef{RefAlias: gtdinst.NameGo})
		nuctor.parent = instvar
		instvar.LetVal = nuctor
		instvar.RefAlias = gtdinst.RefStruct.instOf
		me.Prepend(instvar)
	}
}

func (me *irAst) postMiscFixups() {
	me.walk(func(ast irA) irA {
		switch a := ast.(type) {
		case *irALet:
			if a != nil && a.isConstable() {
				//	turn var=literal's into consts
				return ªConst(&a.irANamedTypeRef, a.LetVal)
			}
		case *irAFunc:
			if a.irANamedTypeRef.RefFunc != nil {
				// coreimp doesn't give us return-args for funcs, prep them with interface{} initially
				if len(a.irANamedTypeRef.RefFunc.Rets) == 0 { // but some do have ret-args from prior gonad ops
					// otherwise, add an empty-for-now 'unknown' (aka interface{}) return type
					a.irANamedTypeRef.RefFunc.Rets = irANamedTypeRefs{&irANamedTypeRef{}}
				}
			} else {
				panic(me.mod.srcFilePath + ": please report as bug, a irAFunc ('" + a.NameGo + "' / '" + a.NamePs + "') had no RefFunc set")
			}
		}
		return ast
	})
}
