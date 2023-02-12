package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		description string
		files       map[string]string
		args        []string
		expectErr   string
	}{
		{
			description: "pass",
			files: map[string]string{
				"go.mod": `
module foo
`,
				"foo_test.go": `
package foo

import "testing"

func TestFoo(t *testing.T) {
	if false {
		t.Errorf("foo failed")
	}
}
`,
			},
		},
		{
			description: "fails",
			files: map[string]string{
				"go.mod": `
module foo
`,
				"foo_test.go": `
package foo

import "testing"

func TestFooFails(t *testing.T) {
	t.Errorf("foo failed")
}
`,
			},
			expectErr: "exit with status 1",
		},
		{
			description: "panic fails",
			files: map[string]string{
				"go.mod": `
module foo
`,
				"foo_test.go": `
package foo

import "testing"

func TestFooPanic(t *testing.T) {
	panic("failed")
}
`,
			},
			expectErr: "exit with status 2",
		},
		{
			description: "panic in goroutine fails",
			files: map[string]string{
				"go.mod": `
module foo
`,
				"foo_test.go": `
package foo

import "testing"

func TestFooGoroutinePanic(t *testing.T) {
	go panic("foo failed")
}
`,
			},
			expectErr: "exit with status 1",
		},
		{
			description: "panic in next run of event loop fails",
			files: map[string]string{
				"go.mod": `
module foo
`,
				"foo_test.go": `
package foo

import (
	"syscall/js"
	"testing"
)

func TestFooNextEventLoopPanic(t *testing.T) {
	js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) interface{} {
		panic("bad")
		return nil
	}), 0)
}
`,
			},
			expectErr: "context canceled",
		},
	} {
		tc := tc // enable parallel sub-tests
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for fileName, contents := range tc.files {
				writeFile(t, dir, fileName, contents)
			}
			wasmFile := buildTestWasm(t, dir)
			_, err := testRun(t, wasmFile, tc.args...)
			assertEqualError(t, tc.expectErr, err)
		})
	}
}

type testWriter struct {
	testingT *testing.T
}

func testLogger(t *testing.T) io.Writer {
	return &testWriter{t}
}

func (w *testWriter) Write(b []byte) (int, error) {
	w.testingT.Helper()
	w.testingT.Log(string(b))
	return len(b), nil
}

func testRun(t *testing.T, wasmFile string, flags ...string) ([]byte, error) {
	var logs bytes.Buffer
	output := io.MultiWriter(testLogger(t), &logs)
	flagSet := flag.NewFlagSet("wasmbrowsertest", flag.ContinueOnError)

	err := run(append([]string{"go_js_wasm_exec", wasmFile, "-test.v"}, flags...), output, flagSet)
	return logs.Bytes(), err
}

// writeFile creates a file at $baseDir/$path with the given contents, where 'path' is slash separated
func writeFile(t *testing.T, baseDir, path, contents string) {
	t.Helper()
	path = filepath.FromSlash(path)
	fullPath := filepath.Join(baseDir, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		t.Fatal("Failed to create file's base directory:", err)
	}
	err = os.WriteFile(fullPath, []byte(contents), 0600)
	if err != nil {
		t.Fatal("Failed to create file:", err)
	}
}

// buildTestWasm builds the given Go package's test binary and returns the output Wasm file
func buildTestWasm(t *testing.T, path string) string {
	t.Helper()
	outputFile := filepath.Join(t.TempDir(), "out.wasm")
	cmd := exec.Command("go", "test", "-c", "-o", outputFile, ".")
	cmd.Dir = path
	cmd.Env = append(os.Environ(),
		"GOOS=js",
		"GOARCH=wasm",
	)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		t.Log(string(output))
	}
	if err != nil {
		t.Fatal("Failed to build Wasm binary:", err)
	}
	return outputFile
}

func assertEqualError(t *testing.T, expected string, err error) {
	t.Helper()
	if expected == "" {
		if err != nil {
			t.Error("Unexpected error:", err)
		}
		return
	}

	if err == nil {
		t.Error("Expected error, got nil")
		return
	}
	message := err.Error()
	if expected != message {
		t.Errorf("Unexpected error message: %q != %q", expected, message)
	}
}
