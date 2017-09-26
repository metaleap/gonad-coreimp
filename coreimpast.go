package main

type CoreImp struct {
	BuiltWith string           `json:"builtWith,omitempty"`
	Imports   []string         `json:"imports,omitempty"`
	Exports   []string         `json:"exports,omitempty"`
	Foreign   []string         `json:"foreign,omitempty"`
	Ast       []CoreImpAstItem `json:"ast,omitempty"`
}

type CoreImpAstItem struct {
	Tag      string        `json:"ast"`
	Contents []interface{} `json:"contents,omitempty"`
}
