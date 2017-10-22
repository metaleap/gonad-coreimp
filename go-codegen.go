package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/metaleap/go-util-str"
)

/*
Emitting Go code.
Ultimately (not 100% yet) in a go-fmt like format,
to save users that additional parse-codegen round-trip.
Looked briefly at using go/ast but that seemed more
ergonomic for dealing with parsed ASTs than synthesizing them.
By now we have our own intermediate-representation AST anyway
(ir-ast-*.go), allowing for all our transform needs.
*/

type goTypeRefResolver func(tref string, markused bool) (pname string, tname string)

const (
	dbgEmitEmptyFuncs                     = true
	areOverlappingInterfacesSupportedByGo = false // this might change hopefully, see https://github.com/golang/go/issues/6977
)

func codeEmitCommaIf(w io.Writer, i int) {
	if i > 0 {
		fmt.Fprint(w, ", ")
	}
}

func codeEmitComments(w io.Writer, singlelineprefix string, comments ...*coreImpComment) {
	for _, c := range comments {
		if len(c.BlockComment) > 0 {
			fmt.Fprintf(w, "/*%s*/", c.BlockComment)
		} else {
			fmt.Fprintf(w, "%s//%s\n", singlelineprefix, c.LineComment)
		}
	}
}

func codeEmitAst(w io.Writer, indent int, ast gIrA, trr goTypeRefResolver) {
	if ast == nil {
		return
	}
	tabs := ""
	if indent > 0 {
		tabs = strings.Repeat("\t", indent)
	}
	switch a := ast.(type) {
	case *gIrALitStr:
		fmt.Fprintf(w, "%q", a.LitStr)
	case *gIrALitBool:
		fmt.Fprintf(w, "%t", a.LitBool)
	case *gIrALitDouble:
		s := fmt.Sprintf("%f", a.LitDouble)
		for strings.HasSuffix(s, "0") {
			s = s[:len(s)-1]
		}
		fmt.Fprint(w, s)
	case *gIrALitInt:
		fmt.Fprintf(w, "%d", a.LitInt)
	case *gIrALitArr:
		codeEmitTypeDecl(w, &a.gIrANamedTypeRef, indent, trr)
		fmt.Fprint(w, "{")
		for i, expr := range a.ArrVals {
			codeEmitCommaIf(w, i)
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprint(w, "}")
	case *gIrALitObj:
		codeEmitTypeDecl(w, &a.gIrANamedTypeRef, -1, trr)
		fmt.Fprint(w, "{")
		for i, namevaluepair := range a.ObjFields {
			codeEmitCommaIf(w, i)
			if len(namevaluepair.NameGo) > 0 {
				fmt.Fprintf(w, "%s: ", namevaluepair.NameGo)
			}
			codeEmitAst(w, indent, namevaluepair.FieldVal, trr)
		}
		fmt.Fprint(w, "}")
	case *gIrAConst:
		fmt.Fprintf(w, "%sconst %s ", tabs, a.NameGo)
		codeEmitTypeDecl(w, &a.gIrANamedTypeRef, -1, trr)
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.ConstVal, trr)
		fmt.Fprint(w, "\n")
	case *gIrASym:
		fmt.Fprint(w, a.NameGo)
	case *gIrALet:
		fmt.Fprintf(w, "%svar %s", tabs, a.NameGo)
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.LetVal, trr)
		fmt.Fprint(w, "\n")
	case *gIrABlock:
		if dbgEmitEmptyFuncs && a != nil && a.parent != nil {
			codeEmitAst(w, indent, ªRet(nil), trr)
		} else if a == nil || len(a.Body) == 0 {
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
	case *gIrAIf:
		fmt.Fprintf(w, "%sif ", tabs)
		codeEmitAst(w, indent, a.If, trr)
		fmt.Fprint(w, " ")
		codeEmitAst(w, indent, a.Then, trr)
		if a.Else != nil {
			fmt.Fprint(w, " else ")
			codeEmitAst(w, indent, a.Else, trr)
		}
		fmt.Fprint(w, "\n")
	case *gIrACall:
		codeEmitAst(w, indent, a.Callee, trr)
		fmt.Fprint(w, "(")
		for i, expr := range a.CallArgs {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			codeEmitAst(w, indent, expr, trr)
		}
		fmt.Fprint(w, ")")
	case *gIrAFunc:
		codeEmitTypeDecl(w, &a.gIrANamedTypeRef, indent, trr)
		codeEmitAst(w, indent, a.FuncImpl, trr)
	case *gIrAComments:
		codeEmitComments(w, tabs, a.Comments...)
	case *gIrARet:
		if a.RetArg == nil {
			fmt.Fprintf(w, "%sreturn", tabs)
		} else {
			fmt.Fprintf(w, "%sreturn ", tabs)
			codeEmitAst(w, indent, a.RetArg, trr)
		}
		if indent >= 0 {
			fmt.Fprint(w, "\n")
		}
	case *gIrAPanic:
		fmt.Fprintf(w, "%spanic(", tabs)
		codeEmitAst(w, indent, a.PanicArg, trr)
		fmt.Fprint(w, ")\n")
	case *gIrADot:
		codeEmitAst(w, indent, a.DotLeft, trr)
		fmt.Fprint(w, ".")
		codeEmitAst(w, indent, a.DotRight, trr)
	case *gIrAIndex:
		codeEmitAst(w, indent, a.IdxLeft, trr)
		fmt.Fprint(w, "[")
		codeEmitAst(w, indent, a.IdxRight, trr)
		fmt.Fprint(w, "]")
	case *gIrAIsType:
		fmt.Fprint(w, "_,øĸ := ")
		codeEmitAst(w, indent, a.ExprToTest, trr)
		fmt.Fprint(w, ".(")
		fmt.Fprint(w, typeNameWithPkgName(trr(a.TypeToTest, true)))
		// codeEmitAst(w, indent, a.TypeToTest, trr)
		fmt.Fprint(w, "); øĸ")
	case *gIrAToType:
		if len(a.TypePkg) == 0 {
			fmt.Fprintf(w, "%s(", a.TypeName)
		} else {
			fmt.Fprintf(w, "%s.%s(", a.TypePkg, a.TypeName)
		}
		codeEmitAst(w, indent, a.ExprToCast, trr)
		fmt.Fprint(w, ")")
	case *gIrAPkgSym:
		if len(a.PkgName) > 0 {
			fmt.Fprintf(w, "%s.", a.PkgName)
		}
		fmt.Fprint(w, a.Symbol)
	case *gIrASet:
		fmt.Fprint(w, tabs)
		codeEmitAst(w, indent, a.SetLeft, trr)
		if a.isInVarGroup {
			fmt.Fprint(w, " ")
			codeEmitTypeDecl(w, &a.gIrANamedTypeRef, indent, trr)
		}
		fmt.Fprint(w, " = ")
		codeEmitAst(w, indent, a.ToRight, trr)
		fmt.Fprint(w, "\n")
	case *gIrAOp1:
		isinop := a.isParentOp()
		if isinop {
			fmt.Fprint(w, "(")
		}
		fmt.Fprint(w, a.Op1)
		codeEmitAst(w, indent, a.Of, trr)
		if isinop {
			fmt.Fprint(w, ")")
		}
	case *gIrAOp2:
		isinop := a.isParentOp()
		if isinop {
			fmt.Fprint(w, "(")
		}
		codeEmitAst(w, indent, a.Left, trr)
		fmt.Fprintf(w, " %s ", a.Op2)
		codeEmitAst(w, indent, a.Right, trr)
		if isinop {
			fmt.Fprint(w, ")")
		}
	case *gIrANil:
		fmt.Fprint(w, "nil")
	case *gIrAFor:
		if a.ForRange != nil {
			fmt.Fprintf(w, "%sfor _, %s := range ", tabs, a.ForRange.NameGo)
			codeEmitAst(w, indent, a.ForRange.LetVal, trr)
			codeEmitAst(w, indent, a.ForDo, trr)
		} else if len(a.ForInit) > 0 || len(a.ForStep) > 0 {
			fmt.Fprint(w, "for ")

			for i, finit := range a.ForInit {
				codeEmitCommaIf(w, i)
				fmt.Fprint(w, finit.NameGo)
			}
			fmt.Fprint(w, " := ")
			for i, finit := range a.ForInit {
				codeEmitCommaIf(w, i)
				codeEmitAst(w, indent, finit.LetVal, trr)
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

func codeEmitGroupedVals(w io.Writer, indent int, consts bool, asts []gIrA, trr goTypeRefResolver) {
	if l := len(asts); l == 1 {
		codeEmitAst(w, indent, asts[0], trr)
	} else if l > 1 {
		if consts {
			fmt.Fprint(w, "const (\n")
		} else {
			fmt.Fprint(w, "var (\n")
		}
		valºnameºtype := func(a gIrA) (val gIrA, name string, typeref *gIrANamedTypeRef) {
			if ac, _ := a.(*gIrAConst); ac != nil && consts {
				val, name, typeref = ac.ConstVal, ac.NameGo, &ac.gIrANamedTypeRef
			} else if av, _ := a.(*gIrALet); av != nil {
				val, name, typeref = av.LetVal, av.NameGo, &av.gIrANamedTypeRef
			}
			return
		}
		for i, a := range asts {
			val, name, typeref := valºnameºtype(a)
			codeEmitAst(w, indent+1, ªsetVarInGroup(name, val, typeref), trr)
			if i < (len(asts) - 1) {
				if _, ok := asts[i+1].(*gIrAComments); ok {
					fmt.Fprint(w, "\n")
				}
			}
		}
		fmt.Fprint(w, ")\n\n")
	}
}

// func codeEmitEnumConsts(w io.Writer, enumconstnames []string, enumconsttype string) {
// 	fmt.Fprint(w, "const (\n")
// 	fmt.Fprintf(w, "\t_ %v= iota\n", strings.Repeat(" ", len(enumconsttype)+len(enumconstnames[0])))
// 	for i, enumconstname := range enumconstnames {
// 		fmt.Fprintf(w, "\t%s", enumconstname)
// 		if i == 0 {
// 			fmt.Fprintf(w, " %s = iota", enumconsttype)
// 		}
// 		fmt.Fprint(w, "\n")
// 	}
// 	fmt.Fprint(w, ")\n\n")
// }

func codeEmitFuncArgs(w io.Writer, methodargs gIrANamedTypeRefs, typerefresolver goTypeRefResolver, isretargs bool, withnames bool) {
	if dbgEmitEmptyFuncs && isretargs && withnames {
		methodargs[0].NameGo = "ret"
	}
	parens := (!isretargs) || len(methodargs) > 1 || (len(methodargs) == 1 && len(methodargs[0].NameGo) > 0)
	if parens {
		fmt.Fprint(w, "(")
	}
	if len(methodargs) > 0 {
		for i, arg := range methodargs {
			codeEmitCommaIf(w, i)
			if withnames && len(arg.NameGo) > 0 {
				fmt.Fprintf(w, "%s ", arg.NameGo)
			}
			codeEmitTypeDecl(w, arg, -1, typerefresolver)
		}
	}
	if parens {
		fmt.Fprint(w, ")")
	}
	if !isretargs {
		fmt.Fprint(w, " ")
	}
}

func codeEmitModImps(w io.Writer, modimps []*gIrMPkgRef) {
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

// func codeEmitTypeAlias(w io.Writer, tname string, ttype string) {
// 	fmt.Fprintf(w, "type %s %s\n\n", tname, ttype)
// }

func codeEmitTypeDecl(w io.Writer, gtd *gIrANamedTypeRef, indlevel int, typerefresolver goTypeRefResolver) {
	if gtd == nil {
		fmt.Fprint(w, "interface{/*gIrANamedTypeRef=Nil*/}")
		return
	}
	toplevel := (indlevel == 0)
	fmtembeds := "\t%s\n"
	isfuncwithbodynotjustsig := gtd.RefFunc != nil && gtd.RefFunc.impl != nil
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
					codeEmitFuncArgs(&buf, ifmethod.RefFunc.Args, typerefresolver, false, false)
					codeEmitFuncArgs(&buf, ifmethod.RefFunc.Rets, typerefresolver, true, false)
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
		fmt.Fprint(w, "func")
		if isfuncwithbodynotjustsig && len(gtd.NameGo) > 0 {
			fmt.Fprintf(w, " %s", gtd.NameGo)
		}
		codeEmitFuncArgs(w, gtd.RefFunc.Args, typerefresolver, false, isfuncwithbodynotjustsig)
		codeEmitFuncArgs(w, gtd.RefFunc.Rets, typerefresolver, true, isfuncwithbodynotjustsig)
	} else {
		fmt.Fprint(w, "interface{/*EmptyNotNil*/}")
	}
	if toplevel && !isfuncwithbodynotjustsig {
		fmt.Fprint(w, "\n\n")
	}
}

func codeEmitStructMethods(w io.Writer, tr *gIrANamedTypeRef, typerefresolver goTypeRefResolver) {
	if tr.RefStruct != nil && len(tr.RefStruct.Methods) > 0 {
		for _, method := range tr.RefStruct.Methods {
			mthis := "_"
			if tr.RefStruct.PassByPtr {
				fmt.Fprintf(w, "func (%s *%s) %s", mthis, tr.NameGo, method.NameGo)
			} else {
				fmt.Fprintf(w, "func (%s %s) %s", mthis, tr.NameGo, method.NameGo)
			}
			codeEmitFuncArgs(w, method.RefFunc.Args, typerefresolver, false, true)
			codeEmitFuncArgs(w, method.RefFunc.Rets, typerefresolver, true, true)
			fmt.Fprint(w, " ")
			codeEmitAst(w, 0, method.RefFunc.impl, typerefresolver)
			fmt.Fprint(w, "\n")
		}
		fmt.Fprint(w, "\n")
	}
}

func codeEmitTypeRef(pname string, tname string) string {
	if len(pname) == 0 {
		return tname
	}
	return pname + "." + tname
}
