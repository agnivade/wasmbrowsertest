package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()
	const bashPrintCleanVars = `env | grep CLEAN_ | sort | tr '\n' ' '`
	for _, tc := range []struct {
		name         string
		env          []string
		args         []string
		expectOutput string
		expectErr    string
	}{
		{
			name:      "zero args",
			expectErr: "not enough args to run a command",
		},
		{
			name: "all env passed through",
			env: []string{
				"CLEAN_BAR=bar",
				"CLEAN_FOO=foo",
			},
			args:         []string{"bash", "-c", bashPrintCleanVars},
			expectOutput: "CLEAN_BAR=bar CLEAN_FOO=foo",
		},
		{
			name: "remove one variable prefix",
			env: []string{
				"CLEAN_BAR=bar",
				"CLEAN_FOO=foo",
			},
			args: []string{
				"-remove-prefix=CLEAN_BAR", "--",
				"bash", "-c", bashPrintCleanVars,
			},
			expectOutput: "CLEAN_FOO=foo",
		},
		{
			name: "remove common variable prefix",
			env: []string{
				"CLEAN_COMMON_BAR=bar",
				"CLEAN_COMMON_BAZ=baz",
				"CLEAN_FOO=foo",
			},
			args: []string{
				"-remove-prefix=CLEAN_COMMON_", "--",
				"bash", "-c", bashPrintCleanVars,
			},
			expectOutput: "CLEAN_FOO=foo",
		},
		{
			name: "remove multiple prefixes",
			env: []string{
				"CLEAN_BAR=bar",
				"CLEAN_BAZ=baz",
				"CLEAN_FOO=foo",
			},
			args: []string{
				"-remove-prefix=CLEAN_BAR",
				"-remove-prefix=CLEAN_FOO", "--",
				"bash", "-c", bashPrintCleanVars,
			},
			expectOutput: "CLEAN_BAZ=baz",
		},
	} {
		tc := tc // enable parallel sub-tests
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			app := App{
				Args:   tc.args,
				Env:    tc.env,
				Out:    &output,
				ErrOut: &output,
			}
			err := app.Run()
			assertEqualError(t, tc.expectErr, err)
			if tc.expectErr != "" {
				return
			}

			outputStr := strings.TrimSpace(output.String())
			if outputStr != tc.expectOutput {
				t.Errorf("Unexpected output: %q != %q", tc.expectOutput, outputStr)
			}
		})
	}
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
