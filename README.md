# wasmbrowsertest

A quick hack to run wasm tests inside your browser. For now, works only for Chrome. Geckodriver doesn't support selenium log API.

Setup steps:

Install the agouti package:
1. `go get github.com/sclevine/agouti`.

Get the ChromeDriver binary: 

2. `curl 'https://chromedriver.storage.googleapis.com/2.35/chromedriver_linux64.zip'` and unzip it.

Build the test binary:

3. `cd $GOPATH/github.com/agnivade/wasmbrowsertest/`
4. `gotip build -o go_js_wasm_exec .` (`gotip` is an alias to the tip binary. We need to build using tip to get the `application/wasm` mime type)

Now we just need the ASSET_FOLDER and CHROME_DRIVER as env vars to run tests.

5. `export ASSET_FOLDER='$GOPATH/github.com/agnivade/wasmbrowsertest/assets'`
6. `export CHROME_DRIVER='path/to/chromedriver-linux64-2.35'`

And finally we are ready to run our tests !

7. `PATH=$PATH:/path/to/go_js_wasm_exec GOOS=js GOARCH=wasm ../bin/go test ./encoding/hex -v -run='^Test'`
