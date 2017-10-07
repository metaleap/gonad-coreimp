package main

import (
	"fmt"
)

type CoreImp struct {
	BuiltWith string      `json:"builtWith,omitempty"`
	Imports   []string    `json:"imports,omitempty"`
	Exports   []string    `json:"exports,omitempty"`
	Foreign   []string    `json:"foreign,omitempty"`
	Body      CoreImpAsts `json:"body,omitempty"`

	namedRequires map[string]string
}

type CoreImpAsts []*CoreImpAst

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
}

func (me *CoreImpAst) astForceIntoBlock(into *GIrABlock) {
	switch body := me.ciAstToGIrAst().(type) {
	case GIrABlock:
		*into = body
	default:
		into.Body = append(into.Body, body)
	}
}

func (cia *CoreImpAst) ciAstToGIrAst() (a GIrA) {
	// istopleveldecl := cia.parent == nil
	switch cia.AstTag {
	case "StringLiteral":
		a = GIrALitStr{LitStr: cia.StringLiteral}
	case "BooleanLiteral":
		a = GIrALitBool{LitBool: cia.BooleanLiteral}
	case "NumericLiteral_Double":
		a = GIrALitDouble{LitDouble: cia.NumericLiteral_Double}
	case "NumericLiteral_Integer":
		a = GIrALitInt{LitInt: cia.NumericLiteral_Integer}
	case "Var":
		v := GIrAVar{}
		v.setBothNamesFromPsName(cia.Var)
		a = v
	case "Block":
		b := GIrABlock{}
		for _, c := range cia.Block {
			b.Body = append(b.Body, c.ciAstToGIrAst())
		}
		a = b
	case "While":
		f := GIrAFor{}
		f.ForCond = cia.While.ciAstToGIrAst()
		cia.AstBody.astForceIntoBlock(&f.GIrABlock)
		a = f
	case "ForIn":
		f := GIrAFor{}
		f.ForRange = GIrAVar{}
		f.ForRange.setBothNamesFromPsName(cia.ForIn)
		f.ForRange.VarVal = cia.AstFor1.ciAstToGIrAst()
		cia.AstBody.astForceIntoBlock(&f.GIrABlock)
		a = f
	case "For":
		f, fv := GIrAFor{}, GIrAVar{}
		fv.setBothNamesFromPsName(cia.For)
		f.ForInit = []GIrASet{{
			Left: fv, Right: cia.AstFor1.ciAstToGIrAst()}}
		f.ForCond = GIrAOp2{Left: fv, Op2: "<", Right: cia.AstFor2.ciAstToGIrAst()}
		f.ForStep = []GIrASet{{Left: fv, Right: GIrAOp2{Left: fv, Op2: "+", Right: GIrALitInt{LitInt: 1}}}}
		cia.AstBody.astForceIntoBlock(&f.GIrABlock)
		a = f
	case "IfElse":
		i := GIrAIf{If: cia.IfElse.ciAstToGIrAst()}
		cia.AstThen.astForceIntoBlock(&i.Then)
		if cia.AstElse != nil {
			cia.AstElse.astForceIntoBlock(&i.Else)
		}
		a = i
	case "App":
		c := GIrACall{Callee: cia.App.ciAstToGIrAst()}
		for _, arg := range cia.AstApplArgs {
			c.CallArgs = append(c.CallArgs, arg.ciAstToGIrAst())
		}
		a = c
	case "Function":
		f := GIrAFunc{}
		f.setBothNamesFromPsName(cia.Function)
		f.RefFunc = &GIrATypeRefFunc{}
		for _, fpn := range cia.AstFuncParams {
			arg := &GIrANamedTypeRef{}
			arg.setBothNamesFromPsName(fpn)
			f.RefFunc.Args = append(f.RefFunc.Args, arg)
		}
		cia.AstBody.astForceIntoBlock(&f.GIrABlock)
		a = f
	case "Unary":
		o := GIrAOp1{Op1: cia.AstOp, Right: cia.Unary.ciAstToGIrAst()}
		switch o.Op1 {
		case "Negate":
			o.Op1 = "-"
		case "Not":
			o.Op1 = "!"
		case "Positive":
			o.Op1 = "+"
		case "BitwiseNot":
			o.Op1 = "^"
		default:
			if o.Op1 != "New" {
				panic("unrecognized unary op '" + o.Op1 + "', please report!")
			}
			o.Op1 = "?" + o.Op1 + "?"
		}
		a = o
	case "Binary":
		o := GIrAOp2{Op2: cia.AstOp, Left: cia.Binary.ciAstToGIrAst(), Right: cia.AstRight.ciAstToGIrAst()}
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
	case "VariableIntroduction":
		c := GIrAVar{}
		c.setBothNamesFromPsName(cia.VariableIntroduction)
		if cia.AstRight != nil {
			c.VarVal = cia.AstRight.ciAstToGIrAst()
		}
		a = c
	case "Comment":
		c := GIrAComments{}
		for _, comment := range cia.Comment {
			if comment != nil {
				c.Comments = append(c.Comments, *comment)
			}
		}
		if cia.AstCommentDecl != nil {
			c.CommentsDecl = cia.AstCommentDecl.ciAstToGIrAst()
		}
		a = c
	case "ObjectLiteral":
		o := GIrALitObj{}
		for _, namevaluepair := range cia.ObjectLiteral {
			for onekey, oneval := range namevaluepair {
				v := GIrAVar{VarVal: oneval.ciAstToGIrAst()}
				v.setBothNamesFromPsName(onekey)
				o.ObjPairs = append(o.ObjPairs, v)
				break
			}
		}
		a = o
	case "ReturnNoResult":
		r := GIrARet{}
		a = r
	case "Return":
		r := GIrARet{RetArg: cia.Return.ciAstToGIrAst()}
		a = r
	case "Throw":
		r := GIrAPanic{PanicArg: cia.Throw.ciAstToGIrAst()}
		a = r
	case "ArrayLiteral":
		l := GIrALitArr{}
		for _, v := range cia.ArrayLiteral {
			l.ArrVals = append(l.ArrVals, v.ciAstToGIrAst())
		}
		a = l
	case "Assignment":
		o := GIrASet{Left: cia.Assignment.ciAstToGIrAst(), Right: cia.AstRight.ciAstToGIrAst()}
		a = o
	case "Indexer":
		if cia.AstRight.AstTag == "StringLiteral" { // TODO will need to differentiate better between a real property or an obj-dict-key
			a = GIrADot{DotLeft: cia.Indexer.ciAstToGIrAst(), DotRight: cia.AstRight.ciAstToGIrAst()}
		} else {
			a = GIrAIndex{IdxLeft: cia.Indexer.ciAstToGIrAst(), IdxRight: cia.AstRight.ciAstToGIrAst()}
		}
	case "InstanceOf":
		a = GIrAIsType{ExprToTest: cia.InstanceOf.ciAstToGIrAst(), TypeToTest: cia.AstRight.ciAstToGIrAst()}
	default:
		panic(fmt.Errorf("Just below %s: unrecognized CoreImp AST-tag, please report: %s", cia.parent, cia.AstTag))
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
	me.setParents(nil, me.Body...)
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
		} else if a.AstTag == "Comment" {
			if a.AstCommentDecl != nil {
				decl := a.AstCommentDecl
				a.AstCommentDecl = nil
				putdeclnexttocomment := append(me.Body[:i+1], decl)
				everythingelse := me.Body[i+1:]
				me.Body = append(putdeclnexttocomment, everythingelse...)
			}
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
		} else {
			return fmt.Errorf("Encountered unexpected top-level AST tag, please report: %s", a.AstTag)
		}
	}
	return nil
}

func (me *CoreImp) setParents(parent *CoreImpAst, asts ...*CoreImpAst) {
	for _, a := range asts {
		if a != nil {
			a.parent = parent
			me.setParents(a, a.App)
			me.setParents(a, a.ArrayLiteral...)
			me.setParents(a, a.Assignment)
			me.setParents(a, a.AstApplArgs...)
			me.setParents(a, a.AstBody)
			me.setParents(a, a.AstCommentDecl)
			me.setParents(a, a.AstFor1)
			me.setParents(a, a.AstFor2)
			me.setParents(a, a.AstElse)
			me.setParents(a, a.AstThen)
			me.setParents(a, a.AstRight)
			me.setParents(a, a.Binary)
			me.setParents(a, a.Block...)
			me.setParents(a, a.IfElse)
			me.setParents(a, a.Indexer)
			me.setParents(a, a.Assignment)
			me.setParents(a, a.InstanceOf)
			me.setParents(a, a.Return)
			me.setParents(a, a.Throw)
			me.setParents(a, a.Unary)
			me.setParents(a, a.While)
			for _, m := range a.ObjectLiteral {
				for _, expr := range m {
					me.setParents(a, expr)
				}
			}
		}
	}
}
