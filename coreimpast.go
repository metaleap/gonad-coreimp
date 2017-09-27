package main

type CoreImp struct {
	BuiltWith string       `json:"builtWith,omitempty"`
	Imports   []string     `json:"imports,omitempty"`
	Exports   []string     `json:"exports,omitempty"`
	Foreign   []string     `json:"foreign,omitempty"`
	Body      []CoreImpAst `json:"body,omitempty"`
}

type CoreImpAst struct {
	sourceSpan    *CoreImpSourceSpan `json:"sourceSpan,omitempty"`
	tag           string             `json:"tag"`
	rightHandSide *CoreImpAst        `json:"rhs,omitempty"`
	body          *CoreImpAst        `json:"body,omitempty"`
	decl          *CoreImpAst        `json:"body,omitempty"`
	appArgs       []*CoreImpAst      `json:"args,omitempty"`
	primOp        string             `json:"op,omitempty"`
	funcParams    []string           `json:"params,omitempty"`
	for1          *CoreImpAst        `json:"for1,omitempty"`
	for2          *CoreImpAst        `json:"for2,omitempty"`
	ifthen        *CoreImpAst        `json:"then,omitempty"`
	ifelse        *CoreImpAst        `json:"else,omitempty"`

	StringLiteral          string
	BooleanLiteral         bool
	NumericLiteral_Integer int64
	NumericLiteral_Double  float64
	Block                  []*CoreImpAst
	Var                    string
	VariableIntroduction   string
	While                  *CoreImpAst
	App                    *CoreImpAst
	Unary                  *CoreImpAst
	Comment                []*CoreImpComment
	Function               string
	Binary                 *CoreImpAst
	ForIn                  string
	For                    string
	IfElse                 *CoreImpAst
	ObjectLiteral          []map[string]*CoreImpAst
	Return                 *CoreImpAst
	Throw                  *CoreImpAst
	ArrayLiteral           []*CoreImpAst
	Assignment             *CoreImpAst
	Indexer                *CoreImpAst
	InstanceOf             *CoreImpAst
	// ReturnNoResult         *CoreImpVoid
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
