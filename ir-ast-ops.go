package main

import (
	"strings"
)

func (me *GonadIrAst) AddEnumishAdtGlobals() (nuglobalsmap map[string]*GIrAVar) {
	//	after we have also created additional structs/interfaces in AddNewExtraTypes, add private globals to represent all arg-less ctors (ie. "one const per enum-value")
	nuglobals := []GIrA{}
	nuglobalsmap = map[string]*GIrAVar{}
	for _, gtd := range me.girM.GoTypeDefs {
		if gtd.RefInterface != nil && gtd.RefInterface.xtd != nil {
			for _, ctor := range gtd.RefInterface.xtd.Ctors {
				if ctor.gtd != nil && len(ctor.Args) == 0 {
					nuvar := ªVar("º"+ctor.Name, "", ªO(&GIrANamedTypeRef{RefAlias: ctor.gtd.NameGo}))
					nuglobalsmap[ctor.Name] = nuvar
					nuglobals = append(nuglobals, nuvar)
				}
			}
		}
	}
	me.Add(nuglobals...)
	return
}

func (me *GonadIrAst) AddNewExtraTypes() {
	//	detect unexported data-type constructors and add the missing structs implementing a newly added single unexported ADT umbrella interface type
	newxtypedatadecl := &GIrMTypeDataDecl{Name: "ª" + me.mod.lName}
	var newextratypes GIrANamedTypeRefs
	var av *GIrAVar
	var fn *GIrAFunc
	for i := 0; i < len(me.Body); i++ {
		av, _ = me.Body[i].(*GIrAVar)
		if av != nil && av.WasTypeFunc {
			if fn, _ = av.VarVal.(*GIrAFunc); fn == nil {
				fn = av.VarVal.(*GIrACall).Callee.(*GIrAFunc).FuncImpl.Body[0].(*GIrAFunc)
			}
			gtd := me.girM.GoTypeDefByPsName(av.NamePs)
			if gtd != nil && gtd.RefInterface != nil {
				continue
			}
			if gtd == nil {
				nuctor := &GIrMTypeDataCtor{Name: av.NamePs}
				for i := 0; i < len(fn.RefFunc.Args); i++ {
					nuctor.Args = append(nuctor.Args, &GIrMTypeRef{})
				}
				newxtypedatadecl.Ctors = append(newxtypedatadecl.Ctors, nuctor)
			}
			me.Body = append(me.Body[:i], me.Body[i+1:]...)
			i--
		}
	}
	if len(newxtypedatadecl.Ctors) > 0 {
		newextratypes = append(newextratypes, me.girM.toGIrADataTypeDefs([]*GIrMTypeDataDecl{newxtypedatadecl}, map[string][]string{}, false)...)
	}
	//	also turn type-class instances into 0-byte structs providing the corresponding interface-implementing method(s)
	for _, tci := range me.girM.ExtTypeClassInsts {
		if gid := findGoTypeByPsQName(tci.ClassName); gid == nil {
			panic(me.mod.srcFilePath + ": type-class " + tci.ClassName + " not found for instance " + tci.Name)
		} else {
			gtd := newextratypes.ByPsName(tci.Name)
			if gtd == nil {
				gtd = &GIrANamedTypeRef{Export: true, instOf: tci.ClassName, RefStruct: &GIrATypeRefStruct{}}
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

func (me *GonadIrAst) ClearTcDictFuncs() (dictfuncs []GIrA) {
	//	ditch all: func tcmethodname(dict){return dict.tcmethodname}
	dictfuncs = me.topLevelDefs(func(a GIrA) bool {
		if fn, _ := a.(*GIrAFunc); fn != nil &&
			fn.RefFunc != nil && len(fn.RefFunc.Args) == 1 && fn.RefFunc.Args[0].NamePs == "dict" &&
			fn.FuncImpl != nil && len(fn.FuncImpl.Body) == 1 {
			if fnret, _ := fn.FuncImpl.Body[0].(*GIrARet); fnret != nil {
				if fnretdot, _ := fnret.RetArg.(*GIrADot); fnretdot != nil {
					if fnretdotl, _ := fnretdot.DotLeft.(*GIrAVar); fnretdotl != nil && fnretdotl.NamePs == "dict" {
						if fnretdotr, _ := fnretdot.DotRight.(*GIrAVar); fnretdotr != nil && fnretdotr.NamePs == fn.NamePs {
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

func (me *GonadIrAst) FixupAmpCtor(a *GIrAOp1, oc *GIrACall) GIrA {
	//	restore data-ctors from calls like (&CtorName(1, '2', "3")) to turn into DataNameˇCtorName{1, '2', "3"}
	var gtd *GIrANamedTypeRef
	if ocdot, _ := oc.Callee.(*GIrADot); ocdot != nil {
		if ocdot1, _ := ocdot.DotLeft.(*GIrAVar); ocdot1 != nil {
			if mod := FindModuleByPName(ocdot1.NamePs); mod != nil {
				if ocdot2, _ := ocdot.DotRight.(*GIrAVar); ocdot2 != nil {
					gtd = mod.girMeta.GoTypeDefByPsName(ocdot.DotRight.(*GIrAVar).NamePs)
				}
			}
		}
	}
	ocv, _ := oc.Callee.(*GIrAVar)
	if gtd == nil && ocv != nil {
		gtd = me.girM.GoTypeDefByPsName(ocv.NamePs)
	}
	if gtd != nil {
		o := ªO(&GIrANamedTypeRef{RefAlias: gtd.NameGo})
		for _, ctorarg := range oc.CallArgs {
			of := ªOFld(ctorarg)
			of.parent = o
			o.ObjFields = append(o.ObjFields, of)
		}
		return o
	} else if ocv != nil && ocv.NamePs == "Error" {
		if len(oc.CallArgs) == 1 {
			if op2, _ := oc.CallArgs[0].(*GIrAOp2); op2 != nil && op2.Op2 == "+" {
				oc.CallArgs[0] = op2.Left
				op2.Left.Base().parent = oc
				if oparr := op2.Right.(*GIrALitArr); oparr != nil {
					for _, oparrelem := range oparr.ArrVals {
						nucallarg := oparrelem
						if oaedot, _ := oparrelem.(*GIrADot); oaedot != nil {
							if oaedot2, _ := oaedot.DotLeft.(*GIrADot); oaedot2 != nil {
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
					me.girM.Imports.AddIfHasnt("reflect", "reflect", "")
					oc.CallArgs[0].(*GIrALitStr).LitStr += strings.Repeat(", ‹%v› %v", (len(oc.CallArgs)-1)/2)[2:]
				}
			}
		}
		me.girM.Imports.AddIfHasnt("fmt", "fmt", "")
		call := ªCall(ªPkgRef("fmt", "Errorf"), oc.CallArgs...)
		return call
	} else if ocv != nil {
		println("TODO:\t" + me.mod.srcFilePath + "\t" + ocv.NamePs)
	}
	return a
}

func (me *GonadIrAst) FixupExportedNames() {
	ensure := func(gntr *GIrANamedTypeRef) {
		if gntr != nil {
			for _, gvd := range me.girM.GoValDecls {
				if gvd.NamePs == gntr.NamePs {
					gntr.Export = true
					gntr.NameGo = gvd.NameGo
					break
				}
			}
		}
	}
	me.topLevelDefs(func(a GIrA) bool {
		if af, _ := a.(*GIrAFunc); af != nil {
			ensure(&af.GIrANamedTypeRef)
		} else if av, _ := a.(*GIrAVar); av != nil {
			ensure(&av.GIrANamedTypeRef)
		}
		return false
	})
}

func (me *GonadIrAst) LinkTcInstFuncsToImplStructs() {
	instfuncvars := me.topLevelDefs(func(a GIrA) bool {
		if v, _ := a.(*GIrAVar); v != nil {
			if vv, _ := v.VarVal.(*GIrALitObj); vv != nil {
				if gtd := me.girM.GoTypeDefByPsName(v.NamePs); gtd != nil {
					return true
				}
			}
		}
		return false
	})
	for _, ifx := range instfuncvars {
		ifv, _ := ifx.(*GIrAVar)
		gtd := me.girM.GoTypeDefByPsName(ifv.NamePs) // the private implementer struct-type
		gtdInstOf := findGoTypeByPsQName(gtd.instOf)
		ifv.Export = gtdInstOf.Export
		ifv.setBothNamesFromPsName(ifv.NamePs)
		var tcctors []GIrA
		var mod *ModuleInfo
		pname, tcname := me.resolveGoTypeRefFromPsQName(gtd.instOf, true)
		if len(pname) == 0 || pname == me.mod.pName {
			mod = me.mod
		} else {
			mod = FindModuleByPName(pname)
		}
		tcctors = mod.girAst.topLevelDefs(func(a GIrA) bool {
			if afn, _ := a.(*GIrAFunc); afn != nil {
				return afn.WasTypeFunc && afn.NamePs == tcname
			}
			if av, _ := a.(*GIrAVar); av != nil {
				return av.WasTypeFunc && av.NamePs == tcname
			}
			return false
		})
		for i := 0; i < len(tcctors); i++ {
			switch x := tcctors[i].(type) {
			case *GIrAVar:
				tcctors[i] = x.VarVal.(*GIrAFunc)
			}
		}
		ifo := ifv.VarVal.(*GIrALitObj) //  something like:  InterfaceName{funcs}
		if len(tcctors) > 0 {
			tcctor, _ := tcctors[0].(*GIrAFunc)
			for i, instfuncarg := range tcctor.RefFunc.Args {
				for _, gtdmethod := range gtd.Methods {
					if gtdmethod.NamePs == instfuncarg.NamePs {
						ifofv := ifo.ObjFields[i].FieldVal
						switch ifa := ifofv.(type) {
						case *GIrAFunc:
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
		nuctor := ªO(&GIrANamedTypeRef{RefAlias: gtd.NameGo})
		nuctor.parent = ifv
		ifv.VarVal = nuctor
		ifv.RefAlias = gtd.instOf
	}
}

func (me *GonadIrAst) MiscPostFixups(dictfuncs []GIrA) {
	me.Walk(func(ast GIrA) GIrA {
		switch a := ast.(type) {
		case *GIrAFunc:
			// marked to be ditched?
			for _, df := range dictfuncs {
				if df == a {
					return nil
				}
			}
			// coreimp doesn't give us return-args for funcs, prep them with interface{} initially
			if len(a.GIrANamedTypeRef.RefFunc.Rets) > 0 {
				panic(me.mod.srcFilePath + ": unexpected at this stage, please report as bug: func return-args present for " + a.NameGo + "/" + a.NamePs)
			}
			// at *this* point, we never seem to run into functions without a return statement at present, so the below check is skipped but kept around:
			if checkhasrets := false; checkhasrets {
				hasrets := false
				walk(a, func(asub GIrA) GIrA {
					switch asub.(type) {
					case *GIrARet:
						hasrets = true
					}
					return asub
				})
				if !hasrets {
					panic(me.mod.srcFilePath + ": unexpected at this stage, please report as bug: no return in func " + a.NameGo + "/" + a.NamePs)
				}
			}
			// finally, add an empty-for-now 'unknown' (aka interface{}) return type
			a.GIrANamedTypeRef.RefFunc.Rets = GIrANamedTypeRefs{&GIrANamedTypeRef{}}
		}
		return ast
	})
}

func (me *GonadIrAst) MiscPrepFixups(nuglobalsmap map[string]*GIrAVar) {
	me.Walk(func(ast GIrA) GIrA {
		if ast != nil {
			switch a := ast.(type) {
			case *GIrAOp2: // coreimp represents Ints JS-like as: expr|0 --- we ditch the |0 part
				if opright, _ := a.Right.(*GIrALitInt); opright != nil && a.Op2 == "|" && opright.LitInt == 0 {
					return a.Left
				}
			case *GIrADot:
				if dl, _ := a.DotLeft.(*GIrAVar); dl != nil {
					if dr, _ := a.DotRight.(*GIrAVar); dr != nil {
						//	find all CtorName.value references and change them to the new globals created in AddEnumishAdtGlobals
						if dr.NameGo == "value" {
							if nuglobalvar, _ := nuglobalsmap[dl.NamePs]; nuglobalvar != nil {
								nuvarsym := ªSym("")
								nuvarsym.GIrANamedTypeRef = nuglobalvar.GIrANamedTypeRef
								nuvarsym.NameGo = nuglobalvar.NameGo
								return nuvarsym
							}
						} else {
							//	if the dot's LHS refers to a package, ensure the import is marked as in-use
							for _, imp := range me.girM.Imports {
								if imp.N == dl.NameGo {
									imp.used = true
									dr.Export = true
									dr.NameGo = sanitizeSymbolForGo(dr.NameGo, dr.Export)
									break
								}
							}
						}
					}
				}
			case *GIrAVar:
				if a != nil && a.VarVal != nil {
					if vc, _ := a.VarVal.(gIrAConstable); vc != nil && vc.isConstable() {
						//	turn var=literal's into consts
						return ªConst(&a.GIrANamedTypeRef, a.VarVal)
					}
				}
			}
		}
		return ast
	})
}
