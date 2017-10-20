package main

type TaggedContents struct {
	Tag      string      `json:"tag,omitempty"`
	Contents interface{} `json:"contents,omitempty"`
}

func newTaggedContents(m map[string]interface{}) (tc TaggedContents) {
	tc.Tag = m["tag"].(string)
	tc.Contents = m["contents"]
	return
}

type PsExt struct {
	modinfo *ModuleInfo

	EfSourceSpan *CoreImpSourceSpan `json:"efSourceSpan,omitempty"`
	EfVersion    string             `json:"efVersion,omitempty"`
	EfModuleName []string           `json:"efModuleName,omitempty"`
	EfExports    []*PsExtRefs       `json:"efExports,omitempty"`
	EfImports    []*PsExtImport     `json:"efImports,omitempty"`
	EfDecls      []*PsExtDecl       `json:"efDeclarations,omitempty"`
}

type PsExtImport struct {
	EiModule     []string         `json:"eiModule,omitempty"`
	EiImportType *PsExtImportType `json:"eiImportType,omitempty"`
	EiImportedAs []string         `json:"eiImportedAs,omitempty"`
}

type PsExtRefs struct {
	TypeRef         []interface{}
	TypeClassRef    []interface{}
	TypeInstanceRef []interface{}
	ValueRef        []interface{}
	ValueOpRef      []interface{}
	ModuleRef       []interface{}
	ReExportRef     []interface{}
}

type PsExtImportType struct {
	Implicit []interface{}
	Explicit []*PsExtRefs
}

type PsExtDecl struct {
	EDClass           *PsExtTypeClass        `json:",omitempty"`
	EDType            *PsExtType             `json:",omitempty"`
	EDTypeSynonym     *PsExtTypeAlias        `json:",omitempty"`
	EDValue           *PsExtVal              `json:",omitempty"`
	EDInstance        *PsExtInst             `json:",omitempty"`
	EDDataConstructor map[string]interface{} `json:",omitempty"`
}

type PsExtIdent struct {
	Ident string `json:",omitempty"`
}

type PsExtVal struct {
	Name PsExtIdent     `json:"edValueName"`
	Type TaggedContents `json:"edValueType"`
}

type PsExtType struct {
	Name     string         `json:"edTypeName,omitempty"`
	Kind     TaggedContents `json:"edTypeKind,omitempty"`
	DeclKind interface{}    `json:"edTypeDeclarationKind,omitempty"`
}

type PsExtTypeAlias struct {
	Name      string          `json:"edTypeSynonymName,omitempty"`
	Arguments []interface{}   `json:"edTypeSynonymArguments,omitempty"`
	Type      *TaggedContents `json:"edTypeSynonymType,omitempty"`
}

type PsExtConstr struct {
	Class []interface{}    `json:"constraintClass,omitempty"`
	Args  []TaggedContents `json:"constraintArgs,omitempty"`
	Data  []interface{}    `json:"constraintData,omitempty"`
}

type PsExtInst struct {
	ClassName   []interface{}    `json:"edInstanceClassName,omitempty"`
	Name        PsExtIdent       `json:"edInstanceName,omitempty"`
	Types       []TaggedContents `json:"edInstanceTypes,omitempty"`
	Constraints []PsExtConstr    `json:"edInstanceConstraints,omitempty"`
	Chain       [][]interface{}  `json:"edInstanceChain,omitempty"`
	ChainIndex  int              `json:"edInstanceChainIndex,omitempty"`
}

type PsExtTypeClass struct {
	Name           string          `json:"edClassName,omitempty"`
	TypeArgs       [][]interface{} `json:"edClassTypeArguments,omitempty"`
	FunctionalDeps []struct {
		Determiners []int `json:"determiners,omitempty"`
		Determined  []int `json:"determined,omitempty"`
	} `json:"edFunctionalDependencies,omitempty"`
	Members     [][]interface{} `json:"edClassMembers,omitempty"`
	Constraints []PsExtConstr   `json:"edClassConstraints,omitempty"`
}

//
func (me *PsExt) findTypeClass(name string) *PsExtTypeClass {
	for _, decl := range me.EfDecls {
		if decl.EDClass != nil && decl.EDClass.Name == name {
			return decl.EDClass
		}
	}
	return nil
}
