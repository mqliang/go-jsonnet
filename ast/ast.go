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

package ast

import (
	"fmt"
)

// Identifier represents a variable / parameter / field name.
//+gen set
type Identifier string
type Identifiers []Identifier

// TODO(jbeda) implement interning of identifiers if necessary.  The C++
// version does so.

type ASTType string

const (
	AST_APPLY                       ASTType = "AST_APPLY"
	AST_APPLY_BRACE                 ASTType = "AST_APPLY_BRACE"
	AST_ARRAY                       ASTType = "AST_ARRAY"
	AST_ARRAY_COMPREHENSION         ASTType = "AST_ARRAY_COMPREHENSION"
	AST_ASSERT                      ASTType = "AST_ASSERT"
	AST_BINARY                      ASTType = "AST_BINARY"
	AST_CONDITIONAL                 ASTType = "AST_CONDITIONAL"
	AST_DESUGARED_OBJECT            ASTType = "AST_DESUGARED_OBJECT"
	AST_DOLLAR                      ASTType = "AST_DOLLAR"
	AST_ERROR                       ASTType = "AST_ERROR"
	AST_FUNCTION                    ASTType = "AST_FUNCTION"
	AST_IMPORT                      ASTType = "AST_IMPORT"
	AST_IMPORTSTR                   ASTType = "AST_IMPORTSTR"
	AST_INDEX                       ASTType = "AST_INDEX"
	AST_SLICE                       ASTType = "AST_SLICE"
	AST_IN_SUPER                    ASTType = "AST_IN_SUPER"
	AST_LITERAL_BOOLEAN             ASTType = "AST_LITERAL_BOOLEAN"
	AST_LITERAL_NULL                ASTType = "AST_LITERAL_NULL"
	AST_LITERAL_NUMBER              ASTType = "AST_LITERAL_NUMBER"
	AST_LITERAL_STRING              ASTType = "AST_LITERAL_STRING"
	AST_LOCAL                       ASTType = "AST_LOCAL"
	AST_OBJECT                      ASTType = "AST_OBJECT"
	AST_OBJECT_COMPREHENSION        ASTType = "AST_OBJECT_COMPREHENSION"
	AST_SELF                        ASTType = "AST_SELF"
	AST_SUPER_INDEX                 ASTType = "AST_SUPER_INDEX"
	AST_UNARY                       ASTType = "AST_UNARY"
	AST_VAR                         ASTType = "AST_VAR"
)

// ---------------------------------------------------------------------------

type Context *string

type Node interface {
	Context() Context
	Loc() *LocationRange
	FreeVariables() Identifiers
	SetFreeVariables(Identifiers)
	SetContext(Context)
	Type() ASTType
}
type Nodes []Node

// ---------------------------------------------------------------------------

type NodeBase struct {
	loc           LocationRange
	context       Context
	freeVariables Identifiers
}

func NewNodeBase(loc LocationRange, freeVariables Identifiers) NodeBase {
	return NodeBase{
		loc:           loc,
		freeVariables: freeVariables,
	}
}

func NewNodeBaseLoc(loc LocationRange) NodeBase {
	return NodeBase{
		loc:           loc,
		freeVariables: []Identifier{},
	}
}

func (n *NodeBase) Loc() *LocationRange {
	return &n.loc
}

func (n *NodeBase) FreeVariables() Identifiers {
	return n.freeVariables
}

func (n *NodeBase) SetFreeVariables(idents Identifiers) {
	n.freeVariables = idents
}

func (n *NodeBase) Context() Context {
	return n.context
}

func (n *NodeBase) SetContext(context Context) {
	n.context = context
}

// ---------------------------------------------------------------------------

type IfSpec struct {
	Expr Node
}

// Example:
// expr for x in arr1 for y in arr2 for z in arr3
// The order is the same as in python, i.e. the leftmost is the outermost.
//
// Our internal representation reflects how they are semantically nested:
// ForSpec(z, outer=ForSpec(y, outer=ForSpec(x, outer=nil)))
// Any ifspecs are attached to the relevant ForSpec.
//
// Ifs are attached to the one on the left, for example:
// expr for x in arr1 for y in arr2 if x % 2 == 0 for z in arr3
// The if is attached to the y forspec.
//
// It desugares to:
// flatMap(\x ->
//         flatMap(\y ->
//                 flatMap(\z -> [expr], arr3)
//                 arr2)
//         arr3)
type ForSpec struct {
	VarName    Identifier
	Expr       Node
	Conditions []IfSpec
	Outer      *ForSpec
}

// ---------------------------------------------------------------------------

// Apply represents a function call
type Apply struct {
	NodeBase
	Target        Node
	Arguments     Arguments
	TrailingComma bool
	TailStrict    bool
}

func (n *Apply) Type() ASTType {
	return AST_APPLY
}

type NamedArgument struct {
	Name Identifier
	Arg  Node
}

type Arguments struct {
	Positional Nodes
	Named      []NamedArgument
}

// ---------------------------------------------------------------------------

// ApplyBrace represents e { }.  Desugared to e + { }.
type ApplyBrace struct {
	NodeBase
	Left  Node
	Right Node
}

func (n *ApplyBrace) Type() ASTType {
	return AST_APPLY_BRACE
}

// ---------------------------------------------------------------------------

// Array represents array constructors [1, 2, 3].
type Array struct {
	NodeBase
	Elements      Nodes
	TrailingComma bool
}

func (n *Array) Type() ASTType {
	return AST_ARRAY
}

// ---------------------------------------------------------------------------

// ArrayComp represents array comprehensions (which are like Python list
// comprehensions)
type ArrayComp struct {
	NodeBase
	Body          Node
	TrailingComma bool
	Spec          ForSpec
}

func (n *ArrayComp) Type() ASTType {
	return AST_ARRAY_COMPREHENSION
}

// ---------------------------------------------------------------------------

// Assert represents an assert expression (not an object-level assert).
//
// After parsing, message can be nil indicating that no message was
// specified. This AST is elimiated by desugaring.
type Assert struct {
	NodeBase
	Cond    Node
	Message Node
	Rest    Node
}

func (n *Assert) Type() ASTType {
	return AST_ASSERT
}

// ---------------------------------------------------------------------------

type BinaryOp int

const (
	BopMult BinaryOp = iota
	BopDiv
	BopPercent

	BopPlus
	BopMinus

	BopShiftL
	BopShiftR

	BopGreater
	BopGreaterEq
	BopLess
	BopLessEq
	BopIn

	BopManifestEqual
	BopManifestUnequal

	BopBitwiseAnd
	BopBitwiseXor
	BopBitwiseOr

	BopAnd
	BopOr
)

var bopStrings = []string{
	BopMult:    "*",
	BopDiv:     "/",
	BopPercent: "%",

	BopPlus:  "+",
	BopMinus: "-",

	BopShiftL: "<<",
	BopShiftR: ">>",

	BopGreater:   ">",
	BopGreaterEq: ">=",
	BopLess:      "<",
	BopLessEq:    "<=",
	BopIn:        "in",

	BopManifestEqual:   "==",
	BopManifestUnequal: "!=",

	BopBitwiseAnd: "&",
	BopBitwiseXor: "^",
	BopBitwiseOr:  "|",

	BopAnd: "&&",
	BopOr:  "||",
}

var BopMap = map[string]BinaryOp{
	"*": BopMult,
	"/": BopDiv,
	"%": BopPercent,

	"+": BopPlus,
	"-": BopMinus,

	"<<": BopShiftL,
	">>": BopShiftR,

	">":  BopGreater,
	">=": BopGreaterEq,
	"<":  BopLess,
	"<=": BopLessEq,
	"in": BopIn,

	"==": BopManifestEqual,
	"!=": BopManifestUnequal,

	"&": BopBitwiseAnd,
	"^": BopBitwiseXor,
	"|": BopBitwiseOr,

	"&&": BopAnd,
	"||": BopOr,
}

func (b BinaryOp) String() string {
	if b < 0 || int(b) >= len(bopStrings) {
		panic(fmt.Sprintf("INTERNAL ERROR: Unrecognised binary operator: %d", b))
	}
	return bopStrings[b]
}

// Binary represents binary operators.
type Binary struct {
	NodeBase
	Left  Node
	Op    BinaryOp
	Right Node
}

func (n *Binary) Type() ASTType {
	return AST_BINARY
}

// ---------------------------------------------------------------------------

// Conditional represents if/then/else.
//
// After parsing, branchFalse can be nil indicating that no else branch
// was specified.  The desugarer fills this in with a LiteralNull
type Conditional struct {
	NodeBase
	Cond        Node
	BranchTrue  Node
	BranchFalse Node
}

func (n *Conditional) Type() ASTType {
	return AST_CONDITIONAL
}

// ---------------------------------------------------------------------------

// Dollar represents the $ keyword
type Dollar struct{ NodeBase }

func (n *Dollar) Type() ASTType {
	return AST_DOLLAR
}

// ---------------------------------------------------------------------------

// Error represents the error e.
type Error struct {
	NodeBase
	Expr Node
}

func (n *Error) Type() ASTType {
	return AST_ERROR
}

// ---------------------------------------------------------------------------

// Function represents a function definition
type Function struct {
	NodeBase
	Parameters    Parameters
	TrailingComma bool
	Body          Node
}

func (n *Function) Type() ASTType {
	return AST_FUNCTION
}

type NamedParameter struct {
	Name       Identifier
	DefaultArg Node
}

type Parameters struct {
	Required Identifiers
	Optional []NamedParameter
}

// ---------------------------------------------------------------------------

// Import represents import "file".
type Import struct {
	NodeBase
	File *LiteralString
}

func (n *Import) Type() ASTType {
	return AST_IMPORT
}

// ---------------------------------------------------------------------------

// ImportStr represents importstr "file".
type ImportStr struct {
	NodeBase
	File *LiteralString
}

func (n *ImportStr) Type() ASTType {
	return AST_IMPORTSTR
}

// ---------------------------------------------------------------------------

// Index represents both e[e] and the syntax sugar e.f.
//
// One of index and id will be nil before desugaring.  After desugaring id
// will be nil.
type Index struct {
	NodeBase
	Target Node
	Index  Node
	Id     *Identifier
}

func (n *Index) Type() ASTType {
	return AST_INDEX
}

// ---------------------------------------------------------------------------
type Slice struct {
	NodeBase
	Target Node

	// Each of these can be nil
	BeginIndex Node
	EndIndex   Node
	Step       Node
}

func (n *Slice) Type() ASTType {
	return AST_SLICE
}

// ---------------------------------------------------------------------------

// LocalBind is a helper struct for astLocal
type LocalBind struct {
	Variable Identifier
	Body     Node
	Fun      *Function
}
type LocalBinds []LocalBind

// Local represents local x = e; e.  After desugaring, functionSugar is false.
type Local struct {
	NodeBase
	Binds LocalBinds
	Body  Node
}

func (n *Local) Type() ASTType {
	return AST_LOCAL
}

// ---------------------------------------------------------------------------

// LiteralBoolean represents true and false
type LiteralBoolean struct {
	NodeBase
	Value bool
}

func (n *LiteralBoolean) Type() ASTType {
	return AST_LITERAL_BOOLEAN
}

// ---------------------------------------------------------------------------

// LiteralNull represents the null keyword
type LiteralNull struct{ NodeBase }

func (n *LiteralNull) Type() ASTType {
	return AST_LITERAL_NULL
}

// ---------------------------------------------------------------------------

// LiteralNumber represents a JSON number
type LiteralNumber struct {
	NodeBase
	Value          float64
	OriginalString string
}

func (n *LiteralNumber) Type() ASTType {
	return AST_LITERAL_NUMBER
}

// ---------------------------------------------------------------------------

type LiteralStringKind int

const (
	StringSingle LiteralStringKind = iota
	StringDouble
	StringBlock
	VerbatimStringDouble
	VerbatimStringSingle
)

func (k LiteralStringKind) FullyEscaped() bool {
	switch k {
	case StringSingle, StringDouble:
		return true
	case StringBlock, VerbatimStringDouble, VerbatimStringSingle:
		return false
	}
	panic(fmt.Sprintf("Unknown string kind: %v", k))
}

// LiteralString represents a JSON string
type LiteralString struct {
	NodeBase
	Value       string
	Kind        LiteralStringKind
	BlockIndent string
}

func (n *LiteralString) Type() ASTType {
	return AST_LITERAL_STRING
}

// ---------------------------------------------------------------------------

type ObjectFieldKind int

const (
	ObjectAssert    ObjectFieldKind = iota // assert expr2 [: expr3]  where expr3 can be nil
	ObjectFieldID                          // id:[:[:]] expr2
	ObjectFieldExpr                        // '['expr1']':[:[:]] expr2
	ObjectFieldStr                         // expr1:[:[:]] expr2
	ObjectLocal                            // local id = expr2
)

type ObjectFieldHide int

const (
	ObjectFieldHidden  ObjectFieldHide = iota // f:: e
	ObjectFieldInherit                        // f: e
	ObjectFieldVisible                        // f::: e
)

// TODO(sbarzowski) consider having separate types for various kinds
type ObjectField struct {
	Kind          ObjectFieldKind
	Hide          ObjectFieldHide // (ignore if kind != astObjectField*)
	SuperSugar    bool            // +:  (ignore if kind != astObjectField*)
	MethodSugar   bool            // f(x, y, z): ...  (ignore if kind  == astObjectAssert)
	Method        *Function
	Expr1         Node // Not in scope of the object
	Id            *Identifier
	Params        *Parameters // If methodSugar == true then holds the params.
	TrailingComma bool        // If methodSugar == true then remembers the trailing comma
	Expr2, Expr3  Node        // In scope of the object (can see self).
}

func ObjectFieldLocalNoMethod(id *Identifier, body Node) ObjectField {
	return ObjectField{ObjectLocal, ObjectFieldVisible, false, false, nil, nil, id, nil, false, body, nil}
}

type ObjectFields []ObjectField

// Object represents object constructors { f: e ... }.
//
// The trailing comma is only allowed if len(fields) > 0.  Converted to
// DesugaredObject during desugaring.
type Object struct {
	NodeBase
	Fields        ObjectFields
	TrailingComma bool
}

func (n *Object) Type() ASTType {
	return AST_OBJECT
}

// ---------------------------------------------------------------------------

type DesugaredObjectField struct {
	Hide      ObjectFieldHide
	Name      Node
	Body      Node
	PlusSuper bool
}
type DesugaredObjectFields []DesugaredObjectField

// DesugaredObject represents object constructors { f: e ... } after
// desugaring.
//
// The assertions either return true or raise an error.
type DesugaredObject struct {
	NodeBase
	Asserts Nodes
	Fields  DesugaredObjectFields
}

func (n *DesugaredObject) Type() ASTType {
	return AST_DESUGARED_OBJECT
}

// ---------------------------------------------------------------------------

// ObjectComp represents object comprehension
//   { [e]: e for x in e for.. if... }.
type ObjectComp struct {
	NodeBase
	Fields        ObjectFields
	TrailingComma bool
	Spec          ForSpec
}

func (n *ObjectComp) Type() ASTType {
	return AST_OBJECT_COMPREHENSION
}

// ---------------------------------------------------------------------------

// Self represents the self keyword.
type Self struct{ NodeBase }

func (n *Self) Type() ASTType {
	return AST_SELF
}

// ---------------------------------------------------------------------------

// SuperIndex represents the super[e] and super.f constructs.
//
// Either index or identifier will be set before desugaring.  After desugaring, id will be
// nil.
type SuperIndex struct {
	NodeBase
	Index Node
	Id    *Identifier
}

func (n *SuperIndex) Type() ASTType {
	return AST_SUPER_INDEX
}

// ---------------------------------------------------------------------------

// Represents the e in super construct.
type InSuper struct {
	NodeBase
	Index Node
}

func (n *InSuper) Type() ASTType {
	return AST_IN_SUPER
}

// ---------------------------------------------------------------------------

type UnaryOp int

const (
	UopNot UnaryOp = iota
	UopBitwiseNot
	UopPlus
	UopMinus
)

var uopStrings = []string{
	UopNot:        "!",
	UopBitwiseNot: "~",
	UopPlus:       "+",
	UopMinus:      "-",
}

var UopMap = map[string]UnaryOp{
	"!": UopNot,
	"~": UopBitwiseNot,
	"+": UopPlus,
	"-": UopMinus,
}

func (u UnaryOp) String() string {
	if u < 0 || int(u) >= len(uopStrings) {
		panic(fmt.Sprintf("INTERNAL ERROR: Unrecognised unary operator: %d", u))
	}
	return uopStrings[u]
}

// Unary represents unary operators.
type Unary struct {
	NodeBase
	Op   UnaryOp
	Expr Node
}

func (n *Unary) Type() ASTType {
	return AST_UNARY
}

// ---------------------------------------------------------------------------

// Var represents variables.
type Var struct {
	NodeBase
	Id Identifier
}

func (n *Var) Type() ASTType {
	return AST_VAR
}

// ---------------------------------------------------------------------------
