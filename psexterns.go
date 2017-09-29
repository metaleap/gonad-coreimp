package main

type TaggedContents struct {
	Tag      string      `json:"tag,omitempty"`
	Contents interface{} `json:"contents,omitempty"`
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
	EDClass struct {
		members []classMember

		EdClassName              string          `json:"edClassName,omitempty"`
		EdClassTypeArguments     [][]string      `json:"edClassTypeArguments,omitempty"`
		EdFunctionalDependencies []interface{}   `json:"edFunctionalDependencies,omitempty"`
		EdClassMembers           [][]interface{} `json:"edClassMembers,omitempty"`
		EdClassConstraints       []struct {
			ConstraintClass []interface{}    `json:"constraintClass,omitempty"`
			ConstraintArgs  []TaggedContents `json:"constraintArgs,omitempty"`
			ConstraintData  interface{}      `json:"constraintData,omitempty"`
		} `json:"edClassConstraints,omitempty"`
	}
	EDType            map[string]interface{}
	EDTypeSynonym     map[string]interface{}
	EDValue           map[string]interface{}
	EDInstance        map[string]interface{}
	EDDataConstructor map[string]interface{}
}

type classMember struct {
	name         string
	argTypeNames []string
}

func (me *PsExt) process() (err error) {
	for _, d := range me.EfDecls {
		if len(d.EDClass.EdClassName) > 0 {
			me.figureTypeClass(d)
		}
	}
	return
}

func (me *PsExt) figureTypeClass(d *PsExtDecl) {
	errmsg := me.modinfo.extFilePath + ": " + d.EDClass.EdClassName + ": unexpected type-class format, please report!"
	walk := func(x []interface{}, mem *classMember) {
		left, right := x[0], x[1]
		switch l := left.(type) {
		case string:
			mem.argTypeNames = append(mem.argTypeNames, l)
		case map[string]interface{}:
			mem.argTypeNames = append(mem.argTypeNames, l["tag"].(string))
		default:
			panic(errmsg)
		}
		if right == nil {
		}
	}
	if len(d.EDClass.EdClassName) > 0 {
		for _, m := range d.EDClass.EdClassMembers {
			if len(m) != 2 {
				panic(errmsg)
			} else {
				mem := classMember{name: m[0].(map[string]interface{})["Ident"].(string)}
				mtc := m[1].(map[string]interface{})["contents"]
				switch x := mtc.(type) {
				case string:
					mem.argTypeNames = append(mem.argTypeNames, x)
				case []interface{}:
					walk(x, &mem)
				}
				d.EDClass.members = append(d.EDClass.members, mem)
			}
		}
	}
	return
}
