package main

import (
	"fmt"
	"strings"
)

type GIrMTypeAlias struct {
	Name string       `json:"tan"`
	Ref  *GIrMTypeRef `json:"tar,omitempty"`
}

type GIrMTypeDataDecl struct {
	Name  string             `json:"tdn"`
	Ctors []GIrMTypeDataCtor `json:"tdc,omitempty"`
	Args  []string           `json:"tda,omitempty"`
}

type GIrMTypeDataCtor struct {
	Name string       `json:"tcn"`
	Args GIrMTypeRefs `json:"tca,omitempty"`
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

func (me *GonadIrMeta) newTypeRefFromExtTc(tc TaggedContents) (tref *GIrMTypeRef) {
	tref = &GIrMTypeRef{}
	switch tc.Tag {
	case "TypeConstructor":
		for _, s := range tc.Contents.([]interface{})[0].([]interface{}) {
			tref.TypeConstructor += (s.(string) + ".")
		}
		tref.TypeConstructor += tc.Contents.([]interface{})[1].(string)
	case "TypeVar":
		tref.TypeVar = tc.Contents.(string)
	case "TUnknown":
		tref.TUnknown = tc.Contents.(int)
	case "REmpty":
		tref.REmpty = true
	case "RCons":
		disc := tc.Contents.([]interface{})
		tcdis, tcsub := disc[1].(map[string]interface{}), disc[2].(map[string]interface{})
		tref.RCons = &GIrMTypeRefRow{
			Label: disc[0].(string), Left: me.newTypeRefFromExtTc(newTaggedContents(tcdis)), Right: me.newTypeRefFromExtTc(newTaggedContents(tcsub))}
	case "ForAll":
		disc := tc.Contents.([]interface{})
		tcdis := disc[1].(map[string]interface{})
		tref.ForAll = &GIrMTypeRefExist{Name: disc[0].(string), Ref: me.newTypeRefFromExtTc(newTaggedContents(tcdis))}
		if len(disc) > 2 && disc[2] != nil {
			if i, ok := disc[2].(int); ok {
				tref.ForAll.SkolemScope = &i
			}
		}
	case "Skolem":
		disc := tc.Contents.([]interface{})
		tref.Skolem = &GIrMTypeRefSkolem{
			Name: disc[0].(string), Value: disc[1].(int), Scope: disc[2].(int)}
	case "TypeApp":
		disc := tc.Contents.([]interface{})
		tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
		tref.TypeApp = &GIrMTypeRefAppl{
			Left: me.newTypeRefFromExtTc(newTaggedContents(tcdis)), Right: me.newTypeRefFromExtTc(newTaggedContents(tcsub))}
	case "ConstrainedType":
		disc := tc.Contents.([]interface{})
		tcdis, tcsub := disc[0].(map[string]interface{}), disc[1].(map[string]interface{})
		tref.ConstrainedType = &GIrMTypeRefConstr{
			Data: tcdis["constraintData"], Ref: me.newTypeRefFromExtTc(newTaggedContents(tcsub))}
		for _, tca := range tcdis["constraintArgs"].([]interface{}) {
			tref.ConstrainedType.Args = append(tref.ConstrainedType.Args, me.newTypeRefFromExtTc(newTaggedContents(tca.(map[string]interface{}))))
		}
		for _, s := range tcdis["constraintClass"].([]interface{})[0].([]interface{}) {
			tref.ConstrainedType.Class += (s.(string) + ".")
		}
		tref.ConstrainedType.Class += tcdis["constraintClass"].([]interface{})[1].(string)
	default:
		fmt.Printf("\n%s?!\n\t%v\n", tc.Tag, tc.Contents)
	}
	return
}

func (me *GonadIrMeta) populateTypeDataDecls() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDType != nil && d.EDType.DeclKind != nil {
			if m_edTypeDeclarationKind, ok := d.EDType.DeclKind.(map[string]interface{}); ok && m_edTypeDeclarationKind != nil {
				if m_DataType, ok := m_edTypeDeclarationKind["DataType"].(map[string]interface{}); ok && m_DataType != nil {
					datadecl := GIrMTypeDataDecl{Name: d.EDType.Name}
					for _, argif := range m_DataType["args"].([]interface{}) {
						datadecl.Args = append(datadecl.Args, argif.([]interface{})[0].(string))
					}
					for _, ctorif := range m_DataType["ctors"].([]interface{}) {
						if ctorarr, ok := ctorif.([]interface{}); (!ok) || len(ctorarr) != 2 {
							panic(fmt.Errorf("%s: unexpected ctor array in %s, please report: %v", me.mod.srcFilePath, datadecl.Name, ctorif))
						} else {
							ctor := GIrMTypeDataCtor{Name: ctorarr[0].(string)}
							for _, ctorarg := range ctorarr[1].([]interface{}) {
								ctor.Args = append(ctor.Args, me.newTypeRefFromExtTc(newTaggedContents(ctorarg.(map[string]interface{}))))
							}
							datadecl.Ctors = append(datadecl.Ctors, ctor)
						}
					}
					me.TypeDataDecls = append(me.TypeDataDecls, datadecl)
				}
			}
		}
	}
}

func (me *GonadIrMeta) populateTypeAliases() {
	for _, d := range me.mod.ext.EfDecls {
		if d.EDTypeSynonym != nil && d.EDTypeSynonym.Type != nil && len(d.EDTypeSynonym.Name) > 0 && me.mod.ext.findTypeClass(d.EDTypeSynonym.Name) == nil {
			ta := GIrMTypeAlias{Name: d.EDTypeSynonym.Name}
			ta.Ref = me.newTypeRefFromExtTc(*d.EDTypeSynonym.Type)
			me.TypeAliases = append(me.TypeAliases, ta)
		}
	}
}

func (me *GonadIrMeta) populateGoTypeDefs() {
	mdict := map[string][]string{}

	for _, ta := range me.TypeAliases {
		tdict := map[string][]string{}
		gtd := &GIrANamedTypeRef{Name: ta.Name}
		gtd.setFrom(me.toGIrATypeRef(mdict, tdict, ta.Ref))
		me.GoTypeDefs = append(me.GoTypeDefs, gtd)
	}

	for _, td := range me.TypeDataDecls {
		tdict := map[string][]string{}
		if numctors := len(td.Ctors); numctors == 0 {
			panic(fmt.Errorf("%s: unexpected ctor absence in %s, please report: %v", me.mod.srcFilePath, td.Name, td))
		} else {
			gtd, noctorargs, isnewtype := &GIrANamedTypeRef{Name: td.Name, RefAlias: td.Name + "Kinds"}, true, false
			for _, ctor := range td.Ctors {
				gtd.EnumConstNames = append(gtd.EnumConstNames, fmt.Sprintf("%s_%s", td.Name, ctor.Name))
				if numargs := len(ctor.Args); numargs > 0 {
					if noctorargs = false; numargs == 1 && numctors == 1 {
						isnewtype = true
					}
				}
			}

			if !noctorargs {
				gtd.RefAlias = ""
				if isnewtype {
					gtd.setFrom(me.toGIrATypeRef(mdict, tdict, td.Ctors[0].Args[0]))
					gtd.EnumConstNames = nil
				} else {
					gtd.RefStruct = &GIrATypeRefStruct{}
					gtd.RefStruct.Fields = append(gtd.RefStruct.Fields, &GIrANamedTypeRef{Name: "kind", RefAlias: td.Name + "Kinds"})
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
			}
			if !noctorargs {
				method_kind := &GIrANamedTypeRef{Name: "Kind", RefFunc: &GIrATypeRefFunc{
					Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: td.Name + "Kinds"}},
				}}
				method_kind.methodBody = append(method_kind.methodBody, &CoreImpAst{
					Ast_tag: "Return", Return: &CoreImpAst{
						Ast_tag:           "Indexer",
						Indexer:           &CoreImpAst{Ast_tag: "Var", Var: "me"},
						Ast_rightHandSide: &CoreImpAst{Ast_tag: "StringLiteral", StringLiteral: "kind"},
					}})
				gtd.Methods = append(gtd.Methods, method_kind)

				if !isnewtype {
					for _, ctor := range td.Ctors {
						method_iskind := &GIrANamedTypeRef{Name: "Is" + ctor.Name, RefFunc: &GIrATypeRefFunc{
							Rets: GIrANamedTypeRefs{&GIrANamedTypeRef{RefAlias: "Prim.Boolean"}},
						}}
						method_iskind.methodBody = append(method_iskind.methodBody, &CoreImpAst{
							Ast_tag: "Return", Return: &CoreImpAst{
								Ast_tag: "Binary",
								Binary: &CoreImpAst{
									Ast_tag:           "Indexer",
									Indexer:           &CoreImpAst{Ast_tag: "Var", Var: "me"},
									Ast_rightHandSide: &CoreImpAst{Ast_tag: "StringLiteral", StringLiteral: "kind"},
								},
								Ast_op:            "EqualTo",
								Ast_rightHandSide: &CoreImpAst{Ast_tag: "Var", Var: gtd.Name + "_" + ctor.Name},
							}})
						gtd.Methods = append(gtd.Methods, method_iskind)
						if len(ctor.Args) > 0 {
							method_ctor := &GIrANamedTypeRef{Name: ctor.Name, RefFunc: &GIrATypeRefFunc{}}
							for i, ctorarg := range ctor.Args {
								if ctorarg.tmp_assoc != nil {
									retarg := &GIrANamedTypeRef{Name: fmt.Sprintf("v%v", i)}
									retarg.setFrom(me.toGIrATypeRef(mdict, tdict, ctorarg))
									method_ctor.RefFunc.Rets = append(method_ctor.RefFunc.Rets, retarg)
									if ctorarg.tmp_assoc == nil {
										println(me.mod.srcFilePath + ": " + td.Name + " : " + ctor.Name + " > " + retarg.Name)
									}
									method_ctor.methodBody = append(method_ctor.methodBody, &CoreImpAst{
										Ast_tag:    "Assignment",
										Assignment: &CoreImpAst{Ast_tag: "Var", Var: retarg.Name},
										Ast_rightHandSide: &CoreImpAst{
											Ast_tag:           "Indexer",
											Indexer:           &CoreImpAst{Ast_tag: "Var", Var: "me"},
											Ast_rightHandSide: &CoreImpAst{Ast_tag: "StringLiteral", StringLiteral: fmt.Sprintf("%v", ctorarg.tmp_assoc.Name)},
										}})
								}
							}
							method_ctor.methodBody = append(method_ctor.methodBody, &CoreImpAst{Ast_tag: "ReturnNoResult"})
							gtd.Methods = append(gtd.Methods, method_ctor)
						}
					}
				}
			}
			me.GoTypeDefs = append(me.GoTypeDefs, gtd)
		}
	}
}

func (me *GonadIrMeta) toGIrATypeRef(mdict map[string][]string, tdict map[string][]string, tr *GIrMTypeRef) interface{} {
	if len(tr.TypeConstructor) > 0 {
		return tr.TypeConstructor
	} else if tr.REmpty {
		return nil
	} else if tr.TUnknown > 0 {
		return tr.TUnknown
	} else if len(tr.TypeVar) > 0 {
		return &GIrATypeRefInterface{Embeds: tdict[tr.TypeVar]}
	} else if tr.ConstrainedType != nil {
		if len(tr.ConstrainedType.Args) == 0 || len(tr.ConstrainedType.Args[0].TypeVar) == 0 {
			panic(fmt.Errorf("%s: unexpected type-class/type-var association %v, please report!", me.mod.srcFilePath, tr.ConstrainedType))
		}
		tdict[tr.ConstrainedType.Args[0].TypeVar] = append(tdict[tr.ConstrainedType.Args[0].TypeVar], tr.ConstrainedType.Class)
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
		} else if tr.TypeApp.Left.TypeApp != nil && tr.TypeApp.Left.TypeApp.Left.TypeConstructor == "Prim.Function" {
			funtype := &GIrATypeRefFunc{}
			funtype.Args = []*GIrANamedTypeRef{&GIrANamedTypeRef{}}
			funtype.Args[0].setFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Left.TypeApp.Right))
			funtype.Rets = []*GIrANamedTypeRef{&GIrANamedTypeRef{}}
			funtype.Rets[0].setFrom(me.toGIrATypeRef(mdict, tdict, tr.TypeApp.Right))
			return funtype
		}
	}
	return nil
}
