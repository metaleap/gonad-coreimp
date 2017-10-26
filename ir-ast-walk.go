package main

/*
Golang intermediate-representation AST:
traversals of the abstract syntax tree
*/

type funcIra2Bool func(irA) bool

type funcIra2Ira func(irA) irA

func irALookupInAncestorBlocks(a irA, check funcIra2Bool) irA {
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

func (me *irABase) outerFunc() *irAFunc {
	for nextup := me.parent; nextup != nil; nextup = nextup.Parent() {
		if nextfn, _ := nextup.(*irAFunc); nextfn != nil {
			return nextfn
		}
	}
	return nil
}

func (me *irABlock) perFuncDown(istoplevel bool, on func(bool, *irAFunc)) {
	walk(me, false, func(a irA) irA { // false == don't recurse into inner func-vals
		switch ax := a.(type) {
		case *irAFunc: // we hit a func-val in the current block
			on(istoplevel, ax)                 // invoke handler for it
			ax.FuncImpl.perFuncDown(false, on) // only now recurse into itself
		}
		return a
	})
}

func (me *irABase) perFuncUp(on func(*irAFunc)) {
	for nextup := me.parent; nextup != nil; nextup = nextup.Parent() {
		if nextfn, _ := nextup.(*irAFunc); nextfn != nil {
			on(nextfn)
		}
	}
}

func (me *irAst) topLevelDefs(okay funcIra2Bool) (defs []irA) {
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

func (me *irAst) walk(on funcIra2Ira) {
	for i, a := range me.Body {
		if a != nil {
			me.Body[i] = walk(a, true, on)
		}
	}
	for _, tr := range me.irM.GoTypeDefs {
		if tr.RefStruct != nil {
			for _, trm := range tr.RefStruct.Methods {
				if trm.RefFunc.impl != nil {
					trm.RefFunc.impl, _ = walk(trm.RefFunc.impl, true, on).(*irABlock)
				}
			}
		}
	}
	for i, tcf := range me.culled.typeCtorFuncs {
		me.culled.typeCtorFuncs[i] = walk(tcf, true, on).(*irACtor)
	}
}

func walk(ast irA, intofuncvals bool, on funcIra2Ira) irA {
	if ast != nil {
		switch a := ast.(type) {
		// why extra nil checks some places below: we do have the rare case of ast!=nil and ast.(type) set and still holding a null-ptr
		// why not everywhere: due to the nature of the ASTs constructed from coreimp, only those cases can potentially be nil if they exist at all
		case *irABlock:
			if a != nil {
				for i, _ := range a.Body {
					a.Body[i] = walk(a.Body[i], intofuncvals, on)
				}
			}
		case *irACall:
			a.Callee = walk(a.Callee, intofuncvals, on)
			for i, _ := range a.CallArgs {
				a.CallArgs[i] = walk(a.CallArgs[i], intofuncvals, on)
			}
		case *irAConst:
			a.ConstVal = walk(a.ConstVal, intofuncvals, on)
		case *irADot:
			a.DotLeft, a.DotRight = walk(a.DotLeft, intofuncvals, on), walk(a.DotRight, intofuncvals, on)
		case *irAFor:
			a.ForCond = walk(a.ForCond, intofuncvals, on)
			if tmp, _ := walk(a.ForRange, intofuncvals, on).(*irALet); tmp != nil {
				a.ForRange = tmp
			}
			if tmp, _ := walk(a.ForDo, intofuncvals, on).(*irABlock); tmp != nil {
				a.ForDo = tmp
			}
			for i, fi := range a.ForInit {
				if tmp, _ := walk(fi, intofuncvals, on).(*irALet); tmp != nil {
					a.ForInit[i] = tmp
				}
			}
			for i, fs := range a.ForStep {
				if tmp, _ := walk(fs, intofuncvals, on).(*irASet); tmp != nil {
					a.ForStep[i] = tmp
				}
			}
		case *irACtor:
			if tmp, _ := walk(a.FuncImpl, intofuncvals, on).(*irABlock); tmp != nil {
				a.FuncImpl = tmp
			}
		case *irAFunc:
			walkinto := intofuncvals
			if !walkinto {
				if pb, _ := a.parent.(*irABlock); pb != nil && pb.parent == nil {
					walkinto = true
				}
			}
			if walkinto {
				if tmp, _ := walk(a.FuncImpl, intofuncvals, on).(*irABlock); tmp != nil {
					a.FuncImpl = tmp
				}
			}
		case *irAIf:
			a.If = walk(a.If, intofuncvals, on)
			if tmp, _ := walk(a.Then, intofuncvals, on).(*irABlock); tmp != nil {
				a.Then = tmp
			}
			if tmp, _ := walk(a.Else, intofuncvals, on).(*irABlock); tmp != nil {
				a.Else = tmp
			}
		case *irAIndex:
			a.IdxLeft, a.IdxRight = walk(a.IdxLeft, intofuncvals, on), walk(a.IdxRight, intofuncvals, on)
		case *irAOp1:
			a.Of = walk(a.Of, intofuncvals, on)
		case *irAOp2:
			a.Left, a.Right = walk(a.Left, intofuncvals, on), walk(a.Right, intofuncvals, on)
		case *irAPanic:
			a.PanicArg = walk(a.PanicArg, intofuncvals, on)
		case *irARet:
			a.RetArg = walk(a.RetArg, intofuncvals, on)
		case *irASet:
			a.SetLeft, a.ToRight = walk(a.SetLeft, intofuncvals, on), walk(a.ToRight, intofuncvals, on)
		case *irALet:
			if a != nil {
				a.LetVal = walk(a.LetVal, intofuncvals, on)
			}
		case *irAIsType:
			a.ExprToTest = walk(a.ExprToTest, intofuncvals, on)
		case *irAToType:
			a.ExprToConv = walk(a.ExprToConv, intofuncvals, on)
		case *irALitArr:
			for i, av := range a.ArrVals {
				a.ArrVals[i] = walk(av, intofuncvals, on)
			}
		case *irALitObj:
			for i, av := range a.ObjFields {
				if tmp, _ := walk(av, intofuncvals, on).(*irALitObjField); tmp != nil {
					a.ObjFields[i] = tmp
				}
			}
		case *irALitObjField:
			a.FieldVal = walk(a.FieldVal, intofuncvals, on)
		case *irAComments, *irAPkgSym, *irANil, *irALitBool, *irALitNum, *irALitInt, *irALitStr, *irASym:
		default:
			panicWithType(ast.Base().srcFilePath(), ast, "walk")
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
