package main

import (
	"fmt"
	"reflect"
)

/*
Golang intermediate-representation AST:
traversals of the abstract syntax tree
*/

func gIrALookupInAncestorBlocks(a gIrA, check func(gIrA) bool) gIrA {
	for nextparent := a.Parent(); nextparent != nil; nextparent = nextparent.Parent() {
		switch p := nextparent.(type) {
		case *gIrABlock:
			for _, stmt := range p.Body {
				if check(stmt) {
					return stmt
				}
			}
		}
	}
	return nil
}

func (me *gonadIrAst) topLevelDefs(okay func(gIrA) bool) (defs []gIrA) {
	for _, ast := range me.Body {
		if okay(ast) {
			defs = append(defs, ast)
		}
	}
	return
}

func (me *gonadIrAst) walk(on func(gIrA) gIrA) {
	for i, a := range me.Body {
		if a != nil {
			me.Body[i] = walk(a, on)
		}
	}
	for _, tr := range me.girM.GoTypeDefs {
		if tr.RefStruct != nil {
			for _, trm := range tr.RefStruct.Methods {
				if trm.RefFunc.impl != nil {
					trm.RefFunc.impl, _ = walk(trm.RefFunc.impl, on).(*gIrABlock)
				}
			}
		}
	}
	for i, tcf := range me.culled.typeCtorFuncs {
		me.culled.typeCtorFuncs[i] = walk(tcf, on).(*gIrACtor)
	}
}

func walk(ast gIrA, on func(gIrA) gIrA) gIrA {
	if ast != nil {
		switch a := ast.(type) {
		// why extra nil checks some places below: we do have the rare case of ast!=nil and ast.(type) set and still holding a null-ptr
		// why not everywhere: due to the nature of the ASTs constructed from coreimp, only those cases can potentially be nil if they exist at all
		case *gIrABlock:
			if a != nil {
				for i, _ := range a.Body {
					a.Body[i] = walk(a.Body[i], on)
				}
			}
		case *gIrACall:
			a.Callee = walk(a.Callee, on)
			for i, _ := range a.CallArgs {
				a.CallArgs[i] = walk(a.CallArgs[i], on)
			}
		case *gIrAConst:
			a.ConstVal = walk(a.ConstVal, on)
		case *gIrADot:
			a.DotLeft, a.DotRight = walk(a.DotLeft, on), walk(a.DotRight, on)
		case *gIrAFor:
			a.ForCond = walk(a.ForCond, on)
			if tmp, _ := walk(a.ForRange, on).(*gIrALet); tmp != nil {
				a.ForRange = tmp
			}
			if tmp, _ := walk(a.ForDo, on).(*gIrABlock); tmp != nil {
				a.ForDo = tmp
			}
			for i, fi := range a.ForInit {
				if tmp, _ := walk(fi, on).(*gIrALet); tmp != nil {
					a.ForInit[i] = tmp
				}
			}
			for i, fs := range a.ForStep {
				if tmp, _ := walk(fs, on).(*gIrASet); tmp != nil {
					a.ForStep[i] = tmp
				}
			}
		case *gIrACtor:
			if tmp, _ := walk(a.FuncImpl, on).(*gIrABlock); tmp != nil {
				a.FuncImpl = tmp
			}
		case *gIrAFunc:
			if tmp, _ := walk(a.FuncImpl, on).(*gIrABlock); tmp != nil {
				a.FuncImpl = tmp
			}
		case *gIrAIf:
			a.If = walk(a.If, on)
			if tmp, _ := walk(a.Then, on).(*gIrABlock); tmp != nil {
				a.Then = tmp
			}
			if tmp, _ := walk(a.Else, on).(*gIrABlock); tmp != nil {
				a.Else = tmp
			}
		case *gIrAIndex:
			a.IdxLeft, a.IdxRight = walk(a.IdxLeft, on), walk(a.IdxRight, on)
		case *gIrAOp1:
			a.Of = walk(a.Of, on)
		case *gIrAOp2:
			a.Left, a.Right = walk(a.Left, on), walk(a.Right, on)
		case *gIrAPanic:
			a.PanicArg = walk(a.PanicArg, on)
		case *gIrARet:
			a.RetArg = walk(a.RetArg, on)
		case *gIrASet:
			a.SetLeft, a.ToRight = walk(a.SetLeft, on), walk(a.ToRight, on)
		case *gIrALet:
			if a != nil {
				a.LetVal = walk(a.LetVal, on)
			}
		case *gIrAIsType:
			a.ExprToTest = walk(a.ExprToTest, on)
		case *gIrAToType:
			a.ExprToCast = walk(a.ExprToCast, on)
		case *gIrALitArr:
			for i, av := range a.ArrVals {
				a.ArrVals[i] = walk(av, on)
			}
		case *gIrALitObj:
			for i, av := range a.ObjFields {
				if tmp, _ := walk(av, on).(*gIrALitObjField); tmp != nil {
					a.ObjFields[i] = tmp
				}
			}
		case *gIrALitObjField:
			a.FieldVal = walk(a.FieldVal, on)
		case *gIrAComments, *gIrAPkgSym, *gIrANil, *gIrALitBool, *gIrALitDouble, *gIrALitInt, *gIrALitStr, *gIrASym:
		default:
			panic(fmt.Errorf("walk() not handling gIrA type '%v' (value: %v), please report", reflect.TypeOf(a), a))
		}
		if nuast := on(ast); nuast != ast {
			if oldp := ast.Parent(); nuast != nil {
				nuast.Base().parent = oldp
			}
			ast = nuast
		}
	}
	return ast
}
