package main

import (
	"fmt"
	"strings"
)

// ConsoleFilter buffers console output and filters out passing tests when in quiet mode.
type ConsoleFilter struct {
	buffer []string
	quiet  bool
	output func(string) // callback to write output
}

func NewConsoleFilter(quiet bool, output func(string)) *ConsoleFilter {
	if output == nil {
		output = func(s string) { fmt.Printf("%s\n", s) }
	}
	return &ConsoleFilter{
		quiet:  quiet,
		output: output,
	}
}

func (cf *ConsoleFilter) Add(input string) {
	// Split input by newlines to ensure we handle line-by-line filtering
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		// allow empty lines?
		if line == "" {
			// Check if we should preserve empty lines or not.
			// console logs usually imply newline at end of each call,
			// but Split might give empty string at end.
			// Let's preserve for fidelity but filtering logic ignores empty lines usually.
			// continue
		}
		cf.addLine(line)
	}
}

func (cf *ConsoleFilter) addLine(line string) {
	if !cf.quiet {
		cf.output(line)
		return
	}

	// Always print global markers immediately and flush buffer
	// FAIL (global), PASS (global), ok, coverage, etc.
	// These usually appear at the very end of the test suite.
	if strings.HasPrefix(line, "FAIL") ||
		strings.HasPrefix(line, "PASS") ||
		strings.HasPrefix(line, "coverage:") ||
		strings.HasPrefix(line, "pkg:") ||
		strings.HasPrefix(line, "ok") ||
		strings.HasPrefix(line, "panic:") ||
		strings.HasPrefix(line, "exit status") {
		cf.Flush()

		// In quiet mode, suppress purely global summary lines "PASS" and "FAIL"
		// and replace them with emoji indicators.
		trimmed := strings.TrimSpace(line)
		if trimmed == "FAIL" {
			cf.output("❌ WASM tests failed")
			return
		}
		if trimmed == "PASS" {
			cf.output("✅ All tests passed!")
			return
		}

		cf.output(line)
		return
	}

	// NOTE: We do NOT flush on individual "--- FAIL:" lines anymore.
	// We buffer them. This allows us to keep the context (logs) of the failing test.
	// Since passing tests are removed from the buffer, the buffer will primarily contain
	// failing tests' logs (and currently running tests' logs).
	// We flush at the end (triggered by global markers or browser exit).

	cf.buffer = append(cf.buffer, line)

	// If a test passed, remove its logs from buffer.
	// "--- PASS: TestName (0.00s)"
	if strings.Contains(line, "--- PASS:") {
		cf.removePassingTestLogs(line)
	}
}

func (cf *ConsoleFilter) removePassingTestLogs(passLine string) {
	// Extract TestName from passLine
	// Fields: "---", "PASS:", "TestName", "(0.00s)"
	fields := strings.Fields(passLine)
	var testName string
	for i, f := range fields {
		if f == "PASS:" && i+1 < len(fields) {
			testName = fields[i+1]
			break
		}
	}

	if testName == "" {
		return
	}

	// Search backwards for "=== RUN TestName"
	foundRun := -1
	runLinesInBetween := false

	// Iterate backwards from the line before the PASS line
	// (PASS line is already in buffer at last index, or we assume it triggered this call)
	// Actually, addLine appends PASS line, THEN calls this. So PASS is at len-1.
	searchStart := len(cf.buffer) - 2 // Skip the PASS line itself
	if searchStart < 0 {
		return
	}

	for i := searchStart; i >= 0; i-- {
		lineFields := strings.Fields(cf.buffer[i])
		if len(lineFields) >= 3 && lineFields[0] == "===" && lineFields[1] == "RUN" {
			runName := lineFields[2]
			if runName == testName {
				foundRun = i
				break
			}
			// Found a RUN line for another test.
			// This means we have interleaved or nested tests.
			runLinesInBetween = true
			// Do NOT abort. Continue searching for our RUN line.
		}
	}

	if foundRun != -1 {
		if !runLinesInBetween {
			// Clean block: No other RUN lines in between. Safe to truncate.
			// Remove from foundRun to end.
			cf.buffer = cf.buffer[:foundRun]
		} else {
			// Interleaved or nested.
			// Removing the block might delete other tests' RUN lines or logs.
			// Safer strategy: Only remove the "=== RUN" line and the "--- PASS" line.

			// Remove the PASS line (last element)
			if len(cf.buffer) > 0 {
				cf.buffer = cf.buffer[:len(cf.buffer)-1]
			}

			// Remove the RUN line (at foundRun)
			// Efficient removal from slice
			cf.buffer = append(cf.buffer[:foundRun], cf.buffer[foundRun+1:]...)
		}
	} else {
		// If we couldn't find the RUN line, but found a PASS line, remove the PASS line.
		if len(cf.buffer) > 0 {
			cf.buffer = cf.buffer[:len(cf.buffer)-1]
		}
	}
}

func (cf *ConsoleFilter) Flush() {
	for _, line := range cf.buffer {
		cf.output(line)
	}
	cf.buffer = nil
}
