package main

import (
	"fmt"
)

/*
Representations of PureScript top-level declarations:
type-alias defs, data-type defs, data-type constructors,
type-classes, type-class instances, and the signatures
of top-level functions.
*/

type coreImpEnv struct {
	TypeSyns   map[string]*coreImpEnvTypeSyn           `json:"typeSynonyms"`
	TypeDefs   map[string]*coreImpEnvTypeDef           `json:"types"`
	DataCtors  map[string]*coreImpEnvTypeCtor          `json:"dataConstructors"`
	Classes    map[string]*coreImpEnvClass             `json:"typeClasses"`
	ClassDicts []map[string]map[string]*coreImpEnvInst `json:"typeClassDictionaries"`
	Functions  map[string]*coreImpEnvName              `json:"names"`
}

func (me *coreImpEnv) prep() {
	for _, ts := range me.TypeSyns {
		ts.prep()
	}
	for _, td := range me.TypeDefs {
		td.prep()
	}
	for _, tdc := range me.DataCtors {
		tdc.prep()
	}
	for _, tc := range me.Classes {
		tc.prep()
	}
	for _, tcdmap := range me.ClassDicts {
		for _, tcdsubmap := range tcdmap {
			for _, tcd := range tcdsubmap {
				tcd.prep()
			}
		}
	}
	for _, fn := range me.Functions {
		fn.prep()
	}
}

type coreImpEnvClass struct {
	CoveringSets   [][]int                  `json:"tcCoveringSets"`
	DeterminedArgs []int                    `json:"tcDeterminedArgs"`
	Args           []*coreImpEnvClassArg    `json:"tcArgs"`
	Members        []*coreImpEnvClassMember `json:"tcMembers"`
	Superclasses   []*coreImpEnvConstr      `json:"tcSuperclasses"`
	Dependencies   []struct {
		Determiners []int `json:"determiners"`
		Determined  []int `json:"determined"`
	} `json:"tcDependencies"`
}

func (me *coreImpEnvClass) prep() {
	for _, tca := range me.Args {
		tca.prep()
	}
	for _, tcm := range me.Members {
		tcm.prep()
	}
	for _, tcs := range me.Superclasses {
		tcs.prep()
	}
}

type coreImpEnvClassArg struct {
	Name string             `json:"tcaName"`
	Type *coreImpEnvTagKind `json:"tcaKind"`
}

func (me *coreImpEnvClassArg) prep() {
	me.Type.prep()
}

type coreImpEnvClassMember struct {
	Ident string             `json:"tcmIdent"`
	Type  *coreImpEnvTagType `json:"tcmType"`
}

func (me *coreImpEnvClassMember) prep() {
	me.Type.prep()
}

type coreImpEnvInst struct {
	Chain []string `json:"tcdChain"`
	Index int      `json:"tcdIndex"`
	Value string   `json:"tcdValue"`
	Path  []struct {
		Class string `json:"tcdpClass"`
		Int   int    `json:"tcdpInt"`
	} `json:"tcdPath"`
	ClassName     string               `json:"tcdClassName"`
	InstanceTypes []*coreImpEnvTagType `json:"tcdInstanceTypes"`
	Dependencies  []*coreImpEnvConstr  `json:"tcdDependencies"`
}

func (me *coreImpEnvInst) prep() {
	if len(me.Path) > 0 {
		panic(notImplErr("tcdPath", me.Path[0].Class, "'typeClassDictionaries'"))
	}
	if me.Index != 0 {
		panic(notImplErr("tcdIndex", fmt.Sprint(me.Index), "'typeClassDictionaries'"))
	}
	for _, it := range me.InstanceTypes {
		it.prep()
	}
	for _, id := range me.Dependencies {
		id.prep()
	}
}

type coreImpEnvConstr struct {
	Class string               `json:"constraintClass"`
	Args  []*coreImpEnvTagType `json:"constraintArgs"`
	Data  interface{}          `json:"constraintData"`
}

func (me *coreImpEnvConstr) prep() {
	if me.Data != nil {
		panic(notImplErr("constraintData", fmt.Sprintf("%v", me.Data), "'typeClasses' or 'typeClassDictionaries'"))
	}
	for _, ca := range me.Args {
		ca.prep()
	}
}

type coreImpEnvTypeSyn struct {
	Args []struct {
		Name string             `json:"tsaName"`
		Kind *coreImpEnvTagKind `json:"tsaKind"`
	} `json:"tsArgs"`
	Type *coreImpEnvTagType `json:"tsType"`
}

func (me *coreImpEnvTypeSyn) prep() {
	if me.Type != nil {
		me.Type.prep()
	}
	for _, tsa := range me.Args {
		tsa.Kind.prep()
	}
}

type coreImpEnvTypeCtor struct {
	Decl string             `json:"cDecl"`
	Type string             `json:"cType"`
	Ctor *coreImpEnvTagType `json:"cCtor"`
	Args []string           `json:"cArgs"` // value0, value1 ..etc.
}

func (me *coreImpEnvTypeCtor) isDeclData() bool    { return me.Decl == "data" }
func (me *coreImpEnvTypeCtor) isDeclNewtype() bool { return me.Decl == "newtype" }
func (me *coreImpEnvTypeCtor) prep() {
	if !(me.isDeclData() || me.isDeclNewtype()) {
		panic(notImplErr("cDecl", me.Decl, "'dataConstructors'"))
	}
	if me.Ctor != nil {
		me.Ctor.prep()
	}
}

type coreImpEnvTypeDef struct {
	Kind *coreImpEnvTagKind  `json:"tKind"`
	Decl *coreImpEnvTypeDecl `json:"tDecl"`
}

func (me *coreImpEnvTypeDef) prep() {
	if me.Kind != nil {
		me.Kind.prep()
	}
	if me.Decl != nil {
		me.Decl.prep()
	}
}

type coreImpEnvTypeDecl struct {
	TypeSynonym       bool
	ExternData        bool
	LocalTypeVariable bool
	ScopedTypeVar     bool
	DataType          *coreImpEnvTypeData
}

func (me *coreImpEnvTypeDecl) prep() {
	if me.LocalTypeVariable {
		panic(notImplErr("tDecl", "LocalTypeVariable", "'types'"))
	}
	if me.ScopedTypeVar {
		panic(notImplErr("tDecl", "ScopedTypeVar", "'types'"))
	}
	if me.DataType != nil {
		me.DataType.prep()
	}
}

type coreImpEnvTypeData struct {
	Args []struct {
		Name string             `json:"dtaName"`
		Kind *coreImpEnvTagKind `json:"dtaKind"`
	} `json:"dtArgs"`
	Ctors []struct {
		Name  string               `json:"dtcName"`
		Types []*coreImpEnvTagType `json:"dtcTypes"`
	} `json:"dtCtors"`
}

func (me *coreImpEnvTypeData) prep() {
	for _, tda := range me.Args {
		tda.Kind.prep()
	}
	for _, tdc := range me.Ctors {
		for _, tdct := range tdc.Types {
			tdct.prep()
		}
	}
}

type coreImpEnvName struct {
	Vis  string             `json:"nVis"`
	Kind string             `json:"nKind"`
	Type *coreImpEnvTagType `json:"nType"`
}

func (me *coreImpEnvName) isVisDefined() bool   { return me.Vis == "Defined" }
func (me *coreImpEnvName) isVisUndefined() bool { return me.Vis == "Undefined" }
func (me *coreImpEnvName) isKindPrivate() bool  { return me.Kind == "Private" }
func (me *coreImpEnvName) isKindPublic() bool   { return me.Kind == "Public" }
func (me *coreImpEnvName) isKindExternal() bool { return me.Kind == "External" }
func (me *coreImpEnvName) prep() {
	if !(me.isVisDefined() || me.isVisUndefined()) {
		panic(notImplErr("nVis", me.Vis, "'names'"))
	}
	if !(me.isKindPublic() || me.isKindPrivate() || me.isKindExternal()) {
		panic(notImplErr("nKind", me.Kind, "'names'"))
	}
	if me.Type != nil {
		me.Type.prep()
	}
}

type coreImpEnvTag struct {
	Tag      string      `json:"tag"`
	Contents interface{} `json:"contents"` // either string or []anything
}

func (_ *coreImpEnvTag) ident2qname(identtuple []interface{}) (qname string) {
	for _, m := range identtuple[0].([]interface{}) {
		qname += (m.(string) + ".")
	}
	switch x := identtuple[1].(type) {
	case map[string]string:
		qname += x["Ident"]
	case map[string]interface{}:
		qname += x["Ident"].(string)
	default:
		qname += x.(string)
	}
	return
}

func (_ *coreImpEnvTag) tagFrom(tc map[string]interface{}) coreImpEnvTag {
	return coreImpEnvTag{Tag: tc["tag"].(string), Contents: tc["contents"]}
}

type coreImpEnvTagKind struct {
	coreImpEnvTag

	num   int
	text  string
	kind0 *coreImpEnvTagKind
	kind1 *coreImpEnvTagKind
}

func (me *coreImpEnvTagKind) isRow() bool       { return me.Tag == "Row" }
func (me *coreImpEnvTagKind) isKUnknown() bool  { return me.Tag == "KUnknown" }
func (me *coreImpEnvTagKind) isFunKind() bool   { return me.Tag == "FunKind" }
func (me *coreImpEnvTagKind) isNamedKind() bool { return me.Tag == "NamedKind" }
func (me *coreImpEnvTagKind) new(tc map[string]interface{}) *coreImpEnvTagKind {
	return &coreImpEnvTagKind{coreImpEnvTag: me.tagFrom(tc), num: -1}
}
func (me *coreImpEnvTagKind) prep() {
	if me != nil {
		//	no type assertions, arr-len checks or nil checks anywhere here: the panic signals us that the coreimp format has changed
		me.num = -1
		if me.isKUnknown() {
			me.num = int(me.Contents.(float64))
		} else if me.isRow() {
			me.kind0 = me.new(me.Contents.(map[string]interface{}))
			me.kind0.prep()
		} else if me.isFunKind() {
			items := me.Contents.([]interface{})
			me.kind0 = me.new(items[0].(map[string]interface{}))
			me.kind0.prep()
			me.kind1 = me.new(items[1].(map[string]interface{}))
			me.kind1.prep()
		} else if me.isNamedKind() {
			me.text = me.ident2qname(me.Contents.([]interface{}))
		} else {
			panic(notImplErr("tagged-kind", me.Tag, me.Contents))
		}
	}
}

type coreImpEnvTagType struct {
	coreImpEnvTag

	num    int
	skolem int
	text   string
	type0  *coreImpEnvTagType
	type1  *coreImpEnvTagType
	constr *coreImpEnvConstr
}

// func (me *coreImpEnvTagType) isTypeWildcard() bool        { return me.Tag == "TypeWildcard" }
// func (me *coreImpEnvTagType) isTypeOp() bool              { return me.Tag == "TypeOp" }
// func (me *coreImpEnvTagType) isProxyType() bool           { return me.Tag == "ProxyType" }
// func (me *coreImpEnvTagType) isKindedType() bool          { return me.Tag == "KindedType" }
// func (me *coreImpEnvTagType) isPrettyPrintFunction() bool { return me.Tag == "PrettyPrintFunction" }
// func (me *coreImpEnvTagType) isPrettyPrintObject() bool   { return me.Tag == "PrettyPrintObject" }
// func (me *coreImpEnvTagType) isPrettyPrintForAll() bool   { return me.Tag == "PrettyPrintForAll" }
// func (me *coreImpEnvTagType) isBinaryNoParensType() bool  { return me.Tag == "BinaryNoParensType" }
// func (me *coreImpEnvTagType) isParensInType() bool        { return me.Tag == "ParensInType" }
// func (me *coreImpEnvTagType) isTUnknown() bool        { return me.Tag == "TUnknown" }
func (me *coreImpEnvTagType) isTypeLevelString() bool { return me.Tag == "TypeLevelString" }
func (me *coreImpEnvTagType) isTypeVar() bool         { return me.Tag == "TypeVar" }
func (me *coreImpEnvTagType) isTypeConstructor() bool { return me.Tag == "TypeConstructor" }
func (me *coreImpEnvTagType) isSkolem() bool          { return me.Tag == "Skolem" }
func (me *coreImpEnvTagType) isREmpty() bool          { return me.Tag == "REmpty" }
func (me *coreImpEnvTagType) isRCons() bool           { return me.Tag == "RCons" }
func (me *coreImpEnvTagType) isTypeApp() bool         { return me.Tag == "TypeApp" }
func (me *coreImpEnvTagType) isForAll() bool          { return me.Tag == "ForAll" }
func (me *coreImpEnvTagType) isConstrainedType() bool { return me.Tag == "ConstrainedType" }
func (me *coreImpEnvTagType) new(tc map[string]interface{}) *coreImpEnvTagType {
	return &coreImpEnvTagType{coreImpEnvTag: me.tagFrom(tc), num: -1, skolem: -1}
}
func (me *coreImpEnvTagType) prep() {
	//	no type assertions, arr-len checks or nil checks anywhere here: the panic signals us that the coreimp format has changed
	me.skolem, me.num = -1, -1
	if me.isTypeVar() {
		me.text = me.Contents.(string)
	} else if me.isForAll() {
		tuple := me.Contents.([]interface{})
		me.text = tuple[0].(string)
		me.type0 = me.new(tuple[1].(map[string]interface{}))
		me.type0.prep()
		if tuple[2] != nil {
			me.skolem = int(tuple[2].(float64))
		}
	} else if me.isTypeApp() {
		items := me.Contents.([]interface{})
		me.type0 = me.new(items[0].(map[string]interface{}))
		me.type0.prep()
		me.type1 = me.new(items[1].(map[string]interface{}))
		me.type1.prep()
	} else if me.isTypeConstructor() {
		me.text = me.ident2qname(me.Contents.([]interface{}))
	} else if me.isConstrainedType() {
		tuple := me.Contents.([]interface{}) // eg [{constrstuff} , {type}]
		me.type0 = me.new(tuple[1].(map[string]interface{}))
		me.type0.prep()
		constr := tuple[0].(map[string]interface{})
		me.constr = &coreImpEnvConstr{Data: constr["constraintData"], Class: me.ident2qname(constr["constraintClass"].([]interface{}))}
		for _, ca := range constr["constraintArgs"].([]interface{}) {
			carg := me.new(ca.(map[string]interface{}))
			me.constr.Args = append(me.constr.Args, carg)
		}
		me.constr.prep()
	} else if me.isSkolem() {
		tuple := me.Contents.([]interface{})
		me.text = tuple[0].(string)
		me.num = int(tuple[1].(float64))
		me.skolem = int(tuple[2].(float64))
	} else if me.isRCons() {
		tuple := me.Contents.([]interface{})
		me.text = tuple[0].(string)
		me.type0 = me.new(tuple[1].(map[string]interface{}))
		me.type0.prep()
		me.type1 = me.new(tuple[2].(map[string]interface{}))
		me.type1.prep()
	} else if me.isREmpty() || me.isTypeLevelString() {
		// nothing to do
	} else {
		panic(notImplErr("tagged-type", me.Tag, me.Contents))
	}
}
