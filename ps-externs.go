package main

/*
We ignore most of the stuff in externs.json now, as it's
provided in coreimp's DeclEnv for both exports & non-exports.

BUT we make use of EfExports still, as coreimp's `exports`
don't capture type synonyms.
*/

type psExt struct {
	// EfSourceSpan *coreImpSourceSpan `json:"efSourceSpan"`
	// EfVersion    string             `json:"efVersion"`
	// EfModuleName []string           `json:"efModuleName"`
	// EfDecls      []*psExtDecl       `json:"efDeclarations"`
	// EfImports    []*psExtImport     `json:"efImports"`
	EfExports []*psExtRefs `json:"efExports"`
}

type psExtImport struct {
	EiModule     []string         `json:"eiModule"`
	EiImportType *psExtImportType `json:"eiImportType"`
	EiImportedAs []string         `json:"eiImportedAs"`
}

type psExtRefs struct {
	TypeRef         []interface{}
	TypeClassRef    []interface{}
	TypeInstanceRef []interface{}
	ValueRef        []interface{}
	ValueOpRef      []interface{}
	ModuleRef       []interface{}
	ReExportRef     []interface{}
}

type psExtImportType struct {
	Implicit []interface{}
	Explicit []*psExtRefs
}

type psExtDecl struct {
	EDClass           *psExtTypeClass
	EDType            *psExtType
	EDTypeSynonym     *psExtTypeAlias
	EDValue           *psExtVal
	EDInstance        *psExtInst
	EDDataConstructor map[string]interface{}
}

type psExtIdent struct {
	Ident string
}

type psExtVal struct {
	Name psExtIdent    `json:"edValueName"`
	Type coreImpEnvTag `json:"edValueType"`
}

type psExtType struct {
	Name     string        `json:"edTypeName"`
	Kind     coreImpEnvTag `json:"edTypeKind"`
	DeclKind interface{}   `json:"edTypeDeclarationKind"`
}

type psExtTypeAlias struct {
	Name      string         `json:"edTypeSynonymName"`
	Arguments []interface{}  `json:"edTypeSynonymArguments"`
	Type      *coreImpEnvTag `json:"edTypeSynonymType"`
}

type psExtConstr struct {
	Class []interface{}   `json:"constraintClass"`
	Args  []coreImpEnvTag `json:"constraintArgs"`
	Data  []interface{}   `json:"constraintData"`
}

type psExtInst struct {
	ClassName   []interface{}   `json:"edInstanceClassName"`
	Name        psExtIdent      `json:"edInstanceName"`
	Types       []coreImpEnvTag `json:"edInstanceTypes"`
	Constraints []psExtConstr   `json:"edInstanceConstraints"`
	Chain       [][]interface{} `json:"edInstanceChain"`
	ChainIndex  int             `json:"edInstanceChainIndex"`
}

type psExtTypeClass struct {
	Name           string          `json:"edClassName"`
	TypeArgs       [][]interface{} `json:"edClassTypeArguments"`
	FunctionalDeps []struct {
		Determiners []int `json:"determiners"`
		Determined  []int `json:"determined"`
	} `json:"edFunctionalDependencies"`
	Members     [][]interface{} `json:"edClassMembers"`
	Constraints []psExtConstr   `json:"edClassConstraints"`
}
