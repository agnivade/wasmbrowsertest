# wasmbrowsertest

Run wasm tests easily in your browser. This is the `go_js_wasm_exec` for browsers. Run this exactly the same way you would run tests for Node, except replace the go_js_wasm_exec for node with the one built using this repo.

`go build -o go_js_wasm_exec`

`PATH=$PATH:/directory/to/go_js_wasm_exec GOOS=js GOARCH=wasm go test`

This tool uses the [ChromeDP](https://chromedevtools.github.io/devtools-protocol/) protocol to run the tests inside a Chrome browser. So Chrome needs to be installed in your machine.
