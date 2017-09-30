package main

type CoreImp struct {
	BuiltWith string        `json:"builtWith,omitempty"`
	Imports   []string      `json:"imports,omitempty"`
	Exports   []string      `json:"exports,omitempty"`
	Foreign   []string      `json:"foreign,omitempty"`
	Body      []*CoreImpAst `json:"body,omitempty"`

	namedRequires map[string]string
}

type CoreImpAst struct {
	Ast_sourceSpan    *CoreImpSourceSpan `json:"sourceSpan,omitempty"`
	Ast_tag           string             `json:"tag,omitempty"`
	Ast_body          *CoreImpAst        `json:"body,omitempty"`
	Ast_rightHandSide *CoreImpAst        `json:"rhs,omitempty"`
	Ast_decl          *CoreImpAst        `json:"decl,omitempty"`
	Ast_appArgs       []*CoreImpAst      `json:"args,omitempty"`
	Ast_op            string             `json:"op,omitempty"`
	Ast_funcParams    []string           `json:"params,omitempty"`
	Ast_for1          *CoreImpAst        `json:"for1,omitempty"`
	Ast_for2          *CoreImpAst        `json:"for2,omitempty"`
	Ast_ifThen        *CoreImpAst        `json:"then,omitempty"`
	Ast_ifElse        *CoreImpAst        `json:"else,omitempty"`

	Function               string                   `json:",omitempty"`
	StringLiteral          string                   `json:",omitempty"`
	BooleanLiteral         bool                     `json:",omitempty"`
	NumericLiteral_Integer int64                    `json:",omitempty"`
	NumericLiteral_Double  float64                  `json:",omitempty"`
	Block                  []*CoreImpAst            `json:",omitempty"`
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
	ArrayLiteral           []*CoreImpAst            `json:",omitempty"`
	Assignment             *CoreImpAst              `json:",omitempty"`
	Indexer                *CoreImpAst              `json:",omitempty"`
	InstanceOf             *CoreImpAst              `json:",omitempty"`

	parent *CoreImpAst
}

type CoreImpComment struct {
	LineComment  string
	BlockComment string
}

type CoreImpSourceSpan struct {
	Name  string `json:"name,omitempty"`
	Start []int  `json:"start,omitempty"`
	End   []int  `json:"end,omitempty"`
}

func (me *CoreImp) preProcessTopLevel() {
	me.namedRequires = map[string]string{}
	me.setParents(nil, me.Body...)
	i := 0
	ditch := func() {
		me.Body = append(me.Body[:i], me.Body[i+1:]...)
		i -= 1
	}
	for i = 0; i < len(me.Body); i++ {
		a := me.Body[i]
		if a.StringLiteral == "use strict" || (a.Assignment != nil && a.Assignment.Indexer != nil && a.Assignment.Indexer.Var == "module" && a.Assignment.Ast_rightHandSide != nil && a.Assignment.Ast_rightHandSide.StringLiteral == "exports") {
			ditch()
		} else if a.Ast_tag == "Comment" && a.Ast_decl != nil {
			me.Body = append(append(me.Body[:i], a.Ast_decl), me.Body[i:]...)
			a.Ast_decl = nil
			me.Body[i] = a
		} else if a.Ast_tag == "VariableIntroduction" {
			if a.Ast_rightHandSide != nil && a.Ast_rightHandSide.App != nil && a.Ast_rightHandSide.App.Var == "require" && len(a.Ast_rightHandSide.Ast_appArgs) == 1 {
				// println("Dropped top-level require()" )
				me.namedRequires[a.VariableIntroduction] = a.Ast_rightHandSide.Ast_appArgs[0].StringLiteral
				ditch()
			} else if a.Ast_rightHandSide != nil && a.Ast_rightHandSide.Ast_tag == "Function" {
				// turn top-level `var foo = func()` into `func foo()`
				a.Ast_rightHandSide.Function = a.VariableIntroduction
				a = a.Ast_rightHandSide
				a.parent, me.Body[i] = nil, a
			}
		}
	}
}

func (me *CoreImp) setParents(parent *CoreImpAst, asts ...*CoreImpAst) {
	for _, a := range asts {
		if a != nil {
			a.parent = parent
			me.setParents(a, a.App)
			me.setParents(a, a.ArrayLiteral...)
			me.setParents(a, a.Assignment)
			me.setParents(a, a.Ast_appArgs...)
			me.setParents(a, a.Ast_body)
			me.setParents(a, a.Ast_decl)
			me.setParents(a, a.Ast_for1)
			me.setParents(a, a.Ast_for2)
			me.setParents(a, a.Ast_ifElse)
			me.setParents(a, a.Ast_ifThen)
			me.setParents(a, a.Ast_rightHandSide)
			me.setParents(a, a.Binary)
			me.setParents(a, a.Block...)
			me.setParents(a, a.IfElse)
			me.setParents(a, a.Indexer)
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
