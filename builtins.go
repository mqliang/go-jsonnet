/*
Copyright 2017 Google Inc. All rights reserved.

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
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"sort"

	"github.com/google/go-jsonnet/ast"
)

// TODO(sbarzowski) Is this the best option? It's the first one that worked for me...
//go:generate esc -o std.go -pkg=jsonnet std/std.jsonnet

func getStdCode() string {
	return FSMustString(false, "/std/std.jsonnet")
}

func builtinPlus(e *evaluator, xp, yp potentialValue) (value, error) {
	// TODO(sbarzowski) more types, mixing types
	// TODO(sbarzowski) perhaps a more elegant way to dispatch
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	y, err := e.evaluate(yp)
	if err != nil {
		return nil, err
	}
	switch right := y.(type) {
	case *valueString:
		left, err := builtinToString(e, xp)
		if err != nil {
			return nil, err
		}
		return concatStrings(left.(*valueString), right), nil

	}
	switch left := x.(type) {
	case *valueNumber:
		right, err := e.getNumber(y)
		if err != nil {
			return nil, err
		}
		return makeValueNumber(left.value + right.value), nil
	case *valueString:
		right, err := builtinToString(e, yp)
		if err != nil {
			return nil, err
		}
		return concatStrings(left, right.(*valueString)), nil
	case valueObject:
		right, err := e.getObject(y)
		if err != nil {
			return nil, err
		}
		return makeValueExtendedObject(left, right), nil
	case *valueArray:
		right, err := e.getArray(y)
		if err != nil {
			return nil, err
		}
		return concatArrays(left, right), nil
	default:
		return nil, e.typeErrorGeneral(x)
	}
}

func builtinMinus(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	y, err := e.evaluateNumber(yp)
	if err != nil {
		return nil, err
	}
	return makeValueNumber(x.value - y.value), nil
}

func builtinMult(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	y, err := e.evaluateNumber(yp)
	if err != nil {
		return nil, err
	}
	return makeValueNumber(x.value * y.value), nil
}

func builtinDiv(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	y, err := e.evaluateNumber(yp)
	if err != nil {
		return nil, err
	}
	if y.value == 0 {
		return nil, e.Error("Division by zero.")
	}
	return makeDoubleCheck(e, x.value/y.value)
}

func builtinModulo(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	y, err := e.evaluateNumber(yp)
	if err != nil {
		return nil, err
	}
	if y.value == 0 {
		return nil, e.Error("Division by zero.")
	}
	return makeDoubleCheck(e, math.Mod(x.value, y.value))
}

func builtinLess(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	switch left := x.(type) {
	case *valueNumber:
		right, err := e.evaluateNumber(yp)
		if err != nil {
			return nil, err
		}
		return makeValueBoolean(left.value < right.value), nil
	case *valueString:
		right, err := e.evaluateString(yp)
		if err != nil {
			return nil, err
		}
		return makeValueBoolean(stringLessThan(left, right)), nil
	default:
		return nil, e.typeErrorGeneral(x)
	}
}

func builtinGreater(e *evaluator, xp, yp potentialValue) (value, error) {
	return builtinLess(e, yp, xp)
}

func builtinGreaterEq(e *evaluator, xp, yp potentialValue) (value, error) {
	res, err := builtinLess(e, xp, yp)
	if err != nil {
		return nil, err
	}
	return res.(*valueBoolean).not(), nil
}

func builtinLessEq(e *evaluator, xp, yp potentialValue) (value, error) {
	res, err := builtinGreater(e, xp, yp)
	if err != nil {
		return nil, err
	}
	return res.(*valueBoolean).not(), nil
}

func builtinAnd(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluateBoolean(xp)
	if err != nil {
		return nil, err
	}
	if !x.value {
		return x, nil
	}
	y, err := e.evaluateBoolean(yp)
	if err != nil {
		return nil, err
	}
	return y, nil
}

func builtinOr(e *evaluator, xp, yp potentialValue) (value, error) {
	x, err := e.evaluateBoolean(xp)
	if err != nil {
		return nil, err
	}
	if x.value {
		return x, nil
	}
	y, err := e.evaluateBoolean(yp)
	if err != nil {
		return nil, err
	}
	return y, nil
}

func builtinLength(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	var num int
	switch x := x.(type) {
	case valueObject:
		num = len(objectFields(x, withoutHidden))
	case *valueArray:
		num = len(x.elements)
	case *valueString:
		num = x.length()
	case *valueFunction:
		num = len(x.parameters())
	default:
		return nil, e.typeErrorGeneral(x)
	}
	return makeValueNumber(float64(num)), nil
}

func builtinToString(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	switch x := x.(type) {
	case *valueString:
		return x, nil
	}
	var buf bytes.Buffer
	err = e.i.manifestJSON(e.trace, x, false, "", &buf)
	if err != nil {
		return nil, err
	}
	return makeValueString(buf.String()), nil
}

func builtinMakeArray(e *evaluator, szp potentialValue, funcp potentialValue) (value, error) {
	sz, err := e.evaluateNumber(szp)
	if err != nil {
		return nil, err
	}
	fun, err := e.evaluateFunction(funcp)
	if err != nil {
		return nil, err
	}
	num := int(sz.value)
	var elems []potentialValue
	for i := 0; i < num; i++ {
		elem := fun.call(args(&readyValue{intToValue(i)}))
		elems = append(elems, elem)
	}
	return makeValueArray(elems), nil
}

func builtinFlatMap(e *evaluator, funcp potentialValue, arrp potentialValue) (value, error) {
	arr, err := e.evaluateArray(arrp)
	if err != nil {
		return nil, err
	}
	fun, err := e.evaluateFunction(funcp)
	if err != nil {
		return nil, err
	}
	num := int(arr.length())
	// Start with capacity of the original array.
	// This may spare us a few reallocations.
	// TODO(sbarzowski) verify that it actually helps
	elems := make([]potentialValue, 0, num)
	for i := 0; i < num; i++ {
		returned, err := e.evaluateArray(fun.call(args(arr.elements[i])))
		if err != nil {
			return nil, err
		}
		for _, elem := range returned.elements {
			elems = append(elems, elem)
		}
	}
	return makeValueArray(elems), nil
}

func builtinFilter(e *evaluator, funcp potentialValue, arrp potentialValue) (value, error) {
	arr, err := e.evaluateArray(arrp)
	if err != nil {
		return nil, err
	}
	fun, err := e.evaluateFunction(funcp)
	if err != nil {
		return nil, err
	}
	num := int(arr.length())
	// Start with capacity of the original array.
	// This may spare us a few reallocations.
	// TODO(sbarzowski) verify that it actually helps
	elems := make([]potentialValue, 0, num)
	for i := 0; i < num; i++ {
		included, err := e.evaluateBoolean(fun.call(args(arr.elements[i])))
		if err != nil {
			return nil, err
		}
		if included.value {
			elems = append(elems, arr.elements[i])
		}
	}
	return makeValueArray(elems), nil
}

func builtinNegation(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluateBoolean(xp)
	if err != nil {
		return nil, err
	}
	return makeValueBoolean(!x.value), nil
}

func builtinBitNeg(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	i := int64(x.value)
	return int64ToValue(^i), nil
}

func builtinIdentity(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	return x, nil
}

func builtinUnaryMinus(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	return makeValueNumber(-x.value), nil
}

func primitiveEquals(e *evaluator, xp potentialValue, yp potentialValue) (value, error) {
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	y, err := e.evaluate(yp)
	if err != nil {
		return nil, err
	}
	if x.typename() != y.typename() { // TODO(sbarzowski) ugh, string comparison
		return makeValueBoolean(false), nil
	}
	switch left := x.(type) {
	case *valueBoolean:
		right, err := e.getBoolean(y)
		if err != nil {
			return nil, err
		}
		return makeValueBoolean(left.value == right.value), nil
	case *valueNumber:
		right, err := e.getNumber(y)
		if err != nil {
			return nil, err
		}
		return makeValueBoolean(left.value == right.value), nil
	case *valueString:
		right, err := e.getString(y)
		if err != nil {
			return nil, err
		}
		return makeValueBoolean(stringEqual(left, right)), nil
	case *valueNull:
		return makeValueBoolean(true), nil
	case *valueFunction:
		return nil, e.Error("Cannot test equality of functions")
	default:
		return nil, e.Error(
			"primitiveEquals operates on primitive types, got " + x.typename(),
		)
	}
}

func builtinType(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluate(xp)
	if err != nil {
		return nil, err
	}
	return makeValueString(x.typename()), nil
}

func builtinMd5(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluateString(xp)
	if err != nil {
		return nil, err
	}
	hash := md5.Sum([]byte(string(x.value)))
	return makeValueString(hex.EncodeToString(hash[:])), nil
}

// Maximum allowed unicode codepoint
// https://en.wikipedia.org/wiki/Unicode#Architecture_and_terminology
const codepointMax = 0x10FFFF

func builtinChar(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluateNumber(xp)
	if err != nil {
		return nil, err
	}
	if x.value > codepointMax {
		return nil, e.Error(fmt.Sprintf("Invalid unicode codepoint, got %v", x.value))
	} else if x.value < 0 {
		return nil, e.Error(fmt.Sprintf("Codepoints must be >= 0, got %v", x.value))
	}
	return makeValueString(string(rune(x.value))), nil
}

func builtinCodepoint(e *evaluator, xp potentialValue) (value, error) {
	x, err := e.evaluateString(xp)
	if err != nil {
		return nil, err
	}
	if x.length() != 1 {
		return nil, e.Error(fmt.Sprintf("codepoint takes a string of length 1, got length %v", x.length()))
	}
	return makeValueNumber(float64(x.value[0])), nil
}

func makeDoubleCheck(e *evaluator, x float64) (value, error) {
	if math.IsNaN(x) {
		return nil, e.Error("Not a number")
	}
	if math.IsInf(x, 0) {
		return nil, e.Error("Overflow")
	}
	return makeValueNumber(x), nil
}

func liftNumeric(f func(float64) float64) func(*evaluator, potentialValue) (value, error) {
	return func(e *evaluator, xp potentialValue) (value, error) {
		x, err := e.evaluateNumber(xp)
		if err != nil {
			return nil, err
		}
		return makeDoubleCheck(e, f(x.value))
	}
}

var builtinSqrt = liftNumeric(math.Sqrt)
var builtinCeil = liftNumeric(math.Ceil)
var builtinFloor = liftNumeric(math.Floor)
var builtinSin = liftNumeric(math.Sin)
var builtinCos = liftNumeric(math.Cos)
var builtinTan = liftNumeric(math.Tan)
var builtinAsin = liftNumeric(math.Asin)
var builtinAcos = liftNumeric(math.Acos)
var builtinAtan = liftNumeric(math.Atan)
var builtinLog = liftNumeric(math.Log)
var builtinExp = liftNumeric(math.Exp)
var builtinMantissa = liftNumeric(func(f float64) float64 {
	mantissa, _ := math.Frexp(f)
	return mantissa
})
var builtinExponent = liftNumeric(func(f float64) float64 {
	_, exponent := math.Frexp(f)
	return float64(exponent)
})

func liftBitwise(f func(int64, int64) int64) func(*evaluator, potentialValue, potentialValue) (value, error) {
	return func(e *evaluator, xp, yp potentialValue) (value, error) {
		x, err := e.evaluateNumber(xp)
		if err != nil {
			return nil, err
		}
		y, err := e.evaluateNumber(yp)
		if err != nil {
			return nil, err
		}
		xInt := int64(x.value)
		yInt := int64(y.value)
		return makeDoubleCheck(e, float64(f(xInt, yInt)))
	}
}

// TODO(sbarzowski) negative shifts
var builtinShiftL = liftBitwise(func(x, y int64) int64 { return x << uint(y) })
var builtinShiftR = liftBitwise(func(x, y int64) int64 { return x >> uint(y) })
var builtinBitwiseAnd = liftBitwise(func(x, y int64) int64 { return x & y })
var builtinBitwiseOr = liftBitwise(func(x, y int64) int64 { return x | y })
var builtinBitwiseXor = liftBitwise(func(x, y int64) int64 { return x ^ y })

func builtinObjectFieldsEx(e *evaluator, objp potentialValue, includeHiddenP potentialValue) (value, error) {
	obj, err := e.evaluateObject(objp)
	if err != nil {
		return nil, err
	}
	includeHidden, err := e.evaluateBoolean(includeHiddenP)
	if err != nil {
		return nil, err
	}
	fields := objectFields(obj, withHiddenFromBool(includeHidden.value))
	sort.Strings(fields)
	elems := []potentialValue{}
	for _, fieldname := range fields {
		elems = append(elems, &readyValue{makeValueString(fieldname)})
	}
	return makeValueArray(elems), nil
}

func builtinObjectHasEx(e *evaluator, objp potentialValue, fnamep potentialValue, includeHiddenP potentialValue) (value, error) {
	obj, err := e.evaluateObject(objp)
	if err != nil {
		return nil, err
	}
	fname, err := e.evaluateString(fnamep)
	if err != nil {
		return nil, err
	}
	includeHidden, err := e.evaluateBoolean(includeHiddenP)
	if err != nil {
		return nil, err
	}
	h := withHiddenFromBool(includeHidden.value)
	fieldp := tryObjectIndex(objectBinding(obj), string(fname.value), h)
	return makeValueBoolean(fieldp != nil), nil
}

func builtinPow(e *evaluator, basep potentialValue, expp potentialValue) (value, error) {
	base, err := e.evaluateNumber(basep)
	if err != nil {
		return nil, err
	}
	exp, err := e.evaluateNumber(expp)
	if err != nil {
		return nil, err
	}
	return makeDoubleCheck(e, math.Pow(base.value, exp.value))
}

func builtinUglyObjectFlatMerge(e *evaluator, objarrp potentialValue) (value, error) {
	objarr, err := e.evaluateArray(objarrp)
	if err != nil {
		return nil, err
	}
	if len(objarr.elements) == 0 {
		return &valueSimpleObject{}, nil
	}
	newFields := make(valueSimpleObjectFieldMap)
	for _, elem := range objarr.elements {
		obj, err := e.evaluateObject(elem)
		if err != nil {
			return nil, err
		}
		// starts getting ugly - we mess with object internals
		simpleObj := obj.(*valueSimpleObject)
		for fieldName, fieldVal := range simpleObj.fields {
			if _, alreadyExists := newFields[fieldName]; alreadyExists {
				return nil, e.Error(duplicateFieldNameErrMsg(fieldName))
			}
			newFields[fieldName] = valueSimpleObjectField{
				hide: fieldVal.hide,
				field: &bindingsUnboundField{
					inner:    fieldVal.field,
					bindings: simpleObj.upValues,
				},
			}
		}
	}
	return makeValueSimpleObject(
		nil, // no binding frame
		newFields,
		[]unboundField{}, // No asserts allowed
	), nil
}

func builtinExtVar(e *evaluator, namep potentialValue) (value, error) {
	name, err := e.evaluateString(namep)
	if err != nil {
		return nil, err
	}
	index := ast.Identifier(name.getString())
	if pv, ok := e.i.extVars[index]; ok {
		return e.evaluate(pv)
	}
	return nil, e.Error("Undefined external variable: " + string(index))
}

type unaryBuiltin func(*evaluator, potentialValue) (value, error)
type binaryBuiltin func(*evaluator, potentialValue, potentialValue) (value, error)
type ternaryBuiltin func(*evaluator, potentialValue, potentialValue, potentialValue) (value, error)

type UnaryBuiltin struct {
	name       ast.Identifier
	function   unaryBuiltin
	parameters ast.Identifiers
}

func getBuiltinEvaluator(e *evaluator, name ast.Identifier) *evaluator {
	loc := ast.MakeLocationRangeMessage("<builtin>")
	context := TraceContext{Name: "builtin function <" + string(name) + ">"}
	trace := TraceElement{loc: &loc, context: &context}
	return &evaluator{i: e.i, trace: &trace}
}

func (b *UnaryBuiltin) EvalCall(args callArguments, e *evaluator) (value, error) {

	// TODO check args
	return b.function(getBuiltinEvaluator(e, b.name), args.positional[0])
}

func (b *UnaryBuiltin) Parameters() ast.Identifiers {
	return b.parameters
}

type BinaryBuiltin struct {
	name       ast.Identifier
	function   binaryBuiltin
	parameters ast.Identifiers
}

func (b *BinaryBuiltin) EvalCall(args callArguments, e *evaluator) (value, error) {
	// TODO check args
	return b.function(getBuiltinEvaluator(e, b.name), args.positional[0], args.positional[1])
}

func (b *BinaryBuiltin) Parameters() ast.Identifiers {
	return b.parameters
}

type TernaryBuiltin struct {
	name       ast.Identifier
	function   ternaryBuiltin
	parameters ast.Identifiers
}

func (b *TernaryBuiltin) EvalCall(args callArguments, e *evaluator) (value, error) {
	// TODO check args
	return b.function(getBuiltinEvaluator(e, b.name), args.positional[0], args.positional[1], args.positional[2])
}

func (b *TernaryBuiltin) Parameters() ast.Identifiers {
	return b.parameters
}

func todoFunc(e *evaluator, x, y potentialValue) (value, error) {
	return nil, e.Error("not implemented yet")
}

// so that we don't get segfaults
var todo = &BinaryBuiltin{function: todoFunc, parameters: ast.Identifiers{"x", "y"}}

var desugaredBop = map[ast.BinaryOp]ast.Identifier{
	ast.BopPercent:         "mod",
	ast.BopManifestEqual:   "equals",
	ast.BopManifestUnequal: "notEquals", // Special case
	ast.BopIn:              "objectHasAll",
}

var bopBuiltins = []*BinaryBuiltin{
	ast.BopMult:    &BinaryBuiltin{name: "operator*", function: builtinMult, parameters: ast.Identifiers{"x", "y"}},
	ast.BopDiv:     &BinaryBuiltin{name: "operator/", function: builtinDiv, parameters: ast.Identifiers{"x", "y"}},
	ast.BopPercent: todo,

	ast.BopPlus:  &BinaryBuiltin{name: "operator+", function: builtinPlus, parameters: ast.Identifiers{"x", "y"}},
	ast.BopMinus: &BinaryBuiltin{name: "operator-", function: builtinMinus, parameters: ast.Identifiers{"x", "y"}},

	ast.BopShiftL: &BinaryBuiltin{name: "operator<<", function: builtinShiftL, parameters: ast.Identifiers{"x", "y"}},
	ast.BopShiftR: &BinaryBuiltin{name: "operator>>", function: builtinShiftR, parameters: ast.Identifiers{"x", "y"}},

	ast.BopGreater:   &BinaryBuiltin{name: "operator>", function: builtinGreater, parameters: ast.Identifiers{"x", "y"}},
	ast.BopGreaterEq: &BinaryBuiltin{name: "operator>=", function: builtinGreaterEq, parameters: ast.Identifiers{"x", "y"}},
	ast.BopLess:      &BinaryBuiltin{name: "operator<,", function: builtinLess, parameters: ast.Identifiers{"x", "y"}},
	ast.BopLessEq:    &BinaryBuiltin{name: "operator<=", function: builtinLessEq, parameters: ast.Identifiers{"x", "y"}},

	// bopManifestEqual:   <desugared>,
	// bopManifestUnequal: <desugared>,

	ast.BopBitwiseAnd: &BinaryBuiltin{name: "operator&", function: builtinBitwiseAnd, parameters: ast.Identifiers{"x", "y"}},
	ast.BopBitwiseXor: &BinaryBuiltin{name: "operator^", function: builtinBitwiseXor, parameters: ast.Identifiers{"x", "y"}},
	ast.BopBitwiseOr:  &BinaryBuiltin{name: "operator|", function: builtinBitwiseOr, parameters: ast.Identifiers{"x", "y"}},

	ast.BopAnd: &BinaryBuiltin{name: "operator&&", function: builtinAnd, parameters: ast.Identifiers{"x", "y"}},
	ast.BopOr:  &BinaryBuiltin{name: "operator||", function: builtinOr, parameters: ast.Identifiers{"x", "y"}},
}

var uopBuiltins = []*UnaryBuiltin{
	ast.UopNot:        &UnaryBuiltin{name: "operator!", function: builtinNegation, parameters: ast.Identifiers{"x"}},
	ast.UopBitwiseNot: &UnaryBuiltin{name: "operator~", function: builtinBitNeg, parameters: ast.Identifiers{"x"}},
	ast.UopPlus:       &UnaryBuiltin{name: "operator+ (unary)", function: builtinIdentity, parameters: ast.Identifiers{"x"}},
	ast.UopMinus:      &UnaryBuiltin{name: "operator- (unary)", function: builtinUnaryMinus, parameters: ast.Identifiers{"x"}},
}

// TODO(sbarzowski) eliminate duplication in function names (e.g. build map from array or constants)
var funcBuiltins = map[string]evalCallable{
	"extVar":          &UnaryBuiltin{name: "extVar", function: builtinExtVar, parameters: ast.Identifiers{"x"}},
	"length":          &UnaryBuiltin{name: "length", function: builtinLength, parameters: ast.Identifiers{"x"}},
	"toString":        &UnaryBuiltin{name: "toString", function: builtinToString, parameters: ast.Identifiers{"x"}},
	"makeArray":       &BinaryBuiltin{name: "makeArray", function: builtinMakeArray, parameters: ast.Identifiers{"sz", "func"}},
	"flatMap":         &BinaryBuiltin{name: "flatMap", function: builtinFlatMap, parameters: ast.Identifiers{"func", "arr"}},
	"filter":          &BinaryBuiltin{name: "filter", function: builtinFilter, parameters: ast.Identifiers{"func", "arr"}},
	"primitiveEquals": &BinaryBuiltin{name: "primitiveEquals", function: primitiveEquals, parameters: ast.Identifiers{"sz", "func"}},
	"objectFieldsEx":  &BinaryBuiltin{name: "objectFields", function: builtinObjectFieldsEx, parameters: ast.Identifiers{"obj", "hidden"}},
	"objectHasEx":     &TernaryBuiltin{name: "objectHasEx", function: builtinObjectHasEx, parameters: ast.Identifiers{"obj", "fname", "hidden"}},
	"type":            &UnaryBuiltin{name: "type", function: builtinType, parameters: ast.Identifiers{"x"}},
	"char":            &UnaryBuiltin{name: "char", function: builtinChar, parameters: ast.Identifiers{"x"}},
	"codepoint":       &UnaryBuiltin{name: "codepoint", function: builtinCodepoint, parameters: ast.Identifiers{"x"}},
	"ceil":            &UnaryBuiltin{name: "ceil", function: builtinCeil, parameters: ast.Identifiers{"x"}},
	"floor":           &UnaryBuiltin{name: "floor", function: builtinFloor, parameters: ast.Identifiers{"x"}},
	"sqrt":            &UnaryBuiltin{name: "sqrt", function: builtinSqrt, parameters: ast.Identifiers{"x"}},
	"sin":             &UnaryBuiltin{name: "sin", function: builtinSin, parameters: ast.Identifiers{"x"}},
	"cos":             &UnaryBuiltin{name: "cos", function: builtinCos, parameters: ast.Identifiers{"x"}},
	"tan":             &UnaryBuiltin{name: "tan", function: builtinTan, parameters: ast.Identifiers{"x"}},
	"asin":            &UnaryBuiltin{name: "asin", function: builtinAsin, parameters: ast.Identifiers{"x"}},
	"acos":            &UnaryBuiltin{name: "acos", function: builtinAcos, parameters: ast.Identifiers{"x"}},
	"atan":            &UnaryBuiltin{name: "atan", function: builtinAtan, parameters: ast.Identifiers{"x"}},
	"log":             &UnaryBuiltin{name: "log", function: builtinLog, parameters: ast.Identifiers{"x"}},
	"exp":             &UnaryBuiltin{name: "exp", function: builtinExp, parameters: ast.Identifiers{"x"}},
	"mantissa":        &UnaryBuiltin{name: "mantissa", function: builtinMantissa, parameters: ast.Identifiers{"x"}},
	"exponent":        &UnaryBuiltin{name: "exponent", function: builtinExponent, parameters: ast.Identifiers{"x"}},
	"pow":             &BinaryBuiltin{name: "pow", function: builtinPow, parameters: ast.Identifiers{"base", "exp"}},
	"modulo":          &BinaryBuiltin{name: "modulo", function: builtinModulo, parameters: ast.Identifiers{"x", "y"}},
	"md5":             &UnaryBuiltin{name: "md5", function: builtinMd5, parameters: ast.Identifiers{"x"}},

	// internal
	"$objectFlatMerge": &UnaryBuiltin{name: "$objectFlatMerge", function: builtinUglyObjectFlatMerge, parameters: ast.Identifiers{"x"}},
}
