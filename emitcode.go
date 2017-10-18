package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/metaleap/go-util-str"
)

type goTypeRefResolver func(tref string, markused bool) (pname string, tname string)

const (
	areOverlappingInterfacesSupportedByGo = false // this might change hopefully, see https://github.com/golang/go/issues/6977
)

func codeEmitCommaIf(w io.Writer, i int) {
	if i > 0 {
		fmt.Fprint(w, ", ")
	}
}

func codeEmitAst(w io.Writer, indent int, ast GIrA, trr goTypeRefResolver) {
	if ast == nil {
		return
	}
	tabs := ""
	if indent > 0 {
		tabs = strings.Repeat("\t", indent)
	}
	switch a := ast.(type) {
	case *GIrALitStr:
		fmt.Fprintf(w, "%q", a.LitStr)
	case *GIrALitBool:
		fmt.Fprintf(w, "%t", a.LitBool)
	case *GIrALitDouble:
		s := fmt.Sprintf("%f", a.LitDouble)
		for strings.HasSuffix(s, "0") {
			s = s[:len(s)-1]
		}
		fmt.Fprint(w, s)
	case *GIrALitInt:
		fmt.Fprintf(w, "%d", a.LitInt)
	case *GIrALitArr:
		codeEmitTypeDecl(w, &a.GIrANamedTypeRef, indent, trr)
		fmt.Fprint(w, "{")
		for i, expr := range a.ArrVals {
			codeEmitCommaIf(w, i)
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprint(w, "}")
	case *GIrALitObj:
		codeEmitTypeDecl(w, &a.GIrANamedTypeRef, -999, trr)
		fmt.Fprint(w, "{")
		for i, namevaluepair := range a.ObjFields {
			codeEmitCommaIf(w, i)
			if len(namevaluepair.NameGo) > 0 {
				fmt.Fprintf(w, "%s: ", namevaluepair.NameGo)
			}
			codeEmitAst(w, indent, namevaluepair.FieldVal, trr)
		}
		fmt.Fprint(w, "}")
	case *GIrAConst:
		fmt.Fprintf(w, "%sconst %s ", tabs, a.NameGo)
		codeEmitTypeDecl(w, &a.GIrANamedTypeRef, -1, trr)
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.ConstVal, trr)
		fmt.Fprint(w, "\n")
	case *GIrAVar:
		if a.VarVal != nil {
			fmt.Fprintf(w, "%svar %s", tabs, a.NameGo)
			fmt.Fprint(w, " = ")
			codeEmitAst(w, indent, a.VarVal, trr)
			fmt.Fprint(w, "\n")
		} else if len(a.NameGo) > 0 {
			fmt.Fprintf(w, "%s", a.NameGo)
		}
	case *GIrABlock:
		if a == nil || len(a.Body) == 0 {
			fmt.Fprint(w, "{}")
			// } else if len(a.Body) == 1 {
			// 	fmt.Fprint(w, "{ ")
			// 	codeEmitAst(w, -1, a.Body[0], trr)
			// 	fmt.Fprint(w, " }")
		} else {
			fmt.Fprint(w, "{\n")
			indent++
			for _, expr := range a.Body {
				codeEmitAst(w, indent, expr, trr)
			}
			fmt.Fprintf(w, "%s}", tabs)
			indent--
		}
	case *GIrAIf:
		fmt.Fprintf(w, "%sif ", tabs)
		codeEmitAst(w, indent, a.If, trr)
		fmt.Fprint(w, " ")
		codeEmitAst(w, indent, a.Then, trr)
		if a.Else != nil {
			fmt.Fprint(w, " else ")
			codeEmitAst(w, indent, a.Else, trr)
		}
		fmt.Fprint(w, "\n")
	case *GIrACall:
		codeEmitAst(w, indent, a.Callee, trr)
		fmt.Fprint(w, "(")
		for i, expr := range a.CallArgs {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprint(w, ")")
	case *GIrAFunc:
		codeEmitTypeDecl(w, &a.GIrANamedTypeRef, indent, trr)
		codeEmitAst(w, indent, a.FuncImpl, trr)
	case *GIrAComments:
		for _, c := range a.Comments {
			if len(c.BlockComment) > 0 {
				fmt.Fprintf(w, "/*%s*/", c.BlockComment)
			} else {
				fmt.Fprintf(w, "%s//%s\n", tabs, c.LineComment)
			}
		}
	case *GIrARet:
		if a.RetArg == nil {
			fmt.Fprintf(w, "%sreturn", tabs)
		} else {
			fmt.Fprintf(w, "%sreturn ", tabs)
			codeEmitAst(w, indent, a.RetArg, trr)
		}
		if indent >= 0 {
			fmt.Fprint(w, "\n")
		}
	case *GIrAPanic:
		fmt.Fprintf(w, "%spanic(", tabs)
		codeEmitAst(w, indent, a.PanicArg, trr)
		fmt.Fprint(w, ")\n")
	case *GIrADot:
		codeEmitAst(w, indent, a.DotLeft, trr)
		fmt.Fprint(w, ".")
		codeEmitAst(w, indent, a.DotRight, trr)
	case *GIrAIndex:
		codeEmitAst(w, indent, a.IdxLeft, trr)
		fmt.Fprint(w, "[")
		codeEmitAst(w, indent, a.IdxRight, trr)
		fmt.Fprint(w, "]")
	case *GIrAIsType:
		fmt.Fprint(w, "_,øĸ := ")
		codeEmitAst(w, indent, a.ExprToTest, trr)
		fmt.Fprint(w, ".(")
		fmt.Fprint(w, typeNameWithPkgName(trr(a.TypeToTest, true)))
		// codeEmitAst(w, indent, a.TypeToTest, trr)
		fmt.Fprint(w, "); øĸ")
	case *GIrAToType:
		if len(a.TypePkg) == 0 {
			fmt.Fprintf(w, "%s(", a.TypeName)
		} else {
			fmt.Fprintf(w, "%s.%s(", a.TypePkg, a.TypeName)
		}
		codeEmitAst(w, indent, a.ExprToCast, trr)
		fmt.Fprint(w, ")")
	case *GIrAPkgRef:
		if len(a.PkgName) > 0 {
			fmt.Fprintf(w, "%s.", a.PkgName)
		}
		fmt.Fprint(w, a.Symbol)
	case *GIrASet:
		fmt.Fprint(w, tabs)
		codeEmitAst(w, indent, a.SetLeft, trr)
		if a.isInVarGroup {
			fmt.Fprint(w, " ")
			codeEmitTypeDecl(w, &a.GIrANamedTypeRef, indent, trr)
		}
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.ToRight, trr)
		fmt.Fprint(w, "\n")
	case *GIrAOp1:
		fmt.Fprintf(w, "(%s", a.Op1)
		codeEmitAst(w, indent, a.Of, trr)
		fmt.Fprint(w, ")")
	case *GIrAOp2:
		fmt.Fprint(w, "(")
		codeEmitAst(w, indent, a.Left, trr)
		switch a.Op2 {
		case "Add", "+":
			fmt.Fprint(w, " + ")
		case "Subtract", "-":
			fmt.Fprint(w, " - ")
		case "Multiply", "*":
			fmt.Fprint(w, " * ")
		case "Divide", "/":
			fmt.Fprint(w, " / ")
		case "Modulus", "%":
			fmt.Fprint(w, " % ")
		case "EqualTo", "==":
			fmt.Fprint(w, " == ")
		case "NotEqualTo", "!=":
			fmt.Fprint(w, " != ")
		case "LessThan", "<":
			fmt.Fprint(w, " < ")
		case "LessThanOrEqualTo", "<=":
			fmt.Fprint(w, " <= ")
		case "GreaterThan", ">":
			fmt.Fprint(w, " > ")
		case "GreaterThanOrEqualTo", ">=":
			fmt.Fprint(w, " >= ")
		case "And", "&&":
			fmt.Fprint(w, " && ")
		case "Or", "||":
			fmt.Fprint(w, " || ")
		case "BitwiseAnd", "&":
			fmt.Fprint(w, " & ")
		case "BitwiseOr", "|":
			fmt.Fprint(w, " | ")
		case "BitwiseXor", "^":
			fmt.Fprint(w, " ^ ")
		case "ShiftLeft", "<<":
			fmt.Fprint(w, " << ")
		case "ShiftRight", ">>":
			fmt.Fprint(w, " >> ")
		case "ZeroFillShiftRight", "&^":
			fmt.Fprint(w, " &^ ")
		default:
			fmt.Fprintf(w, " ?%s? ", a.Op2)
			panic("unrecognized binary op '" + a.Op2 + "', please report!")
		}
		codeEmitAst(w, indent, a.Right, trr)
		fmt.Fprint(w, ")")
	case *GIrANil:
		fmt.Fprint(w, "nil")
	case *GIrAFor:
		if a.ForRange != nil {
			fmt.Fprintf(w, "%sfor _, %s := range ", tabs, a.ForRange.NameGo)
			codeEmitAst(w, indent, a.ForRange.VarVal, trr)
			codeEmitAst(w, indent, a.ForDo, trr)
		} else if len(a.ForInit) > 0 || len(a.ForStep) > 0 {
			fmt.Fprint(w, "for ")

			for i, finit := range a.ForInit {
				codeEmitCommaIf(w, i)
				codeEmitAst(w, indent, finit.SetLeft, trr)
			}
			fmt.Fprint(w, " = ")
			for i, finit := range a.ForInit {
				codeEmitCommaIf(w, i)
				codeEmitAst(w, indent, finit.ToRight, trr)
			}
			fmt.Fprint(w, "; ")

			codeEmitAst(w, indent, a.ForCond, trr)
			fmt.Fprint(w, "; ")

			for i, fstep := range a.ForStep {
				codeEmitCommaIf(w, i)
				codeEmitAst(w, indent, fstep.SetLeft, trr)
			}
			fmt.Fprint(w, " = ")
			for i, fstep := range a.ForStep {
				codeEmitCommaIf(w, i)
				codeEmitAst(w, indent, fstep.ToRight, trr)
			}
			codeEmitAst(w, indent, a.ForDo, trr)
		} else {
			fmt.Fprintf(w, "%sfor ", tabs)
			codeEmitAst(w, indent, a.ForCond, trr)
			codeEmitAst(w, indent, a.ForDo, trr)
		}
	default:
		b, _ := json.Marshal(&ast)
		fmt.Fprintf(w, "/*****%v*****/", string(b))
	}
}

func codeEmitGroupedVals(w io.Writer, indent int, consts bool, asts []GIrA, trr goTypeRefResolver) {
	if l := len(asts); l == 1 {
		codeEmitAst(w, indent, asts[0], trr)
	} else if l > 1 {
		if consts {
			fmt.Fprint(w, "const (\n")
		} else {
			fmt.Fprint(w, "var (\n")
		}
		valºnameºtype := func(a GIrA) (val GIrA, name string, typeref *GIrANamedTypeRef) {
			if ac, ok := a.(*GIrAConst); ok && consts {
				val, name, typeref = ac.ConstVal, ac.NameGo, &ac.GIrANamedTypeRef
			} else if av, ok := a.(*GIrAVar); ok {
				val, name, typeref = av.VarVal, av.NameGo, &av.GIrANamedTypeRef
			}
			return
		}
		for i, a := range asts {
			val, name, typeref := valºnameºtype(a)
			codeEmitAst(w, indent+1, ªsetVarInGroup(name, val, typeref), trr)
			if i < (len(asts) - 1) {
				if _, ok := asts[i+1].(*GIrAComments); ok {
					fmt.Fprint(w, "\n")
				}
			}
		}
		fmt.Fprint(w, ")\n\n")
	}
}

func codeEmitEnumConsts(w io.Writer, enumconstnames []string, enumconsttype string) {
	fmt.Fprint(w, "const (\n")
	fmt.Fprintf(w, "\t_ %v= iota\n", strings.Repeat(" ", len(enumconsttype)+len(enumconstnames[0])))
	for i, enumconstname := range enumconstnames {
		fmt.Fprintf(w, "\t%s", enumconstname)
		if i == 0 {
			fmt.Fprintf(w, " %s = iota", enumconsttype)
		}
		fmt.Fprint(w, "\n")
	}
	fmt.Fprint(w, ")\n\n")
}

func codeEmitFuncArgs(w io.Writer, methodargs GIrANamedTypeRefs, indlevel int, typerefresolver goTypeRefResolver, isretargs bool) {
	parens := (!isretargs) || len(methodargs) > 1 || (len(methodargs) == 1 && len(methodargs[0].NameGo) > 0)
	if parens {
		fmt.Fprint(w, "(")
	}
	if len(methodargs) > 0 {
		for i, arg := range methodargs {
			codeEmitCommaIf(w, i)
			if len(arg.NameGo) > 0 {
				fmt.Fprintf(w, "%s ", arg.NameGo)
			}
			codeEmitTypeDecl(w, arg, indlevel, typerefresolver)
		}
	}
	if parens {
		fmt.Fprint(w, ")")
	}
	if !isretargs {
		fmt.Fprint(w, " ")
	}
}

func codeEmitModImps(w io.Writer, modimps []*GIrMPkgRef) {
	if len(modimps) > 0 {
		fmt.Fprint(w, "import (\n")
		for _, modimp := range modimps {
			if modimp.used {
				if modimp.N == modimp.P {
					fmt.Fprintf(w, "\t%q\n", modimp.P)
				} else {
					fmt.Fprintf(w, "\t%s %q\n", modimp.N, modimp.P)
				}
			} else {
				fmt.Fprintf(w, "\t// %s %q\n", modimp.N, modimp.P)
			}
		}
		fmt.Fprint(w, ")\n\n")
	}
}

func codeEmitPkgDecl(w io.Writer, pname string) {
	fmt.Fprintf(w, "package %s\n\n", pname)
}

func codeEmitTypeAlias(w io.Writer, tname string, ttype string) {
	fmt.Fprintf(w, "type %s %s\n\n", tname, ttype)
}

func codeEmitTypeDecl(w io.Writer, gtd *GIrANamedTypeRef, indlevel int, typerefresolver goTypeRefResolver) {
	if gtd == nil {
		fmt.Fprint(w, "interface{/*GIrANamedTypeRef=Nil*/}")
		return
	}
	toplevel := (indlevel == 0)
	fmtembeds := "\t%s\n"
	isfuncwithbodynotjustsig := gtd.RefFunc != nil && gtd.method.body != nil
	if toplevel && !isfuncwithbodynotjustsig {
		fmt.Fprintf(w, "type %s ", gtd.NameGo)
	}
	if len(gtd.RefAlias) > 0 {
		fmt.Fprint(w, codeEmitTypeRef(typerefresolver(gtd.RefAlias, true)))
	} else if gtd.RefUnknown != 0 {
		fmt.Fprintf(w, "interface{/*%d*/}", gtd.RefUnknown)
	} else if gtd.RefArray != nil {
		fmt.Fprint(w, "[]")
		codeEmitTypeDecl(w, gtd.RefArray.Of, -1, typerefresolver)
	} else if gtd.RefPtr != nil {
		fmt.Fprint(w, "*")
		codeEmitTypeDecl(w, gtd.RefPtr.Of, -1, typerefresolver)
	} else if gtd.RefInterface != nil {
		if len(gtd.RefInterface.Embeds) == 0 && len(gtd.RefInterface.Methods) == 0 {
			fmt.Fprint(w, "interface{}")
		} else {
			var tabind string
			if indlevel > 0 {
				tabind = strings.Repeat("\t", indlevel)
			}
			fmt.Fprint(w, "interface {\n")
			if areOverlappingInterfacesSupportedByGo {
				for _, ifembed := range gtd.RefInterface.Embeds {
					fmt.Fprint(w, tabind)
					fmt.Fprintf(w, fmtembeds, codeEmitTypeRef(typerefresolver(ifembed, true)))
				}
			}
			var buf bytes.Buffer
			for _, ifmethod := range gtd.RefInterface.allMethods() {
				fmt.Fprint(&buf, ifmethod.NameGo)
				if ifmethod.RefFunc == nil {
					panic(gtd.NamePs + "." + ifmethod.NamePs + ": unexpected interface-method (not a func), please report!")
				} else {
					codeEmitFuncArgs(&buf, ifmethod.RefFunc.Args, indlevel+1, typerefresolver, false)
					codeEmitFuncArgs(&buf, ifmethod.RefFunc.Rets, indlevel+1, typerefresolver, true)
				}
				fmt.Fprint(w, tabind)
				fmt.Fprintf(w, fmtembeds, buf.String())
				buf.Reset()
			}
			fmt.Fprintf(w, "%s}", tabind)
		}
	} else if gtd.RefStruct != nil {
		var tabind string
		if indlevel > 0 {
			tabind = strings.Repeat("\t", indlevel)
		}
		if len(gtd.RefStruct.Embeds) == 0 && len(gtd.RefStruct.Fields) == 0 {
			fmt.Fprint(w, "struct{}")
		} else {
			fmt.Fprint(w, "struct {\n")
			for _, structembed := range gtd.RefStruct.Embeds {
				fmt.Fprint(w, tabind)
				fmt.Fprintf(w, fmtembeds, structembed)
			}
			fnlen := 0
			for _, structfield := range gtd.RefStruct.Fields {
				if l := len(structfield.NameGo); l > fnlen {
					fnlen = l
				}
			}
			var buf bytes.Buffer
			for _, structfield := range gtd.RefStruct.Fields {
				codeEmitTypeDecl(&buf, structfield, indlevel+1, typerefresolver)
				fmt.Fprint(w, tabind)
				fmt.Fprintf(w, fmtembeds, ustr.PadRight(structfield.NameGo, fnlen)+" "+buf.String())
				buf.Reset()
			}
			fmt.Fprintf(w, "%s}", tabind)
		}
	} else if gtd.RefFunc != nil {
		ilev := indlevel
		if ilev == 0 {
			ilev += 1
		}
		fmt.Fprint(w, "func")
		if isfuncwithbodynotjustsig && len(gtd.NameGo) > 0 {
			fmt.Fprintf(w, " %s", gtd.NameGo)
		}
		fmt.Fprint(w, "(")
		for i, l := 0, len(gtd.RefFunc.Args); i < l; i++ {
			codeEmitCommaIf(w, i)
			if argname := gtd.RefFunc.Args[i].NameGo; len(argname) > 0 {
				fmt.Fprintf(w, "%s ", argname)
			}
			codeEmitTypeDecl(w, gtd.RefFunc.Args[i], ilev, typerefresolver)
		}
		fmt.Fprint(w, ") ")
		numrets := len(gtd.RefFunc.Rets)
		if numrets > 1 {
			fmt.Fprint(w, "(")
		}
		for i := 0; i < numrets; i++ {
			codeEmitCommaIf(w, i)
			if retname := gtd.RefFunc.Rets[i].NameGo; len(retname) > 0 {
				fmt.Fprintf(w, "%s ", retname)
			}
			codeEmitTypeDecl(w, gtd.RefFunc.Rets[i], ilev, typerefresolver)
		}
		if numrets > 1 {
			fmt.Fprint(w, ")")
		}
	} else {
		fmt.Fprint(w, "interface{/*EmptyNotNil*/}")
	}
	if toplevel && !isfuncwithbodynotjustsig {
		fmt.Fprintln(w, "\n")
	}
}

func codeEmitTypeMethods(w io.Writer, tr *GIrANamedTypeRef, typerefresolver goTypeRefResolver) {
	if len(tr.Methods) > 0 {
		for _, method := range tr.Methods {
			mthis := "this"
			if method.method.hasNoThis {
				mthis = "_"
			}
			if method.method.isNewCtor {
				fmt.Fprintf(w, "func %s", method.NameGo)
			} else if tr.RefStruct.PassByPtr {
				fmt.Fprintf(w, "func (%s *%s) %s", mthis, tr.NameGo, method.NameGo)
			} else {
				fmt.Fprintf(w, "func (%s %s) %s", mthis, tr.NameGo, method.NameGo)
			}
			codeEmitFuncArgs(w, method.RefFunc.Args, -1, typerefresolver, false)
			codeEmitFuncArgs(w, method.RefFunc.Rets, -1, typerefresolver, true)
			fmt.Fprint(w, " ")
			codeEmitAst(w, 0, method.method.body, typerefresolver)
			fmt.Fprint(w, "\n\n")
		}
	}
}

func codeEmitTypeRef(pname string, tname string) string {
	if len(pname) == 0 {
		return tname
	}
	return pname + "." + tname
}
