package main

type CoreImp struct {
	BuiltWith string       `json:"builtWith,omitempty"`
	Imports   []string     `json:"imports,omitempty"`
	Exports   []string     `json:"exports,omitempty"`
	Foreign   []string     `json:"foreign,omitempty"`
	Body      []CoreImpAst `json:"body,omitempty"`
}

type CoreImpAst struct {
	SourceSpan     *CoreImpSourceSpan `json:"sourceSpan,omitempty"`
	_StringLiteral string
	Tag            string        `json:"ast"`
	Contents       []interface{} `json:"contents,omitempty"`
}

type CoreImpSourceSpan struct {
	Name  string `json:"name,omitempty"`
	Start []int  `json:"start,omitempty"`
	End   []int  `json:"end,omitempty"`
}
