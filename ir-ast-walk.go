package main

/*
Golang intermediate-representation AST:
traversals of the abstract syntax tree
*/

func irALookupInAncestorBlocks(a irA, check func(irA) bool) irA {
	for nextparent := a.Parent(); nextparent != nil; nextparent = nextparent.Parent() {
		switch p := nextparent.(type) {
		case *irABlock:
			for _, stmt := range p.Body {
				if check(stmt) {
					return stmt
				}
			}
		}
	}
	return nil
}

func (me *irAst) topLevelDefs(okay func(irA) bool) (defs []irA) {
	for _, ast := range me.Body {
		if okay(ast) {
			defs = append(defs, ast)
		}
	}
	return
}

func (me *irAst) walkTopLevelDefs(on func(irA)) {
	for _, ast := range me.Body {
		on(ast)
	}
}

func (me *irAst) walk(on func(irA) irA) {
	for i, a := range me.Body {
		if a != nil {
			me.Body[i] = walk(a, on)
		}
	}
	for _, tr := range me.irM.GoTypeDefs {
		if tr.RefStruct != nil {
			for _, trm := range tr.RefStruct.Methods {
				if trm.RefFunc.impl != nil {
					trm.RefFunc.impl, _ = walk(trm.RefFunc.impl, on).(*irABlock)
				}
			}
		}
	}
	for i, tcf := range me.culled.typeCtorFuncs {
		me.culled.typeCtorFuncs[i] = walk(tcf, on).(*irACtor)
	}
}

func walk(ast irA, on func(irA) irA) irA {
	if ast != nil {
		switch a := ast.(type) {
		// why extra nil checks some places below: we do have the rare case of ast!=nil and ast.(type) set and still holding a null-ptr
		// why not everywhere: due to the nature of the ASTs constructed from coreimp, only those cases can potentially be nil if they exist at all
		case *irABlock:
			if a != nil {
				for i, _ := range a.Body {
					a.Body[i] = walk(a.Body[i], on)
				}
			}
		case *irACall:
			a.Callee = walk(a.Callee, on)
			for i, _ := range a.CallArgs {
				a.CallArgs[i] = walk(a.CallArgs[i], on)
			}
		case *irAConst:
			a.ConstVal = walk(a.ConstVal, on)
		case *irADot:
			a.DotLeft, a.DotRight = walk(a.DotLeft, on), walk(a.DotRight, on)
		case *irAFor:
			a.ForCond = walk(a.ForCond, on)
			if tmp, _ := walk(a.ForRange, on).(*irALet); tmp != nil {
				a.ForRange = tmp
			}
			if tmp, _ := walk(a.ForDo, on).(*irABlock); tmp != nil {
				a.ForDo = tmp
			}
			for i, fi := range a.ForInit {
				if tmp, _ := walk(fi, on).(*irALet); tmp != nil {
					a.ForInit[i] = tmp
				}
			}
			for i, fs := range a.ForStep {
				if tmp, _ := walk(fs, on).(*irASet); tmp != nil {
					a.ForStep[i] = tmp
				}
			}
		case *irACtor:
			if tmp, _ := walk(a.FuncImpl, on).(*irABlock); tmp != nil {
				a.FuncImpl = tmp
			}
		case *irAFunc:
			if tmp, _ := walk(a.FuncImpl, on).(*irABlock); tmp != nil {
				a.FuncImpl = tmp
			}
		case *irAIf:
			a.If = walk(a.If, on)
			if tmp, _ := walk(a.Then, on).(*irABlock); tmp != nil {
				a.Then = tmp
			}
			if tmp, _ := walk(a.Else, on).(*irABlock); tmp != nil {
				a.Else = tmp
			}
		case *irAIndex:
			a.IdxLeft, a.IdxRight = walk(a.IdxLeft, on), walk(a.IdxRight, on)
		case *irAOp1:
			a.Of = walk(a.Of, on)
		case *irAOp2:
			a.Left, a.Right = walk(a.Left, on), walk(a.Right, on)
		case *irAPanic:
			a.PanicArg = walk(a.PanicArg, on)
		case *irARet:
			a.RetArg = walk(a.RetArg, on)
		case *irASet:
			a.SetLeft, a.ToRight = walk(a.SetLeft, on), walk(a.ToRight, on)
		case *irALet:
			if a != nil {
				a.LetVal = walk(a.LetVal, on)
			}
		case *irAIsType:
			a.ExprToTest = walk(a.ExprToTest, on)
		case *irAToType:
			a.ExprToCast = walk(a.ExprToCast, on)
		case *irALitArr:
			for i, av := range a.ArrVals {
				a.ArrVals[i] = walk(av, on)
			}
		case *irALitObj:
			for i, av := range a.ObjFields {
				if tmp, _ := walk(av, on).(*irALitObjField); tmp != nil {
					a.ObjFields[i] = tmp
				}
			}
		case *irALitObjField:
			a.FieldVal = walk(a.FieldVal, on)
		case *irAComments, *irAPkgSym, *irANil, *irALitBool, *irALitNum, *irALitInt, *irALitStr, *irASym:
		default:
			panicWithType(ast.Base().SrcFilePath(), ast, "walk")
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
