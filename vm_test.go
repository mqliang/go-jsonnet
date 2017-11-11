package jsonnet

import (
	"testing"
	"io/ioutil"
)

func BenchmarkSnippetToAST(b *testing.B) {
	buf, err := ioutil.ReadFile("std/std.jsonnet")
	if err != nil {
		panic(err)
	}

	for n := 0; n < b.N; n++ {
		SnippetToAST("<std>", string(buf))
	}
}