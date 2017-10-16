package main

import (
	"fmt"
)

func (me *GonadIrAst) topLevelDefs(okay func(GIrA) bool) (defs []GIrA) {
	for _, ast := range me.Body {
		if okay(ast) {
			defs = append(defs, ast)
		} else if c, ok := ast.(*GIrAComments); ok {
			var c2 *GIrAComments
			for ok {
				if c2, ok = c.CommentsDecl.(*GIrAComments); ok {
					c = c2
				}
			}
			if okay(c.CommentsDecl) {
				defs = append(defs, ast)
			}
		}
	}
	return
}

func (me *GonadIrAst) Walk(on func(GIrA) GIrA) {
	for i, a := range me.Body {
		if a != nil {
			me.Body[i] = walk(a, on)
		}
	}
}

func walk(ast GIrA, on func(GIrA) GIrA) GIrA {
	if ast != nil {
		switch a := ast.(type) {
		case *GIrABlock:
			if a != nil { // odd that this would happen, given the above, but it did! (go1.7.6)
				for i, _ := range a.Body {
					a.Body[i] = walk(a.Body[i], on)
				}
			}
		case *GIrACall:
			a.Callee = walk(a.Callee, on)
			for i, _ := range a.CallArgs {
				a.CallArgs[i] = walk(a.CallArgs[i], on)
			}
		case *GIrAComments:
			a.CommentsDecl = walk(a.CommentsDecl, on)
		case *GIrAConst:
			a.ConstVal = walk(a.ConstVal, on)
		case *GIrADot:
			a.DotLeft, a.DotRight = walk(a.DotLeft, on), walk(a.DotRight, on)
		case *GIrAFor:
			a.ForCond = walk(a.ForCond, on)
			if tmp, _ := walk(a.ForRange, on).(*GIrAVar); tmp != nil {
				a.ForRange = tmp
			}
			if tmp, _ := walk(a.ForDo, on).(*GIrABlock); tmp != nil {
				a.ForDo = tmp
			}
			for i, fi := range a.ForInit {
				if tmp, _ := walk(fi, on).(*GIrASet); tmp != nil {
					a.ForInit[i] = tmp
				}
			}
			for i, fs := range a.ForStep {
				if tmp, _ := walk(fs, on).(*GIrASet); tmp != nil {
					a.ForStep[i] = tmp
				}
			}
		case *GIrAFunc:
			if tmp, _ := walk(a.FuncImpl, on).(*GIrABlock); tmp != nil {
				a.FuncImpl = tmp
			}
		case *GIrAIf:
			a.If = walk(a.If, on)
			if tmp, _ := walk(a.Then, on).(*GIrABlock); tmp != nil {
				a.Then = tmp
			}
			if tmp, _ := walk(a.Else, on).(*GIrABlock); tmp != nil {
				a.Else = tmp
			}
		case *GIrAIndex:
			a.IdxLeft, a.IdxRight = walk(a.IdxLeft, on), walk(a.IdxRight, on)
		case *GIrAOp1:
			a.Of = walk(a.Of, on)
		case *GIrAOp2:
			a.Left, a.Right = walk(a.Left, on), walk(a.Right, on)
		case *GIrAPanic:
			a.PanicArg = walk(a.PanicArg, on)
		case *GIrARet:
			a.RetArg = walk(a.RetArg, on)
		case *GIrASet:
			a.SetLeft, a.ToRight = walk(a.SetLeft, on), walk(a.ToRight, on)
		case *GIrAVar:
			if a != nil { // odd that this would happen, given the above, but it did! (go1.7.6)
				a.VarVal = walk(a.VarVal, on)
			}
		case *GIrAIsType:
			a.ExprToTest = walk(a.ExprToTest, on)
		case *GIrAToType:
			a.ExprToCast = walk(a.ExprToCast, on)
		case *GIrALitArr:
			for i, av := range a.ArrVals {
				a.ArrVals[i] = walk(av, on)
			}
		case *GIrALitObj:
			for i, av := range a.ObjFields {
				if tmp, _ := walk(av, on).(*GIrALitObjField); tmp != nil {
					a.ObjFields[i] = tmp
				}
			}
		case *GIrALitObjField:
			a.FieldVal = walk(a.FieldVal, on)
		case *GIrAPkgRef, *GIrANil, *GIrALitBool, *GIrALitDouble, *GIrALitInt, *GIrALitStr:
		default:
			fmt.Printf("%v", ast)
			panic("WALK not handling a GIrA type")
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
