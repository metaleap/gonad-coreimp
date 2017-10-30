package main

import (
	"fmt"
	"strings"
)

/*
Golang intermediate-representation AST:
various transforms and operations on the AST,
"prep" ops are called from prepFromCoreImp
and "post" ops are called from finalizePostPrepOps.
*/

func (me *irAst) finalizePostPrepOps() {
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
	me.postInitialFixups()
	me.postEnsureArgTypes()
	me.postPerFuncFixups()
	me.postFinalFixups()
}

func (me *irAst) postEnsureArgTypes() {
	//	first the top-level funcs: no guesswork here, we have the full signature from coreimp.json:declEnv
	for _, a := range me.topLevelDefs(nil) {
		switch atld := a.(type) {
		case *irAFunc:
			if atld.NamePs == "" {
				panic(fmt.Sprintf("%T", atld.parent))
			} else if gvd := me.irM.goValDeclByPsName(atld.NamePs); gvd != nil && gvd.RefFunc != nil {
				if tlcmem := me.irM.tcMember(atld.NamePs); tlcmem == nil {
					atld.RefFunc.copyArgTypesOnlyFrom(false, gvd.RefFunc)
				}
			}
		case *irALet:
			if !atld.ExprType().hasTypeInfo() {
				if gvd := me.irM.goValDeclByPsName(atld.NamePs); gvd != nil {
					atld.copyTypeInfoFrom(gvd)
					if lval := atld.LetVal.Base(); !lval.hasTypeInfo() {
						lval.copyTypeInfoFrom(gvd)
					}
				}
			}
		}
	}
	//	now we're better equipped for further "guesswork" down the line:
	me.perFuncDown(func(fn *irAFunc) {
		if fn.isTopLevel() { // we just did those, we just want to patch up all the scattered anonymous inner-funcs
			return
		}
		if !fn.RefFunc.haveAllArgsTypeInfo() {
			if len(fn.RefFunc.Rets) > 1 {
				panic(notImplErr("multiple ret-args in func", fn.NamePs, me.mod.srcFilePath))
			}
			if !fn.RefFunc.Rets[0].hasTypeInfo() {
				walk(fn.FuncImpl, false, func(stmt irA) irA {
					if !fn.RefFunc.Rets[0].hasTypeInfo() {
						if ret, _ := stmt.(*irARet); ret != nil {
							if tret := ret.ExprType(); tret.hasTypeInfo() {
								fn.RefFunc.Rets[0].copyTypeInfoFrom(tret)
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
									arg.copyTypeInfoFrom(tsym)
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
							fn.RefFunc.copyArgTypesOnlyFrom(false, fnretsig)
						}
					}
				}
			} else if fnletouter, _ := fn.parent.(*irALet); fnletouter != nil {
				if fnouter, _ := fnletouter.parent.Parent().(*irAFunc); fnouter != nil && fnouter.RefFunc.haveAllArgsTypeInfo() {
					for _, fnret := range irALookupBelowˇRet(fnouter.FuncImpl, false) {
						if fnretcall, _ := fnret.RetArg.(*irACall); fnretcall != nil {
							if fnretcallsym, _ := fnretcall.Callee.(*irASym); fnretcallsym != nil && fnretcallsym.NamePs == fnletouter.NamePs {
								fn.RefFunc.copyArgTypesOnlyFrom(false, fnouter.RefFunc)
							}
						} else if fnretsym, _ := fnret.RetArg.(*irASym); fnretsym != nil {
							if symref := fnretsym.refTo(); symref != nil {
								if (!fnretsym.ExprType().hasTypeInfo()) && fnouter.RefFunc.Rets[0].hasTypeInfo() {
									if !symref.ExprType().hasTypeInfo() {
										symref.Base().copyTypeInfoFrom(fnouter.RefFunc.Rets[0])
									}
								}
								if symvar, _ := symref.(*irALet); symvar != nil {
									if symvarset := symvar.setterFromCallTo(fnletouter); symvarset != nil {
										if fnretsym.ExprType().hasTypeInfo() {
											fn.RefFunc.Rets[0].copyTypeInfoFrom(fnretsym.ExprType())
										}
									}
								}
							}
						}
					}
				}
			}
		}
	})
}

func (me *irAst) postFinalFixups() {
	me.walk(func(ast irA) irA {
		switch a := ast.(type) {
		case *irALitObj:
			//	record literals that *could* be matched to an existing struct (hopefully all of them) now need field names fixed up
			if atl := a.ExprType(); atl.RefAlias != "" {
				if _, gtd := findGoTypeByPsQName(me.mod, atl.RefAlias); gtd != nil && gtd.RefStruct != nil {
					if numfields := len(gtd.RefStruct.Fields); numfields != len(a.ObjFields) {
						panic(notImplErr("field-count mismatch", atl.RefAlias, me.mod.srcFilePath))
					} else if a.fieldsNamed() {
						for _, objfield := range a.ObjFields {
							if gtdfield := gtd.RefStruct.Fields.byPsName(objfield.NamePs); gtdfield != nil {
								objfield.NameGo = gtdfield.NameGo
							} else {
								panic(notImplErr("object field name '"+objfield.NamePs+"' not defined for", atl.RefAlias, me.mod.srcFilePath))
							}
						}
					}
				}
			}
		case *irADot:
			//	some dots' rhs refers to a member (field/method) of the lhs and needs the name fixed up
			if atl := a.DotLeft.ExprType(); atl.RefAlias != "" {
				if _, gtd := findGoTypeByPsQName(me.mod, atl.RefAlias); gtd == nil || gtd.RefStruct == nil {
					// panic(notImplErr("unresolvable expression-type ref-alias", atl.RefAlias, me.mod.srcFilePath))
				} else {
					asym := a.DotRight.(*irASym)
					fname := asym.NamePs
					if gtdm := gtd.RefStruct.memberByPsName(fname); gtdm != nil {
						asym.NameGo = gtdm.NameGo
					}
				}
			}
		}
		return ast
	})
}

func (me *irAst) postFixupAmpCtor(a *irAOp1, oc *irACall) irA {
	//	restore data-ctors from calls like (&CtorName(1, '2', "3")) to turn into DataNameˇCtorName{1, '2', "3"}
	//	(half of this just translates the Error(msg) constructor.. =)
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

func (me *irAst) postInitialFixups() {
	me.walk(func(ast irA) irA {
		switch a := ast.(type) {
		case *irALet:
			if a != nil && a.isConstable() {
				//	turn var=literal's into consts
				c := ªConst(&a.irANamedTypeRef, a.LetVal)
				c.copyTypeInfoFrom(a.ExprType())
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

func (me *irAst) postPerFuncFixups() {
	var namescache map[string]string
	convertToTypeOf := func(i int, afn *irAFunc, from irA, totype *irANamedTypeRef) (int, *irASym) {
		if totype.RefAlias == "" && totype.RefPtr == nil {
			// 	panic(fmt.Sprintf("WUT:\t%s: type-conversion via %#v", me.mod.srcFilePath, totype))
		}
		symname, varname := from.Base().NameGo, ªSymGo(irASymStrOr(from, string(rune(i+97)))+"ᕽ")
		varname.copyTypeInfoFrom(totype)
		if existing, _ := namescache[symname]; symname != "" && existing != "" {
			varname.NameGo = existing
		} else {
			if symname != "" {
				namescache[symname] = varname.NameGo
			}
			pname, tname := me.resolveGoTypeRefFromQName(totype.RefAlias)
			vardecl := ªLet(varname.NameGo, "", ªTo(from, pname, tname))
			vardecl.copyTypeInfoFrom(totype)
			afn.FuncImpl.insert(i, vardecl)
			i++
		}
		return i, varname
	}
	me.perFuncDown(func(afn *irAFunc) {
		fargsused := me.countSymRefs(afn.RefFunc.Args)
		for _, farg := range afn.RefFunc.Args {
			if farg.NameGo != "" && fargsused[farg.NameGo] == 0 {
				farg.NameGo = "_"
			}
		}
		namescache = map[string]string{}
		for i := 0; i < len(afn.FuncImpl.Body); i++ {
			var varname *irASym
			switch ax := afn.FuncImpl.Body[i].(type) {
			case *irAIf: // if condition isn't bool (eg testing an interface{}), convert it first to a temp bool var
				axift := ax.If.ExprType()
				switch axif := ax.If.(type) {
				case *irASym:
					if (!axift.hasTypeInfo()) || axift.RefAlias != exprTypeBool.RefAlias {
						i, varname = convertToTypeOf(i, afn, axif, exprTypeBool)
						ax.If, varname.parent = varname, ax
					}
				case *irAOp1:
					if axift = axif.Of.ExprType(); (!axift.hasTypeInfo()) || axift.RefAlias != exprTypeBool.RefAlias {
						i, varname = convertToTypeOf(i, afn, axif.Of, exprTypeBool)
						axif.Of, varname.parent = varname, axif
					}
				}
			default:
				walk(ax, false, func(ast irA) irA {
					switch a := ast.(type) {
					case *irARet:
						if a.RetArg == nil {
							retarg := ªSymPs(afn.RefFunc.Args[0].NamePs, false)
							retarg.copyTypeInfoFrom(afn.RefFunc.Args[0])
							retarg.parent, a.RetArg = a, retarg
						}
						if afn.RefFunc.Rets[0].hasTypeInfo() {
							if aretsym, _ := a.RetArg.(*irASym); aretsym != nil {
								if tretsym := aretsym.ExprType(); (!tretsym.hasTypeInfoBeyondEmptyIface()) && !tretsym.equiv(afn.RefFunc.Rets[0]) {
									i, varname = convertToTypeOf(i, afn, a.RetArg, afn.RefFunc.Rets[0])
									a.RetArg, varname.parent = varname, a
								}
							}
						}
					case *irAOp1:
						if !a.Of.Base().hasTypeInfo() {
							if a.Op1 == "!" {
								i, varname = convertToTypeOf(i, afn, a.Of, exprTypeBool)
								a.Of, varname.parent = varname, a
							}
						}
					case *irAOp2:
						tl, tr := a.Left.ExprType(), a.Right.ExprType()
						ul, ur := !tl.hasTypeInfoBeyondEmptyIface(), !tr.hasTypeInfoBeyondEmptyIface()
						if sl, _ := a.Left.(*irASym); ul && (!ur) && sl != nil {
							i, varname = convertToTypeOf(i, afn, sl, tr)
							a.Left, varname.parent = varname, a
						} else if sr, _ := a.Right.(*irASym); (!ul) && ur && sr != nil {
							i, varname = convertToTypeOf(i, afn, sr, tl)
							a.Right, varname.parent = varname, a
						}
					}
					return ast
				})
			}
		}
	})
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
						fndictarg.turnRefIntoRefPtr()
					}
					fnretarg := irANamedTypeRef{}
					fnretarg.copyTypeInfoFrom(gtd.RefStruct.Fields.byPsName(tcm.Name))
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
				if tcmod, gtd := findGoTypeByPsQName(me.mod, tci.ClassName); gtd == nil || gtd.RefStruct == nil {
					// panic(notImplErr("type-class '"+tci.ClassName+"' (its struct type-def wasn't found) for instance", tci.Name, me.mod.srcFilePath))
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
								case *irASym:
									if gvd := me.irM.goValDeclByPsName(fvx.NamePs); gvd != nil {
										gvd.RefFunc.copyArgTypesOnlyFrom(false, gtd.RefStruct.Fields[i].RefFunc)
									}
								case *irACall:
								case *irAPkgSym:
								case *irALitArr:
								case *irALitNum:
								case *irALitInt:
								case *irALitStr:
								case *irALitBool:
								case *irADot:
								case *irALitObj:
								case *irAOp2:
								default:
									println(fvx.(*irAFunc))
								}
							}
							if ax.RefAlias = axlv.RefAlias; gtd.RefStruct.PassByPtr {
								ax.turnRefIntoRefPtr()
								axctor := ªO1("&", axlv)
								axlv.parent, axctor.parent = axctor, ax
								ax.LetVal = axctor
							}
						case *irAPkgSym:
							ax.RefAlias = tci.ClassName
						case *irACall:
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
								fndictarg.turnRefIntoRefPtr()
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
									fnretarg.turnRefIntoRefPtr()
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
