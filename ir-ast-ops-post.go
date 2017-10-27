package main

import (
	"fmt"
	"strings"
)

/*
Golang intermediate-representation AST:
various transforms and operations on the AST,
"prep" ops are called from prepFromCoreImp
and "post" ops are called from finalizePostPrep.
*/

func (me *irAst) finalizePostPrep() {
	//	various fix-ups
	me.walk(func(ast irA) irA {
		if ast != nil {
			switch a := ast.(type) {
			case *irAOp1:
				if a != nil && a.Op1 == "&" {
					if oc, _ := a.Of.(*irACall); oc != nil {
						return me.postFixupAmpCtor(a, oc)
					}
				}
			}
		}
		return ast
	})

	me.postLinkUpTcMemberFuncs()
	me.postLinkUpTcInstDecls()
	me.postMiscFixups()
	me.postEnsureArgTypes()
	me.postPerFuncFixups()
}

func (me *irAst) postEnsureArgTypes() {
	me.perFuncDown(true, func(istoplevel bool, fn *irAFunc) {
		if !fn.RefFunc.haveAllArgsTypeInfo() {
			if len(fn.RefFunc.Rets) > 1 {
				panic(notImplErr("multiple ret-args in func", fn.NamePs, me.mod.srcFilePath))
			}
			if istoplevel {
				tldname := fn.NamePs
				if tldname == "" {
					println(me.mod.srcFilePath)
					panic(fmt.Sprintf("%T", fn.parent))
				}
			} else {
				if len(fn.RefFunc.Rets) > 0 && !fn.RefFunc.Rets[0].hasTypeInfo() {
					walk(fn.FuncImpl, false, func(stmt irA) irA {
						if !fn.RefFunc.Rets[0].hasTypeInfo() {
							if ret, _ := stmt.(*irARet); ret != nil {
								if tret := ret.ExprType(); tret != nil {
									fn.RefFunc.Rets[0].copyFrom(tret, false, true, false)
								}
							}
						}
						return stmt
					})
				}
				for _, arg := range fn.RefFunc.Args {
					if !arg.hasTypeInfo() {
						walk(fn.FuncImpl, false, func(stmt irA) irA {
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
			if !fn.RefFunc.haveAllArgsTypeInfo() {
				if fnretouter, _ := fn.parent.(*irARet); fnretouter != nil {
					if fnouter, _ := fnretouter.parent.Parent().(*irAFunc); fnouter != nil {
						if fnretsig := fnouter.RefFunc.Rets[0].RefFunc; fnretsig != nil {
							if len(fnretsig.Args) != len(fn.RefFunc.Args) || len(fnretsig.Rets) != len(fn.RefFunc.Rets) {
								panic(notImplErr("func-args count mismatch", fnouter.NamePs, me.mod.srcFilePath))
							} else {
								for i, a := range fnretsig.Args {
									fn.RefFunc.Args[i].copyFrom(a, false, true, false)
								}
								fn.RefFunc.Rets[0].copyFrom(fnretsig.Rets[0], false, true, false)
							}
						}
					}
				}
			}
		}
	})
}

func (me *irAst) postPerFuncFixups() {
	var namescache map[string]string
	convertToTypeOf := func(i int, afn *irAFunc, from irA, totype *irANamedTypeRef) (int, *irASym) {
		symname, varname := from.Base().NameGo, ªSymGo(fmt.Sprintf("ˇ%cˇ", rune(i+97)))
		varname.exprType = totype
		if existing, _ := namescache[symname]; symname != "" && existing != "" {
			varname.NameGo = existing
		} else {
			if symname != "" {
				namescache[symname] = varname.NameGo
			}
			pname, tname := me.resolveGoTypeRefFromQName(totype.RefAlias)
			vardecl := ªLet(varname.NameGo, "", ªTo(from, pname, tname))
			vardecl.exprType = totype
			afn.FuncImpl.insert(i, vardecl)
			i++
		}
		return i, varname
	}
	me.perFuncDown(true, func(istoplevel bool, afn *irAFunc) {
		fargsused := me.countSymRefs(afn.RefFunc.Args)
		for _, farg := range afn.RefFunc.Args {
			if farg.NameGo != "" && fargsused[farg.NameGo] == 0 {
				farg.NameGo = "_"
			}
		}
		if istoplevel { // each top-level func keeps its own fresh names-cache
			namescache = map[string]string{}
		}
		for i := 0; i < len(afn.FuncImpl.Body); i++ {
			var varname *irASym
			switch ax := afn.FuncImpl.Body[i].(type) {
			case *irAIf: // if condition isn't bool (eg testing an interface{}), convert it first to a temp bool var
				axt := ax.If.ExprType()
				if axt == nil || axt.RefAlias != exprTypeBool.RefAlias {
					switch axcond := ax.If.(type) {
					case *irAOp1:
						i, varname = convertToTypeOf(i, afn, axcond.Of.(*irASym), exprTypeBool)
						axcond.Of, varname.parent = varname, axcond
					case *irASym:
						i, varname = convertToTypeOf(i, afn, axcond, exprTypeBool)
						ax.If, varname.parent = varname, ax
					}
				}
			default:
				walk(ax, false, func(ast irA) irA {
					switch a := ast.(type) {
					case *irARet:
						if afn.RefFunc.Rets[0].wellTyped() && a.RetArg != nil {
							if asym, _ := a.RetArg.(*irASym); asym != nil {
								if tsym := asym.ExprType(); tsym == nil || !tsym.hasTypeInfo() || !tsym.wellTyped() {
									if asym.NameGo == "defaultEmptyish" {
										println(asym.ExprType().RefAlias)
									}
									i, varname = convertToTypeOf(i, afn, a.RetArg, afn.RefFunc.Rets[0])
									a.RetArg, varname.parent = varname, a
								}
							}
						}
					case *irAOp1:
						if !a.Of.Base().wellTyped() {
							if a.Op1 == "!" {
								i, varname = convertToTypeOf(i, afn, a.Of, exprTypeBool)
								a.Of, varname.parent = varname, a
							}
						}
					case *irAOp2:
						tl, tr := a.Left.ExprType(), a.Right.ExprType()
						ul, ur := !tl.wellTyped(), !tr.wellTyped()
						if sl, _ := a.Left.(*irASym); ul && (!ur) && sl != nil {
							i, varname = convertToTypeOf(i, afn, sl, a.Right.ExprType())
							a.Left, varname.parent = varname, a
						} else if sr, _ := a.Right.(*irASym); (!ul) && ur && sr != nil {
							i, varname = convertToTypeOf(i, afn, sr, a.Left.ExprType())
							a.Right, varname.parent = varname, a
						}
					}
					return ast
				})
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
				if tcmod, gtd := findGoTypeByPsQName(tci.ClassName); gtd == nil || gtd.RefStruct == nil {
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
							if ax.RefAlias = axlv.RefAlias; gtd.RefStruct.PassByPtr {
								ax.turnRefAliasIntoRefPtr()
								axctor := ªO1("&", axlv)
								axlv.parent, axctor.parent = axctor, ax
								ax.LetVal = axctor
							}
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
		// case *irADot:
		// 	if atl := a.DotLeft.ExprType(); atl != nil && atl.RefAlias != "" {
		// 		if gtd:=findGoTypeByGoQName(me, qname)
		// 		println(me.mod.srcFilePath + "\t\t" + atl.RefAlias + "\t\t" + a.symStr())
		// 	}
		case *irALet:
			if a != nil && a.isConstable() {
				//	turn var=literal's into consts
				c := ªConst(&a.irANamedTypeRef, a.LetVal)
				c.exprType = a.ExprType()
				c.parent = a.parent
				return c
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
