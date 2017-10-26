package main

import (
	"fmt"
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
			if ab := a.Base(); ab != nil && ctor.RefFunc != nil { // but first, check if type-class ctor-func
				if tc := me.irM.tc(ab.NamePs); tc != nil { // constructs type-class tc
					if gtd := me.irM.goTypeDefByPsName(tc.Name); gtd != nil && gtd.RefStruct != nil { // our struct for the type-class
						if numargs := len(ctor.RefFunc.Args); numargs != len(gtd.RefStruct.Fields) {
							panic(notImplErr("args-num mismatch for type-class ", tc.Name, me.mod.srcFilePath))
						} else { // for some freakish reason, ctor-func args are OFTEN BUT NOT ALWAYS in the same order as struct-from-type-syn fields: we fix the field order to match ctor-func args order
							reordered := make(irANamedTypeRefs, numargs, numargs)
							for i := 0; i < numargs; i++ {
								reordered[i] = gtd.RefStruct.Fields.byPsName(ctor.RefFunc.Args[i].NamePs)
							}
							gtd.RefStruct.Fields = reordered
						}
					}
				}
			}
			culled, me.culled.typeCtorFuncs = true, append(me.culled.typeCtorFuncs, ctor)
		}
		if !culled {
			me.add(a)
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
	me.add(nuglobals...)
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
		me.irM.ForeignImp = me.irM.ensureImp(strReplDot2Underscore.Replace(qn), "github.com/metaleap/gonad/"+strReplDot2Slash.Replace(qn), qn)
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
			case *irABlock:
				if a != nil { // any 2 consecutive mutually-negating ifs-without-elses, we flatten into a single if-else. usually bool pattern-matches
					var lastif *irAIf
					for i := 0; i < len(a.Body); i++ {
						switch thisif := a.Body[i].(type) {
						case *irAIf:
							if lastif == nil {
								lastif = thisif
							} else { // two ifs in a row
								if lastif.doesCondNegate(thisif) && lastif.Else == nil {
									lastif.Else = thisif.Then
									thisif.Then, lastif.Else.parent = nil, lastif
									a.Body = append(a.Body[:i], a.Body[i+1:]...)
								}
								lastif = nil
							}
						default:
							lastif = nil
						}
					}
				}
			}
		}
		return ast
	})
}

func (me *irAst) postEnsureArgTypes() {
	for again := true; again; again = false {
		me.walk(func(a irA) irA {
			switch ax := a.(type) {
			case *irAFunc:
				if !ax.RefFunc.haveAllArgsTypeInfo() {
					if len(ax.RefFunc.Rets) > 1 {
						panic(notImplErr("multiple ret-args in func", ax.NamePs, me.mod.srcFilePath))
					}
					if len(ax.RefFunc.Rets) > 0 && !ax.RefFunc.Rets[0].hasTypeInfo() {
						walk(ax.FuncImpl, false, func(stmt irA) irA {
							if !ax.RefFunc.Rets[0].hasTypeInfo() {
								if ret, _ := stmt.(*irARet); ret != nil {
									if tret := ret.ExprType(); tret != nil {
										ax.RefFunc.Rets[0].copyFrom(tret, false, true, false)
									}
								}
							}
							return stmt
						})
					}
					for _, arg := range ax.RefFunc.Args {
						if !arg.hasTypeInfo() {
							walk(ax.FuncImpl, false, func(stmt irA) irA {
								if !arg.hasTypeInfo() {
									if sym, _ := stmt.(*irASym); sym != nil && (sym.NamePs == arg.NamePs || sym.NameGo == arg.NameGo) {
										if tsym := sym.ExprType(); tsym != nil {
											arg.copyFrom(tsym, false, true, false)
										}
									}
								}
								return stmt
							})
						}
					}
				}
			}
			return a
		})
	}
}

func (me *irAst) postEnsureIfaceCasts() {
	me.perFunc(func(afn *irAFunc) {
		for i, a := range afn.FuncImpl.Body {
			switch ax := a.(type) {
			case *irAIf:
				axt := ax.If.ExprType()
				if axt == nil || axt.RefAlias != exprTypeBool.RefAlias {
					symname, pb := fmt.Sprintf("µˇ%v", i), ax.parent.(*irABlock)
					sym, av := ªSymGo(symname), ªLet(symname, "", ªTo(ax.If, "Prim", "Boolean"))
					pb.prepend(av)
					sym.parent, ax.If = ax, sym
				}
			default:
			}
		}
	})
}

func (me *irAst) postFixupAmpCtor(a *irAOp1, oc *irACall) irA {
	//	restore data-ctors from calls like (&CtorName(1, '2', "3")) to turn into DataNameˇCtorName{1, '2', "3"}
	var gtd *irANamedTypeRef
	var mod *modPkg
	if ocpkgsym, _ := oc.Callee.(*irAPkgSym); ocpkgsym != nil {
		if mod = findModuleByPName(ocpkgsym.PkgName); mod != nil {
			gtd = mod.irMeta.goTypeDefByPsName(ocpkgsym.Symbol)
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
					me.irM.ensureImp("reflect", "", "")
					oc.CallArgs[0].(*irALitStr).LitStr += strings.Repeat(", ‹%v› %v", (len(oc.CallArgs)-1)/2)[2:]
				}
			}
		}
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
						fndictarg.turnRefAliasIntoRefPtr()
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
	checkObj := func(tci *irMTypeClassInst, obj *irALitObj, gtd *irANamedTypeRef) (retmod *modPkg, retgtd *irANamedTypeRef) {
		if retmod, retgtd = findGoTypeByGoQName(me.mod, obj.RefAlias); retgtd != gtd {
			panic(notImplErr("obj-lit type-ref", obj.RefAlias, me.mod.srcFilePath))
		} else if len(obj.ObjFields) != len(gtd.RefStruct.Fields) {
			panic(notImplErr("fields mismatch between constructor and struct definition for type-class "+tci.ClassName+" instance", tci.Name, me.mod.srcFilePath))
		}
		return
	}
	me.walkTopLevelDefs(func(a irA) {
		if ab := a.Base(); a != nil {
			if tci := me.irM.tcInst(ab.NamePs); tci != nil {
				if tcmod, gtd := findGoTypeByPsQName(tci.ClassName); gtd == nil {
					panic(notImplErr("type-class '"+tci.ClassName+"' (its struct type-def wasn't found) for instance", tci.Name, me.mod.srcFilePath))
				} else {
					switch ax := a.(type) {
					case *irALet:
						switch axlv := ax.LetVal.(type) {
						case *irALitObj:
							checkObj(tci, axlv, gtd)
							for i := 0; i < len(gtd.RefStruct.Fields); i++ {
								switch fvx := axlv.ObjFields[i].FieldVal.(type) {
								case *irAFunc:
									fvx.RefFunc.copyArgTypesOnlyFrom(true, gtd.RefStruct.Fields[i].RefFunc)
								}
							}
							ax.RefAlias = axlv.RefAlias
						case *irAPkgSym:
							ax.RefAlias = tci.ClassName
						default:
							panicWithType(me.mod.srcFilePath, axlv, ab.NamePs+".LetVal")
						}
					case *irAFunc:
						if len(ax.RefFunc.Args) != 1 {
							panic(notImplErr(tci.ClassName+" type-class instance func args for", tci.Name, me.mod.srcFilePath))
						} else if fndictarg := ax.RefFunc.Args[0]; !strings.HasPrefix(fndictarg.NamePs, "dict") {
							panic(notImplErr(tci.ClassName+" type-class instance '"+tci.Name+"' func arg", fndictarg.NamePs, me.mod.srcFilePath))
						} else if len(ax.RefFunc.Rets) > 0 {
							panic(notImplErr(tci.ClassName+" type-class instance func ret-args for", tci.Name, me.mod.srcFilePath))
						} else if len(ax.RefFunc.impl.Body) != 1 {
							panic(notImplErr(tci.ClassName+" type-class instance func body for", tci.Name, me.mod.srcFilePath))
						} else if afnreturn, _ := ax.RefFunc.impl.Body[0].(*irARet); afnreturn == nil {
							panic(notImplErr(tci.ClassName+" type-class instance func body for", tci.Name, me.mod.srcFilePath))
						} else {
							if fndictarg.RefAlias = tci.ClassName; gtd.RefStruct.PassByPtr {
								fndictarg.turnRefAliasIntoRefPtr()
							}
							var retgtd *irANamedTypeRef
							var retmod *modPkg
							switch axr := afnreturn.RetArg.(type) {
							case *irALitObj:
								if retmod, retgtd = checkObj(tci, axr, gtd); retgtd.RefStruct.PassByPtr {
									afnreturn.RetArg = ªO1("&", axr)
								}
							case *irAFunc:
								fnretarg := irANamedTypeRef{RefFunc: axr.RefFunc.toSig(true)}
								ax.RefFunc.Rets = irANamedTypeRefs{&fnretarg}
							case *irASym:
								if axr.NamePs != fndictarg.NamePs {
									panic(notImplErr("return argument name '"+axr.NamePs+"', expected", fndictarg.NamePs, me.mod.srcFilePath))
								}
								retmod, retgtd = tcmod, gtd
							case *irACall:
								retmod, retgtd = tcmod, gtd
							default:
								panicWithType(me.mod.srcFilePath, axr, tci.Name)
							}
							if retgtd != nil {
								fnretarg := irANamedTypeRef{RefAlias: retgtd.NameGo}
								if retmod != nil && retmod != me.mod {
									fnretarg.RefAlias = retmod.pName + "." + fnretarg.RefAlias
								}
								if retgtd.RefStruct.PassByPtr {
									fnretarg.turnRefAliasIntoRefPtr()
								}
								ax.RefFunc.Rets = irANamedTypeRefs{&fnretarg}
							}
						}
					default:
						panicWithType(me.mod.srcFilePath, ax, tci.Name)
					}
				}
			}
		}
	})
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
