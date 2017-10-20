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

const (
	msgfmt = "Encountered (previously unknown or unneeded) %s '%s',\n\tplease report the case with the *.purs code(base) so that we can support it."
)

func coreImpEnvErr(cat string, name string) error {
	return fmt.Errorf(msgfmt, cat, name)
}

type CoreImpEnv struct {
	TypeSyns   map[string]*CoreImpEnvTypeSyn           `json:"typeSynonyms"`
	TypeDefs   map[string]*CoreImpEnvTypeDef           `json:"types"`
	DataCtors  map[string]*CoreImpEnvTypeCtor          `json:"dataConstructors"`
	Classes    map[string]*CoreImpEnvClass             `json:"typeClasses"`
	ClassDicts []map[string]map[string]*CoreImpEnvInst `json:"typeClassDictionaries"`
	Functions  map[string]*CoreImpEnvName              `json:"names"`
}

func (me *CoreImpEnv) prep() {
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

type CoreImpEnvClass struct {
	CoveringSets   [][]int                       `json:"tcCoveringSets"`
	DeterminedArgs []int                         `json:"tcDeterminedArgs"`
	Args           map[string]*CoreImpEnvTagKind `json:"tcArgs"`
	Members        map[string]*CoreImpEnvTagType `json:"tcMembers"`
	Superclasses   []*CoreImpEnvConstr           `json:"tcSuperclasses"`
	Dependencies   []struct {
		Determiners []int `json:"determiners"`
		Determined  []int `json:"determined"`
	} `json:"tcDependencies"`
}

func (me *CoreImpEnvClass) prep() {
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

type CoreImpEnvInst struct {
	Chain         []string             `json:"tcdChain"`
	Index         int                  `json:"tcdIndex"`
	Value         string               `json:"tcdValue"`
	Path          map[string]int       `json:"tcdPath"`
	ClassName     string               `json:"tcdClassName"`
	InstanceTypes []*CoreImpEnvTagType `json:"tcdInstanceTypes"`
	Dependencies  []*CoreImpEnvConstr  `json:"tcdDependencies"`
}

func (me *CoreImpEnvInst) prep() {
	for _, it := range me.InstanceTypes {
		it.prep()
	}
	for _, id := range me.Dependencies {
		id.prep()
	}
}

type CoreImpEnvConstr struct {
	Class string               `json:"constraintClass"`
	Args  []*CoreImpEnvTagType `json:"constraintArgs"`
	Data  interface{}          `json:"constraintData"`
}

func (me *CoreImpEnvConstr) prep() {
	if me.Data != nil {
		panic(coreImpEnvErr("constraintData", fmt.Sprintf("%v", me.Data)))
	}
	for _, ca := range me.Args {
		ca.prep()
	}
}

type CoreImpEnvTypeSyn struct {
	Args map[string]*CoreImpEnvTagKind `json:"tsArgs"`
	Type *CoreImpEnvTagType            `json:"tsType"`
}

func (me *CoreImpEnvTypeSyn) prep() {
	if me.Type != nil {
		me.Type.prep()
	}
	for _, tsa := range me.Args {
		tsa.prep()
	}
}

type CoreImpEnvTypeCtor struct {
	Decl string             `json:"cDecl"`
	Type string             `json:"cType"`
	Ctor *CoreImpEnvTagType `json:"cCtor"`
	Args []string           `json:"cArgs"` // value0, value1 ..etc.
}

func (me *CoreImpEnvTypeCtor) isDeclData() bool    { return me.Decl == "data" }
func (me *CoreImpEnvTypeCtor) isDeclNewtype() bool { return me.Decl == "newtype" }
func (me *CoreImpEnvTypeCtor) prep() {
	if !(me.isDeclData() || me.isDeclNewtype()) {
		panic(coreImpEnvErr("CoreImpEnvTypeCtor.Decl", me.Decl))
	}
	if me.Ctor != nil {
		me.Ctor.prep()
	}
}

type CoreImpEnvTypeDef struct {
	Kind *CoreImpEnvTagKind  `json:"tKind"`
	Decl *CoreImpEnvTypeDecl `json:"tDecl"`
}

func (me *CoreImpEnvTypeDef) prep() {
	if me.Kind != nil {
		me.Kind.prep()
	}
	if me.Decl != nil {
		me.Decl.prep()
	}
}

type CoreImpEnvTypeDecl struct {
	TypeSynonym       bool                `json:""`
	ExternData        bool                `json:""`
	LocalTypeVariable bool                `json:""`
	ScopedTypeVar     bool                `json:""`
	DataType          *CoreImpEnvTypeData `json:""`
}

func (me *CoreImpEnvTypeDecl) prep() {
	if me.DataType != nil {
		me.DataType.prep()
	}
}

type CoreImpEnvTypeData struct {
	Args  map[string]*CoreImpEnvTagKind   `json:"args"`
	Ctors map[string][]*CoreImpEnvTagType `json:"ctors"`
}

func (me *CoreImpEnvTypeData) prep() {
	for _, tda := range me.Args {
		tda.prep()
	}
	for _, tdcs := range me.Ctors {
		for _, tdc := range tdcs {
			tdc.prep()
		}
	}
}

type CoreImpEnvName struct {
	Vis  string             `json:"nVis"`
	Kind string             `json:"nKind"`
	Type *CoreImpEnvTagType `json:"nType"`
}

func (me *CoreImpEnvName) isVisDefined() bool   { return me.Vis == "Defined" }
func (me *CoreImpEnvName) isVisUndefined() bool { return me.Vis == "Undefined" }
func (me *CoreImpEnvName) isKindPrivate() bool  { return me.Kind == "Private" }
func (me *CoreImpEnvName) isKindPublic() bool   { return me.Kind == "Public" }
func (me *CoreImpEnvName) isKindExternal() bool { return me.Kind == "External" }
func (me *CoreImpEnvName) prep() {
	if !(me.isVisDefined() || me.isVisUndefined()) {
		panic(coreImpEnvErr("CoreImpEnvName.Vis", me.Vis))
	}
	if !(me.isKindPublic() || me.isKindPrivate() || me.isKindExternal()) {
		panic(coreImpEnvErr("CoreImpEnvName.Kind", me.Kind))
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
	qname += identtuple[1].(string)
	return
}

func (_ *coreImpEnvTag) tagFrom(tc map[string]interface{}) coreImpEnvTag {
	return coreImpEnvTag{Tag: tc["tag"].(string), Contents: tc["contents"]}
}

type CoreImpEnvTagKind struct {
	coreImpEnvTag

	num   int
	text  string
	kind0 *CoreImpEnvTagKind
	kind1 *CoreImpEnvTagKind
}

func (me *CoreImpEnvTagKind) isRow() bool       { return me.Tag == "Row" }
func (me *CoreImpEnvTagKind) isKUnknown() bool  { return me.Tag == "KUnknown" }
func (me *CoreImpEnvTagKind) isFunKind() bool   { return me.Tag == "FunKind" }
func (me *CoreImpEnvTagKind) isNamedKind() bool { return me.Tag == "NamedKind" }
func (me *CoreImpEnvTagKind) new(tc map[string]interface{}) *CoreImpEnvTagKind {
	return &CoreImpEnvTagKind{coreImpEnvTag: me.tagFrom(tc), num: -1}
}
func (me *CoreImpEnvTagKind) prep() {
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
			panic(coreImpEnvErr("tagged-kind", me.Tag))
		}
	}
}

type CoreImpEnvTagType struct {
	coreImpEnvTag

	num    int
	skolem int
	text   string
	type0  *CoreImpEnvTagType
	type1  *CoreImpEnvTagType
	constr *CoreImpEnvConstr
}

func (me *CoreImpEnvTagType) isTUnknown() bool            { return me.Tag == "TUnknown" }
func (me *CoreImpEnvTagType) isTypeVar() bool             { return me.Tag == "TypeVar" }
func (me *CoreImpEnvTagType) isTypeLevelString() bool     { return me.Tag == "TypeLevelString" }
func (me *CoreImpEnvTagType) isTypeWildcard() bool        { return me.Tag == "TypeWildcard" }
func (me *CoreImpEnvTagType) isTypeConstructor() bool     { return me.Tag == "TypeConstructor" }
func (me *CoreImpEnvTagType) isTypeOp() bool              { return me.Tag == "TypeOp" }
func (me *CoreImpEnvTagType) isTypeApp() bool             { return me.Tag == "TypeApp" }
func (me *CoreImpEnvTagType) isForAll() bool              { return me.Tag == "ForAll" }
func (me *CoreImpEnvTagType) isConstrainedType() bool     { return me.Tag == "ConstrainedType" }
func (me *CoreImpEnvTagType) isSkolem() bool              { return me.Tag == "Skolem" }
func (me *CoreImpEnvTagType) isREmpty() bool              { return me.Tag == "REmpty" }
func (me *CoreImpEnvTagType) isRCons() bool               { return me.Tag == "RCons" }
func (me *CoreImpEnvTagType) isProxyType() bool           { return me.Tag == "ProxyType" }
func (me *CoreImpEnvTagType) isKindedType() bool          { return me.Tag == "KindedType" }
func (me *CoreImpEnvTagType) isPrettyPrintFunction() bool { return me.Tag == "PrettyPrintFunction" }
func (me *CoreImpEnvTagType) isPrettyPrintObject() bool   { return me.Tag == "PrettyPrintObject" }
func (me *CoreImpEnvTagType) isPrettyPrintForAll() bool   { return me.Tag == "PrettyPrintForAll" }
func (me *CoreImpEnvTagType) isBinaryNoParensType() bool  { return me.Tag == "BinaryNoParensType" }
func (me *CoreImpEnvTagType) isParensInType() bool        { return me.Tag == "ParensInType" }
func (me *CoreImpEnvTagType) new(tc map[string]interface{}) *CoreImpEnvTagType {
	return &CoreImpEnvTagType{coreImpEnvTag: me.tagFrom(tc), num: -1, skolem: -1}
}
func (me *CoreImpEnvTagType) prep() {
	//	no type assertions, arr-len checks or nil checks anywhere here: the panic signals us that the coreimp format has changed
	me.skolem, me.num = -1, -1
	if me.isTUnknown() {
		me.num = int(me.Contents.(float64))
	} else if me.isTypeVar() {
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
		me.constr = &CoreImpEnvConstr{Data: constr["constraintData"], Class: me.ident2qname(constr["constraintClass"].([]interface{}))}
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
	} else if me.isREmpty() {
		// nothing to do
	} else {
		panic(coreImpEnvErr("tagged-type", me.Tag))
	}
}
