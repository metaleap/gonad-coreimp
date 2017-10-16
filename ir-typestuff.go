package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/metaleap/go-util-slice"
	"github.com/metaleap/go-util-str"
)

const (
	saniUpperToLowerPrefix = "µˇ"
)

var (
	sanitizer = strings.NewReplacer("'", "ˇ", "$", "Ø")
)

type GIrANamedTypeRefs []*GIrANamedTypeRef

func (me GIrANamedTypeRefs) ByPsName(psname string) *GIrANamedTypeRef {
	for _, gntr := range me {
		if gntr.NamePs == psname {
			return gntr
		}
	}
	return nil
}

func (me GIrANamedTypeRefs) Eq(cmp GIrANamedTypeRefs) bool {
	if l := len(me); l != len(cmp) {
		return false
	} else {
		for i := 0; i < l; i++ {
			if !me[i].Eq(cmp[i]) {
				return false
			}
		}
	}
	return true
}

type GIrANamedTypeRef struct {
	NamePs string `json:",omitempty"`
	NameGo string `json:",omitempty"`

	RefAlias     string                `json:",omitempty"`
	RefUnknown   int                   `json:",omitempty"`
	RefInterface *GIrATypeRefInterface `json:",omitempty"`
	RefFunc      *GIrATypeRefFunc      `json:",omitempty"`
	RefStruct    *GIrATypeRefStruct    `json:",omitempty"`
	RefArray     *GIrATypeRefArray     `json:",omitempty"`
	RefPtr       *GIrATypeRefPtr       `json:",omitempty"`

	EnumConstNames []string          `json:",omitempty"`
	Methods        GIrANamedTypeRefs `json:",omitempty"`
	Export         bool              `json:",omitempty"`
	WasTypeFunc    bool              `json:",omitempty"`

	method  GIrATypeMethod
	ctor    *GIrMTypeDataCtor
	comment *GIrAComments
	instOf  string
}

type GIrATypeMethod struct {
	body      *GIrABlock
	isNewCtor bool
	hasNoThis bool
}

func (me *GIrANamedTypeRef) Eq(cmp *GIrANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.Eq(cmp.RefInterface) && me.RefFunc.Eq(cmp.RefFunc) && me.RefStruct.Eq(cmp.RefStruct) && me.RefArray.Eq(cmp.RefArray) && me.RefPtr.Eq(cmp.RefPtr))
}

func (me *GIrANamedTypeRef) HasTypeInfo() bool {
	return len(me.RefAlias) > 0 || me.RefArray != nil || me.RefFunc != nil || me.RefInterface != nil || me.RefPtr != nil || me.RefStruct != nil || me.RefUnknown != 0
}

func (me *GIrANamedTypeRef) setBothNamesFromPsName(psname string) {
	me.NamePs = psname
	me.NameGo = sanitizeSymbolForGo(psname, me.Export || me.WasTypeFunc)
}

func (me *GIrANamedTypeRef) setRefFrom(tref interface{}) {
	switch tr := tref.(type) {
	case *GIrANamedTypeRef:
		me.RefAlias = tr.RefAlias
		me.RefArray = tr.RefArray
		me.RefFunc = tr.RefFunc
		me.RefInterface = tr.RefInterface
		me.RefPtr = tr.RefPtr
		me.RefStruct = tr.RefStruct
		me.RefUnknown = tr.RefUnknown
	case *GIrATypeRefInterface:
		me.RefInterface = tr
	case *GIrATypeRefFunc:
		me.RefFunc = tr
	case *GIrATypeRefStruct:
		me.RefStruct = tr
	case *GIrATypeRefArray:
		me.RefArray = tr
	case *GIrATypeRefPtr:
		me.RefPtr = tr
	case int:
		me.RefUnknown = tr
	case string:
		me.RefAlias = tr
	case nil:
	default:
		println(tref.(string)) // in case of future oversight, this triggers immediate panic-msg with actual-type included
	}
}

type GIrATypeRefArray struct {
	Of *GIrANamedTypeRef
}

func (me *GIrATypeRefArray) Eq(cmp *GIrATypeRefArray) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.Eq(cmp.Of))
}

type GIrATypeRefPtr struct {
	Of *GIrANamedTypeRef
}

func (me *GIrATypeRefPtr) Eq(cmp *GIrATypeRefPtr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.Eq(cmp.Of))
}

type GIrATypeRefInterface struct {
	Embeds  []string          `json:",omitempty"`
	Methods GIrANamedTypeRefs `json:",omitempty"`

	xtc              *GIrMTypeClass
	xtd              *GIrMTypeDataDecl
	inheritedMethods GIrANamedTypeRefs
}

func (me *GIrATypeRefInterface) Eq(cmp *GIrATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.Eq(cmp.Methods))
}

func (me *GIrATypeRefInterface) allMethods() (allmethods GIrANamedTypeRefs) {
	allmethods = me.Methods
	if (!areOverlappingInterfacesSupportedByGo) && len(me.Embeds) > 0 {
		if len(me.inheritedMethods) == 0 {
			m := map[string]*GIrANamedTypeRef{}
			for _, embed := range me.Embeds {
				if gtd := findGoTypeByPsQName(embed); gtd == nil || gtd.RefInterface == nil {
					panic(fmt.Errorf("%s: references unknown interface/type-class %s, please report!", me.xtc.Name, embed))
				} else {
					for _, method := range gtd.RefInterface.allMethods() {
						if dupl, _ := m[method.NameGo]; dupl == nil {
							m[method.NameGo], me.inheritedMethods = method, append(me.inheritedMethods, method)
						} else if !dupl.Eq(method) {
							panic("Interface (generated from type-class " + me.xtc.Name + ") would inherit multiple (but different-signature) methods named " + method.NameGo)
						}
					}
				}
			}
		}
		allmethods = append(me.inheritedMethods, allmethods...)
	}
	return
}

type GIrATypeRefFunc struct {
	Args GIrANamedTypeRefs `json:",omitempty"`
	Rets GIrANamedTypeRefs `json:",omitempty"`
}

func (me *GIrATypeRefFunc) Eq(cmp *GIrATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.Eq(cmp.Args) && me.Rets.Eq(cmp.Rets))
}

type GIrATypeRefStruct struct {
	Embeds    []string          `json:",omitempty"`
	Fields    GIrANamedTypeRefs `json:",omitempty"`
	PassByPtr bool              `json:",omitempty"`
}

func (me *GIrATypeRefStruct) Eq(cmp *GIrATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.Eq(cmp.Fields))
}

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
		gif := &GIrATypeRefInterface{xtc: tc}
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
					ifm.RefFunc = &GIrATypeRefFunc{
						Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefUnknown: ifm.RefUnknown /*always 0 so far but whatever*/}},
					}
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

func sanitizeSymbolForGo(name string, upper bool) string {
	if len(name) == 0 {
		return name
	}
	if upper {
		runes := []rune(name)
		runes[0] = unicode.ToUpper(runes[0])
		name = string(runes)
	} else {
		if ustr.BeginsUpper(name) {
			name = saniUpperToLowerPrefix + name
		} else {
			switch name {
			case "append", "false", "iota", "nil", "true":
				return "ˇ" + name
			case "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var":
				return "ˇĸˇ" + name
			}
		}
	}
	return sanitizer.Replace(name)
}

func (me *GonadIrMeta) toGIrADataTypeDefs(exttypedatadecls []*GIrMTypeDataDecl, mdict map[string][]string, forexport bool) (gtds GIrANamedTypeRefs) {
	const USE_LEGACY_METHODS_APPROACH = false
	var tdict map[string][]string
	for _, td := range exttypedatadecls {
		tdict = map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, td.Name, td))
		} else {
			isnewtype, hasselfref, hasctorargs := false, false, false
			gid := &GIrANamedTypeRef{RefInterface: &GIrATypeRefInterface{xtd: td}, Export: forexport}
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

					if USE_LEGACY_METHODS_APPROACH {
						method_iskind := &GIrANamedTypeRef{ctor: ctor, NameGo: "Is" + ctor.Name, RefFunc: &GIrATypeRefFunc{
							Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: "Prim.Boolean"}},
						}}
						gid.RefInterface.Methods = append(gid.RefInterface.Methods, method_iskind)
					}
					method_ret := &GIrANamedTypeRef{RefPtr: &GIrATypeRefPtr{Of: &GIrANamedTypeRef{RefAlias: ctor.gtd.NameGo}}}
					if numargs := len(ctor.Args); numargs > 0 {
						if USE_LEGACY_METHODS_APPROACH {
							method_askind := &GIrANamedTypeRef{ctor: ctor, NameGo: "As" + ctor.Name,
								RefFunc: &GIrATypeRefFunc{Rets: GIrANamedTypeRefs{method_ret}}}
							gid.RefInterface.Methods = append(gid.RefInterface.Methods, method_askind)
						}
					}
					gtds = append(gtds, ctor.gtd)
				}
				if USE_LEGACY_METHODS_APPROACH {
					for _, ctor := range td.Ctors {
						for _, method := range gid.RefInterface.Methods {
							mcopy := *method
							mcopy.method.body, mcopy.method.hasNoThis = ªBlock(), true
							if strings.HasPrefix(method.NameGo, "Is") {
								mcopy.method.body.Add(ªRet(ªB(method.ctor == ctor)))
							} else if strings.HasPrefix(method.NameGo, "As") {
								if method.ctor == ctor {
									mcopy.method.hasNoThis = false
									if ctor.gtd.RefStruct.PassByPtr {
										mcopy.method.body.Add(ªRet(ªSym("this")))
									} else {
										mcopy.method.body.Add(ªRet(ªO1("&", ªSym("this"))))
									}
								} else {
									mcopy.method.body.Add(ªRet(ªNil()))
								}
							}
							ctor.gtd.Methods = append(ctor.gtd.Methods, &mcopy)
						}
					}
				}
			}
			gtds = append(gtds, gid)
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

func typeNameWithPkgName(pkgname string, typename string) (fullname string) {
	if fullname = typename; len(pkgname) > 0 {
		fullname = pkgname + "." + fullname
	}
	return
}
