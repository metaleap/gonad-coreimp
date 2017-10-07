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
	sanitizer = strings.NewReplacer("'", "ª", "$", "º")
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
	mBody GIrABlock `json:",omitempty"`
}

func (me *GIrANamedTypeRef) Eq(cmp *GIrANamedTypeRef) bool {
	return (me == nil && cmp == nil) || (me != nil && cmp != nil && me.RefAlias == cmp.RefAlias && me.RefUnknown == cmp.RefUnknown && me.RefInterface.Eq(cmp.RefInterface) && me.RefFunc.Eq(cmp.RefFunc) && me.RefStruct.Eq(cmp.RefStruct) && me.RefArray.Eq(cmp.RefArray) && me.RefPtr.Eq(cmp.RefPtr))
}

func (me *GIrANamedTypeRef) setBothNamesFromPsName(psname string) {
	me.NamePs = psname
	me.NameGo = sanitizeSymbolForGo(psname, me.Export)
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
	ConstVal         GIrA `json:",omitempty"`
}

type GIrAVar struct {
	GIrANamedTypeRef `json:",omitempty"`
	VarVal           GIrA `json:",omitempty"`
}

type GIrAFunc struct {
	GIrANamedTypeRef `json:",omitempty"`
	GIrABlock        `json:",omitempty"`
}

type GIrALitStr struct {
	LitStr string
}

type GIrALitBool struct {
	LitBool bool
}

type GIrALitDouble struct {
	LitDouble float64
}

type GIrALitInt struct {
	LitInt int
}

type GIrABlock struct {
	Body []GIrA `json:",omitempty"`
}

func (me *GIrABlock) Add(asts ...GIrA) {
	me.Body = append(me.Body, asts...)
}

type GIrAComments struct {
	Comments     []CoreImpComment `json:",omitempty"`
	CommentsDecl GIrA             `json:",omitempty"`
}

type GIrAOp1 struct {
	Op1   string `json:",omitempty"`
	Right GIrA   `json:",omitempty"`
}

type GIrAOp2 struct {
	Left  GIrA   `json:",omitempty"`
	Op2   string `json:",omitempty"`
	Right GIrA   `json:",omitempty"`
}

type GIrASet struct {
	Left  GIrA `json:",omitempty"`
	Right GIrA `json:",omitempty"`
}

type GIrAFor struct {
	GIrABlock `json:",omitempty"`
	ForCond   GIrA      `json:",omitempty"`
	ForInit   []GIrASet `json:",omitempty"`
	ForStep   []GIrASet `json:",omitempty"`
	ForRange  GIrAVar   `json:",omitempty"`
}

type GIrAIf struct {
	If   GIrA      `json:",omitempty"`
	Then GIrABlock `json:",omitempty"`
	Else GIrABlock `json:",omitempty"`
}

type GIrACall struct {
	Callee   GIrA   `json:",omitempty"`
	CallArgs []GIrA `json:",omitempty"`
}

type GIrALitObj struct {
	ObjPairs []GIrAVar `json:",omitempty"`
}

type GIrANil struct {
	Nil struct{} `json:",omitempty"`
}

type GIrARet struct {
	RetArg GIrA `json:",omitempty"`
}

type GIrAPanic struct {
	PanicArg GIrA `json:",omitempty"`
}

type GIrALitArr struct {
	ArrVals []GIrA `json:",omitempty"`
}

type GIrAIndex struct {
	IdxLeft  GIrA `json:",omitempty"`
	IdxRight GIrA `json:",omitempty"`
}

type GIrADot struct {
	DotLeft  GIrA `json:",omitempty"`
	DotRight GIrA `json:",omitempty"`
}

type GIrAIsType struct {
	ExprToTest GIrA `json:",omitempty"`
	TypeToTest GIrA `json:",omitempty"`
}

func (me *GonadIrAst) PopulateFromCoreImp() (err error) {
	ci := me.mod.coreimp
	for _, cia := range ci.Body {
		me.Body = append(me.Body, cia.ciAstToGIrAst())
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

func sanitizeSymbolForGo(name string, forexport bool) string {
	if len(name) == 0 {
		return name
	}
	if forexport {
		name = strings.ToUpper(name[:1]) + name[1:]
	} else {
		if unicode.IsUpper([]rune(name[:1])[0]) {
			name = "_µ_" + name
		} else {
			switch name {
			case "append", "false", "iota", "nil", "true":
				return "_" + name
			case "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var":
				return "_ĸ_" + name
			}
		}
	}
	return sanitizer.Replace(name)
}

func ªDot(left GIrA, right string) GIrADot {
	return GIrADot{DotLeft: left, DotRight: ªV(right)}
}

func ªEq(left GIrA, right GIrA) GIrAOp2 {
	return GIrAOp2{Op2: "==", Left: left, Right: right}
}

func ªO1(op string, operand GIrA) GIrAOp1 {
	return GIrAOp1{Op1: op, Right: operand}
}

func ªO2(left GIrA, op string, right GIrA) GIrAOp2 {
	return GIrAOp2{Op2: op, Left: left, Right: right}
}

func ªRet(retarg GIrA) GIrARet {
	return GIrARet{RetArg: retarg}
}

func ªS(literal string) GIrALitStr {
	return GIrALitStr{LitStr: literal}
}

func ªSet(left string, right GIrA) GIrASet {
	return GIrASet{Left: ªV(left), Right: right}
}

func ªV(name string) GIrAVar {
	return GIrAVar{GIrANamedTypeRef: GIrANamedTypeRef{NameGo: name}}
}
