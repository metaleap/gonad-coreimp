package main

import (
	"fmt"
	"strings"

	"github.com/metaleap/go-util-str"
)

var (
	strunprimer = strings.NewReplacer("$prime", "'")
)

type CoreImp struct {
	BuiltWith string      `json:"builtWith,omitempty"`
	Imports   []string    `json:"imports,omitempty"`
	Exports   []string    `json:"exports,omitempty"`
	Foreign   []string    `json:"foreign,omitempty"`
	Body      CoreImpAsts `json:"body,omitempty"`

	namedRequires map[string]string
	mod           *ModuleInfo
}

type CoreImpAsts []*CoreImpAst

func (me *CoreImpAsts) InsertAt(cia *CoreImpAst, at int) {
	sl := *me
	tail := append(CoreImpAsts{}, sl[at:]...)
	prep := append(sl[:at], cia)
	sl = append(prep, tail...)
	*me = sl
}

type CoreImpAst struct {
	AstSourceSpan  *CoreImpSourceSpan `json:"sourceSpan,omitempty"`
	AstTag         string             `json:"tag,omitempty"`
	AstBody        *CoreImpAst        `json:"body,omitempty"`
	AstRight       *CoreImpAst        `json:"rhs,omitempty"`
	AstCommentDecl *CoreImpAst        `json:"decl,omitempty"`
	AstApplArgs    CoreImpAsts        `json:"args,omitempty"`
	AstOp          string             `json:"op,omitempty"`
	AstFuncParams  []string           `json:"params,omitempty"`
	AstFor1        *CoreImpAst        `json:"for1,omitempty"`
	AstFor2        *CoreImpAst        `json:"for2,omitempty"`
	AstThen        *CoreImpAst        `json:"then,omitempty"`
	AstElse        *CoreImpAst        `json:"else,omitempty"`

	Function               string                   `json:",omitempty"`
	StringLiteral          string                   `json:",omitempty"`
	BooleanLiteral         bool                     `json:",omitempty"`
	NumericLiteral_Integer int                      `json:",omitempty"`
	NumericLiteral_Double  float64                  `json:",omitempty"`
	Block                  CoreImpAsts              `json:",omitempty"`
	Var                    string                   `json:",omitempty"`
	VariableIntroduction   string                   `json:",omitempty"`
	While                  *CoreImpAst              `json:",omitempty"`
	App                    *CoreImpAst              `json:",omitempty"`
	Unary                  *CoreImpAst              `json:",omitempty"`
	Comment                []*CoreImpComment        `json:",omitempty"`
	Binary                 *CoreImpAst              `json:",omitempty"`
	ForIn                  string                   `json:",omitempty"`
	For                    string                   `json:",omitempty"`
	IfElse                 *CoreImpAst              `json:",omitempty"`
	ObjectLiteral          []map[string]*CoreImpAst `json:",omitempty"`
	Return                 *CoreImpAst              `json:",omitempty"`
	Throw                  *CoreImpAst              `json:",omitempty"`
	ArrayLiteral           CoreImpAsts              `json:",omitempty"`
	Assignment             *CoreImpAst              `json:",omitempty"`
	Indexer                *CoreImpAst              `json:",omitempty"`
	Accessor               *CoreImpAst              `json:",omitempty"`
	InstanceOf             *CoreImpAst              `json:",omitempty"`

	parent *CoreImpAst
	root   *CoreImp
}

func (me *CoreImpAst) astForceIntoBlock(into *GIrABlock) {
	switch maybebody := me.ciAstToGIrAst().(type) {
	case *GIrABlock:
		into.Body = maybebody.Body
		for _, a := range into.Body {
			a.Base().parent = into
		}
	default:
		into.Add(maybebody)
	}
}

func (me *CoreImpAst) ciAstToGIrAst() (a GIrA) {
	istopleveldecl := (me.parent == nil)
	if maybecomment := me.parent; maybecomment != nil {
		for maybecomment.AstTag == "Comment" {
			if maybecomment = maybecomment.parent; maybecomment == nil {
				istopleveldecl = true
				break
			}
		}
	}
	switch me.AstTag {
	case "StringLiteral":
		a = ªS(me.StringLiteral)
	case "BooleanLiteral":
		a = ªB(me.BooleanLiteral)
	case "NumericLiteral_Double":
		a = ªF(me.NumericLiteral_Double)
	case "NumericLiteral_Integer":
		a = ªI(me.NumericLiteral_Integer)
	case "Var":
		v := ªVar("", me.Var, nil)
		if gvd := me.root.mod.girMeta.GoValDeclByPsName(me.Var); gvd != nil {
			v.Export = true
		}
		if ustr.BeginsUpper(me.Var) {
			v.WasTypeFunc = true
		}
		a = v
	case "Block":
		b := ªBlock()
		for _, c := range me.Block {
			b.Add(c.ciAstToGIrAst())
		}
		a = b
	case "While":
		f := ªFor()
		f.ForCond = me.While.ciAstToGIrAst()
		f.ForCond.Base().parent = f
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "ForIn":
		f := ªFor()
		f.ForRange = ªVar("", me.ForIn, me.AstFor1.ciAstToGIrAst())
		f.ForRange.parent = f
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "For":
		var fv GIrAVar
		f := ªFor()
		fv.setBothNamesFromPsName(me.For)
		fv1, fv2, fv3, fv4 := fv, fv, fv, fv // quirky that we need these 4 copies but we do
		f.ForInit = []*GIrASet{ªSet(&fv1, me.AstFor1.ciAstToGIrAst())}
		f.ForInit[0].parent = f
		f.ForCond = ªO2(&fv2, "<", me.AstFor2.ciAstToGIrAst())
		f.ForCond.Base().parent = f
		f.ForStep = []*GIrASet{ªSet(&fv3, ªO2(&fv4, "+", ªI(1)))}
		f.ForStep[0].parent = f
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "IfElse":
		i := ªIf(me.IfElse.ciAstToGIrAst())
		me.AstThen.astForceIntoBlock(i.Then)
		if me.AstElse != nil {
			i.Else = ªBlock()
			me.AstElse.astForceIntoBlock(i.Else)
			i.Else.parent = i
		}
		a = i
	case "App":
		c := ªCall(me.App.ciAstToGIrAst())
		for _, carg := range me.AstApplArgs {
			arg := carg.ciAstToGIrAst()
			arg.Base().parent = c
			c.CallArgs = append(c.CallArgs, arg)
		}
		a = c
	case "VariableIntroduction":
		v := ªVar("", me.VariableIntroduction, nil)
		if istopleveldecl {
			if ustr.BeginsUpper(me.VariableIntroduction) {
				v.WasTypeFunc = true
			}
			if gvd := me.root.mod.girMeta.GoValDeclByPsName(me.VariableIntroduction); gvd != nil {
				v.Export = true
			}
		}
		if me.AstRight != nil {
			v.VarVal = me.AstRight.ciAstToGIrAst()
			v.VarVal.Base().parent = v
		}
		a = v
	case "Function":
		f := ªFunc()
		if istopleveldecl && len(me.Function) > 0 {
			if ustr.BeginsUpper(me.Function) {
				f.WasTypeFunc = true
			}
			if gvd := me.root.mod.girMeta.GoValDeclByPsName(me.Function); gvd != nil {
				f.Export = true
			}
		}
		f.setBothNamesFromPsName(me.Function)
		f.RefFunc = &GIrATypeRefFunc{}
		for _, fpn := range me.AstFuncParams {
			arg := &GIrANamedTypeRef{}
			arg.setBothNamesFromPsName(fpn)
			f.RefFunc.Args = append(f.RefFunc.Args, arg)
		}
		me.AstBody.astForceIntoBlock(f.FuncImpl)
		f.method.body = f.FuncImpl
		a = f
	case "Unary":
		o := ªO1(me.AstOp, me.Unary.ciAstToGIrAst())
		switch o.Op1 {
		case "Negate":
			o.Op1 = "-"
		case "Not":
			o.Op1 = "!"
		case "Positive":
			o.Op1 = "+"
		case "BitwiseNot":
			o.Op1 = "^"
		case "New":
			o.Op1 = "&"
		default:
			panic("unrecognized unary op '" + o.Op1 + "', please report!")
			o.Op1 = "?" + o.Op1 + "?"
		}
		a = o
	case "Binary":
		o := ªO2(me.Binary.ciAstToGIrAst(), me.AstOp, me.AstRight.ciAstToGIrAst())
		switch o.Op2 {
		case "Add":
			o.Op2 = "+"
		case "Subtract":
			o.Op2 = "-"
		case "Multiply":
			o.Op2 = "*"
		case "Divide":
			o.Op2 = "/"
		case "Modulus":
			o.Op2 = "%"
		case "EqualTo":
			o.Op2 = "=="
		case "NotEqualTo":
			o.Op2 = "!="
		case "LessThan":
			o.Op2 = "<"
		case "LessThanOrEqualTo":
			o.Op2 = "<="
		case "GreaterThan":
			o.Op2 = ">"
		case "GreaterThanOrEqualTo":
			o.Op2 = ">="
		case "And":
			o.Op2 = "&&"
		case "Or":
			o.Op2 = "||"
		case "BitwiseAnd":
			o.Op2 = "&"
		case "BitwiseOr":
			o.Op2 = "|"
		case "BitwiseXor":
			o.Op2 = "^"
		case "ShiftLeft":
			o.Op2 = "<<"
		case "ShiftRight":
			o.Op2 = ">>"
		case "ZeroFillShiftRight":
			o.Op2 = "&^"
		default:
			o.Op2 = "?" + o.Op2 + "?"
			panic("unrecognized binary op '" + o.Op2 + "', please report!")
		}
		a = o
	case "Comment":
		c := ªComments(me.Comment...)
		if me.AstCommentDecl != nil {
			c.CommentsDecl = me.AstCommentDecl.ciAstToGIrAst()
			c.CommentsDecl.Base().parent = c
		}
		a = c
	case "ObjectLiteral":
		o := ªO(nil)
		for _, namevaluepair := range me.ObjectLiteral {
			for onekey, oneval := range namevaluepair {
				ofv := ªOFld(oneval.ciAstToGIrAst())
				ofv.setBothNamesFromPsName(onekey)
				ofv.parent = o
				o.ObjFields = append(o.ObjFields, ofv)
				break
			}
		}
		a = o
	case "ReturnNoResult":
		r := ªRet(nil)
		a = r
	case "Return":
		r := ªRet(me.Return.ciAstToGIrAst())
		a = r
	case "Throw":
		r := ªPanic(me.Throw.ciAstToGIrAst())
		a = r
	case "ArrayLiteral":
		exprs := make([]GIrA, 0, len(me.ArrayLiteral))
		for _, v := range me.ArrayLiteral {
			exprs = append(exprs, v.ciAstToGIrAst())
		}
		l := ªA(exprs...)
		a = l
	case "Assignment":
		o := ªSet(me.Assignment.ciAstToGIrAst(), me.AstRight.ciAstToGIrAst())
		a = o
	case "Indexer":
		if me.AstRight.AstTag == "StringLiteral" { // TODO will need to differentiate better between a real property or an obj-dict-key
			dv := ªVar("", me.AstRight.StringLiteral, nil)
			a = ªDot(me.Indexer.ciAstToGIrAst(), dv)
		} else {
			a = ªIndex(me.Indexer.ciAstToGIrAst(), me.AstRight.ciAstToGIrAst())
		}
	case "InstanceOf":
		a = ªIs(me.InstanceOf.ciAstToGIrAst(), me.AstRight.ciAstToGIrAst())
	default:
		panic(fmt.Errorf("Just below %v: unrecognized CoreImp AST-tag, please report: %s", me.parent, me.AstTag))
	}
	return
}

type CoreImpComment struct {
	LineComment  string `json:",omitempty"`
	BlockComment string `json:",omitempty"`
}

type CoreImpSourceSpan struct {
	Name  string `json:"name,omitempty"`
	Start []int  `json:"start,omitempty"`
	End   []int  `json:"end,omitempty"`
}

func (me *CoreImp) preProcessTopLevel() error {
	me.namedRequires = map[string]string{}
	me.preProcessAsts(nil, me.Body...)
	i := 0
	ditch := func() {
		me.Body = append(me.Body[:i], me.Body[i+1:]...)
		i -= 1
	}
	for i = 0; i < len(me.Body); i++ {
		a := me.Body[i]
		if a.StringLiteral == "use strict" {
			//	"use strict"
			ditch()
		} else if a.Assignment != nil && a.Assignment.Indexer != nil && a.Assignment.Indexer.Var == "module" && a.Assignment.AstRight != nil && a.Assignment.AstRight.StringLiteral == "exports" {
			//	module.exports = ..
			ditch()
		} else if a.AstTag == "VariableIntroduction" {
			if a.AstRight != nil && a.AstRight.App != nil && a.AstRight.App.Var == "require" && len(a.AstRight.AstApplArgs) == 1 {
				// println("Dropped top-level require()" )
				me.namedRequires[a.VariableIntroduction] = a.AstRight.AstApplArgs[0].StringLiteral
				ditch()
			} else if a.AstRight != nil && a.AstRight.AstTag == "Function" {
				// turn top-level `var foo = func()` into `func foo()`
				a.AstRight.Function = a.VariableIntroduction
				a = a.AstRight
				a.parent, me.Body[i] = nil, a
			}
		} else if a.AstTag != "Function" && a.AstTag != "VariableIntroduction" && a.AstTag != "Comment" {
			return fmt.Errorf("Encountered unexpected top-level AST tag, please report: %s", a.AstTag)
		}
	}
	return nil
}

func (me *CoreImp) preProcessAsts(parent *CoreImpAst, asts ...*CoreImpAst) {
	if parent != nil {
		parent.root = me
	}
	for _, a := range asts {
		if a != nil {
			for _, sym := range []*string{&a.For, &a.ForIn, &a.Function, &a.Var, &a.VariableIntroduction} {
				if len(*sym) > 0 {
					*sym = strunprimer.Replace(*sym)
				}
			}
			for i, mkv := range a.ObjectLiteral {
				for onename, oneval := range mkv {
					if nuname := strunprimer.Replace(onename); nuname != onename {
						mkv = map[string]*CoreImpAst{}
						mkv[nuname] = oneval
						a.ObjectLiteral[i] = mkv
					}
				}
			}
			for i, afp := range a.AstFuncParams {
				a.AstFuncParams[i] = strunprimer.Replace(afp)
			}

			a.root = me
			a.parent = parent
			me.preProcessAsts(a, a.App)
			me.preProcessAsts(a, a.ArrayLiteral...)
			me.preProcessAsts(a, a.Assignment)
			me.preProcessAsts(a, a.AstApplArgs...)
			me.preProcessAsts(a, a.AstBody)
			me.preProcessAsts(a, a.AstCommentDecl)
			me.preProcessAsts(a, a.AstFor1)
			me.preProcessAsts(a, a.AstFor2)
			me.preProcessAsts(a, a.AstElse)
			me.preProcessAsts(a, a.AstThen)
			me.preProcessAsts(a, a.AstRight)
			me.preProcessAsts(a, a.Binary)
			me.preProcessAsts(a, a.Block...)
			me.preProcessAsts(a, a.IfElse)
			me.preProcessAsts(a, a.Indexer)
			me.preProcessAsts(a, a.Assignment)
			me.preProcessAsts(a, a.InstanceOf)
			me.preProcessAsts(a, a.Return)
			me.preProcessAsts(a, a.Throw)
			me.preProcessAsts(a, a.Unary)
			me.preProcessAsts(a, a.While)
			for _, m := range a.ObjectLiteral {
				for _, expr := range m {
					me.preProcessAsts(a, expr)
				}
			}
		}
	}
}
