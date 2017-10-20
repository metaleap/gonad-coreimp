package main

/*
We ignore most of the stuff in externs.json now, as it's
provided in coreimp's DeclEnv for both exports & non-exports.

BUT we make use of EfExports still, as coreimp's `exports`
don't capture type synonyms.
*/

type PsExt struct {
	modinfo *ModuleInfo

	// EfSourceSpan *CoreImpSourceSpan `json:"efSourceSpan,omitempty"`
	// EfVersion    string             `json:"efVersion,omitempty"`
	// EfModuleName []string           `json:"efModuleName,omitempty"`
	// EfDecls      []*PsExtDecl       `json:"efDeclarations,omitempty"`
	// EfImports    []*PsExtImport     `json:"efImports,omitempty"`
	EfExports []*PsExtRefs `json:"efExports,omitempty"`
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
	Name PsExtIdent    `json:"edValueName"`
	Type coreImpEnvTag `json:"edValueType"`
}

type PsExtType struct {
	Name     string        `json:"edTypeName,omitempty"`
	Kind     coreImpEnvTag `json:"edTypeKind,omitempty"`
	DeclKind interface{}   `json:"edTypeDeclarationKind,omitempty"`
}

type PsExtTypeAlias struct {
	Name      string         `json:"edTypeSynonymName,omitempty"`
	Arguments []interface{}  `json:"edTypeSynonymArguments,omitempty"`
	Type      *coreImpEnvTag `json:"edTypeSynonymType,omitempty"`
}

type PsExtConstr struct {
	Class []interface{}   `json:"constraintClass,omitempty"`
	Args  []coreImpEnvTag `json:"constraintArgs,omitempty"`
	Data  []interface{}   `json:"constraintData,omitempty"`
}

type PsExtInst struct {
	ClassName   []interface{}   `json:"edInstanceClassName,omitempty"`
	Name        PsExtIdent      `json:"edInstanceName,omitempty"`
	Types       []coreImpEnvTag `json:"edInstanceTypes,omitempty"`
	Constraints []PsExtConstr   `json:"edInstanceConstraints,omitempty"`
	Chain       [][]interface{} `json:"edInstanceChain,omitempty"`
	ChainIndex  int             `json:"edInstanceChainIndex,omitempty"`
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
