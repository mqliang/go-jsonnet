/*
Copyright 2016 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jsonnet

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/parser"
)

type analysisState struct {
	err      error
	freeVars ast.IdentifierSet
}

func visitNext(a ast.Node, inObject bool, vars ast.IdentifierSet, state *analysisState) {
	if state.err != nil {
		return
	}
	state.err = analyzeVisit(a, inObject, vars)
	state.freeVars.Append(a.FreeVariables())
}

func analyzeVisit(a ast.Node, inObject bool, vars ast.IdentifierSet) error {
	s := &analysisState{freeVars: ast.NewIdentifierSet()}

	// TODO(sbarzowski) Test somehow that we're visiting all the nodes
	switch a.Type() {
	case ast.AST_APPLY:
		a := a.(*ast.Apply)
		visitNext(a.Target, inObject, vars, s)
		for _, arg := range a.Arguments.Positional {
			visitNext(arg, inObject, vars, s)
		}
		for _, arg := range a.Arguments.Named {
			visitNext(arg.Arg, inObject, vars, s)
		}
	case ast.AST_ARRAY:
		a := a.(*ast.Array)
		for _, elem := range a.Elements {
			visitNext(elem, inObject, vars, s)
		}
	case ast.AST_BINARY:
		a := a.(*ast.Binary)
		visitNext(a.Left, inObject, vars, s)
		visitNext(a.Right, inObject, vars, s)
	case ast.AST_CONDITIONAL:
		a := a.(*ast.Conditional)
		visitNext(a.Cond, inObject, vars, s)
		visitNext(a.BranchTrue, inObject, vars, s)
		visitNext(a.BranchFalse, inObject, vars, s)
	case ast.AST_ERROR:
		a := a.(*ast.Error)
		visitNext(a.Expr, inObject, vars, s)
	case ast.AST_FUNCTION:
		a := a.(*ast.Function)
		newVars := vars.Clone()
		for _, param := range a.Parameters.Required {
			newVars.Add(param)
		}
		for _, param := range a.Parameters.Optional {
			newVars.Add(param.Name)
		}
		for _, param := range a.Parameters.Optional {
			visitNext(param.DefaultArg, inObject, newVars, s)
		}
		visitNext(a.Body, inObject, newVars, s)
		// Parameters are free inside the body, but not visible here or outside
		for _, param := range a.Parameters.Required {
			s.freeVars.Remove(param)
		}
		for _, param := range a.Parameters.Optional {
			s.freeVars.Remove(param.Name)
		}
	case ast.AST_IMPORT:
		//nothing to do here
	case ast.AST_IMPORTSTR:
		//nothing to do here
	case ast.AST_IN_SUPER:
		a := a.(*ast.InSuper)
		if !inObject {
			return parser.MakeStaticError("Can't use super outside of an object.", *a.Loc())
		}
		visitNext(a.Index, inObject, vars, s)
	case ast.AST_SUPER_INDEX:
		a := a.(*ast.SuperIndex)
		if !inObject {
			return parser.MakeStaticError("Can't use super outside of an object.", *a.Loc())
		}
		visitNext(a.Index, inObject, vars, s)
	case ast.AST_INDEX:
		a := a.(*ast.Index)
		visitNext(a.Target, inObject, vars, s)
		visitNext(a.Index, inObject, vars, s)
	case ast.AST_LOCAL:
		a := a.(*ast.Local)
		newVars := vars.Clone()
		for _, bind := range a.Binds {
			newVars.Add(bind.Variable)
		}
		// Binds in local can be mutually or even self recursive
		for _, bind := range a.Binds {
			visitNext(bind.Body, inObject, newVars, s)
		}
		visitNext(a.Body, inObject, newVars, s)

		// Any usage of newly created variables inside are considered free
		// but they are not here or outside
		for _, bind := range a.Binds {
			s.freeVars.Remove(bind.Variable)
		}
	case ast.AST_LITERAL_BOOLEAN:
		//nothing to do here
	case ast.AST_LITERAL_NULL:
		//nothing to do here
	case ast.AST_LITERAL_NUMBER:
		//nothing to do here
	case ast.AST_LITERAL_STRING:
		//nothing to do here
	case ast.AST_DESUGARED_OBJECT:
		a := a.(*ast.DesugaredObject)
		for _, field := range a.Fields {
			// Field names are calculated *outside* of the object
			visitNext(field.Name, inObject, vars, s)
			visitNext(field.Body, true, vars, s)
		}
		for _, assert := range a.Asserts {
			visitNext(assert, true, vars, s)
		}
	case ast.AST_SELF:
		a := a.(*ast.Self)
		if !inObject {
			return parser.MakeStaticError("Can't use self outside of an object.", *a.Loc())
		}
	case ast.AST_UNARY:
		a := a.(*ast.Unary)
		visitNext(a.Expr, inObject, vars, s)
	case ast.AST_VAR:
		a := a.(*ast.Var)
		if !vars.Contains(a.Id) {
			return parser.MakeStaticError(fmt.Sprintf("Unknown variable: %v", a.Id), *a.Loc())
		}
		s.freeVars.Add(a.Id)
	default:
		panic(fmt.Sprintf("Unexpected node %#v", a))
	}
	a.SetFreeVariables(s.freeVars.ToOrderedSlice())
	return s.err
}

func analyze(node ast.Node) error {
	return analyzeVisit(node, false, ast.NewIdentifierSet("std"))
}
