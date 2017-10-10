package main

import (
	"fmt"
	"strings"

	"github.com/metaleap/go-util-slice"
)

func ensureIfaceForTvar(tdict map[string][]string, tvar string, ifacetname string) {
	if ifaces4tvar := tdict[tvar]; !uslice.StrHas(ifaces4tvar, ifacetname) {
		ifaces4tvar = append(ifaces4tvar, ifacetname)
		tdict[tvar] = ifaces4tvar
	}
}

func findGoTypeByPsQName(qname string) *GIrANamedTypeRef {
	var pname, tname string
	i := strings.LastIndex(qname, ".")
	if tname = qname[i+1:]; i > 0 {
		pname = qname[:i]
		if mod := FindModuleByQName(pname); mod == nil {
			panic(fmt.Errorf("Unknown module qname %s", pname))
		} else {
			return mod.girMeta.GoTypeDefByPsName(tname)
		}
	} else {
		panic("Unexpected non-qualified type-name encountered, please report with your PS module (and its output-directory json files)!")
	}
	return nil
}

func (me *GonadIrMeta) populateGoTypeDefs() {
	mdict := map[string][]string{}
	var tdict map[string][]string

	for _, ta := range me.ExtTypeAliases {
		tdict = map[string][]string{}
		gtd := &GIrANamedTypeRef{NamePs: ta.Name, NameGo: ta.Name, Export: true}
		gtd.setRefFrom(me.toGIrATypeRef(mdict, tdict, ta.Ref))
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}

	for _, tc := range me.ExtTypeClasses {
		tdict = map[string][]string{}
		gif := &GIrATypeRefInterface{xtc: &tc}
		for _, tcc := range tc.Constraints {
			for _, tcca := range tcc.Args {
				ensureIfaceForTvar(tdict, tcca.TypeVar, tcc.Class)
			}
			if !uslice.StrHas(gif.Embeds, tcc.Class) {
				gif.Embeds = append(gif.Embeds, tcc.Class)
			}
		}
		for _, tcm := range tc.Members {
			ifm := &GIrANamedTypeRef{NamePs: tcm.Name, NameGo: sanitizeSymbolForGo(tcm.Name, true)}
			ifm.setRefFrom(me.toGIrATypeRef(mdict, tdict, tcm.Ref))
			if ifm.RefFunc == nil {
				if ifm.RefInterface != nil {
					ifm.RefFunc = &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{}},
					}
					ifm.RefFunc.Rets[0].setRefFrom(ifm.RefInterface)
					ifm.RefInterface = nil
				} else if len(ifm.RefAlias) > 0 {
					ifm.RefFunc = &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: ifm.RefAlias}},
					}
					ifm.RefAlias = ""
				} else {
					panic(me.mod.srcFilePath + ": " + tc.Name + "." + ifm.NamePs + ": strangely unrecognized or missing typevar-typeclass relation, please report!")
				}
			}
			gif.Methods = append(gif.Methods, ifm)
		}
		tgif := &GIrANamedTypeRef{NamePs: tc.Name, NameGo: tc.Name, Export: true}
		tgif.setRefFrom(gif)
		me.GoTypeDefs = append(me.GoTypeDefs, tgif)
	}

	me.GoTypeDefs = append(me.GoTypeDefs, me.toGIrADataTypeDefs(me.ExtTypeDataDecls, mdict, true)...)
}

func (me *GonadIrMeta) toGIrADataTypeDefs(exttypedatadecls []GIrMTypeDataDecl, mdict map[string][]string, forexport bool) (gtds GIrANamedTypeRefs) {
	var tdict map[string][]string
	for _, td := range exttypedatadecls {
		tdict = map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, td.Name, td))
		} else {
			isnewtype, hasselfref, hasctorargs := false, false, false
			gid := &GIrANamedTypeRef{RefInterface: &GIrATypeRefInterface{}, Export: forexport}
			gid.setBothNamesFromPsName(td.Name)
			for _, ctor := range td.Ctors {
				if numargs := len(ctor.Args); numargs > 0 {
					if hasctorargs = true; numargs == 1 && numctors == 1 {
						if ctor.Args[0].TypeConstructor != (me.mod.qName + "." + td.Name) {
							isnewtype = true
						} else {
							hasselfref = true
						}
					}
				}
			}
			gtds = append(gtds, gid)
			if isnewtype {
				gid.RefInterface = nil
				gid.setRefFrom(me.toGIrATypeRef(mdict, tdict, td.Ctors[0].Args[0]))
			} else {
				for _, ctor := range td.Ctors {
					ctor.gtd = &GIrANamedTypeRef{Export: forexport, RefStruct: &GIrATypeRefStruct{PassByPtr: hasselfref || (len(ctor.Args) > 1 && hasctorargs)}}
					ctor.gtd.setBothNamesFromPsName(gid.NamePs + "ˇ" + ctor.Name)
					ctor.gtd.NamePs = ctor.Name
					ctor.gtd.comment = ctor.comment
					for ia, ctorarg := range ctor.Args {
						field := &GIrANamedTypeRef{}
						field.setRefFrom(me.toGIrATypeRef(mdict, tdict, ctorarg))
						ctorarg.tmp_assoc = field
						field.NameGo = fmt.Sprintf("%s%d", sanitizeSymbolForGo(ctor.Name, true), ia)
						ctor.gtd.RefStruct.Fields = append(ctor.gtd.RefStruct.Fields, field)
					}

					method_iskind := &GIrANamedTypeRef{ctor: ctor, NameGo: "Is" + ctor.Name, RefFunc: &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: "Prim.Boolean"}},
					}}
					gid.RefInterface.Methods = append(gid.RefInterface.Methods, method_iskind)
					method_ret := &GIrANamedTypeRef{RefPtr: &GIrATypeRefPtr{Of: &GIrANamedTypeRef{RefAlias: ctor.gtd.NameGo}}}
					if numargs := len(ctor.Args); numargs > 0 {
						method_askind := &GIrANamedTypeRef{ctor: ctor, NameGo: "As" + ctor.Name,
							RefFunc: &GIrATypeRefFunc{Rets: GIrANamedTypeRefs{method_ret}}}
						gid.RefInterface.Methods = append(gid.RefInterface.Methods, method_askind)
					}
					gtds = append(gtds, ctor.gtd)
				}
				for _, ctor := range td.Ctors {
					for _, method := range gid.RefInterface.Methods {
						mcopy := *method
						mcopy.method.body, mcopy.method.hasNoThis = &GIrABlock{}, true
						if strings.HasPrefix(method.NameGo, "Is") {
							mcopy.method.body.Add(ªRet(ªB(method.ctor == ctor)))
						} else if strings.HasPrefix(method.NameGo, "As") {
							if method.ctor == ctor {
								mcopy.method.hasNoThis = false
								if ctor.gtd.RefStruct.PassByPtr {
									mcopy.method.body.Add(ªRet(ªV("this")))
								} else {
									mcopy.method.body.Add(ªRet(ªO1("&", ªV("this"))))
								}
							} else {
								mcopy.method.body.Add(ªRet(&GIrANil{}))
							}
						}
						ctor.gtd.Methods = append(ctor.gtd.Methods, &mcopy)
					}
				}
			}
		}
	}
	return
}

func toGIrAEnumConstName(dataname string, ctorname string) string {
	return "tag_" + dataname + "_" + ctorname
}

func toGIrAEnumTypeName(dataname string) string {
	return "tags_" + dataname
}

func (me *GonadIrMeta) toGIrATypeRef(mdict map[string][]string, tdict map[string][]string, tr *GIrMTypeRef) interface{} {
	if len(tr.TypeConstructor) > 0 {
		return tr.TypeConstructor
	} else if tr.REmpty {
		return nil
	} else if tr.TUnknown > 0 {
		return tr.TUnknown
	} else if len(tr.TypeVar) > 0 {
		embeds := tdict[tr.TypeVar]
		if len(embeds) == 1 {
			return embeds[0]
		}
		return &GIrATypeRefInterface{Embeds: embeds}
	} else if tr.ConstrainedType != nil {
		if len(tr.ConstrainedType.Args) == 0 || len(tr.ConstrainedType.Args[0].TypeVar) == 0 {
			ensureIfaceForTvar(tdict, "", tr.ConstrainedType.Class) // TODO deal with this properly
		} else {
			ensureIfaceForTvar(tdict, tr.ConstrainedType.Args[0].TypeVar, tr.ConstrainedType.Class)
		}
		return me.toGIrATypeRef(mdict, tdict, tr.ConstrainedType.Ref)
	} else if tr.ForAll != nil {
		return me.toGIrATypeRef(mdict, tdict, tr.ForAll.Ref)
	} else if tr.Skolem != nil {
		return fmt.Sprintf("Skolem_%s_scope%d_value%d", tr.Skolem.Name, tr.Skolem.Scope, tr.Skolem.Value)
	} else if tr.RCons != nil {
		rectype := &GIrATypeRefStruct{PassByPtr: true, Fields: GIrANamedTypeRefs{&GIrANamedTypeRef{NamePs: tr.RCons.Label, NameGo: sanitizeSymbolForGo(tr.RCons.Label, false)}}}
		rectype.Fields[0].setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.RCons.Left))
		if nextrow := me.toGIrATypeRef(mdict, tdict, tr.RCons.Right); nextrow != nil {
			rectype.Fields = append(rectype.Fields, nextrow.(*GIrATypeRefStruct).Fields...)
		}
		return rectype
	} else if tr.TypeApp != nil {
		if tr.TypeApp.Left.TypeConstructor == "Prim.Record" {
			return me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor == "Prim.Array" {
			array := &GIrATypeRefArray{Of: &GIrANamedTypeRef{}}
			array.Of.setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return array
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" {
			funtype := &GIrATypeRefFunc{}
			funtype.Args = GIrANamedTypeRefs{&GIrANamedTypeRef{}}
			funtype.Args[0].setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left.TypeApp.Right))
			funtype.Rets = GIrANamedTypeRefs{&GIrANamedTypeRef{}}
			funtype.Rets[0].setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return funtype
		} else if len(tr.TypeApp.Left.TypeConstructor) > 0 {
			if len(tr.TypeApp.Right.TypeVar) > 0 {
				//	`Maybe a`. for now:
				return me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left)
			} else if len(tr.TypeApp.Right.TypeConstructor) > 0 {
				//	`Maybe Int`. for now
				return me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left)
			} else {
				//	I'll deal with it when it occurs
				panic(me.mod.srcFilePath + ": type-application of " + tr.TypeApp.Left.TypeConstructor + " to unrecognized right-hand side, please report! ")
			}
		} else {
			//	Nested stuff ie. (Either foo) bar
		}
	}
	return nil
}
