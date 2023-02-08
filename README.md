# wasmbrowsertest [![Build Status](https://travis-ci.org/agnivade/wasmbrowsertest.svg?branch=master)](https://travis-ci.org/agnivade/wasmbrowsertest)

Run Go wasm tests easily in your browser.

If you have a codebase targeting the wasm platform, chances are you would want to test your code in a browser. Currently, that process is a bit cumbersome:
- The test needs to be compiled to a wasm file.
- Then loaded into an HTML file along with the wasm_exec.js.
- And finally, this needs to be served with a static file server and then loaded in the browser.

This tool automates all of that. So you just have to type `GOOS=js GOARCH=wasm go test`, and it automatically executes the tests inside a browser !

## Quickstart

- `go install github.com/agnivade/wasmbrowsertest@latest`. This will place the binary in $GOPATH/bin, or $GOBIN, if that has a different value.
- Rename the binary to `go_js_wasm_exec`.
- Add $GOBIN to $PATH if it is not already done.
- Run tests as usual: `GOOS=js GOARCH=wasm go test`.
- You can also take a cpu profile. Set the `-cpuprofile` flag for that.

## Ok, but how does the magic work ?

`go test` allows invocation of a different binary to run a test. `go help test` has a line: 

```
-exec xprog
	    Run the test binary using xprog. The behavior is the same as
	    in 'go run'. See 'go help run' for details.
```

And `go help run` says:

```
By default, 'go run' runs the compiled binary directly: 'a.out arguments...'.
If the -exec flag is given, 'go run' invokes the binary using xprog:
	'xprog a.out arguments...'.
If the -exec flag is not given, GOOS or GOARCH is different from the system
default, and a program named go_$GOOS_$GOARCH_exec can be found
on the current search path, 'go run' invokes the binary using that program,
for example 'go_nacl_386_exec a.out arguments...'. This allows execution of
cross-compiled programs when a simulator or other execution method is
available.
```

So essentially, there are 2 ways:
- Either have a binary with the name of `go_js_wasm_exec` in your $PATH.
- Or set the `-exec` flag in your tests.

Use whatever works for you.

### How is a CPU profile taken ?

A CPU profile is run during the duration of the test, and then converted to the pprof format so that it can be natively analyzed with the Go toolchain.

### Can I run something which is not a test ?

Yep. `GOOS=js GOARCH=wasm go run main.go` also works. If you want to actually see the application running in the browser, set the `WASM_HEADLESS` variable to `off` like so `WASM_HEADLESS=off GOOS=js GOARCH=wasm go run main.go`.

### Can I use this inside Travis ?

Sure.

Add these lines to your `.travis.yml`

```
addons:
  chrome: stable

install:
- go install github.com/agnivade/wasmbrowsertest@latest
- mv $GOPATH/bin/wasmbrowsertest $GOPATH/bin/go_js_wasm_exec
- export PATH=$GOPATH/bin:$PATH
```

Now, just setting `GOOS=js GOARCH=wasm` will run your tests using `wasmbrowsertest`. For other CI environments, you have to do something similar.

### Can I use this inside Github Action?

Sure.

Add these lines to your `.github/workflows/ci.yml`

PS: adjust the go version you need in go-version section

```
on: [push, pull_request]
name: Unit Test
jobs:
  test:
    strategy:
      matrix:
        go-version: [1.xx.x]
        os: [ubuntu-latest]
    runs-on: ${{ matrix.os }}
    steps:
    - name: Install Go
      uses: actions/setup-go@v2
      with:
        go-version: ${{ matrix.go-version }}
    - name: Install chrome
      uses: browser-actions/setup-chrome@latest
    - name: Install dep
      run: go install github.com/agnivade/wasmbrowsertest@latest
    - name: Setup wasmexec
      run: mv $(go env GOPATH)/bin/wasmbrowsertest $(go env GOPATH)/bin/go_js_wasm_exec
    - name: Checkout code
      uses: actions/checkout@v2
```

### What sorts of browsers are supported ?

This tool uses the [ChromeDP](https://chromedevtools.github.io/devtools-protocol/) protocol to run the tests inside a Chrome browser. So Chrome or any blink-based browser will work.

### Why not firefox ?

Great question. The initial idea was to use a Selenium API and drive any browser to run the tests. But unfortunately, geckodriver does not support the ability to capture console logs - https://github.com/mozilla/geckodriver/issues/284. Hence, the shift to use the ChromeDP protocol circumvents the need to have any external driver binary and just have a browser installed in the machine.

## Errors

### `total length of command line and environment variables exceeds limit`

If the error `total length of command line and environment variables exceeds limit` appears, then
the current environment variables' total size has exceeded the maximum when executing Go Wasm binaries.

To resolve this issue, install `cleanenv` and use it to prefix your command.

For example, if these commands are used:
```bash
export GOOS=js GOARCH=wasm
go test -cover ./...
```
The new commands should be the following:
```bash
go install github.com/agnivade/wasmbrowsertest/cmd/cleanenv@latest

export GOOS=js GOARCH=wasm
cleanenv -remove-prefix GITHUB_ -- go test -cover ./...
```

The `cleanenv` command above removes all environment variables prefixed with `GITHUB_` before running the command after the `--`.
The `-remove-prefix` flag can be repeated multiple times to remove even more environment variables.
