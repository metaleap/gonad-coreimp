package main

import (
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
		} else if cfgTc2Ifaces, ab := Proj.BowerJsonFile.Gonad.CodeGen.TypeClasses2Interfaces, a.Base(); cfgTc2Ifaces && ab != nil {
			// check if helper function related to type-classes / type-class instances:
			// tci := me.irM.tcInst(ab.NamePs)
			// if culled = (tci != nil); culled {
			// 	dictargname, tlval, again, tcinst := "", a, true, irTcInstImpl{tci: tci}
			// 	for again {
			// 		tldb := tlval.Base()
			// 		switch topleveldecl := tlval.(type) {
			// 		case *irAFunc:
			// 			if len(topleveldecl.RefFunc.Args) != 1 || !strings.HasPrefix(topleveldecl.RefFunc.Args[0].NamePs, "dict") {
			// 				panic(notImplErr("RefFunc args for", tldb.NamePs, me.mod.srcFilePath))
			// 			} else if fnret, _ := topleveldecl.RefFunc.impl.Body[0].(*irARet); fnret == nil || len(topleveldecl.RefFunc.impl.Body) != 1 {
			// 				panic(notImplErr("RefFunc rets for", tldb.NamePs, me.mod.srcFilePath))
			// 			} else {
			// 				dictargname = topleveldecl.RefFunc.Args[0].NamePs
			// 				tlval = fnret.RetArg
			// 				_, again = tlval.(*irAFunc)
			// 			}
			// 		case *irALet:
			// 			again = false
			// 			tlval = topleveldecl.LetVal
			// 		default:
			// 			panicWithType(me.mod.srcFilePath, tlval, tci.Name+":topleveldecl")
			// 		}
			// 	}
			// 	for again = true; again; again = false {
			// 		switch tlvx := tlval.(type) {
			// 		case *irAOp1:
			// 			if tcctorcall, _ := tlvx.Of.(*irACall); tcctorcall == nil || tlvx.Op1 != "&" {
			// 				panic(notImplErr(ab.NamePs+" operator and/or operand", tlvx.Op1, me.mod.srcFilePath))
			// 			} else {
			// 				var tcx interface{}
			// 				switch dotºsym := tcctorcall.Callee.(type) {
			// 				case *irASym:
			// 					tcinst.tciProper.tcMod, tcx = findPsTypeByQName(me.mod.qName + "." + dotºsym.NamePs)
			// 				case *irADot:
			// 					tcinst.tciProper.tcMod, tcx = findPsTypeByQName(findModuleByPName(dotºsym.DotLeft.Base().NamePs).qName + "." + dotºsym.DotRight.Base().NamePs)
			// 				}
			// 				switch maybetc := tcx.(type) {
			// 				case *irMTypeClass:
			// 					if tcname := tcinst.tciProper.tcMod.qName + "." + maybetc.Name; tcname != tci.ClassName {
			// 						panic(notImplErr(ab.NamePs+" instance type-class", tcname, me.mod.srcFilePath))
			// 					}
			// 					tcinst.tciProper.tc = maybetc
			// 				default:
			// 					panicWithType(me.mod.srcFilePath, maybetc, tci.Name+":tcx")
			// 				}
			// 				tcinst.tciProper.tcMemberImpls = tcctorcall.CallArgs
			// 			}
			// 		case *irASym:
			// 			if tlvx.NamePs != dictargname {
			// 				panic(notImplErr(tci.Name+" constructor pass-through", tlvx.NamePs, me.mod.srcFilePath))
			// 			}
			// 			tcinst.tciPassThrough = true
			// 		case *irACall:
			// 			again, tlval = true, tlvx.Callee.(*irADot) // keep the `dot`, go `again`, it'll jump to:
			// 		case *irADot:
			// 			tcinst.tciAlias = findModuleByPName(tlvx.DotLeft.Base().NamePs).qName + "." + tlvx.DotRight.Base().NamePs
			// 		default:
			// 			panicWithType(me.mod.srcFilePath, tlval, tci.Name+":tlvx")
			// 		}
			// 	}
			// 	me.culled.tcInstImpls = append(me.culled.tcInstImpls, &tcinst)
			// } else if culled = (me.irM.tcMember(ab.NamePs) != nil); culled {
			// 	me.culled.tcDictDecls = append(me.culled.tcDictDecls, a)
			// }
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

func (me *irAst) prepAddNewExtraTypesˇTypeClassInstances() {
	// var newextratypes irANamedTypeRefs
	// //	turn type-class instances into unexported 0-byte structs providing the corresponding interface-implementing method(s)
	// for _, tci := range me.irM.EnvTypeClassInsts {
	// 	if gid := findGoTypeByPsQName(tci.ClassName); gid == nil {
	// 		panic(notImplErr("type-class '"+tci.ClassName+"' (not found) for instance", tci.Name, me.mod.srcFilePath))
	// 	} else {
	// 		gtd := newextratypes.byPsName(tci.Name)
	// 		if gtd == nil {
	// 			gtd = &irANamedTypeRef{Export: false, RefStruct: &irATypeRefStruct{}}
	// 			gtd.setBothNamesFromPsName(tci.Name)
	// 			gtd.NameGo = "ıˇ" + gtd.NameGo
	// 			newextratypes = append(newextratypes, gtd)
	// 		}
	// 		for _, method := range gid.RefInterface.Methods {
	// 			mcopy := *method
	// 			gtd.RefStruct.Methods = append(gtd.RefStruct.Methods, &mcopy)
	// 		}
	// 	}
	// }
	// if len(newextratypes) > 0 {
	// 	sort.Sort(newextratypes)
	// 	for i, gtd := range newextratypes {
	// 		gtd.sortIndex = i + len(me.irM.GoTypeDefs)
	// 	}
	// 	me.irM.GoTypeDefs = append(me.irM.GoTypeDefs, newextratypes...)
	// }
}

func (me *irAst) prepFixupNameCasings() {
	ensure := func(gntr *irANamedTypeRef) *irANamedTypeRef {
		if gvd := me.irM.goValDeclByPsName(gntr.NamePs); gvd != nil {
			gntr.copyFrom(gvd, true, false, true)
			return gvd
		}
		return nil
	}
	me.walkTopLevelDefs(func(a irA) {
		if av, _ := a.(*irALet); av != nil {
			ensure(&av.irANamedTypeRef)
		} else if af, _ := a.(*irAFunc); af != nil {
			ensure(&af.irANamedTypeRef)
		}
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
	var mod *modPkg
	if ocdot, _ := oc.Callee.(*irADot); ocdot != nil {
		if ocdot1, _ := ocdot.DotLeft.(*irASym); ocdot1 != nil {
			if mod = findModuleByPName(ocdot1.NamePs); mod != nil {
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
		if mod != nil {
			o.RefAlias = mod.pName + "." + o.RefAlias
		}
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
						oc.CallArgs = append(oc.CallArgs, ªCall(ªPkgSym("reflect", "TypeOf"), nucallarg))
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
		// println("TODO:\t" + me.mod.srcFilePath + "\t" + ocv.NamePs)
	}
	return a
}

func (me *irAst) postLinkUpTcMemberFuncs() {
	me.walkTopLevelDefs(func(a irA) {
		if afn, _ := a.(*irAFunc); afn != nil {
			if tcm := me.irM.tcMember(afn.NamePs); tcm != nil {
				if len(afn.RefFunc.Args) != 1 {
					panic(notImplErr(tcm.tc.Name+" type-class member func args for", tcm.Name, me.mod.srcFilePath))
				} else if len(afn.RefFunc.Rets) > 0 {
					panic(notImplErr(tcm.tc.Name+" type-class member func ret-args for", tcm.Name, me.mod.srcFilePath))
				} else if fndictarg := afn.RefFunc.Args[0]; fndictarg.NamePs != "dict" {
					panic(notImplErr(tcm.tc.Name+" type-class member '"+tcm.Name+"' func arg", fndictarg.NamePs, me.mod.srcFilePath))
				} else if gtd := me.irM.goTypeDefByPsName(tcm.tc.Name); gtd == nil {
					panic(notImplErr("type-class '"+tcm.tc.Name+"' (its struct type-def wasn't found) for member", tcm.Name, me.mod.srcFilePath))
				} else {
					if fndictarg.RefAlias = gtd.NamePs; gtd.RefStruct.PassByPtr {
						fndictarg.RefPtr = &irATypeRefPtr{Of: &irANamedTypeRef{RefAlias: fndictarg.RefAlias}}
						fndictarg.RefAlias = ""
					}
					fnretarg := irANamedTypeRef{}
					fnretarg.copyFrom(gtd.RefStruct.Fields.byPsName(tcm.Name), false, true, false)
					afn.RefFunc.Rets = irANamedTypeRefs{&fnretarg}
				}
			}
		}
	})
}

func (me *irAst) postLinkUpTcInstDecls() {
	if !Proj.BowerJsonFile.Gonad.CodeGen.TypeClasses2Interfaces {
		me.walkTopLevelDefs(func(a irA) {
			if ab := a.Base(); a != nil {
				if tci := me.irM.tcInst(ab.NamePs); tci != nil {
					switch ax := a.(type) {
					case *irAFunc:
						if len(ax.RefFunc.Args) != 1 {
							panic(notImplErr(tci.ClassName+" type-class instance func args for", tci.Name, me.mod.srcFilePath))
						} else if fndictarg := ax.RefFunc.Args[0]; !strings.HasPrefix(fndictarg.NamePs, "dict") {
							panic(notImplErr(tci.ClassName+" type-class instance '"+tci.Name+"' func arg", fndictarg.NamePs, me.mod.srcFilePath))
						} else if len(ax.RefFunc.Rets) > 0 {
							panic(notImplErr(tci.ClassName+" type-class instance func ret-args for", tci.Name, me.mod.srcFilePath))
						} else if gtd := findGoTypeByPsQName(tci.ClassName); gtd == nil {
							panic(notImplErr("type-class '"+tci.ClassName+"' (its struct type-def wasn't found) for instance", tci.Name, me.mod.srcFilePath))
						} else if len(ax.RefFunc.impl.Body) != 1 {
							panic(notImplErr(tci.ClassName+" type-class instance func body for", tci.Name, me.mod.srcFilePath))
						} else if afnret, _ := ax.RefFunc.impl.Body[0].(*irARet); afnret == nil {
							panic(notImplErr(tci.ClassName+" type-class instance func body for", tci.Name, me.mod.srcFilePath))
						} else {
							if fndictarg.RefAlias = gtd.NamePs; gtd.RefStruct.PassByPtr {
								fndictarg.RefPtr = &irATypeRefPtr{Of: &irANamedTypeRef{RefAlias: fndictarg.RefAlias}}
								fndictarg.RefAlias = ""
							}
							fnretarg := irANamedTypeRef{}
							switch axr := afnret.RetArg.(type) {
							case *irALitObj:
								if ctorgtd := findGoTypeByGoQName(me.mod, axr.RefAlias); ctorgtd != gtd {
									panic(notImplErr("obj-lit type-ref", axr.RefAlias, me.mod.srcFilePath))
								} else {
									fnretarg.copyFrom(&axr.irANamedTypeRef, false, true, false)
								}
							case *irAFunc:
							case *irASym:
							case *irACall:
							default:
								panicWithType(me.mod.srcFilePath, axr, tci.Name)
							}
							ax.RefFunc.Rets = irANamedTypeRefs{&fnretarg}
						}
					case *irALet:
					default:
						panicWithType(me.mod.srcFilePath, ax, tci.Name)
					}
				}
			}
		})
	} else {
		// for _, impl := range me.culled.tcInstImpls {
		// 	if impl.tciPassThrough {
		// 		//	not sure yet how to handle =)
		// 	} else {
		// 		gtdinst := me.irM.goTypeDefByPsName(impl.tci.Name) // the private implementer struct-type
		// 		instvar := ªLet("", "", nil)
		// 		instvar.Export = me.irM.hasExport(impl.tci.Name)
		// 		instvar.setBothNamesFromPsName(impl.tci.Name)
		// 		nuctor := ªO(&irANamedTypeRef{RefAlias: gtdinst.NameGo})
		// 		nuctor.parent = instvar
		// 		instvar.LetVal = nuctor
		// 		instvar.RefAlias = impl.tci.ClassName
		// 		me.Prepend(instvar)
		// 		if len(impl.tciAlias) > 0 {
		// 			println("ALIAS:\t" + me.mod.srcFilePath + "\t" + impl.tci.Name)
		// 		} else {
		// 			for _, tcim := range impl.tciProper.tcMemberImpls {
		// 				if tcim != nil {
		// 				}
		// 			}
		// 		}
		// 	}
		// }

		/* BELOW: OLDER; KEEP COMMENTED WHEN RESUMING THE ABOVE */

		// for _, ifx := range me.culled.tcInstImpls {
		// 	instvar, _ := ifx.(*irALet)
		// 	instvar.Export = me.irM.hasExport(instvar.NamePs)
		// 	instvar.setBothNamesFromPsName(instvar.NamePs)
		// 	gtdinst := me.irM.goTypeDefByPsName(instvar.NamePs) // the private implementer struct-type
		// 	tcmod, tcx := findPsTypeByQName(gtdinst.RefStruct.instOf)
		// 	tc := tcx.(*irMTypeClass)

		// 	if tcjsctorfunc := tcmod.irAst.typeCtorFunc(tc.Name); tcjsctorfunc == nil {
		// 		panic(me.mod.srcFilePath + ": type-class ctor func not found for type-class '" + tc.Name + "' / '" + gtdinst.RefStruct.instOf + "' of instance '" + instvar.NamePs + "', please report")
		// 	} else {
		// 		for tcjsctorfuncargindex, tcjsctorfuncarg := range tcjsctorfunc.RefFunc.Args { // we want to preserve this ordering
		// 			for _, gtdinstmethod := range gtdinst.RefStruct.Methods { // we find and use the impl-struct method..
		// 				if gtdinstmethod.NamePs == tcjsctorfuncarg.NamePs { // .. associated with this JS-ctor-func arg
		// 					if tcjsctorfuncargindex >= 0 {
		// 					}
		// 					switch instvarx := instvar.LetVal.(type) {
		// 					case *irALitObj:
		// 						panic("This again?!")
		// 						ifofv := instvarx.ObjFields[tcjsctorfuncargindex].FieldVal
		// 						switch ifa := ifofv.(type) {
		// 						case *irAFunc:
		// 							gtdinstmethod.RefFunc.impl = ifa.FuncImpl
		// 						default:
		// 							oldp := ifofv.Parent()
		// 							gtdinstmethod.RefFunc.impl = ªBlock(ªRet(ifofv))
		// 							gtdinstmethod.RefFunc.impl.parent = oldp
		// 						}
		// 					case *irADot:
		// 						callargs := []irA{}
		// 						for _, gma := range gtdinstmethod.RefFunc.Args {
		// 							callargs = append(callargs, ªSymGo(gma.NameGo))
		// 						}
		// 						gtdinstmethod.RefFunc.impl = ªBlock(ªRet(ªCall(instvarx, callargs...)))
		// 					case *irAOp1:
		// 						if opcall, _ := instvarx.Of.(*irACall); opcall == nil || instvarx.Op1 != "&" {
		// 							panic(notImplErr("operator and/or operand", instvarx.Op1, me.mod.srcFilePath))
		// 						} else if opcb := opcall.Callee.Base(); opcb.NamePs != tcjsctorfunc.NamePs {
		// 							panic(notImplErr("tc-inst-ctor callee", opcb.NamePs, me.mod.srcFilePath))
		// 						} else {
		// 							if strings.HasPrefix(me.mod.srcFilePath, "src/") || strings.Contains(me.mod.srcFilePath, "Data/Show") {
		// 								println(opcall.Callee.Base().NamePs)
		// 							}
		// 						}
		// 					case *irAFunc:
		// 						if strings.HasPrefix(me.mod.srcFilePath, "src/") || strings.Contains(me.mod.srcFilePath, "Data/Show") {
		// 							println("FUN\t" + me.mod.srcFilePath + "\t\t" + instvar.NamePs)
		// 						}
		// 					default:
		// 						panicWithType(me.mod.srcFilePath, instvar.LetVal, )
		// 					}
		// 					break
		// 				}
		// 			}
		// 		}
		// 	}
		// 	nuctor := ªO(&irANamedTypeRef{RefAlias: gtdinst.NameGo})
		// 	nuctor.parent = instvar
		// 	instvar.LetVal = nuctor
		// 	instvar.RefAlias = gtdinst.RefStruct.instOf
		// 	me.Prepend(instvar)
		// }
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
				panic(notImplErr("lack of RefFunc in irAFunc", a.NameGo+"/"+a.NamePs, me.mod.srcFilePath))
			}
		}
		return ast
	})
}
