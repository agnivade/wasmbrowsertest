# wasmbrowsertest [![Build Status](https://travis-ci.org/agnivade/wasmbrowsertest.svg?branch=master)](https://travis-ci.org/agnivade/wasmbrowsertest)

Run Go wasm tests easily in your browser.

If you have a codebase targeting the wasm platform, chances are you would want to test your code in a browser. Currently, that process is a bit cumbersome:
- The test needs to be compiled to a wasm file.
- Then loaded into an HTML file along with the wasm_exec.js.
- And finally, this needs to be served with a static file server and then loaded in the browser.

This tool automates all of that. So you just have to type `GOOS=js GOARCH=wasm go test`, and it automatically executes the tests inside a browser !

## Quickstart

- `go get github.com/agnivade/wasmbrowsertest`. This will place the binary in $GOPATH/bin, or $GOBIN, if that has a different value.
- Rename the binary to `go_js_wasm_exec`.
- Add $GOBIN to $PATH if it is not already done.
- Run tests as usual: `GOOS=js GOARCH=wasm go test`.

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

### If I have a wasm binary file, can this run it in the browser ?

Yep. Just pass the wasm file as the first argument - `wasmbrowsertest test.wasm`.

### What sorts of browsers are supported ?

This tool uses the [ChromeDP](https://chromedevtools.github.io/devtools-protocol/) protocol to run the tests inside a Chrome browser. So Chrome or any blink-based browser will work.

### Why not firefox ?

Great question. The initial idea was to use a Selenium API and drive any browser to run the tests. But unfortunately, geckodriver does not support the ability to capture console logs - https://github.com/mozilla/geckodriver/issues/284. Hence, the shift to use the ChromeDP protocol circumvents the need to have any external driver binary and just have a browser installed in the machine.
