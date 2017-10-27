package main

/*
Golang intermediate-representation AST:
various transforms and operations on the AST,
"prep" ops are called from prepFromCoreImp
and "post" ops are called from finalizePostPrep.
*/

func (me *irAst) prepFromCoreImp() {
	me.irABlock.root = me
	//	transform coreimp.json AST into our own leaner Go-focused AST format
	//	mostly focus on discovering new type-defs, final transforms once all
	//	type-defs in all modules are known happen in FinalizePostPrep
	for _, cia := range me.mod.coreimp.Body {
		me.prepAddOrCull(cia.ciAstToIrAst())
	}
	for i, tcf := range me.culled.typeCtorFuncs {
		if tcfb := tcf.Base(); tcfb != nil {
			if gtd := me.irM.goTypeDefByPsName(tcfb.NamePs); gtd != nil {
				gtd.sortIndex = i
			}
		}
	}
	me.prepForeigns()
	me.prepFixupNameCasings()
	nuglobals := me.prepAddEnumishAdtGlobals()
	me.prepMiscFixups(nuglobals)
}

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
	if reqforeign := me.mod.coreimp.namedRequires["$foreign"]; reqforeign != "" {
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
				if a != nil && Proj.BowerJsonFile.Gonad.CodeGen.FlattenIfs { // any 2 consecutive ifs-without-elses offer opportunities
					var lastif *irAIf
					for i := 0; i < len(a.Body); i++ {
						switch thisif := a.Body[i].(type) {
						case *irAIf:
							if lastif == nil {
								lastif = thisif
							} else { // two ifs in a row
								if lastif.Else == nil && thisif.Else == nil {
									if lastif.doesCondNegate(thisif) { // mutually-negating: turn the 2nd then into the else of the 1st
										lastif.Else = thisif.Then
										thisif.Then, lastif.Else.parent = nil, lastif
										a.removeAt(i)
									} else if lastif.Then.Equiv(thisif.Then) { // both have same then branch: unify into a single if with both conditions OR'd
										opor := ªO2(lastif.If, "||", thisif.If)
										lastif.If, opor.parent = opor, lastif
										a.removeAt(i)
									}
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
