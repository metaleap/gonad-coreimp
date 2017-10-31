package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/metaleap/go-util/dev/ps"
	"github.com/metaleap/go-util/str"
)

/*
Golang intermediate-representation AST:
represents the code in a generated Go package, minus
"IR meta stuff" that is, imports & type declarations
(see ir-meta & ir-typestuff), also struct methods.
This latter 'design accident' should probably be revamped.
*/

var (
	exprTypeInt  = &irANamedTypeRef{RefAlias: "Prim.Int"}
	exprTypeNum  = &irANamedTypeRef{RefAlias: "Prim.Number"}
	exprTypeStr  = &irANamedTypeRef{RefAlias: "Prim.String"}
	exprTypeBool = &irANamedTypeRef{RefAlias: "Prim.Boolean"}
)

type irAst struct {
	irABlock

	culled struct {
		typeCtorFuncs []*irACtor
		tcDictDecls   []irA
		tcInstImpls   []*irTcInstImpl
	}
	mod *modPkg
	irM *irMeta
}

type irTcInstImpl struct {
	tci            *irMTypeClassInst
	tciAlias       string
	tciPassThrough bool
	tciProper      struct {
		tc            *irMTypeClass
		tcMod         *modPkg
		tcMemberImpls []irA
	}
}

type irA interface {
	Ast() *irAst
	Base() *irABase
	Equiv(irA) bool // not struct equality but semantic equivalency comparison (ignoring comments, parent etc) as we need it where we do
	ExprType() *irANamedTypeRef
	Parent() irA
}

type irABase struct {
	irANamedTypeRef                       // don't use all of this, but exprs with names and/or types do as needed
	Comments        []*udevps.CoreComment `json:",omitempty"`
	parent          irA
	root            *irAst // usually nil but set in top-level irABlock. for the rare occasions a irA impl needs this, it uses Ast() which traverses parents to the root then stores in ast --- rather than passing the root to all irA constructors etc
}

func (me *irABase) Ast() *irAst {
	if me.root == nil && me.parent != nil {
		me.root = me.parent.Ast()
	}
	return me.root
}

func (me *irABase) Base() *irABase             { return me }
func (me *irABase) ExprType() *irANamedTypeRef { return &me.irANamedTypeRef }
func (me *irABase) Parent() irA                { return me.parent }
func (me *irABase) Equiv(cmp irA) bool {
	ab := cmp.Base()
	return (me == nil && ab == nil) || (me != nil && ab != nil && me.irANamedTypeRef.equiv(&ab.irANamedTypeRef) && me.NameGo == ab.NameGo && me.NamePs == ab.NamePs)
}

func (me *irABase) isTopLevel() bool {
	return me.parent == &me.Ast().irABlock
}

func (me *irABase) parentOp() (po1 *irAOp1, po2 *irAOp2) {
	switch op := me.parent.(type) {
	case *irAOp1:
		po1 = op
	case *irAOp2:
		po2 = op
	}
	return
}

func (me *irABase) srcFilePath() (srcfilepath string) {
	if root := me.Ast(); root != nil {
		srcfilepath = root.mod.srcFilePath
	}
	return
}

type irAConstable interface {
	isConstable() bool
}

type irASymStr interface {
	symStr() string
}

type irAConst struct {
	irABase
	ConstVal irA
}

func (me *irAConst) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAConst)
	return me.Base().Equiv(c) && (c == nil || me.ConstVal.Equiv(c.ConstVal))
}

func (me *irAConst) isConstable() bool { return true }

type irALet struct {
	irABase
	LetVal irA

	typeConv struct {
		okname string
		vused  bool
	}
}

func (me *irALet) callers() (all []*irACall) {
	irALookupBelow(me.parent, true, func(a irA) bool {
		if acall, _ := a.(*irACall); acall != nil {
			if acallee, _ := acall.Callee.(*irASym); acallee != nil && acallee.refTo() == me {
				all = append(all, acall)
			}
		}
		return false
	})
	return
}

func (me *irALet) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALet)
	return me.Base().Equiv(c) && (c == nil || (me.typeConv == c.typeConv && me.LetVal.Equiv(c.LetVal)))
}

func (me *irALet) isConstable() bool {
	if c, _ := me.LetVal.(irAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

func (me *irALet) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		if me.LetVal != nil {
			me.copyTypeInfoFrom(me.LetVal.ExprType())
		}
	}
	return &me.irANamedTypeRef
}

func (me *irALet) setterFromCallTo(fn *irALet) (set *irASet) {
	for _, setter := range me.setters() {
		if acall, _ := setter.ToRight.(*irACall); acall != nil {
			if acallee, _ := acall.Callee.(*irASym); acallee != nil && acallee.refTo() == fn {
				return setter
			}
		}
	}
	return
}

func (me *irALet) setters() (all []*irASet) {
	irALookupBelow(me.parent, true, func(a irA) bool {
		if aset, _ := a.(*irASet); aset != nil {
			if asym, _ := aset.SetLeft.(*irASym); asym != nil && asym.refTo() == me {
				all = append(all, aset)
			}
		}
		return false
	})
	return
}

type irASym struct {
	irABase
	refto    irA
	reftoarg *irANamedTypeRef
	Sym__    interface{} // useless except we want to see it in the gonadast.json
}

func (me *irASym) Equiv(sym irA) bool {
	s, _ := sym.(*irASym)
	if s != nil && me != nil {
		if me.NameGo != "" && s.NameGo != "" {
			return me.NameGo == s.NameGo
		} else {
			return me.NamePs == s.NamePs
		}
	}
	return s == nil && me == nil
}

func (me *irASym) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		if ref := me.refTo(); ref != nil {
			if reft := ref.ExprType(); reft.hasTypeInfo() {
				me.copyTypeInfoFrom(reft)
			}
		}
	}
	if !me.hasTypeInfo() {
		if refarg := me.refToArg(); refarg != nil && refarg.hasTypeInfo() {
			me.copyTypeInfoFrom(refarg)
		}
	}
	return &me.irANamedTypeRef
}

func (me *irASym) isConstable() bool {
	if c, _ := me.refTo().(irAConstable); c != nil {
		return c.isConstable()
	}
	return false
}

func (me *irASym) refTo() irA {
	if me.refto == nil {
		me.refto = irALookupInAncestorBlocks(me, func(stmt irA) (isref bool) {
			switch stmt.(type) {
			case *irALet, *irAConst, *irAFunc:
				isref = (me.NamePs == stmt.Base().NamePs)
			}
			return
		})
	}
	return me.refto
}

func (me *irASym) refToArg() (argref *irANamedTypeRef) {
	if argref = me.reftoarg; argref == nil {
		me.perFuncUp(func(outerfunc *irAFunc) {
			if argref == nil {
				for _, fnarg := range outerfunc.RefFunc.Args {
					if fnarg.NameGo == me.NameGo || fnarg.NamePs == me.NamePs {
						argref = fnarg
						break
					}
				}
			}
		})
		me.reftoarg = argref
	}
	return
}

func (me *irASym) symStr() string {
	return me.NameGo
}

type irAFunc struct {
	irABase
	FuncImpl *irABlock
}

func (me *irAFunc) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAFunc)
	return me.Base().Equiv(c) && (c == nil || me.FuncImpl.Equiv(c.FuncImpl))
}

func (me *irAFunc) isTopLevel() (istld bool) {
	if istld = me.irABase.isTopLevel(); !istld {
		if pv, _ := me.parent.(*irALet); pv != nil {
			istld = pv.isTopLevel()
		} else if pc, _ := me.parent.(*irAConst); pc != nil {
			istld = pc.isTopLevel()
		}
	}
	return
}

type irALitStr struct {
	irABase
	LitStr string
}

func (_ *irALitStr) ExprType() *irANamedTypeRef { return exprTypeStr }
func (me *irALitStr) isConstable() bool         { return true }

func (me *irALitStr) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitStr)
	return (me == nil && c == nil) || (me != nil && c != nil && me.LitStr == c.LitStr)
}

type irALitBool struct {
	irABase
	LitBool bool
}

func (_ *irALitBool) ExprType() *irANamedTypeRef { return exprTypeBool }
func (_ irALitBool) isConstable() bool           { return true }

func (me *irALitBool) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitBool)
	return (me == nil && c == nil) || (me != nil && c != nil && me.LitBool == c.LitBool)
}

type irALitNum struct {
	irABase
	LitNum float64
}

func (_ *irALitNum) ExprType() *irANamedTypeRef { return exprTypeNum }
func (_ irALitNum) isConstable() bool           { return true }

func (me *irALitNum) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitNum)
	return (me == nil && c == nil) || (me != nil && c != nil && me.LitNum == c.LitNum)
}

type irALitInt struct {
	irABase
	LitInt int
}

func (me *irALitInt) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitInt)
	return (me == nil && c == nil) || (me != nil && c != nil && me.LitInt == c.LitInt)
}

func (_ *irALitInt) ExprType() *irANamedTypeRef { return exprTypeInt }
func (_ irALitInt) isConstable() bool           { return true }

type irABlock struct {
	irABase

	Body []irA
}

func (me *irABlock) Equiv(cmp irA) bool {
	c, _ := cmp.(*irABlock)
	if me != nil && c != nil && len(me.Body) == len(c.Body) {
		for i, a := range me.Body {
			if !a.Equiv(c.Body[i]) {
				return false
			}
		}
		return true
	}
	return c == nil && me == nil

}

func (me *irABlock) add(asts ...irA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(me.Body, asts...)
}

func (me *irABlock) countSymRefs(gonames irANamedTypeRefs) (m map[string]int) {
	m = make(map[string]int, len(gonames))
	for _, goname := range gonames {
		m[goname.NameGo] = 0
	}
	walk(me, true, func(a irA) irA {
		if asym, _ := a.(*irASym); asym != nil {
			if count, exists := m[asym.NameGo]; exists {
				m[asym.NameGo] = count + 1
			}
		}
		return a
	})
	return
}

func (me *irABlock) refersToSym(namego string) (itdoes bool) {
	walk(me, true, func(a irA) irA {
		if !itdoes {
			if asym, _ := a.(*irASym); asym != nil && asym.NameGo == namego {
				itdoes = true
			}
		}
		return a
	})
	return
}

func (me *irABlock) prepend(asts ...irA) {
	for _, a := range asts {
		a.Base().parent = me
	}
	me.Body = append(asts, me.Body...)
}

func (me *irABlock) insert(i int, a irA) {
	a.Base().parent = me
	me.Body = append(me.Body[:i], append([]irA{a}, me.Body[i:]...)...)
}

func (me *irABlock) removeAt(i int) {
	me.Body = append(me.Body[:i], me.Body[i+1:]...)
}

type irAComments struct {
	irABase
}

type irACtor struct {
	irAFunc
	Ctor__ interface{} // useless except we want to see it in the gonadast.json
}

type irAOp1 struct {
	irABase
	Op1 string
	Of  irA
}

func (me *irAOp1) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAOp1)
	if c != nil && me != nil && c.Op1 == me.Op1 {
		return me.Of.Equiv(c.Of)
	}
	return c == nil && me == nil
}

func (me irAOp1) isConstable() bool {
	if c, ok := me.Of.(irAConstable); ok {
		return c.isConstable()
	}
	return false
}

func (me *irAOp1) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		if me.Op1 == "!" {
			me.copyTypeInfoFrom(exprTypeBool)
		} else if ofb := me.Of.Base(); ofb.hasTypeInfo() {
			if me.copyTypeInfoFrom(&ofb.irANamedTypeRef); me.Op1 == "&" {
				me.turnRefIntoRefPtr()
			}
		}
	}
	return &me.irANamedTypeRef
}

type irAOp2 struct {
	irABase
	Left  irA
	Op2   string
	Right irA
}

func (me *irAOp2) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAOp2)
	if c != nil && me != nil && c.Op2 == me.Op2 {
		return me.Left.Equiv(c.Left) && me.Right.Equiv(c.Right)
	}
	return c == nil && me == nil
}

func (me *irAOp2) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		switch me.Op2 {
		case "==", "!=", "<", "<=", ">", ">=", "&&", "||", "&", "|":
			me.copyTypeInfoFrom(exprTypeBool)
		default:
			if tl, tr := me.Left.ExprType(), me.Right.ExprType(); tl.hasTypeInfo() {
				me.copyTypeInfoFrom(tl)
			} else if tr.hasTypeInfo() {
				me.copyTypeInfoFrom(tr)
			}
		}
	}
	return &me.irANamedTypeRef
}

func (me irAOp2) isConstable() bool {
	if cl, _ := me.Left.(irAConstable); cl != nil && cl.isConstable() {
		if cr, _ := me.Right.(irAConstable); cr != nil && cr.isConstable() {
			return true
		}
	}
	return false
}

type irASet struct {
	irABase
	SetLeft irA
	ToRight irA

	isInVarGroup bool
}

func (me *irASet) Equiv(cmp irA) bool {
	c, _ := cmp.(*irASet)
	return (me == nil && c == nil) || (me != nil && c != nil && me.SetLeft.Equiv(c.SetLeft) && me.ToRight.Equiv(c.ToRight))
}

type irAFor struct {
	irABase
	ForDo    *irABlock
	ForCond  irA
	ForInit  []*irALet
	ForStep  []*irASet
	ForRange *irALet
}

func (me *irAFor) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAFor)
	if me != nil && c != nil && me.ForDo.Equiv(c.ForDo) && me.ForRange.Equiv(c.ForRange) && me.ForCond.Equiv(c.ForCond) && len(me.ForInit) == len(c.ForInit) && len(me.ForStep) == len(c.ForStep) {
		for i, l := range me.ForInit {
			if !l.Equiv(c.ForInit[i]) {
				return false
			}
		}
		for i, s := range me.ForStep {
			if !s.Equiv(c.ForStep[i]) {
				return false
			}
		}
		return true
	}
	return me == nil && c == nil
}

type irAIf struct {
	irABase
	If   irA
	Then *irABlock
	Else *irABlock
}

func (_ *irAIf) ExprType() *irANamedTypeRef { return exprTypeBool }

func (me *irAIf) condNegates(other *irAIf) bool {
	mop, _ := me.If.(*irAOp1)
	oop, _ := other.If.(*irAOp1)
	if mop != nil && mop.Op1 != "!" {
		mop = nil
	}
	if oop != nil && oop.Op1 != "!" {
		oop = nil
	}
	if mop == nil && oop != nil {
		return me.If.Equiv(oop.Of) // always true so far, but coreimp output formats can always change, so we test correctly
	} else if mop != nil && oop == nil {
		return mop.Of.Equiv(other.If) // dito
	}
	return false
}

func (me *irAIf) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAIf)
	return (me == nil && c == nil) || (me != nil && c != nil && me.If.Equiv(c.If) && me.Then.Equiv(c.Then) && me.Else.Equiv(c.Else))
}

func (me *irAIf) typeAssertions() []*irAIsType {
	return irALookupBelowˇIsType(me, false)
}

type irACall struct {
	irABase
	Callee   irA
	CallArgs []irA
}

func (me *irACall) Equiv(cmp irA) bool {
	c, _ := cmp.(*irACall)
	if me != nil && c != nil && me.Callee.Equiv(c.Callee) && len(me.CallArgs) == len(c.CallArgs) {
		for i, a := range me.CallArgs {
			if !a.Equiv(c.CallArgs[i]) {
				return false
			}
		}
		return true
	}
	return me == nil && c == nil
}

type irALitObj struct {
	irABase
	ObjFields []*irALitObjField
}

func (me *irALitObj) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitObj)
	if me.Base().Equiv(c) && c != nil && len(me.ObjFields) == len(c.ObjFields) {
		for i, f := range me.ObjFields {
			if !f.Equiv(c.ObjFields[i]) {
				return false
			}
		}
		return true
	}
	return me == nil && c == nil
}

func (me *irALitObj) fieldsNamed() (named bool) {
	for i, f := range me.ObjFields {
		if n := f.hasName(); i > 0 && n != named {
			panic(notImplErr("mix of named and unnamed fields", me.NamePs, me.root.mod.srcFilePath))
		} else {
			named = n
		}
	}
	return
}

type irALitObjField struct {
	irABase
	FieldVal irA
}

func (me *irALitObjField) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitObjField)
	return me.Base().Equiv(c) && (c == nil || me.FieldVal.Equiv(c.FieldVal))
}

type irANil struct {
	irABase
	Nil__ interface{} // useless except we want to see it in the gonadast.json
}

type irARet struct {
	irABase
	RetArg irA
}

func (me *irARet) Equiv(cmp irA) bool {
	c, _ := cmp.(*irARet)
	return me.Base().Equiv(c) && (c == nil || me.RetArg.Equiv(c.RetArg))
}

func (me *irARet) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		if me.RetArg != nil {
			if tret := me.RetArg.ExprType(); tret.hasTypeInfo() {
				me.copyTypeInfoFrom(tret)
			}
		}
	}
	return &me.irANamedTypeRef
}

type irAPanic struct {
	irABase
	PanicArg irA
}

func (me *irAPanic) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAPanic)
	return me.Base().Equiv(c) && (c == nil || me.PanicArg.Equiv(c.PanicArg))
}

type irALitArr struct {
	irABase
	ArrVals []irA
}

func (me *irALitArr) Equiv(cmp irA) bool {
	c, _ := cmp.(*irALitArr)
	if me != nil && c != nil && len(me.ArrVals) == len(c.ArrVals) {
		for i, v := range me.ArrVals {
			if !v.Equiv(c.ArrVals[i]) {
				return false
			}
		}
		return true
	}
	return me == nil && c == nil
}

type irAIndex struct {
	irABase
	IdxLeft  irA
	IdxRight irA
}

func (me *irAIndex) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAIndex)
	return (me == nil && c == nil) || (me != nil && c != nil && me.IdxLeft.Equiv(c.IdxLeft) && me.IdxRight.Equiv(c.IdxRight))
}

type irADot struct {
	irABase
	DotLeft  irA
	DotRight irA
}

func (me *irADot) Equiv(cmp irA) bool {
	c, _ := cmp.(*irADot)
	return (me == nil && c == nil) || (me != nil && c != nil && me.DotLeft.Equiv(c.DotLeft) && me.DotRight.Equiv(c.DotRight))
}

func (me *irADot) symStr() (symstr string) {
	if sl, _ := me.DotLeft.(irASymStr); sl != nil {
		symstr = sl.symStr()
	}
	symstr += "ꓸ"
	if sr, _ := me.DotRight.(irASymStr); sr != nil {
		symstr += sr.symStr()
	}
	return
}

type irAIsType struct {
	irABase
	ExprToTest irA
	TypeToTest string

	names struct {
		v, t string
	}
}

func (me *irAIsType) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAIsType)
	return (me == nil && c == nil) || (me != nil && c != nil && me.TypeToTest == c.TypeToTest && me.names == c.names && me.ExprToTest.Equiv(c.ExprToTest))
}

func (_ *irAIsType) ExprType() *irANamedTypeRef { return exprTypeBool }

type irAToType struct {
	irABase
	ExprToConv irA
	TypePkg    string
	TypeName   string
}

func (me *irAToType) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		me.copyTypeInfoFrom(&irANamedTypeRef{RefAlias: ustr.PrefixWithSep(me.TypePkg, ".", me.TypeName)})
	}
	return &me.irANamedTypeRef
}

func (me *irAToType) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAToType)
	return (me == nil && c == nil) || (me != nil && c != nil && me.TypePkg == c.TypePkg && me.TypeName == c.TypeName && me.ExprToConv.Equiv(c.ExprToConv))
}

type irAPkgSym struct {
	irABase
	PkgName string
	Symbol  string
}

func (me *irAPkgSym) Equiv(cmp irA) bool {
	c, _ := cmp.(*irAPkgSym)
	return (me == nil && c == nil) || (me != nil && c != nil && me.PkgName == c.PkgName && me.Symbol == c.Symbol)
}

func (me *irAPkgSym) ExprType() *irANamedTypeRef {
	if !me.hasTypeInfo() {
		if mod := findModuleByPName(me.PkgName); mod != nil {
			if ref := mod.irMeta.goValDeclByGoName(me.Symbol); ref != nil {
				me.copyTypeInfoFrom(ref)
			}
		}
		if !me.hasTypeInfo() {
			me.copyTypeInfoFrom(&irANamedTypeRef{RefAlias: ustr.PrefixWithSep(me.PkgName, ".", me.Symbol)})
		}
	}
	return &me.irANamedTypeRef
}

func (me *irAPkgSym) symStr() string {
	return me.PkgName + "ꓸ" + me.Symbol
}

func (me *irAst) typeCtorFunc(nameps string) *irACtor {
	for _, tcf := range me.culled.typeCtorFuncs {
		if tcf.NamePs == nameps {
			return tcf
		}
	}
	return nil
}

func (me *irAst) writeAsJsonTo(w io.Writer) error {
	jsonenc := json.NewEncoder(w)
	jsonenc.SetIndent("", "\t")
	return jsonenc.Encode(me)
}

func (me *irAst) writeAsGoTo(writer io.Writer) (err error) {
	var buf = &bytes.Buffer{}

	sort.Sort(me.irM.GoTypeDefs)
	for _, gtd := range me.irM.GoTypeDefs {
		me.codeGenTypeDef(buf, gtd)
		me.codeGenStructMethods(buf, gtd)
	}

	toplevelconsts := me.topLevelDefs(func(a irA) bool { ac, _ := a.(*irAConst); return ac != nil })
	toplevelvars := me.topLevelDefs(func(a irA) bool { al, _ := a.(*irALet); return al != nil })
	me.codeGenGroupedVals(buf, true, toplevelconsts)
	me.codeGenGroupedVals(buf, false, toplevelvars)

	toplevelfuncs := me.topLevelDefs(func(a irA) bool { af, _ := a.(*irAFunc); return af != nil })
	for _, ast := range toplevelfuncs {
		me.codeGenAst(buf, 0, ast)
		fmt.Fprint(buf, "\n\n")
	}

	if err = me.codeGenPkgDecl(writer); err == nil {
		if err = me.codeGenModImps(writer); err == nil {
			_, err = buf.WriteTo(writer)
		}
	}
	return
}
