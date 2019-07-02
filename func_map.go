package main

import (
	"os"

	"github.com/go-interpreter/wagon/wasm"
)

func getFuncMap(wasmFile string) (map[int]string, error) {
	funcMap := make(map[int]string)
	wasmFd, err := os.Open(wasmFile)
	if err != nil {
		return funcMap, err
	}
	defer wasmFd.Close()
	mod, err := wasm.ReadModule(wasmFd, nil)
	if err != nil {
		return funcMap, err
	}

	// populating imports
	counter := 0
	for i, e := range mod.Import.Entries {
		funcMap[i] = e.FieldName
		counter++
	}
	// Skipping the imported functions
	for i, f := range mod.FunctionIndexSpace[counter:] {
		funcMap[i] = f.Name
	}
	return funcMap, nil
}
