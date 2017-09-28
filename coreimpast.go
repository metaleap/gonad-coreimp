package main

type CoreImp struct {
	BuiltWith string        `json:"builtWith,omitempty"`
	Imports   []string      `json:"imports,omitempty"`
	Exports   []string      `json:"exports,omitempty"`
	Foreign   []string      `json:"foreign,omitempty"`
	Body      []*CoreImpAst `json:"body,omitempty"`
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
