package main

/*
Golang intermediate-representation AST:
handy constructors for expressions and blocks
except type declarations (ie. "actual code")
*/

func ªA(exprs ...gIrA) *gIrALitArr {
	a := &gIrALitArr{ArrVals: exprs}
	a.RefArray = &gIrATypeRefArray{}
	typefound := false
	for _, expr := range a.ArrVals {
		eb := expr.Base()
		if eb.parent = a; !typefound && eb.hasTypeInfo() {
			a.RefArray.Of = &eb.gIrANamedTypeRef
		}
	}
	return a
}

func ªB(literal bool) *gIrALitBool {
	a := &gIrALitBool{LitBool: literal}
	a.RefAlias = "Prim.Boolean"
	return a
}

func ªF(literal float64) *gIrALitDouble {
	a := &gIrALitDouble{LitDouble: literal}
	a.RefAlias = "Prim.Number"
	return a
}

func ªI(literal int) *gIrALitInt {
	a := &gIrALitInt{LitInt: literal}
	a.RefAlias = "Prim.Int"
	return a
}

func ªO(typeref *gIrANamedTypeRef, fields ...*gIrALitObjField) *gIrALitObj {
	a := &gIrALitObj{ObjFields: fields}
	if typeref != nil {
		a.gIrANamedTypeRef = *typeref
	}
	for _, of := range a.ObjFields {
		of.parent = a
	}
	return a
}

func ªOFld(fieldval gIrA) *gIrALitObjField {
	a := &gIrALitObjField{FieldVal: fieldval}
	return a
}

func ªS(literal string) *gIrALitStr {
	a := &gIrALitStr{LitStr: literal}
	a.RefAlias = "Prim.String"
	return a
}

func ªBlock(asts ...gIrA) *gIrABlock {
	a := &gIrABlock{Body: asts}
	for _, expr := range a.Body {
		expr.Base().parent = a
	}
	return a
}

func ªCall(callee gIrA, callargs ...gIrA) *gIrACall {
	a := &gIrACall{Callee: callee, CallArgs: callargs}
	a.Callee.Base().parent = a
	for _, expr := range callargs {
		expr.Base().parent = a
	}
	return a
}

func ªComments(comments ...*coreImpComment) *gIrAComments {
	a := &gIrAComments{}
	a.Comments = comments
	return a
}

func ªConst(name *gIrANamedTypeRef, val gIrA) *gIrAConst {
	a, v := &gIrAConst{ConstVal: val}, val.Base()
	v.parent, a.gIrANamedTypeRef = a, v.gIrANamedTypeRef
	a.NameGo, a.NamePs = name.NameGo, name.NamePs
	return a
}

func ªDot(left gIrA, right gIrA) *gIrADot {
	a := &gIrADot{DotLeft: left, DotRight: right}
	a.DotLeft.Base().parent, a.DotRight.Base().parent = a, a
	return a
}

func ªDotNamed(left string, right string) *gIrADot {
	return ªDot(ªSym(left, ""), ªSym(right, ""))
}

func ªEq(left gIrA, right gIrA) *gIrAOp2 {
	a := &gIrAOp2{Op2: "==", Left: left, Right: right}
	a.Left.Base().parent, a.Right.Base().parent = a, a
	return a
}

func ªFor() *gIrAFor {
	a := &gIrAFor{ForDo: ªBlock()}
	a.ForDo.parent = a
	return a
}

func ªFunc() *gIrAFunc {
	a := &gIrAFunc{FuncImpl: ªBlock()}
	a.FuncImpl.parent = a
	return a
}

func ªIf(cond gIrA) *gIrAIf {
	a := &gIrAIf{If: cond, Then: ªBlock()}
	a.If.Base().parent, a.Then.parent = a, a
	return a
}

func ªIndex(left gIrA, right gIrA) *gIrAIndex {
	a := &gIrAIndex{IdxLeft: left, IdxRight: right}
	a.IdxLeft.Base().parent, a.IdxRight.Base().parent = a, a
	return a
}

func ªIs(expr gIrA, typeexpr string) *gIrAIsType {
	a := &gIrAIsType{ExprToTest: expr, TypeToTest: typeexpr}
	a.ExprToTest.Base().parent = a
	return a
}

func ªLet(namego string, nameps string, val gIrA) *gIrALet {
	a := &gIrALet{LetVal: val}
	if val != nil {
		vb := val.Base()
		vb.parent = a
		a.gIrANamedTypeRef = vb.gIrANamedTypeRef
	}
	if len(namego) == 0 && len(nameps) > 0 {
		a.setBothNamesFromPsName(nameps)
	} else {
		a.NameGo, a.NamePs = namego, nameps
	}
	return a
}

func ªNil() *gIrANil {
	a := &gIrANil{}
	return a
}

func ªO1(op string, operand gIrA) *gIrAOp1 {
	a := &gIrAOp1{Op1: op, Of: operand}
	a.Of.Base().parent = a
	return a
}

func ªO2(left gIrA, op string, right gIrA) *gIrAOp2 {
	a := &gIrAOp2{Op2: op, Left: left, Right: right}
	a.Left.Base().parent, a.Right.Base().parent = a, a
	return a
}

func ªPanic(errarg gIrA) *gIrAPanic {
	a := &gIrAPanic{PanicArg: errarg}
	a.PanicArg.Base().parent = a
	return a
}

func ªPkgSym(pkgname string, symbol string) *gIrAPkgSym {
	a := &gIrAPkgSym{PkgName: pkgname, Symbol: symbol}
	return a
}

func ªRet(retarg gIrA) *gIrARet {
	a := &gIrARet{RetArg: retarg}
	if a.RetArg != nil {
		a.RetArg.Base().parent = a
	}
	return a
}

func ªSet(left gIrA, right gIrA) *gIrASet {
	a := &gIrASet{SetLeft: left, ToRight: right}
	a.SetLeft.Base().parent, a.ToRight.Base().parent = a, a
	if rb := right.Base(); rb.hasTypeInfo() {
		a.gIrANamedTypeRef = rb.gIrANamedTypeRef
	}
	return a
}

func ªsetVarInGroup(namego string, right gIrA, typespec *gIrANamedTypeRef) *gIrASet {
	a := ªSet(ªSym(namego, ""), right)
	if typespec != nil && typespec.hasTypeInfo() {
		a.gIrANamedTypeRef = *typespec
	}
	a.isInVarGroup = true
	return a
}

func ªSym(namego string, nameps string) *gIrASym {
	a := &gIrASym{}
	if len(namego) == 0 && len(nameps) > 0 {
		a.setBothNamesFromPsName(nameps)
	} else {
		a.NameGo, a.NamePs = namego, nameps
	}
	return a
}

func ªTo(expr gIrA, pname string, tname string) *gIrAToType {
	a := &gIrAToType{ExprToCast: expr, TypePkg: pname, TypeName: tname}
	a.ExprToCast.Base().parent = a
	return a
}
