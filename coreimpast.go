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

func (me *CoreImpAsts) Add(asts ...*CoreImpAst) {
	(*me) = append(*me, asts...)
}

func (me CoreImpAsts) Last() *CoreImpAst {
	if l := len(me); l > 0 {
		return me[l-1]
	}
	return nil
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
	NumericLiteral_Integer int64                    `json:",omitempty"`
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

func (me *CoreImpAst) IsFunction() bool               { return me.AstTag == "Function" }
func (me *CoreImpAst) IsStringLiteral() bool          { return me.AstTag == "StringLiteral" }
func (me *CoreImpAst) IsBooleanLiteral() bool         { return me.AstTag == "BooleanLiteral" }
func (me *CoreImpAst) IsNumericLiteral_Integer() bool { return me.AstTag == "NumericLiteral_Integer" }
func (me *CoreImpAst) IsNumericLiteral_Double() bool  { return me.AstTag == "NumericLiteral_Double" }
func (me *CoreImpAst) IsBlock() bool                  { return me.AstTag == "Block" }
func (me *CoreImpAst) IsVar() bool                    { return me.AstTag == "Var" }
func (me *CoreImpAst) IsVariableIntroduction() bool   { return me.AstTag == "VariableIntroduction" }
func (me *CoreImpAst) IsWhile() bool                  { return me.AstTag == "While" }
func (me *CoreImpAst) IsApp() bool                    { return me.AstTag == "App" }
func (me *CoreImpAst) IsUnary() bool                  { return me.AstTag == "Unary" }
func (me *CoreImpAst) IsComment() bool                { return me.AstTag == "Comment" }
func (me *CoreImpAst) IsBinary() bool                 { return me.AstTag == "Binary" }
func (me *CoreImpAst) IsForIn() bool                  { return me.AstTag == "ForIn" }
func (me *CoreImpAst) IsFor() bool                    { return me.AstTag == "For" }
func (me *CoreImpAst) IsIfElse() bool                 { return me.AstTag == "IfElse" }
func (me *CoreImpAst) IsObjectLiteral() bool          { return me.AstTag == "ObjectLiteral" }
func (me *CoreImpAst) IsReturn() bool                 { return me.AstTag == "Return" }
func (me *CoreImpAst) IsThrow() bool                  { return me.AstTag == "Throw" }
func (me *CoreImpAst) IsArrayLiteral() bool           { return me.AstTag == "ArrayLiteral" }
func (me *CoreImpAst) IsAssignment() bool             { return me.AstTag == "Assignment" }
func (me *CoreImpAst) IsIndexer() bool                { return me.AstTag == "Indexer" }
func (me *CoreImpAst) IsAccessor() bool               { return me.AstTag == "Accessor" }
func (me *CoreImpAst) IsInstanceOf() bool             { return me.AstTag == "InstanceOf" }

type CoreImpComment struct {
	LineComment  string
	BlockComment string
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
		} else if a.IsComment() {
			if a.AstCommentDecl != nil {
				decl := a.AstCommentDecl
				a.AstCommentDecl = nil
				putdeclnexttocomment := append(me.Body[:i+1], decl)
				everythingelse := me.Body[i+1:]
				me.Body = append(putdeclnexttocomment, everythingelse...)
			}
		} else if a.IsVariableIntroduction() {
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

func ſDot(left *CoreImpAst, right string) *CoreImpAst {
	return &CoreImpAst{AstTag: "Accessor", Accessor: left, AstRight: ſV(right)}
}

func ſEq(left *CoreImpAst, right *CoreImpAst) *CoreImpAst {
	return ſO2(left, "==", right)
}

func ſO1(op string, operand *CoreImpAst) *CoreImpAst {
	return &CoreImpAst{AstOp: op, AstTag: "Unary", Unary: operand}
}

func ſO2(left *CoreImpAst, op string, right *CoreImpAst) *CoreImpAst {
	return &CoreImpAst{AstOp: op, AstTag: "Binary", Binary: left, AstRight: right}
}

func ſRet(expr *CoreImpAst) *CoreImpAst {
	if expr == nil {
		return &CoreImpAst{AstTag: "ReturnNoResult"}
	}
	return &CoreImpAst{AstTag: "Return", Return: expr}
}

func ſS(literal string) *CoreImpAst {
	return &CoreImpAst{AstTag: "StringLiteral", StringLiteral: literal}
}

func ſSet(left string, right *CoreImpAst) *CoreImpAst {
	return &CoreImpAst{AstTag: "Assignment", Assignment: ſV(left), AstRight: right}
}

func ſV(name string) *CoreImpAst {
	return &CoreImpAst{AstTag: "Var", Var: name}
}
