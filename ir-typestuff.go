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
of Golang named-and-typed declarations. This irANamedTypeRef
is embedded both in declarations that have a name and/or a type
(such as vars, args, consts, funcs etc) and in actual type-defs
(covered by the 'irATypeRefFoo' types in this it-typestuff.go).

More details in ir-meta.go.
*/

const (
	areOverlappingInterfacesSupportedByGo = false // this might change hopefully, see https://github.com/golang/go/issues/6977
	legacyIfaceEmbeds                     = false
)

var (
	strReplSanitizer = strings.NewReplacer("'", "ˇ", "$", "Ø")
)

type irANamedTypeRefs []*irANamedTypeRef

func (me irANamedTypeRefs) Len() int { return len(me) }
func (me irANamedTypeRefs) Less(i, j int) bool {
	return strings.ToLower(me[i].NameGo) < strings.ToLower(me[j].NameGo)
}
func (me irANamedTypeRefs) Swap(i, j int) { me[i], me[j] = me[j], me[i] }

func (me irANamedTypeRefs) byPsName(psname string) *irANamedTypeRef {
	for _, gntr := range me {
		if gntr.NamePs == psname {
			return gntr
		}
	}
	return nil
}

func (me irANamedTypeRefs) eq(cmp irANamedTypeRefs) bool {
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

type irANamedTypeRef struct {
	NamePs string `json:",omitempty"`
	NameGo string `json:",omitempty"`

	RefAlias     string               `json:",omitempty"`
	RefUnknown   int                  `json:",omitempty"`
	RefInterface *irATypeRefInterface `json:",omitempty"`
	RefFunc      *irATypeRefFunc      `json:",omitempty"`
	RefStruct    *irATypeRefStruct    `json:",omitempty"`
	RefArray     *irATypeRefArray     `json:",omitempty"`
	RefPtr       *irATypeRefPtr       `json:",omitempty"`

	Export bool `json:",omitempty"`
}

func (me *irANamedTypeRef) copyFrom(from *irANamedTypeRef, names bool, trefs bool, export bool) {
	if names {
		me.NameGo, me.NamePs = from.NameGo, from.NamePs
	}
	if trefs {
		me.RefAlias, me.RefUnknown, me.RefInterface, me.RefFunc, me.RefStruct, me.RefArray, me.RefPtr = from.RefAlias, from.RefUnknown, from.RefInterface, from.RefFunc, from.RefStruct, from.RefArray, from.RefPtr
	}
	if export {
		me.Export = from.Export
	}
}

func (me *irANamedTypeRef) eq(cmp *irANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.eq(cmp.RefInterface) && me.RefFunc.eq(cmp.RefFunc) && me.RefStruct.eq(cmp.RefStruct) && me.RefArray.eq(cmp.RefArray) && me.RefPtr.eq(cmp.RefPtr))
}

func (me *irANamedTypeRef) hasTypeInfo() bool {
	return len(me.RefAlias) > 0 || me.RefArray != nil || me.RefFunc != nil || me.RefInterface != nil || me.RefPtr != nil || me.RefStruct != nil || me.RefUnknown != 0
}

func (me *irANamedTypeRef) setBothNamesFromPsName(psname string) {
	me.NamePs = psname
	me.NameGo = sanitizeSymbolForGo(psname, me.Export)
}

func (me *irANamedTypeRef) setRefFrom(tref interface{}) {
	switch tr := tref.(type) {
	case *irANamedTypeRef:
		me.RefAlias = tr.RefAlias
		me.RefArray = tr.RefArray
		me.RefFunc = tr.RefFunc
		me.RefInterface = tr.RefInterface
		me.RefPtr = tr.RefPtr
		me.RefStruct = tr.RefStruct
		me.RefUnknown = tr.RefUnknown
	case *irATypeRefInterface:
		me.RefInterface = tr
	case *irATypeRefFunc:
		me.RefFunc = tr
	case *irATypeRefStruct:
		me.RefStruct = tr
	case *irATypeRefArray:
		me.RefArray = tr
	case *irATypeRefPtr:
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

type irATypeRefArray struct {
	Of *irANamedTypeRef
}

func (me *irATypeRefArray) eq(cmp *irATypeRefArray) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.eq(cmp.Of))
}

type irATypeRefPtr struct {
	Of *irANamedTypeRef
}

func (me *irATypeRefPtr) eq(cmp *irATypeRefPtr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.eq(cmp.Of))
}

type irATypeRefInterface struct {
	Embeds  []string         `json:",omitempty"`
	Methods irANamedTypeRefs `json:",omitempty"`

	xtc              *irMTypeClass
	xtd              *irMTypeDataDecl
	inheritedMethods irANamedTypeRefs
}

func (me *irATypeRefInterface) eq(cmp *irATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.eq(cmp.Methods))
}

func (me *irATypeRefInterface) allMethods() (allmethods irANamedTypeRefs) {
	allmethods = me.Methods
	if legacyIfaceEmbeds && (!areOverlappingInterfacesSupportedByGo) && len(me.Embeds) > 0 {
		if len(me.inheritedMethods) == 0 {
			m := map[string]*irANamedTypeRef{}
			for _, embed := range me.Embeds {
				if gtd := findGoTypeByPsQName(embed); gtd == nil || gtd.RefInterface == nil {
					panic(fmt.Errorf("%s: references unknown interface/type-class %s, please report", me.xtc.Name, embed))
				} else {
					for _, method := range gtd.RefInterface.allMethods() {
						if dupl := m[method.NameGo]; dupl == nil {
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

type irATypeRefFunc struct {
	Args irANamedTypeRefs `json:",omitempty"`
	Rets irANamedTypeRefs `json:",omitempty"`

	impl *irABlock
}

func (me *irATypeRefFunc) eq(cmp *irATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.eq(cmp.Args) && me.Rets.eq(cmp.Rets))
}

type irATypeRefStruct struct {
	Embeds    []string         `json:",omitempty"`
	Fields    irANamedTypeRefs `json:",omitempty"`
	PassByPtr bool             `json:",omitempty"`
	Methods   irANamedTypeRefs `json:",omitempty"`

	instOf string
}

func (me *irATypeRefStruct) eq(cmp *irATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.eq(cmp.Fields))
}

func (me *irMeta) populateGoTypeDefs() {
	for _, ts := range me.EnvTypeSyns {
		tdict := map[string][]string{}
		gtd := &irANamedTypeRef{Export: me.hasExport(ts.Name)}
		gtd.setBothNamesFromPsName(ts.Name)
		gtd.setRefFrom(me.toIrATypeRef(tdict, ts.Ref))
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}

	for _, tc := range me.EnvTypeClasses {
		tdict := map[string][]string{}
		gif := &irATypeRefInterface{xtc: tc}
		for _, tcc := range tc.Constraints {
			for _, tcca := range tcc.Args {
				ensureIfaceForTvar(tdict, tcca.TypeVar, tcc.Class)
			}
			if legacyIfaceEmbeds && !uslice.StrHas(gif.Embeds, tcc.Class) {
				gif.Embeds = append(gif.Embeds, tcc.Class)
			}
		}
		for _, tcm := range tc.Members {
			ifm := &irANamedTypeRef{NamePs: tcm.Name, NameGo: sanitizeSymbolForGo(tcm.Name, true)}
			ifm.setRefFrom(me.toIrATypeRef(tdict, tcm.Ref))
			if ifm.RefFunc == nil {
				if ifm.RefInterface != nil {
					ifm.RefFunc = &irATypeRefFunc{
						Rets: irANamedTypeRefs{&irANamedTypeRef{}},
					}
					ifm.RefFunc.Rets[0].setRefFrom(ifm.RefInterface)
					ifm.RefInterface = nil
				} else if len(ifm.RefAlias) > 0 {
					ifm.RefFunc = &irATypeRefFunc{
						Rets: irANamedTypeRefs{&irANamedTypeRef{RefAlias: ifm.RefAlias}},
					}
					ifm.RefAlias = ""
				} else if ifm.RefArray != nil || ifm.RefPtr != nil || ifm.RefStruct != nil || ifm.RefUnknown > 0 {
					panic(me.mod.srcFilePath + ": some ifm.RefFoo was set, please report to handle")
				} else {
					ifm.RefFunc = &irATypeRefFunc{
						Rets: irANamedTypeRefs{&irANamedTypeRef{}},
					}
				}
			} else {
				ifm.RefFunc.Args[0].setBothNamesFromPsName("v")
			}
			gif.Methods = append(gif.Methods, ifm)
		}
		tgif := &irANamedTypeRef{Export: me.hasExport(tc.Name)}
		tgif.setBothNamesFromPsName(tc.Name)
		tgif.setRefFrom(gif)
		me.GoTypeDefs = append(me.GoTypeDefs, tgif)
	}

	me.GoTypeDefs = append(me.GoTypeDefs, me.toIrADataTypeDefs(me.EnvTypeDataDecls)...)
}

func (me *irMeta) toIrADataTypeDefs(typedatadecls []*irMTypeDataDecl) (gtds irANamedTypeRefs) {
	for _, td := range typedatadecls {
		tdict := map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, td.Name, td))
		} else {
			isnewtype, hasselfref, hasctorargs := false, false, false
			gid := &irANamedTypeRef{RefInterface: &irATypeRefInterface{xtd: td}, Export: me.hasExport(td.Name)}
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
				gid.setRefFrom(me.toIrATypeRef(tdict, td.Ctors[0].Args[0]))
			} else {
				for _, ctor := range td.Ctors {
					ctor.gtd = &irANamedTypeRef{Export: me.hasExport(gid.NamePs + "ĸ" + ctor.Name),
						RefStruct: &irATypeRefStruct{PassByPtr: hasselfref || (len(ctor.Args) > 1 && hasctorargs)}}
					ctor.gtd.setBothNamesFromPsName(gid.NamePs + "ˇ" + ctor.Name)
					ctor.gtd.NamePs = ctor.Name
					for ia, ctorarg := range ctor.Args {
						field := &irANamedTypeRef{}
						field.setRefFrom(me.toIrATypeRef(tdict, ctorarg))
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

func (me *irMeta) toIrATypeRef(tdict map[string][]string, tr *irMTypeRef) interface{} {
	if len(tr.TypeConstructor) > 0 {
		return tr.TypeConstructor
	} else if tr.REmpty {
		return nil
	} else if len(tr.TypeVar) > 0 {
		embeds := tdict[tr.TypeVar]
		if len(embeds) == 1 {
			return embeds[0]
		}
		return &irATypeRefInterface{Embeds: embeds}
	} else if tr.ConstrainedType != nil {
		if len(tr.ConstrainedType.Args) == 0 || len(tr.ConstrainedType.Args[0].TypeVar) == 0 {
			ensureIfaceForTvar(tdict, "", tr.ConstrainedType.Class) // TODO deal with this properly
		} else {
			ensureIfaceForTvar(tdict, tr.ConstrainedType.Args[0].TypeVar, tr.ConstrainedType.Class)
		}
		return me.toIrATypeRef(tdict, tr.ConstrainedType.Ref)
	} else if tr.ForAll != nil {
		return me.toIrATypeRef(tdict, tr.ForAll.Ref)
	} else if tr.Skolem != nil {
		return fmt.Sprintf("Skolem_%s_scope%d_value%d", tr.Skolem.Name, tr.Skolem.Scope, tr.Skolem.Value)
	} else if tr.RCons != nil {
		rectype := &irATypeRefStruct{PassByPtr: true, Fields: irANamedTypeRefs{&irANamedTypeRef{NamePs: tr.RCons.Label, NameGo: sanitizeSymbolForGo(tr.RCons.Label, false)}}}
		rectype.Fields[0].setRefFrom(me.toIrATypeRef(tdict, tr.RCons.Left))
		if nextrow := me.toIrATypeRef(tdict, tr.RCons.Right); nextrow != nil {
			rectype.Fields = append(rectype.Fields, nextrow.(*irATypeRefStruct).Fields...)
		}
		return rectype
	} else if tr.TypeApp != nil {
		if tr.TypeApp.Left.TypeConstructor == "Prim.Record" {
			return me.toIrATypeRef(tdict, tr.TypeApp.Right)
		} else if tr.TypeApp.Left.TypeConstructor == "Prim.Array" {
			array := &irATypeRefArray{Of: &irANamedTypeRef{}}
			array.Of.setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Right))
			return array
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" {
			funtype := &irATypeRefFunc{}
			funtype.Args = irANamedTypeRefs{&irANamedTypeRef{}}
			funtype.Args[0].setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Left.TypeApp.Right))
			funtype.Rets = irANamedTypeRefs{&irANamedTypeRef{}}
			funtype.Rets[0].setRefFrom(me.toIrATypeRef(tdict, tr.TypeApp.Right))
			return funtype
		} else if len(tr.TypeApp.Left.TypeConstructor) > 0 {
			return me.toIrATypeRef(tdict, tr.TypeApp.Left)
			// } else {
			//	Nested stuff ie. (Either foo) bar
		}
	}
	return nil
}

func ensureIfaceForTvar(tdict map[string][]string, tvar string, ifacetname string) {
	if ifaces4tvar := tdict[tvar]; !uslice.StrHas(ifaces4tvar, ifacetname) {
		ifaces4tvar = append(ifaces4tvar, ifacetname)
		tdict[tvar] = ifaces4tvar
	}
}

func findGoTypeByPsQName(qname string) *irANamedTypeRef {
	var pname, tname string
	i := strings.LastIndex(qname, ".")
	if tname = qname[i+1:]; i > 0 {
		pname = qname[:i]
		if mod := findModuleByQName(pname); mod == nil {
			panic(fmt.Errorf("Unknown module qname %s", pname))
		} else {
			return mod.irMeta.goTypeDefByPsName(tname)
		}
	} else {
		panic("Unexpected non-qualified type-name encountered, please report with your *.purs code-base (and its output-directory *.json files)!")
	}
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
				return "ˇ" + name + "ˇ"
			case "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var":
				return "ˇ" + name
			}
		}
	}
	return strReplSanitizer.Replace(name)
}

func typeNameWithPkgName(pkgname string, typename string) (fullname string) {
	if fullname = typename; len(pkgname) > 0 {
		fullname = pkgname + "." + fullname
	}
	return
}
