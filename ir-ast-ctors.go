package main

/*
Golang intermediate-representation AST:
handy constructors for expressions and blocks
except type declarations (ie. "actual code")
*/

func ªA(exprs ...irA) *irALitArr {
	a := &irALitArr{ArrVals: exprs}
	a.RefArray = &irATypeRefArray{}
	typefound := false
	for _, expr := range a.ArrVals {
		eb := expr.Base()
		if eb.parent = a; !typefound && eb.hasTypeInfo() {
			a.RefArray.Of = &eb.irANamedTypeRef
		}
	}
	return a
}

func ªB(literal bool) *irALitBool {
	a := &irALitBool{LitBool: literal}
	a.RefAlias = "Prim.Boolean"
	return a
}

func ªN(literal float64) *irALitNum {
	a := &irALitNum{LitDouble: literal}
	a.RefAlias = "Prim.Number"
	return a
}

func ªI(literal int) *irALitInt {
	a := &irALitInt{LitInt: literal}
	a.RefAlias = "Prim.Int"
	return a
}

func ªO(typeref *irANamedTypeRef, fields ...*irALitObjField) *irALitObj {
	a := &irALitObj{ObjFields: fields}
	if typeref != nil {
		a.irANamedTypeRef = *typeref
	}
	for _, of := range a.ObjFields {
		of.parent = a
	}
	return a
}

func ªOFld(fieldval irA) *irALitObjField {
	a := &irALitObjField{FieldVal: fieldval}
	return a
}

func ªS(literal string) *irALitStr {
	a := &irALitStr{LitStr: literal}
	a.RefAlias = "Prim.String"
	return a
}

func ªBlock(asts ...irA) *irABlock {
	a := &irABlock{Body: asts}
	for _, expr := range a.Body {
		expr.Base().parent = a
	}
	return a
}

func ªCall(callee irA, callargs ...irA) *irACall {
	a := &irACall{Callee: callee, CallArgs: callargs}
	a.Callee.Base().parent = a
	for _, expr := range callargs {
		expr.Base().parent = a
	}
	return a
}

func ªComments(comments ...*coreImpComment) *irAComments {
	a := &irAComments{}
	a.Comments = comments
	return a
}

func ªConst(name *irANamedTypeRef, val irA) *irAConst {
	a, v := &irAConst{ConstVal: val}, val.Base()
	v.parent, a.irANamedTypeRef = a, v.irANamedTypeRef
	a.NameGo, a.NamePs = name.NameGo, name.NamePs
	return a
}

func ªDot(left irA, right irA) *irADot {
	a := &irADot{DotLeft: left, DotRight: right}
	lb, rb := left.Base(), right.Base()
	lb.parent, rb.parent = a, a
	return a
}

func ªDotNamed(left string, right string) *irADot {
	return ªDot(ªSymGo(left), ªSymGo(right))
}

func ªEq(left irA, right irA) *irAOp2 {
	a := &irAOp2{Op2: "==", Left: left, Right: right}
	a.Left.Base().parent, a.Right.Base().parent = a, a
	return a
}

func ªFor() *irAFor {
	a := &irAFor{ForDo: ªBlock()}
	a.ForDo.parent = a
	return a
}

func ªFunc() *irAFunc {
	a := &irAFunc{FuncImpl: ªBlock()}
	a.FuncImpl.parent = a
	return a
}

func ªIf(cond irA) *irAIf {
	a := &irAIf{If: cond, Then: ªBlock()}
	a.If.Base().parent, a.Then.parent = a, a
	return a
}

func ªIndex(left irA, right irA) *irAIndex {
	a := &irAIndex{IdxLeft: left, IdxRight: right}
	a.IdxLeft.Base().parent, a.IdxRight.Base().parent = a, a
	return a
}

func ªIs(expr irA, typeexpr string) *irAIsType {
	a := &irAIsType{ExprToTest: expr, TypeToTest: typeexpr}
	a.ExprToTest.Base().parent = a
	return a
}

func ªLet(namego string, nameps string, val irA) *irALet {
	a := &irALet{LetVal: val}
	if val != nil {
		vb := val.Base()
		vb.parent = a
		a.irANamedTypeRef = vb.irANamedTypeRef
	}
	if len(namego) == 0 && len(nameps) > 0 {
		a.setBothNamesFromPsName(nameps)
	} else {
		a.NameGo, a.NamePs = namego, nameps
	}
	return a
}

func ªNil() *irANil {
	a := &irANil{}
	return a
}

func ªO1(op string, operand irA) *irAOp1 {
	a := &irAOp1{Op1: op, Of: operand}
	a.Of.Base().parent = a
	return a
}

func ªO2(left irA, op string, right irA) *irAOp2 {
	a := &irAOp2{Op2: op, Left: left, Right: right}
	a.Left.Base().parent, a.Right.Base().parent = a, a
	return a
}

func ªPanic(errarg irA) *irAPanic {
	a := &irAPanic{PanicArg: errarg}
	a.PanicArg.Base().parent = a
	return a
}

func ªPkgSym(pkgname string, symbol string) *irAPkgSym {
	a := &irAPkgSym{PkgName: pkgname, Symbol: symbol}
	return a
}

func ªRet(retarg irA) *irARet {
	a := &irARet{RetArg: retarg}
	if a.RetArg != nil {
		a.RetArg.Base().parent = a
	}
	return a
}

func ªSet(left irA, right irA) *irASet {
	a := &irASet{SetLeft: left, ToRight: right}
	a.SetLeft.Base().parent, a.ToRight.Base().parent = a, a
	if rb := right.Base(); rb.hasTypeInfo() {
		a.irANamedTypeRef = rb.irANamedTypeRef
	}
	return a
}

func ªsetVarInGroup(namego string, right irA, typespec *irANamedTypeRef) *irASet {
	a := ªSet(ªSymGo(namego), right)
	if typespec != nil && typespec.hasTypeInfo() {
		a.irANamedTypeRef = *typespec
	}
	a.isInVarGroup = true
	return a
}

func ªSymGo(namego string) *irASym {
	a := &irASym{}
	a.NameGo = namego
	return a
}

func ªSymPs(nameps string, exported bool) *irASym {
	a := &irASym{}
	a.Export = exported
	a.setBothNamesFromPsName(nameps)
	return a
}

func ªTo(expr irA, pname string, tname string) *irAToType {
	a := &irAToType{ExprToCast: expr, TypePkg: pname, TypeName: tname}
	a.ExprToCast.Base().parent = a
	return a
}
