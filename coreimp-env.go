package main

type CoreImpEnv struct {
	TypeSyns   map[string]*CoreImpEnvTypeSyn           `json:"typeSynonyms,omitempty"`
	TypeDefs   map[string]*CoreImpEnvTypeDef           `json:"types,omitempty"`
	DataCtors  map[string]*CoreImpEnvTypeCtor          `json:"dataConstructors,omitempty"`
	Classes    map[string]*CoreImpEnvClass             `json:"typeClasses,omitempty"`
	ClassDicts []map[string]map[string]*CoreImpEnvInst `json:"typeClassDictionaries,omitempty"`
	Names      map[string]*CoreImpEnvName              `json:"names,omitempty"`
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
	for _, n := range me.Names {
		n.prep()
	}
}

type CoreImpEnvClass struct {
	CoveringSets   [][]int                       `json:"tcCoveringSets,omitempty"`
	DeterminedArgs []int                         `json:"tcDeterminedArgs,omitempty"`
	Args           map[string]*CoreImpEnvTagKind `json:"tcArgs,omitempty"`
	Members        map[string]*CoreImpEnvTagType `json:"tcMembers,omitempty"`
	Superclasses   map[string]*CoreImpEnvConstr  `json:"tcSuperclasses,omitempty"`
	Dependencies   []struct {
		Determiners []int `json:"determiners,omitempty"`
		Determined  []int `json:"determined,omitempty"`
	} `json:"tcDependencies,omitempty"`
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
	Chain         []string                     `json:"tcdChain,omitempty"`
	Index         int                          `json:"tcdIndex,omitempty"`
	Value         string                       `json:"tcdValue,omitempty"`
	Path          map[string]int               `json:"tcdPath,omitempty"`
	ClassName     string                       `json:"tcdClassName,omitempty"`
	InstanceTypes []*CoreImpEnvTagType         `json:"tcdInstanceTypes,omitempty"`
	Dependencies  map[string]*CoreImpEnvConstr `json:"tcdDependencies,omitempty"`
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
	Args []*CoreImpEnvTagType `json:"constraintArgs,omitempty"`
	Data struct {
		PartialConstraintData *struct {
			Binders   [][]string `json:",omitempty"`
			Truncated bool       `json:",omitempty"`
		} `json:",omitempty"`
	} `json:"constraintData,omitempty"`
}

func (me *CoreImpEnvConstr) prep() {
	for _, ca := range me.Args {
		ca.prep()
	}
}

type CoreImpEnvTypeSyn struct {
	Args map[string]*CoreImpEnvTagKind `json:"tsArgs,omitempty"`
	Type *CoreImpEnvTagType            `json:"tsType,omitempty"`
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
	Decl string             `json:"cDecl,omitempty"` // data or newtype
	Type string             `json:"cType,omitempty"`
	Ctor *CoreImpEnvTagType `json:"cCtor,omitempty"`
	Args []string           `json:"cArgs,omitempty"` // value0, value1 ..etc.
}

func (me *CoreImpEnvTypeCtor) prep() {
	if me.Ctor != nil {
		me.Ctor.prep()
	}
}

type CoreImpEnvTypeDef struct {
	Kind *CoreImpEnvTagKind  `json:"tKind,omitempty"`
	Decl *CoreImpEnvTypeDecl `json:"tDecl,omitempty"`
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
	TypeSynonym       bool                `json:",omitempty"`
	ExternData        bool                `json:",omitempty"`
	LocalTypeVariable bool                `json:",omitempty"`
	ScopedTypeVar     bool                `json:",omitempty"`
	DataType          *CoreImpEnvTypeData `json:",omitempty"`
}

func (me *CoreImpEnvTypeDecl) prep() {
	if me.DataType != nil {
		me.DataType.prep()
	}
}

type CoreImpEnvTypeData struct {
	Args  map[string]*CoreImpEnvTagKind   `json:"args,omitempty"`
	Ctors map[string][]*CoreImpEnvTagType `json:"ctors,omitempty"`
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
	Vis  string             `json:"nVis,omitempty"`  // Environment.hs:Defined or Undefined
	Kind string             `json:"nKind,omitempty"` // Environment.hs: Private or Public or External
	Type *CoreImpEnvTagType `json:"nType,omitempty"`
}

func (me *CoreImpEnvName) prep() {
	if me.Type != nil {
		me.Type.prep()
	}
}

type coreImpEnvTag struct {
	Tag      string      `json:"tag,omitempty"`
	Contents interface{} `json:"contents,omitempty"` // either string or []anything
}

type CoreImpEnvTagKind struct {
	coreImpEnvTag

	fun struct {
		ok    bool
		kind0 *CoreImpEnvTagKind
		kind1 *CoreImpEnvTagKind
	}
	row struct {
		ok   bool
		kind *CoreImpEnvTagKind
	}
	unknown struct {
		ok                  bool
		unificationVariable int
	}
	named struct {
		ok    bool
		qName string
	}
}

func (me *CoreImpEnvTagKind) prep() {
	if me != nil {
		switch me.Tag {
		case "FunKind":
			me.fun.ok = true
			items := me.Contents.([]interface{})
			kind0, kind1 := items[0].(map[string]interface{}), items[1].(map[string]interface{})
			me.fun.kind0 = &CoreImpEnvTagKind{coreImpEnvTag: coreImpEnvTag{Tag: kind0["tag"].(string), Contents: kind0["contents"]}}
			me.fun.kind0.prep()
			me.fun.kind1 = &CoreImpEnvTagKind{coreImpEnvTag: coreImpEnvTag{Tag: kind1["tag"].(string), Contents: kind1["contents"]}}
			me.fun.kind1.prep()
		case "Row":
			me.row.ok = true
			item := me.Contents.(map[string]interface{})
			me.row.kind = &CoreImpEnvTagKind{coreImpEnvTag: coreImpEnvTag{Tag: item["tag"].(string), Contents: item["contents"]}}
			me.row.kind.prep()
		case "KUnknown":
			me.unknown.ok = true
			me.unknown.unificationVariable = me.Contents.(int)
		case "NamedKind":
			me.named.ok = true
			tuple := me.Contents.([]interface{}) // eg. [["Control","Monad","Eff"],"Effect"]
			for _, m := range tuple[0].([]interface{}) {
				me.named.qName += (m.(string) + ".")
			}
			me.named.qName += tuple[1].(string)
		default:
			panic("Unhandled kind, report immediately:\t" + me.Tag)
		}
	}
}

type CoreImpEnvTagType struct {
	coreImpEnvTag

	unknown struct {
		ok                  bool
		unificationVariable int
	}
	typeVar struct {
		ok   bool
		name string
	}
	typeLevelString struct {
		ok       bool
		wildcard string
	}
}

func (me *CoreImpEnvTagType) prep() {
	if me != nil {
		switch me.Tag {
		case "TUnknown":
			me.unknown.ok = true
			me.unknown.unificationVariable = me.Contents.(int)
		case "TypeVar":
			me.typeVar.ok = true
			me.typeVar.name = me.Contents.(string)
		case "TypeLevelString":
			me.typeLevelString.ok = true
			me.typeLevelString.wildcard = me.Contents.(string)
		default:
			println("TYPE:\t" + me.Tag)
		}
	}
}
