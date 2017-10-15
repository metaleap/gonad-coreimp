package main

func ªA(exprs ...GIrA) *GIrALitArr {
	a := &GIrALitArr{ArrVals: exprs}
	a.RefArray = &GIrATypeRefArray{}
	typefound := false
	for _, expr := range a.ArrVals {
		eb := expr.Base()
		if eb.parent = a; !typefound && eb.HasTypeInfo() {
			a.RefArray.Of = &eb.GIrANamedTypeRef
		}
	}
	return a
}

func ªB(literal bool) *GIrALitBool {
	a := &GIrALitBool{LitBool: literal}
	a.RefAlias = "Prim.Boolean"
	return a
}

func ªF(literal float64) *GIrALitDouble {
	a := &GIrALitDouble{LitDouble: literal}
	a.RefAlias = "Prim.Number"
	return a
}

func ªI(literal int) *GIrALitInt {
	a := &GIrALitInt{LitInt: literal}
	a.RefAlias = "Prim.Int"
	return a
}

func ªO(typeref *GIrANamedTypeRef, fields ...*GIrALitObjField) *GIrALitObj {
	a := &GIrALitObj{ObjFields: fields}
	if typeref != nil {
		a.GIrANamedTypeRef = *typeref
	}
	for _, of := range a.ObjFields {
		of.parent = a
	}
	return a
}

func ªOFld(fieldval GIrA) *GIrALitObjField {
	a := &GIrALitObjField{FieldVal: fieldval}
	return a
}

func ªS(literal string) *GIrALitStr {
	a := &GIrALitStr{LitStr: literal}
	a.RefAlias = "Prim.String"
	return a
}

func ªBlock(asts ...GIrA) *GIrABlock {
	a := &GIrABlock{Body: asts}
	for _, expr := range a.Body {
		expr.Base().parent = a
	}
	return a
}

func ªCall(callee GIrA, callargs ...GIrA) *GIrACall {
	a := &GIrACall{Callee: callee, CallArgs: callargs}
	a.Callee.Base().parent = a
	for _, expr := range callargs {
		expr.Base().parent = a
	}
	return a
}

func ªComments(comments ...*CoreImpComment) *GIrAComments {
	a := &GIrAComments{Comments: comments}
	return a
}

func ªConst(name *GIrANamedTypeRef, val GIrA) *GIrAConst {
	a, v := &GIrAConst{ConstVal: val}, val.Base()
	v.parent, a.GIrANamedTypeRef = a, v.GIrANamedTypeRef
	a.NameGo, a.NamePs = name.NameGo, name.NamePs
	return a
}

func ªDot(left GIrA, right GIrA) *GIrADot {
	a := &GIrADot{DotLeft: left, DotRight: right}
	a.DotLeft.Base().parent, a.DotRight.Base().parent = a, a
	return a
}

func ªDotNamed(left string, right string) *GIrADot {
	return ªDot(ªSym(left), ªSym(right))
}

func ªEq(left GIrA, right GIrA) *GIrAOp2 {
	a := &GIrAOp2{Op2: "==", Left: left, Right: right}
	a.Left.Base().parent, a.Right.Base().parent = a, a
	return a
}

func ªFor() *GIrAFor {
	a := &GIrAFor{ForDo: ªBlock()}
	a.ForDo.parent = a
	return a
}

func ªFunc() *GIrAFunc {
	a := &GIrAFunc{FuncImpl: ªBlock()}
	a.FuncImpl.parent = a
	return a
}

func ªIf(cond GIrA) *GIrAIf {
	a := &GIrAIf{If: cond, Then: ªBlock()}
	a.If.Base().parent, a.Then.parent = a, a
	return a
}

func ªIndex(left GIrA, right GIrA) *GIrAIndex {
	a := &GIrAIndex{IdxLeft: left, IdxRight: right}
	a.IdxLeft.Base().parent, a.IdxRight.Base().parent = a, a
	return a
}

func ªIs(expr GIrA, typeexpr GIrA) *GIrAIsType {
	a := &GIrAIsType{ExprToTest: expr, TypeToTest: typeexpr}
	a.ExprToTest.Base().parent, a.TypeToTest.Base().parent = a, a
	return a
}

func ªNil() *GIrANil {
	a := &GIrANil{}
	return a
}

func ªO1(op string, operand GIrA) *GIrAOp1 {
	a := &GIrAOp1{Op1: op, Of: operand}
	a.Of.Base().parent = a
	return a
}

func ªO2(left GIrA, op string, right GIrA) *GIrAOp2 {
	a := &GIrAOp2{Op2: op, Left: left, Right: right}
	a.Left.Base().parent, a.Right.Base().parent = a, a
	return a
}

func ªPanic(errarg GIrA) *GIrAPanic {
	a := &GIrAPanic{PanicArg: errarg}
	a.PanicArg.Base().parent = a
	return a
}

func ªPkgRef(pkgname string, symbol string) *GIrAPkgRef {
	a := &GIrAPkgRef{PkgName: pkgname, Symbol: symbol}
	return a
}

func ªRet(retarg GIrA) *GIrARet {
	a := &GIrARet{RetArg: retarg}
	if a.RetArg != nil {
		a.RetArg.Base().parent = a
	}
	return a
}

func ªSet(left GIrA, right GIrA) *GIrASet {
	a := &GIrASet{SetLeft: left, ToRight: right}
	a.SetLeft.Base().parent, a.ToRight.Base().parent = a, a
	if rb := right.Base(); rb.HasTypeInfo() {
		a.GIrANamedTypeRef = rb.GIrANamedTypeRef
	}
	return a
}

func ªsetVarInGroup(name string, right GIrA, typespec *GIrANamedTypeRef) *GIrASet {
	a := ªSet(ªSym(name), right)
	if typespec != nil && typespec.HasTypeInfo() {
		a.GIrANamedTypeRef = *typespec
	}
	a.isInVarGroup = true
	return a
}

func ªSym(name string) *GIrAVar {
	return ªVar(name, "", nil)
}

func ªTo(expr GIrA, pname string, tname string) *GIrAToType {
	a := &GIrAToType{ExprToCast: expr, TypePkg: pname, TypeName: tname}
	a.ExprToCast.Base().parent = a
	return a
}

func ªVar(namego string, nameps string, val GIrA) *GIrAVar {
	a := &GIrAVar{VarVal: val}
	if val != nil {
		vb := val.Base()
		vb.parent = a
		a.GIrANamedTypeRef = vb.GIrANamedTypeRef
	}
	if len(a.NameGo) == 0 && len(nameps) > 0 {
		a.setBothNamesFromPsName(nameps)
	} else {
		a.NameGo, a.NamePs = namego, nameps
	}
	return a
}
