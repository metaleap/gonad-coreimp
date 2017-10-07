package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/metaleap/go-util-str"
)

type goTypeRefResolver func(tref string, markused bool) (pname string, tname string)

const (
	areOverlappingInterfacesSupportedByGo = false // this might change hopefully, see https://github.com/golang/go/issues/6977
)

func codeEmitAst(w io.Writer, indent int, ast GIrA, trr goTypeRefResolver) {
	tabs := strings.Repeat("\t", indent)
	switch a := ast.(type) {
	case GIrALitStr:
		fmt.Fprintf(w, "%q", a.LitStr)
	case GIrALitBool:
		fmt.Fprintf(w, "%t", a.LitBool)
	case GIrALitDouble:
		fmt.Fprintf(w, "%f", a.LitDouble)
	case GIrALitInt:
		fmt.Fprintf(w, "%d", a.LitInt)
	case GIrAConst:
		fmt.Fprintf(w, "%sconst %s", tabs, a.NameGo)
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.ConstVal, trr)
		fmt.Fprint(w, "\n")
	case GIrAVar:
		if a.VarVal != nil {
			fmt.Fprintf(w, "%svar %s", tabs, a.NameGo)
			fmt.Fprint(w, " = ")
			codeEmitAst(w, indent, a.VarVal, trr)
			fmt.Fprint(w, "\n")
		} else {
			fmt.Fprintf(w, "%s", a.NameGo)
		}
	case GIrABlock:
		fmt.Fprint(w, "{\n")
		indent++
		for _, expr := range a.Body {
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprintf(w, "%s}", tabs)
		indent--
	case GIrAIf:
		fmt.Fprintf(w, "%sif ", tabs)
		codeEmitAst(w, indent, a.If, trr)
		fmt.Fprint(w, " ")
		codeEmitAst(w, indent, a.Then, trr)
		if len(a.Else.Body) > 0 {
			fmt.Fprint(w, " else ")
			codeEmitAst(w, indent, a.Else, trr)
		}
		fmt.Fprint(w, "\n")
	case GIrACall:
		codeEmitAst(w, indent, a.Callee, trr)
		fmt.Fprint(w, "(")
		for i, expr := range a.CallArgs {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprint(w, ")")
	case GIrAFunc:
		codeEmitTypeDecl(w, &a.GIrANamedTypeRef, -1, trr)
		// fmt.Fprintf(w, "func %s(", a.NameGo)
		// for i, arg := range a.RefFunc.Args {
		// 	if i > 0 {
		// 		fmt.Fprint(w, ",")
		// 	}
		// 	fmt.Fprint(w, arg.NameGo)
		// }
		// fmt.Fprint(w, ") ")
		codeEmitAsts(w, indent, &a.GIrABlock, trr)
	case GIrAComments:
		for _, c := range a.Comments {
			if len(c.BlockComment) > 0 {
				fmt.Fprintf(w, "/*%s*/", c.BlockComment)
			} else {
				fmt.Fprintf(w, "%s//%s\n", tabs, c.LineComment)
			}
		}
		if a.CommentsDecl != nil {
			codeEmitAst(w, indent, a.CommentsDecl, trr)
		}
	case GIrALitObj:
		fmt.Fprint(w, "{")
		for i, namevaluepair := range a.ObjPairs {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprintf(w, "%s: ", namevaluepair.NameGo)
			codeEmitAst(w, indent, namevaluepair.VarVal, trr)
			break
		}
		fmt.Fprint(w, "}")
	case GIrARet:
		if a.RetArg == nil {
			fmt.Fprintf(w, "%sreturn\n", tabs)
		} else {
			fmt.Fprintf(w, "%sreturn ", tabs)
			codeEmitAst(w, indent, a.RetArg, trr)
			fmt.Fprint(w, "\n")
		}
	case GIrAPanic:
		fmt.Fprintf(w, "%spanic(", tabs)
		codeEmitAst(w, indent, a.PanicArg, trr)
		fmt.Fprint(w, ")\n")
	case GIrALitArr:
		fmt.Fprint(w, "[]ARRAY{")
		for i, expr := range a.ArrVals {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprint(w, "}")
	case GIrADot:
		codeEmitAst(w, indent, a.DotLeft, trr)
		fmt.Fprint(w, ".")
		codeEmitAst(w, indent, a.DotRight, trr)
	case GIrAIndex:
		codeEmitAst(w, indent, a.IdxLeft, trr)
		fmt.Fprint(w, "[")
		codeEmitAst(w, indent, a.IdxRight, trr)
		fmt.Fprint(w, "]")
	case GIrAIsType:
		codeEmitAst(w, indent, a.ExprToTest, trr)
		fmt.Fprint(w, " __IS__ ")
		codeEmitAst(w, indent, a.TypeToTest, trr)
	case GIrASet:
		fmt.Fprint(w, tabs)
		codeEmitAst(w, indent, a.Left, trr)
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.Right, trr)
		fmt.Fprint(w, "\n")
	case GIrAOp1:
		fmt.Fprintf(w, "(%s", a.Op1)
		codeEmitAst(w, indent, a.Right, trr)
		fmt.Fprint(w, ")")
	case GIrAOp2:
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
	case GIrANil:
		fmt.Fprint(w, "nil")
	case GIrAFor:
		if len(a.ForRange.NameGo) > 0 && a.ForRange.VarVal != nil {
			fmt.Fprintf(w, "%sfor _, %s := range ", tabs, a.ForRange.NameGo)
			codeEmitAst(w, indent, a.ForRange.VarVal, trr)
			codeEmitAsts(w, indent, &a.GIrABlock, trr)
		} else if len(a.ForInit) > 0 || len(a.ForStep) > 0 {
			fmt.Fprint(w, "for ")
			for i, finit := range a.ForInit {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}
				codeEmitAst(w, indent, finit.Left, trr)
			}
			fmt.Fprint(w, " = ")
			for i, finit := range a.ForInit {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}
				codeEmitAst(w, indent, finit.Right, trr)
			}
			fmt.Fprint(w, "; ")
			codeEmitAst(w, indent, a.ForCond, trr)
			fmt.Fprint(w, "; ")
			for i, fstep := range a.ForStep {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}
				codeEmitAst(w, indent, fstep.Left, trr)
			}
			fmt.Fprint(w, " = ")
			for i, fstep := range a.ForStep {
				if i > 0 {
					fmt.Fprint(w, ", ")
				}
				codeEmitAst(w, indent, fstep.Right, trr)
			}
			codeEmitAsts(w, indent, &a.GIrABlock, trr)
		} else {
			fmt.Fprintf(w, "%sfor ", tabs)
			codeEmitAst(w, indent, a.ForCond, trr)
			codeEmitAsts(w, indent, &a.GIrABlock, trr)
		}
	default:
		panic(fmt.Errorf("Unhandled AST: %v", ast))
	}
}

func codeEmitAsts(w io.Writer, indent int, asts *GIrABlock, trr goTypeRefResolver) {
	for _, ast := range asts.Body {
		codeEmitAst(w, indent, ast, trr)
	}
}

func codeEmitEnumConsts(buf io.Writer, enumconstnames []string, enumconsttype string) {
	fmt.Fprint(buf, "const (\n")
	fmt.Fprintf(buf, "\t_ %v= iota\n", strings.Repeat(" ", len(enumconsttype)+len(enumconstnames[0])))
	for i, enumconstname := range enumconstnames {
		fmt.Fprintf(buf, "\t%s", enumconstname)
		if i == 0 {
			fmt.Fprintf(buf, " %s = iota", enumconsttype)
		}
		fmt.Fprint(buf, "\n")
	}
	fmt.Fprint(buf, ")\n\n")
}

func codeEmitFuncArgs(w io.Writer, methodargs GIrANamedTypeRefs, indlevel int, typerefresolver goTypeRefResolver, isretargs bool) {
	parens := (!isretargs) || len(methodargs) > 1 || (len(methodargs) == 1 && len(methodargs[0].NameGo) > 0)
	if parens {
		fmt.Fprint(w, "(")
	}
	if len(methodargs) > 0 {
		for i, arg := range methodargs {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			if len(arg.NameGo) > 0 {
				fmt.Fprintf(w, "%s ", arg.NameGo)
			}
			codeEmitTypeDecl(w, arg, indlevel, typerefresolver)
		}
	}
	if parens {
		fmt.Fprint(w, ") ")
	} else if len(methodargs) > 0 {
		fmt.Fprint(w, " ")
	}
}

func codeEmitModImps(writer io.Writer, modimps []*GIrMPkgRef) {
	if len(modimps) > 0 {
		fmt.Fprint(writer, "import (\n")
		for _, modimp := range modimps {
			if modimp.used {
				fmt.Fprintf(writer, "\t%s %q\n", modimp.N, modimp.P)
			} else {
				fmt.Fprintf(writer, "\t// %s %q\n", modimp.N, modimp.P)
			}
		}
		fmt.Fprint(writer, ")\n\n")
	}
}

func codeEmitPkgDecl(writer io.Writer, pname string) {
	fmt.Fprintf(writer, "package %s\n\n", pname)
}

func codeEmitTypeAlias(buf io.Writer, tname string, ttype string) {
	fmt.Fprintf(buf, "type %s %s\n\n", tname, ttype)
}

func codeEmitTypeDecl(w io.Writer, gtd *GIrANamedTypeRef, indlevel int, typerefresolver goTypeRefResolver) {
	toplevel := indlevel == 0
	fmtembeds := "\t%s\n"
	if toplevel {
		fmt.Fprintf(w, "type %s ", gtd.NameGo)
	}
	if len(gtd.RefAlias) > 0 {
		fmt.Fprint(w, codeEmitTypeRef(typerefresolver(gtd.RefAlias, true)))
	} else if gtd.RefUnknown > 0 {
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
	} else if gtd.RefFunc != nil {
		ilev := indlevel
		if ilev == 0 {
			ilev += 1
		}
		fmt.Fprint(w, "func(")
		for i, l := 0, len(gtd.RefFunc.Args); i < l; i++ {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
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
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			if retname := gtd.RefFunc.Rets[i].NameGo; len(retname) > 0 {
				fmt.Fprintf(w, "%s ", retname)
			}
			codeEmitTypeDecl(w, gtd.RefFunc.Rets[i], ilev, typerefresolver)
		}
		if numrets > 1 {
			fmt.Fprint(w, ")")
		}
	}
	if toplevel {
		fmt.Fprintln(w, "\n")
	}
}

func codeEmitTypeMethods(w io.Writer, tr *GIrANamedTypeRef, typerefresolver goTypeRefResolver) {
	for _, method := range tr.Methods {
		if method.mCtor {
			fmt.Fprintf(w, "func %s", method.NameGo)
		} else if tr.RefStruct.PassByPtr {
			fmt.Fprintf(w, "func (this *%s) %s", tr.NameGo, method.NameGo)
		} else {
			fmt.Fprintf(w, "func (this %s) %s", tr.NameGo, method.NameGo)
		}
		codeEmitFuncArgs(w, method.RefFunc.Args, -1, typerefresolver, false)
		codeEmitFuncArgs(w, method.RefFunc.Rets, -1, typerefresolver, true)
		fmt.Fprint(w, "{\n")
		codeEmitAsts(w, 1, &method.mBody, typerefresolver)
		fmt.Fprintln(w, "}\n")
	}
}

func codeEmitTypeRef(pname string, tname string) string {
	if len(pname) == 0 {
		return tname
	}
	return pname + "." + tname
}
