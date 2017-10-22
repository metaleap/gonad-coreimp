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

func (me *gonadIrAst) prepAddOrCull(a gIrA) {
	if a != nil {
		culled := false
		if ctor, _ := a.(*gIrACtor); ctor != nil {
			// PureScript CoreImp contains constructor functions for each ADT "sub-type", we drop those
			culled, me.culled.typeCtorFuncs = true, append(me.culled.typeCtorFuncs, ctor)
		} else if ab := a.Base(); ab != nil {
			// check if helper function related to type-classes / type-class instances:
			if culled = me.girM.tcInst(ab.NamePs) != nil; culled {
				// func instname(..)
				if afn, _ := a.(*gIrAFunc); afn != nil {
					p := afn.parent
					av := ªLet(afn.NameGo, afn.NamePs, afn)
					av.parent = p
					a = av
				}
				me.culled.tcInstDecls = append(me.culled.tcInstDecls, a)
			} else if culled = me.girM.tcMember(ab.NamePs) != nil; culled {
				me.culled.tcDictDecls = append(me.culled.tcDictDecls, a)
			}
		}
		if !culled {
			me.Add(a)
		}
	}
}

func (me *gonadIrAst) prepAddEnumishAdtGlobals() (nuglobalsmap map[string]*gIrALet) {
	//	add private globals to represent all arg-less ctors (ie. "one const per enum-value")
	nuglobals := []gIrA{}
	nuglobalsmap = map[string]*gIrALet{}
	for _, gtd := range me.girM.GoTypeDefs {
		if gtd.RefInterface != nil && gtd.RefInterface.xtd != nil {
			for _, ctor := range gtd.RefInterface.xtd.Ctors {
				if ctor.gtd != nil && len(ctor.Args) == 0 {
					nuvar := ªLet("º"+ctor.Name, "", ªO(&gIrANamedTypeRef{RefAlias: ctor.gtd.NameGo}))
					nuglobalsmap[ctor.Name] = nuvar
					nuglobals = append(nuglobals, nuvar)
				}
			}
		}
	}
	me.Add(nuglobals...)
	return
}

func (me *gonadIrAst) prepAddNewExtraTypes() {
	var newextratypes gIrANamedTypeRefs
	//	turn type-class instances into unexported 0-byte structs providing the corresponding interface-implementing method(s)
	for _, tci := range me.girM.EnvTypeClassInsts {
		if gid := findGoTypeByPsQName(tci.ClassName); gid == nil {
			panic(me.mod.srcFilePath + ": type-class '" + tci.ClassName + "' not found for instance '" + tci.Name + "'")
		} else {
			gtd := newextratypes.byPsName(tci.Name)
			if gtd == nil {
				gtd = &gIrANamedTypeRef{Export: true, RefStruct: &gIrATypeRefStruct{instOf: tci.ClassName}}
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
		me.girM.GoTypeDefs = append(me.girM.GoTypeDefs, newextratypes...)
	}
}

func (me *gonadIrAst) prepFixupExportedNames() {
	ensure := func(isfunc bool, gntr *gIrANamedTypeRef) *gIrANamedTypeRef {
		if gvd := me.girM.goValDeclByPsName(gntr.NamePs); gvd != nil {
			gntr.copyFrom(gvd, true, !isfunc, true)
			return gvd
		}
		return nil
	}
	me.topLevelDefs(func(a gIrA) bool {
		if av, _ := a.(*gIrALet); av != nil {
			ensure(false, &av.gIrANamedTypeRef)
		} else if af, _ := a.(*gIrAFunc); af != nil {
			if gvd := ensure(true, &af.gIrANamedTypeRef); gvd != nil {
				if gvd.RefFunc == nil {
					panic(notImplErr("NIL RefFunc for", gvd.NamePs, me.mod.srcFilePath))
				} else {
					for i, gvdfuncarg := range gvd.RefFunc.Args {
						af.RefFunc.Args[i].copyFrom(gvdfuncarg, false, true, false)
					}
					if len(af.RefFunc.Rets) > 0 {
						panic(notImplErr("RET values for", gvd.NamePs, me.mod.srcFilePath))
					}
					for _, gvdfuncret := range gvd.RefFunc.Rets {
						af.RefFunc.Rets = append(af.RefFunc.Rets, gvdfuncret)
					}
				}
			}

		}
		return false
	})
}

func (me *gonadIrAst) prepForeigns() {
	if reqforeign := me.mod.coreimp.namedRequires["$foreign"]; len(reqforeign) > 0 {
		qn := nsPrefixDefaultFfiPkg + me.mod.qName
		me.girM.ForeignImp = me.girM.Imports.addIfHasnt(strReplDot2Underscore.Replace(qn), "github.com/metaleap/gonad/"+strReplDot2Slash.Replace(qn), qn)
		me.girM.save = true
	}
}

func (me *gonadIrAst) prepMiscFixups(nuglobalsmap map[string]*gIrALet) {
	me.walk(func(ast gIrA) gIrA {
		if ast != nil {
			switch a := ast.(type) {
			case *gIrAOp2: // coreimp represents Ints JS-like as: expr|0 --- we ditch the |0 part
				if opright, _ := a.Right.(*gIrALitInt); opright != nil && a.Op2 == "|" && opright.LitInt == 0 {
					return a.Left
				}
			case *gIrADot:
				if dl, _ := a.DotLeft.(*gIrASym); dl != nil {
					if dr, _ := a.DotRight.(*gIrASym); dr != nil {
						//	find all CtorName.value references and change them to the new globals created in AddEnumishAdtGlobals
						if dr.NameGo == "value" {
							if nuglobalvar := nuglobalsmap[dl.NamePs]; nuglobalvar != nil {
								sym4nuvar := ªSymGo(nuglobalvar.NameGo)
								sym4nuvar.gIrANamedTypeRef = nuglobalvar.gIrANamedTypeRef
								return sym4nuvar
							}
						} else {
							//	if the dot's LHS refers to a package, ensure the import is marked as in-use and switch out dot for pkgsym
							for _, imp := range me.girM.Imports {
								if imp.N == dl.NameGo || (dl.NamePs == "$foreign" && imp == me.girM.ForeignImp) {
									imp.used = true
									dr.Export = true
									dr.NameGo = sanitizeSymbolForGo(dr.NameGo, dr.Export)
									return ªPkgSym(imp.N, dr.NameGo)
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

func (me *gonadIrAst) postFixupAmpCtor(a *gIrAOp1, oc *gIrACall) gIrA {
	//	restore data-ctors from calls like (&CtorName(1, '2', "3")) to turn into DataNameˇCtorName{1, '2', "3"}
	var gtd *gIrANamedTypeRef
	if ocdot, _ := oc.Callee.(*gIrADot); ocdot != nil {
		if ocdot1, _ := ocdot.DotLeft.(*gIrASym); ocdot1 != nil {
			if mod := findModuleByPName(ocdot1.NamePs); mod != nil {
				if ocdot2, _ := ocdot.DotRight.(*gIrASym); ocdot2 != nil {
					gtd = mod.girMeta.goTypeDefByPsName(ocdot2.NamePs)
				}
			}
		}
	}
	ocv, _ := oc.Callee.(*gIrASym)
	if gtd == nil && ocv != nil {
		gtd = me.girM.goTypeDefByPsName(ocv.NamePs)
	}
	if gtd != nil {
		o := ªO(&gIrANamedTypeRef{RefAlias: gtd.NameGo})
		for _, ctorarg := range oc.CallArgs {
			of := ªOFld(ctorarg)
			of.parent = o
			o.ObjFields = append(o.ObjFields, of)
		}
		return o
	} else if ocv != nil && ocv.NamePs == "Error" {
		if len(oc.CallArgs) == 1 {
			if op2, _ := oc.CallArgs[0].(*gIrAOp2); op2 != nil && op2.Op2 == "+" {
				oc.CallArgs[0] = op2.Left
				op2.Left.Base().parent = oc
				if oparr := op2.Right.(*gIrALitArr); oparr != nil {
					for _, oparrelem := range oparr.ArrVals {
						nucallarg := oparrelem
						if oaedot, _ := oparrelem.(*gIrADot); oaedot != nil {
							if oaedot2, _ := oaedot.DotLeft.(*gIrADot); oaedot2 != nil {
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
					me.girM.Imports.addIfHasnt("reflect", "reflect", "")
					me.girM.save = true
					oc.CallArgs[0].(*gIrALitStr).LitStr += strings.Repeat(", ‹%v› %v", (len(oc.CallArgs)-1)/2)[2:]
				}
			}
		}
		me.girM.Imports.addIfHasnt("fmt", "fmt", "")
		me.girM.save = true
		call := ªCall(ªPkgSym("fmt", "Errorf"), oc.CallArgs...)
		return call
	} else if ocv != nil {
		println("TODO:\t" + me.mod.srcFilePath + "\t" + ocv.NamePs)
	}
	return a
}

func (me *gonadIrAst) postLinkTcInstFuncsToImplStructs() {
	// instfuncvars := me.topLevelDefs(func(a gIrA) bool {
	// 	if v, _ := a.(*gIrALet); v != nil {
	// 		if vv, _ := v.LetVal.(*gIrALitObj); vv != nil {
	// 			if gtd := me.girM.goTypeDefByPsName(v.NamePs); gtd != nil {
	// 				return true
	// 			}
	// 		}
	// 	}
	// 	return false
	// })
	for _, ifx := range me.culled.tcInstDecls {
		ifv, _ := ifx.(*gIrALet)
		if ifv == nil {
			println(me.mod.srcFilePath + "\t\t\t\t" + ifx.(*gIrAFunc).NamePs)
			continue
		}
		gtd := me.girM.goTypeDefByPsName(ifv.NamePs) // the private implementer struct-type
		gtdInstOf := findGoTypeByPsQName(gtd.RefStruct.instOf)
		ifv.Export = gtdInstOf.Export
		ifv.setBothNamesFromPsName(ifv.NamePs)
		var mod *modPkg
		pname, tcname := me.resolveGoTypeRefFromPsQName(gtd.RefStruct.instOf, true)
		if len(pname) == 0 || pname == me.mod.pName {
			mod = me.mod
		} else {
			mod = findModuleByPName(pname)
		}
		if tcctor := mod.girAst.typeCtorFunc(tcname); tcctor == nil {
			panic(me.mod.srcFilePath + ": instance ctor func not found for " + ifv.NamePs + ", please report")
		} else {
			ifo, _ := ifv.LetVal.(*gIrALitObj) //  something like:  InterfaceName{funcs}
			if ifo == nil {
				// println(me.mod.srcFilePath + "\t" + ifx.Base().NamePs)
			} else {
				for i, instfuncarg := range tcctor.RefFunc.Args {
					for _, gtdmethod := range gtd.RefStruct.Methods {
						if gtdmethod.NamePs == instfuncarg.NamePs {
							ifofv := ifo.ObjFields[i].FieldVal
							switch ifa := ifofv.(type) {
							case *gIrAFunc:
								gtdmethod.RefFunc.impl = ifa.FuncImpl
							default:
								oldp := ifofv.Parent()
								gtdmethod.RefFunc.impl = ªBlock(ªRet(ifofv))
								gtdmethod.RefFunc.impl.parent = oldp
							}
							break
						}
					}
				}
			}
		}
		nuctor := ªO(&gIrANamedTypeRef{RefAlias: gtd.NameGo})
		nuctor.parent = ifv
		ifv.LetVal = nuctor
		ifv.RefAlias = gtd.RefStruct.instOf
	}
}

func (me *gonadIrAst) postMiscFixups() {
	me.walk(func(ast gIrA) gIrA {
		switch a := ast.(type) {
		case *gIrALet:
			if a != nil && a.isConstable() {
				//	turn var=literal's into consts
				return ªConst(&a.gIrANamedTypeRef, a.LetVal)
			}
		case *gIrAFunc:
			if a.gIrANamedTypeRef.RefFunc != nil {
				// coreimp doesn't give us return-args for funcs, prep them with interface{} initially
				if len(a.gIrANamedTypeRef.RefFunc.Rets) == 0 { // but some do have ret-args from prior gonad ops
					// otherwise, add an empty-for-now 'unknown' (aka interface{}) return type
					a.gIrANamedTypeRef.RefFunc.Rets = gIrANamedTypeRefs{&gIrANamedTypeRef{}}
				}
			} else {
				panic(me.mod.srcFilePath + ": please report as bug, a gIrAFunc ('" + a.NameGo + "' / '" + a.NamePs + "') had no RefFunc set")
			}
		}
		return ast
	})
}
