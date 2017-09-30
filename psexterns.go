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
	EDValue           map[string]interface{} `json:",omitempty"`
	EDInstance        map[string]interface{} `json:",omitempty"`
	EDDataConstructor map[string]interface{} `json:",omitempty"`
}

type PsExtType struct {
	Name     string          `json:"edTypeName,omitempty"`
	Kind     *TaggedContents `json:"edTypeKind,omitempty"`
	DeclKind interface{}     `json:"edTypeDeclarationKind,omitempty"`
}

type PsExtTypeAlias struct {
	Name      string          `json:"edTypeSynonymName,omitempty"`
	Arguments []interface{}   `json:"edTypeSynonymArguments,omitempty"`
	Type      *TaggedContents `json:"edTypeSynonymType,omitempty"`
}

type PsExtTypeClass struct {
	Name           string          `json:"edClassName,omitempty"`
	TypeArgs       [][]string      `json:"edClassTypeArguments,omitempty"`
	FunctionalDeps []interface{}   `json:"edFunctionalDependencies,omitempty"`
	Members        [][]interface{} `json:"edClassMembers,omitempty"`
	Constraints    []struct {
		Class []interface{}    `json:"constraintClass,omitempty"`
		Args  []TaggedContents `json:"constraintArgs,omitempty"`
		Data  interface{}      `json:"constraintData,omitempty"`
	} `json:"edClassConstraints,omitempty"`
}

func (me *PsExt) findTypeClass(name string) *PsExtTypeClass {
	for _, decl := range me.EfDecls {
		if decl.EDClass != nil && decl.EDClass.Name == name {
			return decl.EDClass
		}
	}
	return nil
}
