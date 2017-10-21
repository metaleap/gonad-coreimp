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

func (me *gonadIrAst) prepAddEnumishAdtGlobals() (nuglobalsmap map[string]*gIrAVar) {
	//	add private globals to represent all arg-less ctors (ie. "one const per enum-value")
	nuglobals := []gIrA{}
	nuglobalsmap = map[string]*gIrAVar{}
	for _, gtd := range me.girM.GoTypeDefs {
		if gtd.RefInterface != nil && gtd.RefInterface.xtd != nil {
			for _, ctor := range gtd.RefInterface.xtd.Ctors {
				if ctor.gtd != nil && len(ctor.Args) == 0 {
					nuvar := ªVar("º"+ctor.Name, "", ªO(&gIrANamedTypeRef{RefAlias: ctor.gtd.NameGo}))
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
			panic(me.mod.srcFilePath + ": type-class " + tci.ClassName + " not found for instance " + tci.Name)
		} else {
			gtd := newextratypes.byPsName(tci.Name)
			if gtd == nil {
				gtd = &gIrANamedTypeRef{Export: true, instOf: tci.ClassName, RefStruct: &gIrATypeRefStruct{}}
				gtd.setBothNamesFromPsName(tci.Name)
				gtd.NameGo = "ı" + gtd.NameGo
				newextratypes = append(newextratypes, gtd)
			}
			for _, method := range gid.RefInterface.Methods {
				mcopy := *method
				mcopy.method.hasNoThis = true
				gtd.Methods = append(gtd.Methods, &mcopy)
			}
		}
	}
	if len(newextratypes) > 0 {
		me.girM.GoTypeDefs = append(me.girM.GoTypeDefs, newextratypes...)
	}
}

func (me *gonadIrAst) prepFixupExportedNames() {
	ensure := func(gntr *gIrANamedTypeRef) {
		if gntr != nil {
			for _, gvd := range me.girM.GoValDecls {
				if gvd.NamePs == gntr.NamePs {
					gntr.Export = gvd.Export
					gntr.NameGo = gvd.NameGo
					break
				}
			}
		}
	}
	me.topLevelDefs(func(a gIrA) bool {
		if af, _ := a.(*gIrAFunc); af != nil {
			ensure(&af.gIrANamedTypeRef)
		} else if av, _ := a.(*gIrAVar); av != nil {
			ensure(&av.gIrANamedTypeRef)
		}
		return false
	})
}

func (me *gonadIrAst) prepForeigns() {
	if reqforeign, _ := me.mod.coreimp.namedRequires["$foreign"]; len(reqforeign) > 0 {
		qn := nsPrefixDefaultFfiPkg + me.mod.qName
		me.girM.ForeignImp = me.girM.Imports.addIfHasnt(strReplDot2Underscore.Replace(qn), "github.com/metaleap/gonad/"+strReplDot2Slash.Replace(qn), qn)
		me.girM.save = true
	}
}

func (me *gonadIrAst) prepMiscFixups(nuglobalsmap map[string]*gIrAVar) {
	me.walk(func(ast gIrA) gIrA {
		if ast != nil {
			switch a := ast.(type) {
			case *gIrAOp2: // coreimp represents Ints JS-like as: expr|0 --- we ditch the |0 part
				if opright, _ := a.Right.(*gIrALitInt); opright != nil && a.Op2 == "|" && opright.LitInt == 0 {
					return a.Left
				}
			case *gIrADot:
				if dl, _ := a.DotLeft.(*gIrAVar); dl != nil {
					if dr, _ := a.DotRight.(*gIrAVar); dr != nil {
						//	find all CtorName.value references and change them to the new globals created in AddEnumishAdtGlobals
						if dr.NameGo == "value" {
							if nuglobalvar, _ := nuglobalsmap[dl.NamePs]; nuglobalvar != nil {
								nuvarsym := ªSym("")
								nuvarsym.gIrANamedTypeRef = nuglobalvar.gIrANamedTypeRef
								nuvarsym.NameGo = nuglobalvar.NameGo
								return nuvarsym
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

func (me *gonadIrAst) postClearTcDictFuncs() (dictfuncs []gIrA) {
	//	ditch all: func tcmethodname(dict){return dict.tcmethodname}
	dictfuncs = me.topLevelDefs(func(a gIrA) bool {
		if fn, _ := a.(*gIrAFunc); fn != nil &&
			fn.RefFunc != nil && len(fn.RefFunc.Args) == 1 && fn.RefFunc.Args[0].NamePs == "dict" &&
			fn.FuncImpl != nil && len(fn.FuncImpl.Body) == 1 {
			if fnret, _ := fn.FuncImpl.Body[0].(*gIrARet); fnret != nil {
				if fnretdot, _ := fnret.RetArg.(*gIrADot); fnretdot != nil {
					if fnretdotl, _ := fnretdot.DotLeft.(*gIrAVar); fnretdotl != nil && fnretdotl.NamePs == "dict" {
						if fnretdotr, _ := fnretdot.DotRight.(*gIrAVar); fnretdotr != nil && fnretdotr.NamePs == fn.NamePs {
							return true
						}
					}
				}
			}
		}
		return false
	})
	return
}

func (me *gonadIrAst) postFixupAmpCtor(a *gIrAOp1, oc *gIrACall) gIrA {
	//	restore data-ctors from calls like (&CtorName(1, '2', "3")) to turn into DataNameˇCtorName{1, '2', "3"}
	var gtd *gIrANamedTypeRef
	if ocdot, _ := oc.Callee.(*gIrADot); ocdot != nil {
		if ocdot1, _ := ocdot.DotLeft.(*gIrAVar); ocdot1 != nil {
			if mod := findModuleByPName(ocdot1.NamePs); mod != nil {
				if ocdot2, _ := ocdot.DotRight.(*gIrAVar); ocdot2 != nil {
					gtd = mod.girMeta.goTypeDefByPsName(ocdot.DotRight.(*gIrAVar).NamePs)
				}
			}
		}
	}
	ocv, _ := oc.Callee.(*gIrAVar)
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
	instfuncvars := me.topLevelDefs(func(a gIrA) bool {
		if v, _ := a.(*gIrAVar); v != nil {
			if vv, _ := v.VarVal.(*gIrALitObj); vv != nil {
				if gtd := me.girM.goTypeDefByPsName(v.NamePs); gtd != nil {
					return true
				}
			}
		}
		return false
	})
	for _, ifx := range instfuncvars {
		ifv, _ := ifx.(*gIrAVar)
		gtd := me.girM.goTypeDefByPsName(ifv.NamePs) // the private implementer struct-type
		gtdInstOf := findGoTypeByPsQName(gtd.instOf)
		ifv.Export = gtdInstOf.Export
		ifv.setBothNamesFromPsName(ifv.NamePs)
		var tcctors []gIrA
		var mod *modPkg
		pname, tcname := me.resolveGoTypeRefFromPsQName(gtd.instOf, true)
		if len(pname) == 0 || pname == me.mod.pName {
			mod = me.mod
		} else {
			mod = findModuleByPName(pname)
		}
		tcctors = mod.girAst.topLevelDefs(func(a gIrA) bool {
			if afn, _ := a.(*gIrAFunc); afn != nil {
				return afn.WasTypeFunc && afn.NamePs == tcname
			}
			if av, _ := a.(*gIrAVar); av != nil {
				return av.WasTypeFunc && av.NamePs == tcname
			}
			return false
		})
		for i := 0; i < len(tcctors); i++ {
			switch av := tcctors[i].(type) {
			case *gIrAVar:
				tcctors[i] = av.VarVal.(*gIrAFunc)
			}
		}
		ifo := ifv.VarVal.(*gIrALitObj) //  something like:  InterfaceName{funcs}
		if len(tcctors) > 0 {
			tcctor := tcctors[0].(*gIrAFunc)
			for i, instfuncarg := range tcctor.RefFunc.Args {
				for _, gtdmethod := range gtd.Methods {
					if gtdmethod.NamePs == instfuncarg.NamePs {
						ifofv := ifo.ObjFields[i].FieldVal
						switch ifa := ifofv.(type) {
						case *gIrAFunc:
							gtdmethod.method.body = ifa.FuncImpl
						default:
							oldp := ifofv.Parent()
							gtdmethod.method.body = ªBlock(ªRet(ifofv))
							gtdmethod.method.body.parent = oldp
						}
						break
					}
				}
			}
		} else {
			if ifv.NamePs == "showBoolean" && strings.Contains(me.mod.srcFilePath, "Show") {
			}
		}
		nuctor := ªO(&gIrANamedTypeRef{RefAlias: gtd.NameGo})
		nuctor.parent = ifv
		ifv.VarVal = nuctor
		ifv.RefAlias = gtd.instOf
	}
}

func (me *gonadIrAst) postMiscFixups(dictfuncs []gIrA) {
	me.walk(func(ast gIrA) gIrA {
		switch a := ast.(type) {
		case *gIrAVar:
			if a != nil && a.VarVal != nil {
				if vc, _ := a.VarVal.(gIrAConstable); vc != nil && vc.isConstable() {
					//	turn var=literal's into consts
					return ªConst(&a.gIrANamedTypeRef, a.VarVal)
				}
			}
		case *gIrAFunc:
			// marked to be ditched?
			for _, df := range dictfuncs {
				if df == a {
					return nil
				}
			}
			// coreimp doesn't give us return-args for funcs, prep them with interface{} initially
			if len(a.gIrANamedTypeRef.RefFunc.Rets) > 0 {
				panic(me.mod.srcFilePath + ": unexpected at this stage, please report as bug: func return-args present for " + a.NameGo + "/" + a.NamePs)
			}
			// at *this* point, we never seem to run into functions without a return statement at present, so the below check is skipped but kept around:
			if checkhasrets := false; checkhasrets {
				hasrets := false
				walk(a, func(asub gIrA) gIrA {
					switch asub.(type) {
					case *gIrARet:
						hasrets = true
					}
					return asub
				})
				if !hasrets {
					panic(me.mod.srcFilePath + ": unexpected at this stage, please report as bug: no return in func " + a.NameGo + "/" + a.NamePs)
				}
			}
			// finally, add an empty-for-now 'unknown' (aka interface{}) return type
			a.gIrANamedTypeRef.RefFunc.Rets = gIrANamedTypeRefs{&gIrANamedTypeRef{}}
		}
		return ast
	})
}
