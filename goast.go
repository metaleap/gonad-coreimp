package main

import (
	"errors"
	"fmt"
	"io"
	"path"
)

// "Go figure".. the go/ast & co were a bit too convoluted & impractical for the kind of usage needed here
type GoAst struct {
	modinfo *ModuleInfo
	proj    *BowerProject

	PkgName string
	Imports []*ModuleInfo
	Decls   []*CoreImpAst
}

func (me *GoAst) PopulateFrom(coreimp *CoreImp) (err error) {
	for _, ast := range coreimp.Body {
		switch ast.Ast_tag {
		case "StringLiteral", "Assignment":
		case "VariableIntroduction":
			if ast.Ast_rightHandSide != nil && ast.Ast_rightHandSide.App != nil && ast.Ast_rightHandSide.App.Var == "require" && len(ast.Ast_rightHandSide.Ast_appArgs) == 1 {
				// println("Skipped require()")
			} else {
				if ast.Ast_rightHandSide != nil && ast.Ast_rightHandSide.Ast_tag == "Function" {
					// turn top-level "var foo = func(..){..}" into top-level "func foo(..){..}"
					ast.Ast_rightHandSide.Function = ast.VariableIntroduction
					ast = ast.Ast_rightHandSide
				}
				me.Decls = append(me.Decls, ast)
			}
		case "Function", "Comment":
			me.Decls = append(me.Decls, ast)
		default:
			return errors.New(me.modinfo.impFilePath + ": unrecognized top-level tag, please report: " + ast.Ast_tag)
		}
	}
	return
}

var (
	warnedops = map[string]bool{}
)

func (me *GoAst) WriteTo(w io.Writer) (err error) {
	if _, err = fmt.Fprintf(w, "package %s\n", me.PkgName); err == nil {
		for _, impmod := range me.Imports {
			if _, err = fmt.Fprintf(w, "import %s %q\n", impmod.pName, path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath)); err != nil {
				return
			}
		}
		var printast func(ast *CoreImpAst)
		printast = func(ast *CoreImpAst) {
			switch ast.Ast_tag {
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
				fmt.Fprint(w, "{")
				for _, expr := range ast.Block {
					printast(expr)
				}
				fmt.Fprint(w, "}")
			case "While":
				fmt.Fprint(w, "\nfor ")
				printast(ast.While)
				printast(ast.Ast_body)
			case "For":
				fmt.Fprintf(w, "\nfor %s ; ", ast.For)
				printast(ast.Ast_for1)
				fmt.Fprint(w, " ; ")
				printast(ast.Ast_for2)
				fmt.Fprint(w, " ")
				printast(ast.Ast_body)
			case "ForIn":
				fmt.Fprintf(w, "\nfor _, %s := range ", ast.ForIn)
				printast(ast.Ast_for1)
				printast(ast.Ast_body)
			case "IfElse":
				fmt.Fprint(w, "if ")
				printast(ast.IfElse)
				fmt.Fprint(w, " ")
				printast(ast.Ast_ifThen)
				if ast.Ast_ifElse != nil {
					fmt.Fprint(w, " else ")
					printast(ast.Ast_ifElse)
				}
			case "App":
				printast(ast.App)
				fmt.Fprint(w, "(")
				for i, expr := range ast.Ast_appArgs {
					if i > 0 {
						fmt.Fprint(w, ",")
					}
					printast(expr)
				}
				fmt.Fprint(w, ")")
			case "Function":
				fmt.Fprintf(w, "func %s (", ast.Function)
				for i, argname := range ast.Ast_funcParams {
					if i > 0 {
						fmt.Fprint(w, ",")
					}
					fmt.Fprint(w, argname)
				}
				fmt.Fprint(w, ") ")
				printast(ast.Ast_body)
			case "Unary":
				fmt.Fprint(w, "(")
				switch ast.Ast_op {
				case "Negate":
					fmt.Fprint(w, "-")
				case "Not":
					fmt.Fprint(w, "!")
				case "Positive":
					fmt.Fprint(w, "+")
				case "BitwiseNot":
					fmt.Fprint(w, "^")
				default:
					fmt.Fprint(w, ast.Ast_op)
					if !warnedops[ast.Ast_op] {
						warnedops[ast.Ast_op] = true
						println(me.modinfo.srcFilePath + ": unrecognized unary op '" + ast.Ast_op + "', please report!")
					}
				}
				printast(ast.Unary)
				fmt.Fprint(w, ")")
			case "Binary":
				fmt.Fprint(w, "(")
				printast(ast.Binary)
				switch ast.Ast_op {
				case "Add":
					fmt.Fprint(w, "+")
				case "Subtract":
					fmt.Fprint(w, "-")
				case "Multiply":
					fmt.Fprint(w, "*")
				case "Divide":
					fmt.Fprint(w, "/")
				case "Modulus":
					fmt.Fprint(w, "%")
				case "EqualTo":
					fmt.Fprint(w, "==")
				case "NotEqualTo":
					fmt.Fprint(w, "!=")
				case "LessThan":
					fmt.Fprint(w, "<")
				case "LessThanOrEqualTo":
					fmt.Fprint(w, "<=")
				case "GreaterThan":
					fmt.Fprint(w, ">")
				case "GreaterThanOrEqualTo":
					fmt.Fprint(w, ">=")
				case "And":
					fmt.Fprint(w, "&&")
				case "Or":
					fmt.Fprint(w, "||")
				case "BitwiseAnd":
					fmt.Fprint(w, "&")
				case "BitwiseOr":
					fmt.Fprint(w, "|")
				case "BitwiseXor":
					fmt.Fprint(w, "^")
				case "ShiftLeft":
					fmt.Fprint(w, "<<")
				case "ShiftRight":
					fmt.Fprint(w, ">>")
				case "ZeroFillShiftRight":
					fmt.Fprint(w, "&^")
				default:
					fmt.Fprint(w, ast.Ast_op)
					if !warnedops[ast.Ast_op] {
						warnedops[ast.Ast_op] = true
						println(me.modinfo.srcFilePath + ": unrecognized binary op '" + ast.Ast_op + "', please report!")
					}
				}
				printast(ast.Ast_rightHandSide)
				fmt.Fprint(w, ")")
			case "VariableIntroduction":
				fmt.Fprintf(w, "\nvar %s", ast.VariableIntroduction)
				if ast.Ast_rightHandSide != nil {
					fmt.Fprint(w, " = ")
					printast(ast.Ast_rightHandSide)
				}
				fmt.Fprint(w, "\n")
			case "Comment":
				for _, c := range ast.Comment {
					if c != nil {
						if len(c.BlockComment) > 0 {
							fmt.Fprintf(w, "/*%s*/", c.BlockComment)
						} else {
							fmt.Fprintf(w, "\n//%s", c.LineComment)
						}
					}
				}
				if ast.Ast_decl != nil {
					printast(ast.Ast_decl)
				}
			case "ObjectLiteral":
				fmt.Fprint(w, "{")
				for i, namevaluepair := range ast.ObjectLiteral {
					if i > 0 {
						fmt.Fprint(w, ", ")
					}
					for onekey, oneval := range namevaluepair {
						fmt.Fprintf(w, "%s: ", onekey)
						printast(oneval)
						break
					}
				}
				fmt.Fprint(w, "}")
			case "ReturnNoResult":
				fmt.Fprint(w, "\nreturn\n")
			case "Return":
				fmt.Fprint(w, "return ")
				printast(ast.Return)
			case "Throw":
				fmt.Fprint(w, "return ")
				printast(ast.Throw)
			case "ArrayLiteral":
				fmt.Fprint(w, "[]notypeyet{")
				for i, expr := range ast.ArrayLiteral {
					if i > 0 {
						fmt.Fprint(w, ", ")
					}
					printast(expr)
				}
				fmt.Fprint(w, "}")
			case "Assignment":
				printast(ast.Assignment)
				fmt.Fprint(w, " = ")
				printast(ast.Ast_rightHandSide)
			case "Indexer":
				printast(ast.Ast_rightHandSide)
				fmt.Fprint(w, ".")
				printast(ast.Indexer)
			case "InstanceOf":
				printast(ast.InstanceOf)
				fmt.Fprint(w, " is ")
				printast(ast.Ast_rightHandSide)
			default:
				println(me.modinfo.srcFilePath + ": unhandled " + ast.Ast_tag)
			}
		}
		for _, topleveldecl := range me.Decls {
			printast(topleveldecl)
		}
	}
	return
}
