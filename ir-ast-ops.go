package main

import (
	"strings"
)

func (me *GonadIrAst) AddEnumishAdtGlobals() (nuglobalsmap map[string]*GIrAVar) {
	//	after we have also created additional structs/interfaces in AddNewExtraTypes, add private globals to represent all arg-less ctors (ie. "one const per enum value")
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
	//	detect unexported data-type constructors and add the missing structs implementing a new unexported single-per-pkg ADT interface type
	newxtypedatadecl := &GIrMTypeDataDecl{Name: "ª" + me.mod.lName}
	var newextratypes GIrANamedTypeRefs
	var av *GIrAVar
	var ac *GIrAComments
	for i := 0; i < len(me.Body); i++ {
		if ac, _ = me.Body[i].(*GIrAComments); ac != nil && ac.CommentsDecl != nil {
			for tmp, _ := ac.CommentsDecl.(*GIrAComments); tmp != nil; tmp, _ = ac.CommentsDecl.(*GIrAComments) {
				ac = tmp
			}
			av, _ = ac.CommentsDecl.(*GIrAVar)
		} else {
			av, _ = me.Body[i].(*GIrAVar)
		}
		if av != nil && av.WasTypeFunc {
			if ac != nil {
				ac.CommentsDecl = nil
			}
			if fn, _ := av.VarVal.(*GIrAFunc); fn != nil {
				// TODO catches type-classes but not all
				// fmt.Printf("%v\t%s\t%s\t%s\n", len(fn.RefFunc.Args), av.NameGo, av.NamePs, me.mod.srcFilePath)
				// me.Body = append(me.Body[:i], me.Body[i+1:]...)
				// i--
			} else {
				fn := av.VarVal.(*GIrACall).Callee.(*GIrAFunc).FuncImpl.Body[0].(*GIrAFunc)
				if gtd := me.girM.GoTypeDefByPsName(av.NamePs); gtd == nil {
					nuctor := &GIrMTypeDataCtor{Name: av.NamePs, comment: ac}
					for i := 0; i < len(fn.RefFunc.Args); i++ {
						nuctor.Args = append(nuctor.Args, &GIrMTypeRef{})
					}
					newxtypedatadecl.Ctors = append(newxtypedatadecl.Ctors, nuctor)
				} else {
					gtd.comment = ac
				}
				me.Body = append(me.Body[:i], me.Body[i+1:]...)
				i--
			}
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
				mcopy.method.body = ªBlock(ªRet(nil))
				mcopy.method.hasNoThis = true
				gtd.Methods = append(gtd.Methods, &mcopy)
			}
		}
	}
	if len(newextratypes) > 0 {
		me.girM.GoTypeDefs = append(me.girM.GoTypeDefs, newextratypes...)
		me.girM.rebuildLookups()
	}
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
	me.topLevelDefs(func(a GIrA) bool {
		if afn, _ := a.(*GIrAFunc); afn != nil {
			for _, gvd := range me.girM.GoValDecls {
				if gvd.NamePs == afn.NamePs {
					afn.Export = true
					afn.NameGo = gvd.NameGo
				}
			}
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
		if ifv == nil {
			ifv = ifx.(*GIrAComments).CommentsDecl.(*GIrAVar)
		}
		gtd := me.girM.GoTypeDefByPsName(ifv.NamePs) // the private implementer struct-type
		gtdInstOf := findGoTypeByPsQName(gtd.instOf)
		ifv.Export = gtdInstOf.Export
		ifv.setBothNamesFromPsName(ifv.NamePs)
		ifo := ifv.VarVal.(*GIrALitObj) //  something like:  InterfaceName{funcs}
		var tcctors []GIrA
		var mod *ModuleInfo
		pname, tcname := me.resolveGoTypeRef(gtd.instOf, true)
		if len(pname) == 0 || pname == me.mod.pName {
			mod = me.mod
		} else {
			mod = FindModuleByPName(pname)
		}
		tcctors = mod.girAst.topLevelDefs(func(a GIrA) bool {
			if fn, _ := a.(*GIrAFunc); fn != nil {
				return fn.WasTypeFunc && fn.NamePs == tcname
			}
			return false
		})
		if len(tcctors) > 0 {
			tcctor := tcctors[0].(*GIrAFunc)
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
		}
		nuctor := ªO(&GIrANamedTypeRef{RefAlias: gtd.NameGo})
		nuctor.parent = ifv
		ifv.VarVal = nuctor
		ifv.RefAlias = gtd.instOf
	}
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
						}
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
			case *GIrAVar:
				if a != nil {
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
