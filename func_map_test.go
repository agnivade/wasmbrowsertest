package main

import "testing"

func TestFuncMap(t *testing.T) {
	fMap, err := getFuncMap("testdata/test.wasm")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		index int
		name  string
	}{
		{0, "go.buildid"},
		{10, "sync_atomic.LoadUint64"},
		{100, "runtime.cgoCheckBits"},
	}
	for i, test := range tests {
		if fMap[test.index] != test.name {
			t.Errorf("[%d] incorrect function name; expected %q, got %q.", i, test.name, fMap[test.index])
		}
	}
}
