package main

type CoreImpEnv struct {
	TypeSyns   map[string]*CoreImpEnvTypeSyn           `json:"typeSynonyms,omitempty"`
	TypeDefs   map[string]*CoreImpEnvTypeDef           `json:"types,omitempty"`
	DataCtors  map[string]*CoreImpEnvTypeCtor          `json:"dataConstructors,omitempty"`
	Classes    map[string]*CoreImpEnvClass             `json:"typeClasses,omitempty"`
	ClassDicts []map[string]map[string]*CoreImpEnvInst `json:"typeClassDictionaries,omitempty"`
	Names      map[string]*CoreImpEnvName              `json:"names,omitempty"`
}

type CoreImpEnvTag struct {
	Tag      string      `json:"tag,omitempty"`
	Contents interface{} `json:"contents,omitempty"` // either string or []anything

	typeVar string
}

func (me *CoreImpEnvTag) Create() {
	switch me.Tag {
	case "TypeVar":
		me.typeVar = me.Contents.(string)

	}
}

type CoreImpEnvClass struct {
	CoveringSets   [][]int                      `json:"tcCoveringSets,omitempty"`
	DeterminedArgs []int                        `json:"tcDeterminedArgs,omitempty"`
	Args           map[string]*CoreImpEnvTag    `json:"tcArgs,omitempty"`
	Members        map[string]*CoreImpEnvTag    `json:"tcMembers,omitempty"`
	Superclasses   map[string]*CoreImpEnvConstr `json:"tcSuperclasses,omitempty"`
	Dependencies   []struct {
		Determiners []int `json:"determiners,omitempty"`
		Determined  []int `json:"determined,omitempty"`
	} `json:"tcDependencies,omitempty"`
}

type CoreImpEnvInst struct {
	Chain         []string                     `json:"tcdChain,omitempty"`
	Index         int                          `json:"tcdIndex,omitempty"`
	Value         string                       `json:"tcdValue,omitempty"`
	Path          map[string]int               `json:"tcdPath,omitempty"`
	ClassName     string                       `json:"tcdClassName,omitempty"`
	InstanceTypes []*CoreImpEnvTag             `json:"tcdInstanceTypes,omitempty"`
	Dependencies  map[string]*CoreImpEnvConstr `json:"tcdDependencies,omitempty"`
}

type CoreImpEnvConstr struct {
	Args []*CoreImpEnvTag `json:"constraintArgs,omitempty"`
	Data struct {
		PartialConstraintData *struct {
			Binders   [][]string `json:",omitempty"`
			Truncated bool       `json:",omitempty"`
		} `json:",omitempty"`
	} `json:"constraintData,omitempty"`
}

type CoreImpEnvTypeSyn struct {
	Args map[string]*CoreImpEnvTag `json:"tsArgs,omitempty"`
	Type *CoreImpEnvTag            `json:"tsType,omitempty"`
}

type CoreImpEnvTypeCtor struct {
	Decl string         `json:"cDecl,omitempty"` // data or newtype
	Type string         `json:"cType,omitempty"`
	Ctor *CoreImpEnvTag `json:"cCtor,omitempty"`
	Args []string       `json:"cArgs,omitempty"` // value0, value1 ..etc.
}

type CoreImpEnvTypeDef struct {
	Kind *CoreImpEnvTag      `json:"tKind,omitempty"`
	Decl *CoreImpEnvTypeDecl `json:"tDecl,omitempty"`
}

type CoreImpEnvTypeDecl struct {
	TypeSynonym       bool                `json:",omitempty"`
	ExternData        bool                `json:",omitempty"`
	LocalTypeVariable bool                `json:",omitempty"`
	ScopedTypeVar     bool                `json:",omitempty"`
	DataType          *CoreImpEnvTypeData `json:",omitempty"`
}

type CoreImpEnvTypeData struct {
	Args  map[string]*CoreImpEnvTag   `json:"args,omitempty"`
	Ctors map[string][]*CoreImpEnvTag `json:"ctors,omitempty"`
}

type CoreImpEnvName struct {
	Vis  string         `json:"nVis,omitempty"`  // Environment.hs:Defined or Undefined
	Kind string         `json:"nKind,omitempty"` // Environment.hs: Private or Public or External
	Type *CoreImpEnvTag `json:"nType,omitempty"`
}
