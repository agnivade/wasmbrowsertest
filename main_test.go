package main

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows. See https://github.com/agnivade/wasmbrowsertest/issues/59")
	}
	for _, tc := range []struct {
		description string
		files       map[string]string
		args        []string
		expectErr   string
	}{
		{
			description: "handle panic",
			files: map[string]string{
				"go.mod": `
module foo

go 1.20
`,
				"foo.go": `
package main

func main() {
	panic("failed")
}
`,
			},
			expectErr: "exit with status 2",
		},
		{
			description: "handle panic in next run of event loop",
			files: map[string]string{
				"go.mod": `
		module foo

		go 1.20
		`,
				"foo.go": `
		package main

		import (
			"syscall/js"
		)

		func main() {
			js.Global().Call("setTimeout", js.FuncOf(func(js.Value, []js.Value) any {
				panic("bad")
				return nil
			}), 0)
		}
		`,
			},
			expectErr: "",
		},
		{
			description: "handle callback after test exit",
			files: map[string]string{
				"go.mod": `
		module foo

		go 1.20
		`,
				"foo.go": `
		package main

		import (
			"syscall/js"
			"fmt"
		)

		func main() {
			js.Global().Call("setInterval", js.FuncOf(func(js.Value, []js.Value) any {
				fmt.Println("callback")
				return nil
			}), 5)
			fmt.Println("done")
		}
		`,
			},
			expectErr: "",
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
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

func testRun(t *testing.T, wasmFile string, flags ...string) ([]byte, error) {
	var logs bytes.Buffer
	flagSet := flag.NewFlagSet("wasmbrowsertest", flag.ContinueOnError)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	err := run(ctx, append([]string{"go_js_wasm_exec", wasmFile}, flags...), &logs, flagSet)
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
	cmd := exec.Command("go", "build", "-o", outputFile, ".")
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
