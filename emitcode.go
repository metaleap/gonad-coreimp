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

func codeEmitCoreImp(w io.Writer, indent int, ast *CoreImpAst) {
	tabs := strings.Repeat("\t", indent)
	switch ast.AstTag {
	case "StringLiteral":
		fmt.Fprintf(w, "%q", ast.StringLiteral)
	case "BooleanLiteral":
		fmt.Fprintf(w, "%t", ast.BooleanLiteral)
	case "NumericLiteral_Double":
		fmt.Fprintf(w, "%f", ast.NumericLiteral_Double)
	case "NumericLiteral_Integer":
		fmt.Fprintf(w, "%d", ast.NumericLiteral_Integer)
	case "Var":
		fmt.Fprintf(w, "%s", ast.Var)
	case "Block":
		fmt.Fprint(w, "{\n")
		indent++
		for _, expr := range ast.Block {
			codeEmitCoreImp(w, indent, expr)
		}
		fmt.Fprintf(w, "%s}", tabs)
		indent--
	case "While":
		fmt.Fprintf(w, "%sfor ", tabs)
		codeEmitCoreImp(w, indent, ast.While)
		codeEmitCoreImp(w, indent, ast.AstBody)
	case "For":
		fmt.Fprintf(w, "%sfor %s ; ", tabs, ast.For)
		codeEmitCoreImp(w, indent, ast.AstFor1)
		fmt.Fprint(w, " ; ")
		codeEmitCoreImp(w, indent, ast.AstFor2)
		fmt.Fprint(w, " ")
		codeEmitCoreImp(w, indent, ast.AstBody)
	case "ForIn":
		fmt.Fprintf(w, "%sfor _, %s := range ", tabs, ast.ForIn)
		codeEmitCoreImp(w, indent, ast.AstFor1)
		codeEmitCoreImp(w, indent, ast.AstBody)
	case "IfElse":
		fmt.Fprintf(w, "%sif ", tabs)
		codeEmitCoreImp(w, indent, ast.IfElse)
		fmt.Fprint(w, " ")
		codeEmitCoreImp(w, indent, ast.AstThen)
		if ast.AstElse != nil {
			fmt.Fprint(w, " else ")
			codeEmitCoreImp(w, indent, ast.AstElse)
		}
		fmt.Fprint(w, "\n")
	case "App":
		codeEmitCoreImp(w, indent, ast.App)
		fmt.Fprint(w, "(")
		for i, expr := range ast.AstApplArgs {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			codeEmitCoreImp(w, indent, expr)
		}
		fmt.Fprint(w, ")")
	case "Function":
		fmt.Fprintf(w, "func %s(", ast.Function)
		for i, argname := range ast.AstFuncParams {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprint(w, argname)
		}
		fmt.Fprint(w, ") ")
		codeEmitCoreImp(w, indent, ast.AstBody)
	case "Unary":
		fmt.Fprint(w, "(")
		switch ast.AstOp {
		case "Negate", "-":
			fmt.Fprint(w, "-")
		case "Not", "!":
			fmt.Fprint(w, "!")
		case "Positive", "+":
			fmt.Fprint(w, "+")
		case "BitwiseNot", "^":
			fmt.Fprint(w, "^")
		default:
			fmt.Fprintf(w, "?%s?", ast.AstOp)
			panic("unrecognized unary op '" + ast.AstOp + "', please report!")
		}
		codeEmitCoreImp(w, indent, ast.Unary)
		fmt.Fprint(w, ")")
	case "Binary":
		fmt.Fprint(w, "(")
		codeEmitCoreImp(w, indent, ast.Binary)
		switch ast.AstOp {
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
			fmt.Fprintf(w, " ?%s? ", ast.AstOp)
			panic("unrecognized binary op '" + ast.AstOp + "', please report!")
		}
		codeEmitCoreImp(w, indent, ast.AstRight)
		fmt.Fprint(w, ")")
	case "VariableIntroduction":
		fmt.Fprintf(w, "%svar %s", tabs, ast.VariableIntroduction)
		if ast.AstRight != nil {
			fmt.Fprint(w, " = ")
			codeEmitCoreImp(w, indent, ast.AstRight)
		}
		fmt.Fprint(w, "\n")
	case "Comment":
		for _, c := range ast.Comment {
			if c != nil {
				if len(c.BlockComment) > 0 {
					fmt.Fprintf(w, "/*%s*/", c.BlockComment)
				} else {
					fmt.Fprintf(w, "%s//%s\n", tabs, c.LineComment)
				}
			}
		}
		if ast.AstCommentDecl != nil {
			codeEmitCoreImp(w, indent, ast.AstCommentDecl)
		}
	case "ObjectLiteral":
		fmt.Fprint(w, "{")
		for i, namevaluepair := range ast.ObjectLiteral {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			for onekey, oneval := range namevaluepair {
				fmt.Fprintf(w, "%s: ", onekey)
				codeEmitCoreImp(w, indent, oneval)
				break
			}
		}
		fmt.Fprint(w, "}")
	case "ReturnNoResult":
		fmt.Fprintf(w, "%sreturn\n", tabs)
	case "Return":
		fmt.Fprintf(w, "%sreturn ", tabs)
		codeEmitCoreImp(w, indent, ast.Return)
		fmt.Fprint(w, "\n")
	case "Throw":
		fmt.Fprintf(w, "%spanic(", tabs)
		codeEmitCoreImp(w, indent, ast.Throw)
		fmt.Fprint(w, ")\n")
	case "ArrayLiteral":
		fmt.Fprint(w, "[]ARRAY{")
		for i, expr := range ast.ArrayLiteral {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			codeEmitCoreImp(w, indent, expr)
		}
		fmt.Fprint(w, "}")
	case "Assignment":
		fmt.Fprint(w, tabs)
		codeEmitCoreImp(w, indent, ast.Assignment)
		fmt.Fprint(w, " = ")
		codeEmitCoreImp(w, indent, ast.AstRight)
		fmt.Fprint(w, "\n")
	case "Accessor":
		codeEmitCoreImp(w, indent, ast.Accessor)
		fmt.Fprintf(w, ".%s", ast.AstRight.Var)
	case "Indexer":
		codeEmitCoreImp(w, indent, ast.Indexer)
		// if ast.AstRight.AstTag == "StringLiteral" {
		fmt.Fprint(w, "[")
		codeEmitCoreImp(w, indent, ast.AstRight)
		fmt.Fprint(w, "]")
	case "InstanceOf":
		codeEmitCoreImp(w, indent, ast.InstanceOf)
		fmt.Fprint(w, " is ")
		codeEmitCoreImp(w, indent, ast.AstRight)
	default:
		panic("CoreImp unhandled AST-tag, please report: " + ast.AstTag)
	}
}

func codeEmitCoreImps(w io.Writer, indent int, body CoreImpAsts) {
	for _, ast := range body {
		codeEmitCoreImp(w, indent, ast)
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
		codeEmitCoreImps(w, 1, method.mBody)
		fmt.Fprintln(w, "}\n")
	}
}

func codeEmitTypeRef(pname string, tname string) string {
	if len(pname) == 0 {
		return tname
	}
	return pname + "." + tname
}
