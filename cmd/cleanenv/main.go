// Command cleanenv removes all environment variables that match given prefixes before running its arguments as a command.
//
// For example, this is useful in GitHub Actions:
//
//	export GOOS=js GOARCH=wasm
//	cleanenv -remove-prefix GITHUB_ -- go test -cover ./...
//
// The '-remove-prefix' flag can be repeated multiple times to remove even more environment variables.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func main() {
	app := App{
		Args:   os.Args[1:],
		Env:    os.Environ(),
		StdOut: os.Stdout,
		ErrOut: os.Stderr,
	}
	err := app.Run()
	if err != nil {
		fmt.Fprintln(app.ErrOut, err)
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		os.Exit(exitCode)
	}
}

type App struct {
	Args           []string
	Env            []string
	StdOut, ErrOut io.Writer
}

func (a App) Run() error {
	set := flag.NewFlagSet("cleanenv", flag.ContinueOnError)
	var removePrefixes StringSliceFlag
	set.Var(&removePrefixes, "remove-prefix", "Remove one or more environment variables with the given prefixes.")
	if err := set.Parse(a.Args); err != nil {
		return err
	}

	var cleanEnv []string
	for _, keyValue := range a.Env {
		tokens := strings.SplitN(keyValue, "=", 2)
		if allowEnvName(tokens[0], removePrefixes) {
			cleanEnv = append(cleanEnv, keyValue)
		}
	}

	arg0, argv, err := splitArgs(set.Args())
	if err != nil {
		return err
	}
	cmd := exec.Command(arg0, argv...)
	cmd.Env = cleanEnv
	cmd.Stdout = a.StdOut
	cmd.Stderr = a.ErrOut
	return cmd.Run()
}

type StringSliceFlag []string

func (s *StringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (s *StringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func allowEnvName(name string, removePrefixes []string) bool {
	for _, prefix := range removePrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

func splitArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, errors.New("not enough args to run a command")
	}
	return args[0], args[1:], nil
}
