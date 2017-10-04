package main

// import (
// 	"errors"
// 	"fmt"
// 	"io"
// 	"path"
// 	"strings"
// )

// // "Go figure".. the go/ast & co were a bit too convoluted & impractical for the kind of usage needed here
// type GoAst struct {
// 	modinfo *ModuleInfo
// 	proj    *BowerProject

// 	PkgName string
// 	Imports []*ModuleInfo
// 	Decls   []*CoreImpAst
// }

// type GoAstTypeDecl struct {
// 	NtCtor struct {
// 		Name string
// 	}
// }

// func (me *GoAst) PopulateFrom(coreimp *CoreImp) (err error) {
// 	for _, ast := range coreimp.Body {
// 		switch ast.Ast_tag {
// 		case "VariableIntroduction", "Function", "Comment":
// 			me.Decls = append(me.Decls, ast)
// 		default:
// 			return errors.New(me.modinfo.impFilePath + ": unrecognized top-level tag, please report: " + ast.Ast_tag)
// 		}
// 	}
// 	return
// }

// var (
// 	warnedops = map[string]bool{}
// )

// func (me *GoAst) WriteTo(w io.Writer) {
// 	fmt.Fprintf(w, "package %s\n", me.PkgName)
// 	for _, impmod := range me.Imports {
// 		fmt.Fprintf(w, "\nimport %s %q", impmod.pName, path.Join(impmod.proj.GoOut.PkgDirPath, impmod.goOutDirPath))
// 	}
// 	fmt.Fprint(w, "\n\n")
// 	me.writeTypeDeclsTo(w)
// 	me.writeTopLevelDeclsTo(w)
// }

// func (me *GoAst) writeTypeDeclsTo(w io.Writer) {
// 	if ext := me.modinfo.ext; ext != nil {
// 		for _, d := range ext.EfDecls {
// 			if false && len(d.EDClass.Name) > 0 {
// 				me.writeTypeClassToInterface(w, d)
// 			}
// 		}
// 	}
// }

// func (me *GoAst) writeTypeClassToInterface(w io.Writer, d *PsExtDecl) {
// 	fmt.Fprintf(w, "type %s interface {\n", d.EDClass.Name)
// 	// for _, m := range d.EDClass.members {
// 	// 	fmt.Fprintf(w, "\t%s(", m.name)
// 	// 	for i, l := 0, len(m.argTypeNames); i < l; i++ {
// 	// 		if i == l-1 {
// 	// 			fmt.Fprint(w, ") ")
// 	// 		} else if i > 0 {
// 	// 			fmt.Fprint(w, ", ")
// 	// 		}
// 	// 		fmt.Fprint(w, m.argTypeNames[i])
// 	// 	}
// 	// 	fmt.Fprint(w, "\n")
// 	// }
// 	fmt.Fprintf(w, "}\n\n")
// }

// func (me *GoAst) writeTopLevelDeclsTo(w io.Writer) {
// 	var printast func(ast *CoreImpAst)
// 	indent := 0
// 	printast = func(ast *CoreImpAst) {
// 		tabs := strings.Repeat("\t", indent)
// 		switch ast.Ast_tag {
// 		case "StringLiteral":
// 			fmt.Fprintf(w, "%q", ast.StringLiteral)
// 		case "BooleanLiteral":
// 			fmt.Fprintf(w, "%t", ast.BooleanLiteral)
// 		case "NumericLiteral_Double":
// 			fmt.Fprintf(w, "%f", ast.NumericLiteral_Double)
// 		case "NumericLiteral_Integer":
// 			fmt.Fprintf(w, "%d", ast.NumericLiteral_Integer)
// 		case "Var":
// 			fmt.Fprintf(w, "%s", ast.Var)
// 		case "Block":
// 			fmt.Fprint(w, "{\n")
// 			indent++
// 			for _, expr := range ast.Block {
// 				printast(expr)
// 			}
// 			fmt.Fprintf(w, "%s}", tabs)
// 			indent--
// 		case "While":
// 			fmt.Fprintf(w, "%sfor ", tabs)
// 			printast(ast.While)
// 			printast(ast.Ast_body)
// 		case "For":
// 			fmt.Fprintf(w, "%sfor %s ; ", tabs, ast.For)
// 			printast(ast.Ast_for1)
// 			fmt.Fprint(w, " ; ")
// 			printast(ast.Ast_for2)
// 			fmt.Fprint(w, " ")
// 			printast(ast.Ast_body)
// 		case "ForIn":
// 			fmt.Fprintf(w, "%sfor _, %s := range ", tabs, ast.ForIn)
// 			printast(ast.Ast_for1)
// 			printast(ast.Ast_body)
// 		case "IfElse":
// 			fmt.Fprintf(w, "%sif ", tabs)
// 			printast(ast.IfElse)
// 			fmt.Fprint(w, " ")
// 			printast(ast.Ast_ifThen)
// 			if ast.Ast_ifElse != nil {
// 				fmt.Fprint(w, " else ")
// 				printast(ast.Ast_ifElse)
// 			}
// 			fmt.Fprint(w, "\n")
// 		case "App":
// 			printast(ast.App)
// 			fmt.Fprint(w, "(")
// 			for i, expr := range ast.Ast_appArgs {
// 				if i > 0 {
// 					fmt.Fprint(w, ",")
// 				}
// 				printast(expr)
// 			}
// 			fmt.Fprint(w, ")")
// 		case "Function":
// 			// if ast.typedecl != nil {
// 			// 	fmt.Fprint(w, "/*\n")
// 			// }
// 			fmt.Fprintf(w, "func %s(", ast.Function)
// 			for i, argname := range ast.Ast_funcParams {
// 				if i > 0 {
// 					fmt.Fprint(w, ",")
// 				}
// 				fmt.Fprint(w, argname)
// 			}
// 			fmt.Fprint(w, ") ")
// 			printast(ast.Ast_body)
// 			// if ast.typedecl != nil {
// 			// 	fmt.Fprint(w, "*/\n")
// 			// 	if len(ast.typedecl.NtCtor.Name) > 0 {
// 			// 		fmt.Fprintf(w, "type %s struct{}", ast.typedecl.NtCtor.Name)
// 			// 	}
// 			// }
// 		case "Unary":
// 			fmt.Fprint(w, "(")
// 			switch ast.Ast_op {
// 			case "Negate":
// 				fmt.Fprint(w, "-")
// 			case "Not":
// 				fmt.Fprint(w, "!")
// 			case "Positive":
// 				fmt.Fprint(w, "+")
// 			case "BitwiseNot":
// 				fmt.Fprint(w, "^")
// 			default:
// 				fmt.Fprintf(w, "?%s?", ast.Ast_op)
// 				if !warnedops[ast.Ast_op] {
// 					warnedops[ast.Ast_op] = true
// 					println(me.modinfo.srcFilePath + ": unrecognized unary op '" + ast.Ast_op + "', please report!")
// 				}
// 			}
// 			printast(ast.Unary)
// 			fmt.Fprint(w, ")")
// 		case "Binary":
// 			fmt.Fprint(w, "(")
// 			printast(ast.Binary)
// 			switch ast.Ast_op {
// 			case "Add":
// 				fmt.Fprint(w, "+")
// 			case "Subtract":
// 				fmt.Fprint(w, "-")
// 			case "Multiply":
// 				fmt.Fprint(w, "*")
// 			case "Divide":
// 				fmt.Fprint(w, "/")
// 			case "Modulus":
// 				fmt.Fprint(w, "%")
// 			case "EqualTo":
// 				fmt.Fprint(w, "==")
// 			case "NotEqualTo":
// 				fmt.Fprint(w, "!=")
// 			case "LessThan":
// 				fmt.Fprint(w, "<")
// 			case "LessThanOrEqualTo":
// 				fmt.Fprint(w, "<=")
// 			case "GreaterThan":
// 				fmt.Fprint(w, ">")
// 			case "GreaterThanOrEqualTo":
// 				fmt.Fprint(w, ">=")
// 			case "And":
// 				fmt.Fprint(w, "&&")
// 			case "Or":
// 				fmt.Fprint(w, "||")
// 			case "BitwiseAnd":
// 				fmt.Fprint(w, "&")
// 			case "BitwiseOr":
// 				fmt.Fprint(w, "|")
// 			case "BitwiseXor":
// 				fmt.Fprint(w, "^")
// 			case "ShiftLeft":
// 				fmt.Fprint(w, "<<")
// 			case "ShiftRight":
// 				fmt.Fprint(w, ">>")
// 			case "ZeroFillShiftRight":
// 				fmt.Fprint(w, "&^")
// 			default:
// 				fmt.Fprintf(w, "?%s?", ast.Ast_op)
// 				if !warnedops[ast.Ast_op] {
// 					warnedops[ast.Ast_op] = true
// 					println(me.modinfo.srcFilePath + ": unrecognized binary op '" + ast.Ast_op + "', please report!")
// 				}
// 			}
// 			printast(ast.Ast_rightHandSide)
// 			fmt.Fprint(w, ")")
// 		case "VariableIntroduction":
// 			fmt.Fprintf(w, "%svar %s", tabs, ast.VariableIntroduction)
// 			if ast.Ast_rightHandSide != nil {
// 				fmt.Fprint(w, " = ")
// 				printast(ast.Ast_rightHandSide)
// 			}
// 			fmt.Fprint(w, "\n")
// 		case "Comment":
// 			for _, c := range ast.Comment {
// 				if c != nil {
// 					if len(c.BlockComment) > 0 {
// 						fmt.Fprintf(w, "/*%s*/", c.BlockComment)
// 					} else {
// 						fmt.Fprintf(w, "%s//%s\n", tabs, c.LineComment)
// 					}
// 				}
// 			}
// 			if ast.Ast_decl != nil {
// 				printast(ast.Ast_decl)
// 			}
// 		case "ObjectLiteral":
// 			fmt.Fprint(w, "{")
// 			for i, namevaluepair := range ast.ObjectLiteral {
// 				if i > 0 {
// 					fmt.Fprint(w, ", ")
// 				}
// 				for onekey, oneval := range namevaluepair {
// 					fmt.Fprintf(w, "%s: ", onekey)
// 					printast(oneval)
// 					break
// 				}
// 			}
// 			fmt.Fprint(w, "}")
// 		case "ReturnNoResult":
// 			fmt.Fprintf(w, "%sreturn\n", tabs)
// 		case "Return":
// 			fmt.Fprintf(w, "%sreturn ", tabs)
// 			printast(ast.Return)
// 			fmt.Fprint(w, "\n")
// 		case "Throw":
// 			fmt.Fprintf(w, "%spanic(", tabs)
// 			printast(ast.Throw)
// 			fmt.Fprint(w, ")\n")
// 		case "ArrayLiteral":
// 			fmt.Fprint(w, "[]ARRAY{")
// 			for i, expr := range ast.ArrayLiteral {
// 				if i > 0 {
// 					fmt.Fprint(w, ", ")
// 				}
// 				printast(expr)
// 			}
// 			fmt.Fprint(w, "}")
// 		case "Assignment":
// 			fmt.Fprint(w, tabs)
// 			printast(ast.Assignment)
// 			fmt.Fprint(w, " = ")
// 			printast(ast.Ast_rightHandSide)
// 			fmt.Fprint(w, "\n")
// 		case "Indexer":
// 			printast(ast.Indexer)
// 			if ast.Ast_rightHandSide.Ast_tag == "StringLiteral" {
// 				fmt.Fprintf(w, ".%s", ast.Ast_rightHandSide.StringLiteral)
// 				// printast(ast.Ast_rightHandSide)
// 			} else {
// 				fmt.Fprint(w, "[")
// 				printast(ast.Ast_rightHandSide)
// 				fmt.Fprint(w, "]")
// 			}
// 		case "InstanceOf":
// 			printast(ast.InstanceOf)
// 			fmt.Fprint(w, " is ")
// 			printast(ast.Ast_rightHandSide)
// 		default:
// 			println(me.modinfo.srcFilePath + ": unhandled " + ast.Ast_tag)
// 		}
// 	}
// 	for _, topleveldecl := range me.Decls {
// 		if topleveldecl.Ast_sourceSpan != nil {
// 			// fmt.Fprint(w, "/*\n")
// 			// fmt.Fprintf(w, "%s %v %v", topleveldecl.Ast_sourceSpan.Name, topleveldecl.Ast_sourceSpan.Start, topleveldecl.Ast_sourceSpan.End)
// 			// fmt.Fprint(w, "*/\n")
// 		}
// 		printast(topleveldecl)
// 		fmt.Fprint(w, "\n\n\n")
// 	}
// }
