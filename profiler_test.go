package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/chromedp/cdproto/profiler"
)

func TestWriteProfile(t *testing.T) {
	buf, err := ioutil.ReadFile("testdata/wasm.prof")
	if err != nil {
		t.Fatal(err)
	}
	var outBuf bytes.Buffer

	cProf := profiler.Profile{}
	err = json.Unmarshal(buf, &cProf)
	if err != nil {
		t.Fatal(err)
	}
	fnMap := make(map[int]string)
	err = WriteProfile(&cProf, &outBuf, fnMap)
	if err != nil {
		t.Error(err)
	}

	golden, err := ioutil.ReadFile("testdata/pprof.out")
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(outBuf.Bytes(), golden) {
		t.Errorf("generated profile is not correct")
	}
}
