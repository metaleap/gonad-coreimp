package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/metaleap/go-util-slice"
	"github.com/metaleap/go-util-str"
)

/*
Intermediate-representation:
of Golang named-and-typed declarations. This gIrANamedTypeRef
is embedded both in declarations that have a name and/or a type
(such as vars, args, consts, funcs etc) and in actual type-defs
(covered by the 'gIrATypeRefFoo' types in this it-typestuff.go).

More details in ir-meta.go.
*/

var (
	strReplSanitizer = strings.NewReplacer("'", "ˇ", "$", "Ø")
)

type gIrANamedTypeRefs []*gIrANamedTypeRef

func (me gIrANamedTypeRefs) byPsName(psname string) *gIrANamedTypeRef {
	for _, gntr := range me {
		if gntr.NamePs == psname {
			return gntr
		}
	}
	return nil
}

func (me gIrANamedTypeRefs) eq(cmp gIrANamedTypeRefs) bool {
	if l := len(me); l != len(cmp) {
		return false
	} else {
		for i := 0; i < l; i++ {
			if !me[i].eq(cmp[i]) {
				return false
			}
		}
	}
	return true
}

type gIrANamedTypeRef struct {
	NamePs string `json:",omitempty"`
	NameGo string `json:",omitempty"`

	RefAlias     string                `json:",omitempty"`
	RefUnknown   int                   `json:",omitempty"`
	RefInterface *gIrATypeRefInterface `json:",omitempty"`
	RefFunc      *gIrATypeRefFunc      `json:",omitempty"`
	RefStruct    *gIrATypeRefStruct    `json:",omitempty"`
	RefArray     *gIrATypeRefArray     `json:",omitempty"`
	RefPtr       *gIrATypeRefPtr       `json:",omitempty"`

	EnumConstNames []string          `json:",omitempty"`
	Methods        gIrANamedTypeRefs `json:",omitempty"`
	Export         bool              `json:",omitempty"`
	WasTypeFunc    bool              `json:",omitempty"`

	method gIrATypeMethod
	ctor   *gIrMTypeDataCtor
	instOf string
}

type gIrATypeMethod struct {
	body      *gIrABlock
	isNewCtor bool
	hasNoThis bool
}

func (me *gIrANamedTypeRef) eq(cmp *gIrANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.eq(cmp.RefInterface) && me.RefFunc.eq(cmp.RefFunc) && me.RefStruct.eq(cmp.RefStruct) && me.RefArray.eq(cmp.RefArray) && me.RefPtr.eq(cmp.RefPtr))
}

func (me *gIrANamedTypeRef) hasTypeInfo() bool {
	return len(me.RefAlias) > 0 || me.RefArray != nil || me.RefFunc != nil || me.RefInterface != nil || me.RefPtr != nil || me.RefStruct != nil || me.RefUnknown != 0
}

func (me *gIrANamedTypeRef) setBothNamesFromPsName(psname string) {
	me.NamePs = psname
	me.NameGo = sanitizeSymbolForGo(psname, me.Export || me.WasTypeFunc)
}

func (me *gIrANamedTypeRef) setRefFrom(tref interface{}) {
	switch tr := tref.(type) {
	case *gIrANamedTypeRef:
		me.RefAlias = tr.RefAlias
		me.RefArray = tr.RefArray
		me.RefFunc = tr.RefFunc
		me.RefInterface = tr.RefInterface
		me.RefPtr = tr.RefPtr
		me.RefStruct = tr.RefStruct
		me.RefUnknown = tr.RefUnknown
	case *gIrATypeRefInterface:
		me.RefInterface = tr
	case *gIrATypeRefFunc:
		me.RefFunc = tr
	case *gIrATypeRefStruct:
		me.RefStruct = tr
	case *gIrATypeRefArray:
		me.RefArray = tr
	case *gIrATypeRefPtr:
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

type gIrATypeRefArray struct {
	Of *gIrANamedTypeRef
}

func (me *gIrATypeRefArray) eq(cmp *gIrATypeRefArray) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.eq(cmp.Of))
}

type gIrATypeRefPtr struct {
	Of *gIrANamedTypeRef
}

func (me *gIrATypeRefPtr) eq(cmp *gIrATypeRefPtr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.eq(cmp.Of))
}

type gIrATypeRefInterface struct {
	Embeds  []string          `json:",omitempty"`
	Methods gIrANamedTypeRefs `json:",omitempty"`

	xtc              *gIrMTypeClass
	xtd              *gIrMTypeDataDecl
	inheritedMethods gIrANamedTypeRefs
}

func (me *gIrATypeRefInterface) eq(cmp *gIrATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.eq(cmp.Methods))
}

func (me *gIrATypeRefInterface) allMethods() (allmethods gIrANamedTypeRefs) {
	allmethods = me.Methods
	if (!areOverlappingInterfacesSupportedByGo) && len(me.Embeds) > 0 {
		if len(me.inheritedMethods) == 0 {
			m := map[string]*gIrANamedTypeRef{}
			for _, embed := range me.Embeds {
				if gtd := findGoTypeByPsQName(embed); gtd == nil || gtd.RefInterface == nil {
					panic(fmt.Errorf("%s: references unknown interface/type-class %s, please report!", me.xtc.Name, embed))
				} else {
					for _, method := range gtd.RefInterface.allMethods() {
						if dupl, _ := m[method.NameGo]; dupl == nil {
							m[method.NameGo], me.inheritedMethods = method, append(me.inheritedMethods, method)
						} else if !dupl.eq(method) {
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

type gIrATypeRefFunc struct {
	Args gIrANamedTypeRefs `json:",omitempty"`
	Rets gIrANamedTypeRefs `json:",omitempty"`
}

func (me *gIrATypeRefFunc) eq(cmp *gIrATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.eq(cmp.Args) && me.Rets.eq(cmp.Rets))
}

type gIrATypeRefStruct struct {
	Embeds    []string          `json:",omitempty"`
	Fields    gIrANamedTypeRefs `json:",omitempty"`
	PassByPtr bool              `json:",omitempty"`
}

func (me *gIrATypeRefStruct) eq(cmp *gIrATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.eq(cmp.Fields))
}

func ensureIfaceForTvar(tdict map[string][]string, tvar string, ifacetname string) {
	if ifaces4tvar := tdict[tvar]; !uslice.StrHas(ifaces4tvar, ifacetname) {
		ifaces4tvar = append(ifaces4tvar, ifacetname)
		tdict[tvar] = ifaces4tvar
	}
}

func findGoTypeByPsQName(qname string) *gIrANamedTypeRef {
	var pname, tname string
	i := strings.LastIndex(qname, ".")
	if tname = qname[i+1:]; i > 0 {
		pname = qname[:i]
		if mod := findModuleByQName(pname); mod == nil {
			panic(fmt.Errorf("Unknown module qname %s", pname))
		} else {
			return mod.girMeta.goTypeDefByPsName(tname)
		}
	} else {
		panic("Unexpected non-qualified type-name encountered, please report with your PS module (and its output-directory json files)!")
	}
	return nil
}

func (me *gonadIrMeta) populateGoTypeDefs() {
	mdict := map[string][]string{}
	var tdict map[string][]string

	for _, ts := range me.EnvTypeSyns {
		tdict = map[string][]string{}
		gtd := &gIrANamedTypeRef{Export: me.hasExport(ts.Name)}
		gtd.setBothNamesFromPsName(ts.Name)
		gtd.setRefFrom(me.toGIrATypeRef(mdict, tdict, ts.Ref))
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}

	for _, tc := range me.EnvTypeClasses {
		tdict = map[string][]string{}
		gif := &gIrATypeRefInterface{xtc: tc}
		for _, tcc := range tc.Constraints {
			for _, tcca := range tcc.Args {
				ensureIfaceForTvar(tdict, tcca.TypeVar, tcc.Class)
			}
			if !uslice.StrHas(gif.Embeds, tcc.Class) {
				gif.Embeds = append(gif.Embeds, tcc.Class)
			}
		}
		for _, tcm := range tc.Members {
			ifm := &gIrANamedTypeRef{NamePs: tcm.Name, NameGo: sanitizeSymbolForGo(tcm.Name, true)}
			ifm.setRefFrom(me.toGIrATypeRef(mdict, tdict, tcm.Ref))
			if ifm.RefFunc == nil {
				if ifm.RefInterface != nil {
					ifm.RefFunc = &gIrATypeRefFunc{
						Rets: gIrANamedTypeRefs{&gIrANamedTypeRef{}},
					}
					ifm.RefFunc.Rets[0].setRefFrom(ifm.RefInterface)
					ifm.RefInterface = nil
				} else if len(ifm.RefAlias) > 0 {
					ifm.RefFunc = &gIrATypeRefFunc{
						Rets: gIrANamedTypeRefs{&gIrANamedTypeRef{RefAlias: ifm.RefAlias}},
					}
					ifm.RefAlias = ""
				} else {
					ifm.RefFunc = &gIrATypeRefFunc{
						Rets: gIrANamedTypeRefs{&gIrANamedTypeRef{RefUnknown: ifm.RefUnknown /*always 0 so far but whatever*/}},
					}
				}
			}
			gif.Methods = append(gif.Methods, ifm)
		}
		tgif := &gIrANamedTypeRef{NamePs: tc.Name, NameGo: tc.Name, Export: me.hasExport(tc.Name)}
		tgif.setRefFrom(gif)
		me.GoTypeDefs = append(me.GoTypeDefs, tgif)
	}

	me.GoTypeDefs = append(me.GoTypeDefs, me.toGIrADataTypeDefs(me.EnvTypeDataDecls, mdict)...)
}

func (me *gonadIrMeta) toGIrADataTypeDefs(typedatadecls []*gIrMTypeDataDecl, mdict map[string][]string) (gtds gIrANamedTypeRefs) {
	var tdict map[string][]string
	for _, td := range typedatadecls {
		tdict = map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, td.Name, td))
		} else {
			isnewtype, hasselfref, hasctorargs := false, false, false
			gid := &gIrANamedTypeRef{RefInterface: &gIrATypeRefInterface{xtd: td}, Export: me.hasExport(td.Name)}
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
					ctor.gtd = &gIrANamedTypeRef{Export: me.hasExport(gid.NamePs + "ĸ" + ctor.Name),
						RefStruct: &gIrATypeRefStruct{PassByPtr: hasselfref || (len(ctor.Args) > 1 && hasctorargs)}}
					ctor.gtd.setBothNamesFromPsName(gid.NamePs + "ˇ" + ctor.Name)
					ctor.gtd.NamePs = ctor.Name
					for ia, ctorarg := range ctor.Args {
						field := &gIrANamedTypeRef{}
						field.setRefFrom(me.toGIrATypeRef(mdict, tdict, ctorarg))
						ctorarg.tmp_assoc = field
						field.NameGo = fmt.Sprintf("%s%d", sanitizeSymbolForGo(ctor.Name, true), ia)
						ctor.gtd.RefStruct.Fields = append(ctor.gtd.RefStruct.Fields, field)
					}
					gtds = append(gtds, ctor.gtd)
				}
			}
			gtds = append(gtds, gid)
		}
	}
	return
}

func (me *gonadIrMeta) toGIrATypeRef(mdict map[string][]string, tdict map[string][]string, tr *gIrMTypeRef) interface{} {
	if len(tr.TypeConstructor) > 0 {
		return tr.TypeConstructor
	} else if tr.REmpty {
		return nil
	} else if len(tr.TypeVar) > 0 {
		embeds := tdict[tr.TypeVar]
		if len(embeds) == 1 {
			return embeds[0]
		}
		return &gIrATypeRefInterface{Embeds: embeds}
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
		rectype := &gIrATypeRefStruct{PassByPtr: true, Fields: gIrANamedTypeRefs{&gIrANamedTypeRef{NamePs: tr.RCons.Label, NameGo: sanitizeSymbolForGo(tr.RCons.Label, false)}}}
		rectype.Fields[0].setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.RCons.Left))
		if nextrow := me.toGIrATypeRef(mdict, tdict, tr.RCons.Right); nextrow != nil {
			rectype.Fields = append(rectype.Fields, nextrow.(*gIrATypeRefStruct).Fields...)
		}
		return rectype
	} else if tr.TypeApp != nil {
		if tr.TypeApp.Left.TypeConstructor == "Prim.Record" {
			return me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor == "Prim.Array" {
			array := &gIrATypeRefArray{Of: &gIrANamedTypeRef{}}
			array.Of.setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return array
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" {
			funtype := &gIrATypeRefFunc{}
			funtype.Args = gIrANamedTypeRefs{&gIrANamedTypeRef{}}
			funtype.Args[0].setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left.TypeApp.Right))
			funtype.Rets = gIrANamedTypeRefs{&gIrANamedTypeRef{}}
			funtype.Rets[0].setRefFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return funtype
		} else if len(tr.TypeApp.Left.TypeConstructor) > 0 {
			return me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left)
		} else {
			//	Nested stuff ie. (Either foo) bar
		}
	}
	return nil
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
			runes := []rune(name)
			runes[0] = unicode.ToLower(runes[0])
			name = string(runes)
		} else {
			switch name {
			case "append", "false", "iota", "nil", "true":
				return "ˇ" + name
			case "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var":
				return "ˇĸˇ" + name
			}
		}
	}
	return strReplSanitizer.Replace(name)
}

func toGIrAEnumConstName(dataname string, ctorname string) string {
	return "tag_" + dataname + "_" + ctorname
}

func toGIrAEnumTypeName(dataname string) string {
	return "tags_" + dataname
}

func typeNameWithPkgName(pkgname string, typename string) (fullname string) {
	if fullname = typename; len(pkgname) > 0 {
		fullname = pkgname + "." + fullname
	}
	return
}
