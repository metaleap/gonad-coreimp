package main

import (
	"fmt"
	"strings"
	"unicode"

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
	switch body := me.ciAstToGIrAst().(type) {
	case *GIrABlock:
		into.Body = body.Body
	default:
		into.Body = append(into.Body, body)
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
		a = &GIrALitStr{LitStr: me.StringLiteral}
	case "BooleanLiteral":
		a = &GIrALitBool{LitBool: me.BooleanLiteral}
	case "NumericLiteral_Double":
		a = &GIrALitDouble{LitDouble: me.NumericLiteral_Double}
	case "NumericLiteral_Integer":
		a = &GIrALitInt{LitInt: me.NumericLiteral_Integer}
	case "Var":
		v := &GIrAVar{}
		if gvd := me.root.mod.girMeta.GoValDeclByPsName(me.Var); gvd != nil {
			v.Export = true
		} else if unicode.IsUpper(ustr.FirstRune(me.Var)) {
			v.WasTypeFunc = true
		}
		v.setBothNamesFromPsName(me.Var)
		a = v
	case "Block":
		b := &GIrABlock{}
		for _, c := range me.Block {
			b.Body = append(b.Body, c.ciAstToGIrAst())
		}
		a = b
	case "While":
		f := &GIrAFor{ForDo: &GIrABlock{}}
		f.ForCond = me.While.ciAstToGIrAst()
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "ForIn":
		f := &GIrAFor{ForDo: &GIrABlock{}}
		f.ForRange = &GIrAVar{}
		f.ForRange.setBothNamesFromPsName(me.ForIn)
		f.ForRange.VarVal = me.AstFor1.ciAstToGIrAst()
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "For":
		f, fv := &GIrAFor{ForDo: &GIrABlock{}}, &GIrAVar{}
		fv.setBothNamesFromPsName(me.For)
		f.ForInit = []*GIrASet{&GIrASet{
			SetLeft: fv, ToRight: me.AstFor1.ciAstToGIrAst()}}
		f.ForCond = &GIrAOp2{Left: fv, Op2: "<", Right: me.AstFor2.ciAstToGIrAst()}
		f.ForStep = []*GIrASet{&GIrASet{SetLeft: fv, ToRight: &GIrAOp2{Left: fv, Op2: "+", Right: &GIrALitInt{LitInt: 1}}}}
		me.AstBody.astForceIntoBlock(f.ForDo)
		a = f
	case "IfElse":
		i := &GIrAIf{If: me.IfElse.ciAstToGIrAst(), Then: &GIrABlock{}}
		me.AstThen.astForceIntoBlock(i.Then)
		if me.AstElse != nil {
			i.Else = &GIrABlock{}
			me.AstElse.astForceIntoBlock(i.Else)
		}
		a = i
	case "App":
		c := &GIrACall{Callee: me.App.ciAstToGIrAst()}
		for _, arg := range me.AstApplArgs {
			c.CallArgs = append(c.CallArgs, arg.ciAstToGIrAst())
		}
		a = c
	case "VariableIntroduction":
		v := &GIrAVar{}
		if istopleveldecl {
			if unicode.IsUpper(ustr.FirstRune(me.VariableIntroduction)) {
				v.WasTypeFunc = true
			}
			if gvd := me.root.mod.girMeta.GoValDeclByPsName(me.VariableIntroduction); gvd != nil {
				v.Export = true
			}
		}
		v.setBothNamesFromPsName(me.VariableIntroduction)
		if me.AstRight != nil {
			v.VarVal = me.AstRight.ciAstToGIrAst()
		}
		a = v
	case "Function":
		f := &GIrAFunc{FuncImpl: &GIrABlock{}}
		if istopleveldecl && len(me.Function) > 0 {
			if unicode.IsUpper(ustr.FirstRune(me.Function)) {
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
		o := &GIrAOp1{Op1: me.AstOp, Of: me.Unary.ciAstToGIrAst()}
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
		o := &GIrAOp2{Op2: me.AstOp, Left: me.Binary.ciAstToGIrAst(), Right: me.AstRight.ciAstToGIrAst()}
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
		c := &GIrAComments{}
		for _, comment := range me.Comment {
			if comment != nil {
				c.Comments = append(c.Comments, *comment)
			}
		}
		if me.AstCommentDecl != nil {
			c.CommentsDecl = me.AstCommentDecl.ciAstToGIrAst()
		}
		a = c
	case "ObjectLiteral":
		o := &GIrALitObj{}
		for _, namevaluepair := range me.ObjectLiteral {
			for onekey, oneval := range namevaluepair {
				v := &GIrALitObjField{FieldVal: oneval.ciAstToGIrAst()}
				v.setBothNamesFromPsName(onekey)
				o.ObjFields = append(o.ObjFields, v)
				break
			}
		}
		a = o
	case "ReturnNoResult":
		r := &GIrARet{}
		a = r
	case "Return":
		r := &GIrARet{RetArg: me.Return.ciAstToGIrAst()}
		a = r
	case "Throw":
		r := &GIrAPanic{PanicArg: me.Throw.ciAstToGIrAst()}
		a = r
	case "ArrayLiteral":
		l := &GIrALitArr{}
		for _, v := range me.ArrayLiteral {
			l.ArrVals = append(l.ArrVals, v.ciAstToGIrAst())
		}
		a = l
	case "Assignment":
		o := &GIrASet{SetLeft: me.Assignment.ciAstToGIrAst(), ToRight: me.AstRight.ciAstToGIrAst()}
		a = o
	case "Indexer":
		if me.AstRight.AstTag == "StringLiteral" { // TODO will need to differentiate better between a real property or an obj-dict-key
			a = &GIrADot{DotLeft: me.Indexer.ciAstToGIrAst(), DotRight: ªV(me.AstRight.StringLiteral)}
		} else {
			a = &GIrAIndex{IdxLeft: me.Indexer.ciAstToGIrAst(), IdxRight: me.AstRight.ciAstToGIrAst()}
		}
	case "InstanceOf":
		a = &GIrAIsType{ExprToTest: me.InstanceOf.ciAstToGIrAst(), TypeToTest: me.AstRight.ciAstToGIrAst()}
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
