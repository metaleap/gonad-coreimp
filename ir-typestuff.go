package main

import (
	"fmt"
	"strings"

	"github.com/metaleap/go-util-slice"
)

type GIrMNamedTypeRef struct {
	Name string       `json:"tnn"`
	Ref  *GIrMTypeRef `json:"tnr,omitempty"`
}

type GIrMTypeClass struct {
	Name        string               `json:"tcn"`
	Constraints []*GIrMTypeRefConstr `json:"tcc,omitempty"`
	TypeArgs    []string             `json:"tca,omitempty"`
	Members     []GIrMNamedTypeRef   `json:"tcm,omitempty"`
}

type GIrMTypeDataDecl struct {
	Name  string             `json:"tdn"`
	Ctors []GIrMTypeDataCtor `json:"tdc,omitempty"`
	Args  []string           `json:"tda,omitempty"`
}

type GIrMTypeDataCtor struct {
	Name string       `json:"tdcn"`
	Args GIrMTypeRefs `json:"tdca,omitempty"`
}

type GIrMTypeRefs []*GIrMTypeRef

type GIrMTypeRef struct {
	TypeConstructor string             `json:"tc,omitempty"`
	TypeVar         string             `json:"tv,omitempty"`
	REmpty          bool               `json:"re,omitempty"`
	TUnknown        int                `json:"tu,omitempty"`
	TypeApp         *GIrMTypeRefAppl   `json:"ta,omitempty"`
	ConstrainedType *GIrMTypeRefConstr `json:"ct,omitempty"`
	RCons           *GIrMTypeRefRow    `json:"rc,omitempty"`
	ForAll          *GIrMTypeRefExist  `json:"fa,omitempty"`
	Skolem          *GIrMTypeRefSkolem `json:"sk,omitempty"`

	tmp_assoc *GIrANamedTypeRef
}

func (me *GIrMTypeRef) Eq(cmp *GIrMTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.TypeConstructor == cmp.TypeConstructor && me.TypeVar == cmp.TypeVar && me.REmpty == cmp.REmpty && me.TUnknown == cmp.TUnknown && me.TypeApp.Eq(cmp.TypeApp) && me.ConstrainedType.Eq(cmp.ConstrainedType) && me.RCons.Eq(cmp.RCons) && me.ForAll.Eq(cmp.ForAll) && me.Skolem.Eq(cmp.Skolem))
}

func (me GIrMTypeRefs) Eq(cmp GIrMTypeRefs) bool {
	if len(me) != len(cmp) {
		return false
	}
	for i, _ := range me {
		if !me[i].Eq(cmp[i]) {
			return false
		}
	}
	return true
}

type GIrMTypeRefAppl struct {
	Left  *GIrMTypeRef `json:"t1,omitempty"`
	Right *GIrMTypeRef `json:"t2,omitempty"`
}

func (me *GIrMTypeRefAppl) Eq(cmp *GIrMTypeRefAppl) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Left.Eq(cmp.Left) && me.Right.Eq(cmp.Right))
}

type GIrMTypeRefRow struct {
	Label string       `json:"rl,omitempty"`
	Left  *GIrMTypeRef `json:"r1,omitempty"`
	Right *GIrMTypeRef `json:"r2,omitempty"`
}

func (me *GIrMTypeRefRow) Eq(cmp *GIrMTypeRefRow) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Label == cmp.Label && me.Left.Eq(cmp.Left) && me.Right.Eq(cmp.Right))
}

type GIrMTypeRefConstr struct {
	Class string       `json:"cc,omitempty"`
	Data  interface{}  `json:"cd,omitempty"` // when needed: Data = [[Text]] Bool
	Args  GIrMTypeRefs `json:"ca,omitempty"`
	Ref   *GIrMTypeRef `json:"cr,omitempty"`
}

func (me *GIrMTypeRefConstr) Eq(cmp *GIrMTypeRefConstr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Class == cmp.Class && me.Data == cmp.Data && me.Ref.Eq(cmp.Ref) && me.Args.Eq(cmp.Args))
}

type GIrMTypeRefExist struct {
	Name        string       `json:"en,omitempty"`
	Ref         *GIrMTypeRef `json:"er,omitempty"`
	SkolemScope *int         `json:"es,omitempty"`
}

func (me *GIrMTypeRefExist) Eq(cmp *GIrMTypeRefExist) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Ref.Eq(cmp.Ref) && me.SkolemScope == cmp.SkolemScope)
}

type GIrMTypeRefSkolem struct {
	Name  string `json:"sn,omitempty"`
	Value int    `json:"sv,omitempty"`
	Scope int    `json:"ss,omitempty"`
}

func (me *GIrMTypeRefSkolem) Eq(cmp *GIrMTypeRefSkolem) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Name == cmp.Name && me.Value == cmp.Value && me.Scope == cmp.Scope)
}

func ensureIfaceForTvar(tdict map[string][]string, tvar string, ifacetname string) {
	if ifaces4tvar := tdict[tvar]; !uslice.StrHas(ifaces4tvar, ifacetname) {
		ifaces4tvar = append(ifaces4tvar, ifacetname)
		tdict[tvar] = ifaces4tvar
	}
}

func (me *GonadIrMeta) populateGoTypeDefs() {
	mdict := map[string][]string{}
	var tdict map[string][]string

	for _, ta := range me.ExtTypeAliases {
		tdict = map[string][]string{}
		gtd := &GIrANamedTypeRef{Name: ta.Name, Export: true}
		gtd.setFrom(me.toGIrATypeRef(mdict, tdict, ta.Ref))
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
			ifm := &GIrANamedTypeRef{Name: tcm.Name}
			ifm.setFrom(me.toGIrATypeRef(mdict, tdict, tcm.Ref))
			if ifm.RefFunc == nil {
				if ifm.RefInterface != nil {
					ifm.RefFunc = &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{}},
					}
					ifm.RefFunc.Rets[0].setFrom(ifm.RefInterface)
					ifm.RefInterface = nil
				} else if len(ifm.RefAlias) > 0 {
					ifm.RefFunc = &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: ifm.RefAlias}},
					}
					ifm.RefAlias = ""
				} else {
					panic(me.mod.srcFilePath + ": " + tc.Name + "." + ifm.Name + ": strangely unrecognized or missing typevar-typeclass relation, please report!")
				}
			}
			gif.Methods = append(gif.Methods, ifm)
		}
		tgif := &GIrANamedTypeRef{Name: tc.Name, Export: true}
		tgif.setFrom(gif)
		me.GoTypeDefs = append(me.GoTypeDefs, tgif)
	}

	for _, td := range me.ExtTypeDataDecls {
		tdict = map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, td.Name, td))
		} else {
			gtd, isnewtype := &GIrANamedTypeRef{Name: td.Name, RefAlias: toGIrAEnumTypeName(td.Name)}, false
			for _, ctor := range td.Ctors {
				gtd.EnumConstNames = append(gtd.EnumConstNames, toGIrAEnumConstName(td.Name, ctor.Name))
				if numargs := len(ctor.Args); numargs > 0 {
					// noctorargs = false
					if numargs == 1 && numctors == 1 {
						isnewtype = true
					}
				}
			}

			gtd.RefAlias = ""
			if isnewtype {
				gtd.setFrom(me.toGIrATypeRef(mdict, tdict, td.Ctors[0].Args[0]))
				gtd.EnumConstNames = nil
			} else {
				gtd.RefStruct = &GIrATypeRefStruct{}
				gtd.RefStruct.Fields = append(gtd.RefStruct.Fields, &GIrANamedTypeRef{Name: "kindtag", RefAlias: toGIrAEnumTypeName(td.Name)})
				for _, ctor := range td.Ctors {
					for ia, ctorarg := range ctor.Args {
						prefix, hasfieldherewithsametype := fmt.Sprintf("v%d_", ia), false
						field := &GIrANamedTypeRef{}
						field.setFrom(me.toGIrATypeRef(mdict, tdict, ctorarg))
						ctorarg.tmp_assoc = field
						for _, f := range gtd.RefStruct.Fields {
							if strings.HasPrefix(f.Name, prefix) && f.Eq(field) {
								hasfieldherewithsametype, ctorarg.tmp_assoc = true, f
								f.Name = fmt.Sprintf("%s_%s", f.Name, ctor.Name)
								break
							}
						}
						if !hasfieldherewithsametype {
							field.Name = fmt.Sprintf("%s%s", prefix, ctor.Name)
							gtd.RefStruct.Fields = append(gtd.RefStruct.Fields, field)
						}
					}
				}
			}
			if !isnewtype {
				for _, ctor := range td.Ctors {
					method_iskind := &GIrANamedTypeRef{Name: "Is" + ctor.Name, RefFunc: &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: "Prim.Boolean"}},
					}}
					method_iskind.mBody.Add(
						ſRet(ſEq(ſDot(ſV("this"), "kindtag"), ſV(toGIrAEnumConstName(gtd.Name, ctor.Name)))))
					gtd.Methods = append(gtd.Methods, method_iskind)

					method_new := &GIrANamedTypeRef{mCtor: true, Name: gtd.Name + "As" + ctor.Name, RefFunc: &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{Name: "this", RefAlias: gtd.Name}},
					}}
					method_new.mBody.Add(ſSet("this.kindtag", ſV(toGIrAEnumConstName(gtd.Name, ctor.Name))))

					if numargs := len(ctor.Args); numargs > 0 {
						method_ctor := &GIrANamedTypeRef{Name: ctor.Name, RefFunc: &GIrATypeRefFunc{}}
						for i, ctorarg := range ctor.Args {
							if ctorarg.tmp_assoc != nil {
								retarg := &GIrANamedTypeRef{Name: fmt.Sprintf("v%v", i)}
								retarg.setFrom(me.toGIrATypeRef(mdict, tdict, ctorarg))
								method_new.RefFunc.Args = append(method_new.RefFunc.Args, retarg)
								method_new.mBody.Add(ſSet("this."+ctorarg.tmp_assoc.Name, ſV(retarg.Name)))
								method_ctor.RefFunc.Rets = append(method_ctor.RefFunc.Rets, retarg)
								method_ctor.mBody.Add(
									ſSet(retarg.Name, ſDot(ſV("this"), fmt.Sprintf("%v", ctorarg.tmp_assoc.Name))))
								if numargs > 1 {
									method_ctorarg := &GIrANamedTypeRef{Name: fmt.Sprintf("%s%d", ctor.Name, i),
										RefFunc: &GIrATypeRefFunc{Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{}}}}
									method_ctorarg.RefFunc.Rets[0].setFrom(me.toGIrATypeRef(mdict, tdict, ctorarg))
									method_ctorarg.mBody.Add(ſRet(ſDot(ſV("this"), ctorarg.tmp_assoc.Name)))
									gtd.Methods = append(gtd.Methods, method_ctorarg)
								}
							}
						}
						method_ctor.mBody.Add(ſRet(nil))
						gtd.Methods = append(gtd.Methods, method_ctor)
					}
					method_new.mBody.Add(ſRet(nil))
					gtd.Methods = append(gtd.Methods, method_new)
				}
			}
			me.GoTypeDefs = append(me.GoTypeDefs, gtd)
		}
	}
}

func toGIrAEnumConstName(dataname string, ctorname string) string {
	return "ĸ" + dataname + "_" + ctorname
}

func toGIrAEnumTypeName(dataname string) string {
	return "ĸ" + dataname
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
			panic(fmt.Errorf("%s: unexpected type-class/type-var association %v, please report!", me.mod.srcFilePath, tr.ConstrainedType))
		}
		ensureIfaceForTvar(tdict, tr.ConstrainedType.Args[0].TypeVar, tr.ConstrainedType.Class)
		return me.toGIrATypeRef(mdict, tdict, tr.ConstrainedType.Ref)
	} else if tr.ForAll != nil {
		return me.toGIrATypeRef(mdict, tdict, tr.ForAll.Ref)
	} else if tr.Skolem != nil {
		return fmt.Sprintf("Skolem_%s_scope%d_value%d", tr.Skolem.Name, tr.Skolem.Scope, tr.Skolem.Value)
	} else if tr.RCons != nil {
		rectype := &GIrATypeRefStruct{Fields: []*GIrANamedTypeRef{&GIrANamedTypeRef{Name: tr.RCons.Label}}}
		rectype.Fields[0].setFrom(me.toGIrATypeRef(mdict, tdict, tr.RCons.Left))
		if nextrow := me.toGIrATypeRef(mdict, tdict, tr.RCons.Right); nextrow != nil {
			rectype.Fields = append(rectype.Fields, nextrow.(*GIrATypeRefStruct).Fields...)
		}
		return rectype
	} else if tr.TypeApp != nil {
		if tr.TypeApp.Left.TypeConstructor == "Prim.Record" {
			return me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor == "Prim.Array" {
			array := &GIrATypeRefArray{Of: &GIrANamedTypeRef{}}
			array.Of.setFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return array
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" {
			funtype := &GIrATypeRefFunc{}
			funtype.Args = []*GIrANamedTypeRef{&GIrANamedTypeRef{}}
			funtype.Args[0].setFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left.TypeApp.Right))
			funtype.Rets = []*GIrANamedTypeRef{&GIrANamedTypeRef{}}
			funtype.Rets[0].setFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return funtype
		} else {
			// println(me.mod.srcFilePath + "\n\t" + tr.TypeApp.Left.TypeConstructor + "\t" + tr.TypeApp.Right.TypeConstructor)
		}
	}
	return nil
}
