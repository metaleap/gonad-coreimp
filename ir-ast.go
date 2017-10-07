package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/metaleap/go-util-slice"
)

const (
	nsPrefixDefaultFfiPkg = "Ps2GoFFI."
)

var (
	sanitizer = strings.NewReplacer("'", "_")
)

type GIrANamedTypeRefs []*GIrANamedTypeRef

func (me GIrANamedTypeRefs) Eq(cmp GIrANamedTypeRefs) bool {
	if l := len(me); l != len(cmp) {
		return false
	} else {
		for i := 0; i < l; i++ {
			if !me[i].Eq(cmp[i]) {
				return false
			}
		}
	}
	return true
}

type GIrANamedTypeRef struct {
	NamePs string `json:",omitempty"`
	NameGo string `json:",omitempty"`

	RefAlias     string                `json:",omitempty"`
	RefUnknown   int                   `json:",omitempty"`
	RefInterface *GIrATypeRefInterface `json:",omitempty"`
	RefFunc      *GIrATypeRefFunc      `json:",omitempty"`
	RefStruct    *GIrATypeRefStruct    `json:",omitempty"`
	RefArray     *GIrATypeRefArray     `json:",omitempty"`
	RefPtr       *GIrATypeRefPtr       `json:",omitempty"`

	EnumConstNames []string          `json:",omitempty"`
	Methods        GIrANamedTypeRefs `json:",omitempty"`
	Export         bool              `json:",omitempty"`

	mCtor bool
	mBody CoreImpAsts
}

func (me *GIrANamedTypeRef) Eq(cmp *GIrANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.Eq(cmp.RefInterface) && me.RefFunc.Eq(cmp.RefFunc) && me.RefStruct.Eq(cmp.RefStruct) && me.RefArray.Eq(cmp.RefArray) && me.RefPtr.Eq(cmp.RefPtr))
}

func (me *GIrANamedTypeRef) setRefFrom(tref interface{}) {
	switch tr := tref.(type) {
	case *GIrANamedTypeRef:
		me.RefAlias = tr.RefAlias
		me.RefArray = tr.RefArray
		me.RefFunc = tr.RefFunc
		me.RefInterface = tr.RefInterface
		me.RefPtr = tr.RefPtr
		me.RefStruct = tr.RefStruct
		me.RefUnknown = tr.RefUnknown
	case *GIrATypeRefInterface:
		me.RefInterface = tr
	case *GIrATypeRefFunc:
		me.RefFunc = tr
	case *GIrATypeRefStruct:
		me.RefStruct = tr
	case *GIrATypeRefArray:
		me.RefArray = tr
	case *GIrATypeRefPtr:
		me.RefPtr = tr
	case int:
		me.RefUnknown = tr
	case string:
		me.RefAlias = tr
	case nil:
		me.RefAlias = "interface{/*TodoTRefWasNil*/}"
	default:
		println(tref.(float32))
	}
}

type GIrATypeRefArray struct {
	Of *GIrANamedTypeRef
}

func (me *GIrATypeRefArray) Eq(cmp *GIrATypeRefArray) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.Eq(cmp.Of))
}

type GIrATypeRefPtr struct {
	Of *GIrANamedTypeRef
}

func (me *GIrATypeRefPtr) Eq(cmp *GIrATypeRefPtr) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Of.Eq(cmp.Of))
}

type GIrATypeRefInterface struct {
	Embeds  []string          `json:",omitempty"`
	Methods GIrANamedTypeRefs `json:",omitempty"`

	xtc              *GIrMTypeClass
	inheritedMethods GIrANamedTypeRefs
}

func (me *GIrATypeRefInterface) Eq(cmp *GIrATypeRefInterface) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Methods.Eq(cmp.Methods))
}

func (me *GIrATypeRefInterface) allMethods() (allmethods GIrANamedTypeRefs) {
	allmethods = me.Methods
	if (!areOverlappingInterfacesSupportedByGo) && len(me.Embeds) > 0 {
		if len(me.inheritedMethods) == 0 {
			m := map[string]*GIrANamedTypeRef{}
			for _, embed := range me.Embeds {
				if gtd := findGoTypeByQName(embed); gtd == nil || gtd.RefInterface == nil {
					panic(fmt.Errorf("%s: references unknown interface/type-class %s, please report!", me.xtc.Name, embed))
				} else {
					for _, method := range gtd.RefInterface.allMethods() {
						if dupl, _ := m[method.NameGo]; dupl == nil {
							m[method.NameGo], me.inheritedMethods = method, append(me.inheritedMethods, method)
						} else if !dupl.Eq(method) {
							panic("Interface (generated from type-class " + me.xtc.Name + ") would inherit multiple (but different-signature) methods named " + method.NameGo)
						}
					}
				}
			}
		}
		allmethods = append(me.inheritedMethods, allmethods...)
	}
	return
}

type GIrATypeRefFunc struct {
	Args GIrANamedTypeRefs `json:",omitempty"`
	Rets GIrANamedTypeRefs `json:",omitempty"`
}

func (me *GIrATypeRefFunc) Eq(cmp *GIrATypeRefFunc) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.Args.Eq(cmp.Args) && me.Rets.Eq(cmp.Rets))
}

type GIrATypeRefStruct struct {
	Embeds    []string          `json:",omitempty"`
	Fields    GIrANamedTypeRefs `json:",omitempty"`
	PassByPtr bool              `json:",omitempty"`
}

func (me *GIrATypeRefStruct) Eq(cmp *GIrATypeRefStruct) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && uslice.StrEq(me.Embeds, cmp.Embeds) && me.Fields.Eq(cmp.Fields))
}

type GonadIrAst struct {
	GIrABlock `json:",omitempty"`

	mod  *ModuleInfo
	proj *BowerProject
	girM *GonadIrMeta
}

type GIrA interface{}

type GIrAConst struct {
	GIrANamedTypeRef `json:",omitempty"`
	Literal          GIrA `json:",omitempty"`
}

type GIrAVar struct {
	GIrANamedTypeRef `json:",omitempty"`
	Initial          GIrA `json:",omitempty"`
}

type GIrAFunc struct {
	GIrANamedTypeRef `json:",omitempty"`
	Params           []string `json:",omitempty"`
	GIrABlock        `json:",omitempty"`
}

type GIrALitStr string

type GIrALitBool bool

type GIrALitDouble float64

type GIrALitInt int

type GIrABlock struct {
	Body []GIrA `json:",omitempty"`
}

type GIrAComments struct {
	All  []CoreImpComment `json:",omitempty"`
	Decl GIrA             `json:",omitempty"`
}

type GIrAOp1 struct {
	Op    string `json:",omitempty"`
	Right GIrA   `json:",omitempty"`
}

type GIrAOp2 struct {
	Left  GIrA   `json:",omitempty"`
	Op    string `json:",omitempty"`
	Right GIrA   `json:",omitempty"`
}

type GIrAFor struct {
	GIrABlock `json:",omitempty"`
	Cond      GIrA      `json:",omitempty"`
	OnInit    []GIrAOp2 `json:",omitempty"`
	OnStepped []GIrAOp2 `json:",omitempty"`
	RangeVar  GIrAVar   `json:",omitempty"`
	RangeOver GIrA      `json:",omitempty"`
}

type GIrAIf struct {
	Cond GIrA      `json:",omitempty"`
	Then GIrABlock `json:",omitempty"`
	Else GIrABlock `json:",omitempty"`
}

type GIrACall struct {
	Func GIrA   `json:",omitempty"`
	Args []GIrA `json:",omitempty"`
}

type GIrAObjLit struct {
	Pairs []GIrAVar `json:",omitempty"`
}

func (me *GonadIrAst) astForceIntoBlock(cia *CoreImpAst, into *GIrABlock) {
	switch body := me.astFromCoreImp(cia).(type) {
	case GIrABlock:
		*into = body
	default:
		into.Body = append(into.Body, body)
	}

}

func (me *GonadIrAst) astFromCoreImp(cia *CoreImpAst) (a GIrA) {
	switch cia.AstTag {
	case "StringLiteral":
		a = GIrALitStr(cia.StringLiteral)
	case "BooleanLiteral":
		a = GIrALitBool(cia.BooleanLiteral)
	case "NumericLiteral_Double":
		a = GIrALitDouble(cia.NumericLiteral_Double)
	case "NumericLiteral_Integer":
		a = GIrALitInt(cia.NumericLiteral_Integer)
	case "Var":
		v := GIrAVar{}
		v.NamePs = cia.Var
		a = v
	case "Block":
		b := GIrABlock{}
		for _, c := range cia.Block {
			b.Body = append(b.Body, me.astFromCoreImp(c))
		}
		a = b
	case "While":
		f := GIrAFor{}
		f.Cond = me.astFromCoreImp(cia.While)
		me.astForceIntoBlock(cia.AstBody, &f.GIrABlock)
		a = f
	case "ForIn":
		f := GIrAFor{}
		f.RangeVar = GIrAVar{}
		f.RangeVar.NamePs = cia.ForIn
		f.RangeOver = me.astFromCoreImp(cia.AstFor1)
		me.astForceIntoBlock(cia.AstBody, &f.GIrABlock)
		a = f
	case "For":
		f, v := GIrAFor{}, GIrAVar{}
		v.NamePs, f.OnInit = cia.For, []GIrAOp2{{
			Left: v, Op: "=", Right: me.astFromCoreImp(cia.AstFor1)}}
		f.Cond = GIrAOp2{Left: v, Op: "<", Right: me.astFromCoreImp(cia.AstFor2)}
		me.astForceIntoBlock(cia.AstBody, &f.GIrABlock)
		a = f
	case "IfElse":
		i := GIrAIf{Cond: me.astFromCoreImp(cia.IfElse)}
		me.astForceIntoBlock(cia.AstThen, &i.Then)
		if cia.AstElse != nil {
			me.astForceIntoBlock(cia.AstElse, &i.Else)
		}
		a = i
	case "App":
		c := GIrACall{Func: me.astFromCoreImp(cia.App)}
		for _, arg := range cia.AstApplArgs {
			c.Args = append(c.Args, me.astFromCoreImp(arg))
		}
		a = c
	case "Function":
		f := GIrAFunc{Params: cia.AstFuncParams}
		f.NamePs = cia.Function
		me.astForceIntoBlock(cia.AstBody, &f.GIrABlock)
		a = f
	case "Unary":
		o := GIrAOp1{Op: cia.AstOp, Right: me.astFromCoreImp(cia.Unary)}
		a = o
	case "Binary":
		o := GIrAOp2{Op: cia.AstOp, Left: me.astFromCoreImp(cia.Binary), Right: me.astFromCoreImp(cia.AstRight)}
		a = o
	case "VariableIntroduction":
		c := GIrAVar{}
		c.NamePs = cia.VariableIntroduction
		if cia.AstRight != nil {
			c.Initial = me.astFromCoreImp(cia.AstRight)
		}
		a = c
	case "Comment":
		c := GIrAComments{}
		for _, comment := range cia.Comment {
			if comment != nil {
				c.All = append(c.All, *comment)
			}
		}
		if cia.AstCommentDecl != nil {
			c.Decl = me.astFromCoreImp(cia.AstCommentDecl)
		}
		a = c
	case "ObjectLiteral":
		o := GIrAObjLit{}
		for _, namevaluepair := range cia.ObjectLiteral {
			for onekey, oneval := range namevaluepair {
				v := GIrAVar{}
				v.NamePs, v.Initial = onekey, me.astFromCoreImp(oneval)
				o.Pairs = append(o.Pairs, v)
				break
			}
		}
		a = o
	case "ReturnNoResult":
	case "Return":
	case "Throw":
	case "ArrayLiteral":
	case "Assignment":
	case "Accessor":
	case "Indexer":
	case "InstanceOf":
	default:
		panic(fmt.Errorf("%s: unrecognized CoreImp AST-tag, please report: %s", me.mod.srcFilePath, cia.AstTag))
	}
	return
}

func (me *GonadIrAst) PopulateFromCoreImp() (err error) {
	ci := me.mod.coreimp
	for _, cia := range ci.Body {
		me.Body = append(me.Body, me.astFromCoreImp(cia))
	}

	if err == nil {
	}
	return
}

func (me *GonadIrAst) WriteAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}

func (me *GonadIrAst) WriteAsGoTo(writer io.Writer) (err error) {
	var buf = &bytes.Buffer{}

	for _, gtd := range me.girM.GoTypeDefs {
		codeEmitTypeDecl(buf, gtd, 0, me.resolveGoTypeRef)
		if len(gtd.EnumConstNames) > 0 {
			enumtypename := toGIrAEnumTypeName(gtd.NamePs)
			codeEmitTypeAlias(buf, enumtypename, "int")
			codeEmitEnumConsts(buf, gtd.EnumConstNames, enumtypename)
			codeEmitTypeMethods(buf, gtd, me.resolveGoTypeRef)
		}
	}

	codeEmitPkgDecl(writer, me.mod.pName)
	sort.Sort(me.girM.Imports)
	codeEmitModImps(writer, me.girM.Imports)
	buf.WriteTo(writer)
	return
}

func (me *GonadIrAst) resolveGoTypeRef(tref string, markused bool) (pname string, tname string) {
	i := strings.LastIndex(tref, ".")
	if tname = tref[i+1:]; i > 0 {
		pname = tref[:i]
		if pname == me.mod.qName {
			pname = ""
		} else if pname == "Prim" {
			pname = ""
			switch tname {
			case "String":
				tname = "string"
			case "Boolean":
				tname = "bool"
			case "Number":
				tname = "float64"
			case "Int":
				tname = "int"
			default:
				panic("Unknown Prim type: " + tname)
			}
		} else {
			qn, foundimport, isffi := pname, false, strings.HasPrefix(pname, nsPrefixDefaultFfiPkg)
			var mod *ModuleInfo
			if isffi {
				pname = dot2underscore.Replace(pname)
			} else {
				if mod = FindModuleByQName(pname); mod == nil {
					panic(fmt.Errorf("%s: unknown module qname %s", me.mod.srcFilePath, qn))
				}
				pname = mod.pName
			}
			for _, imp := range me.girM.Imports {
				if imp.Q == qn {
					if foundimport = true; markused {
						imp.used = true
					}
				}
			}
			if !foundimport {
				var imp *GIrMPkgRef
				if isffi {
					imp = &GIrMPkgRef{P: "github.com/metaleap/gonad/" + dot2slash.Replace(qn), Q: qn, N: pname}
				} else {
					imp = newModImp(mod)
				}
				if me.girM.imports, me.girM.Imports = append(me.girM.imports, mod), append(me.girM.Imports, imp); markused {
					imp.used = true
				}
			}
		}
	}
	return
}

func (me *GonadIrMeta) sanitizeSymbolForGo(name string, forexport bool) string {
	if forexport {
		name = strings.Title(name)
	} else {
		if unicode.IsUpper([]rune(name[:1])[0]) {
			name = "_µ_" + name
		} else {
			switch name {
			case "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var":
				return "_ĸ_" + name
			}
		}
	}
	return sanitizer.Replace(name)
}
