package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

type wasmServer struct {
	indexTmpl  *template.Template
	wasmFile   string
	wasmExecJS []byte
	args       []string
	logger     *log.Logger
}

func NewWASMServer(wasmFile string, args []string, l *log.Logger) (http.Handler, error) {
	var err error
	srv := &wasmServer{
		wasmFile: wasmFile,
		args:     args,
		logger:   l,
	}

	buf, err := ioutil.ReadFile(path.Join(runtime.GOROOT(), "misc/wasm/wasm_exec.js"))
	if err != nil {
		return nil, err
	}
	srv.wasmExecJS = buf

	srv.indexTmpl, err = template.New("index").Parse(indexHTML)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func (ws *wasmServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// log.Println(r.URL.Path)
	switch r.URL.Path {
	case "/", "/index.html":
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		data := struct {
			WASMFile string
			Args     []string
		}{
			WASMFile: filepath.Base(ws.wasmFile),
			Args:     ws.args,
		}
		err := ws.indexTmpl.Execute(w, data)
		if err != nil {
			ws.logger.Println(err)
		}
	case "/" + filepath.Base(ws.wasmFile):
		f, err := os.Open(ws.wasmFile)
		if err != nil {
			ws.logger.Println(err)
			return
		}
		defer func() {
			err := f.Close()
			if err != nil {
				ws.logger.Println(err)
			}
		}()
		http.ServeContent(w, r, r.URL.Path, time.Now(), f)
	case "/wasm_exec.js":
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Content-Length", strconv.Itoa(len(ws.wasmExecJS)))
		if _, err := w.Write(ws.wasmExecJS); err != nil {
			ws.logger.Println("unable to write wasm_exec.")
		}
	}
}

const indexHTML = `<!doctype html>
<!--
Copyright 2018 The Go Authors. All rights reserved.
Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
-->
<html>

<head>
	<meta charset="utf-8">
	<title>Go wasm</title>
</head>

<body>
	<!--
	Add the following polyfill for Microsoft Edge 17/18 support:
	<script src="https://cdn.jsdelivr.net/npm/text-encoding@0.7.0/lib/encoding.min.js"></script>
	(see https://caniuse.com/#feat=textencoder)
	-->
	<script src="wasm_exec.js"></script>
	<script>
		if (!WebAssembly.instantiateStreaming) { // polyfill
			WebAssembly.instantiateStreaming = async (resp, importObject) => {
				const source = await (await resp).arrayBuffer();
				return await WebAssembly.instantiate(source, importObject);
			};
		}

		let exitCode = 0;
		function goExit(code) {
			exitCode = code;
		}

		(async() => {
			const go = new Go();
			go.argv = [{{range $i, $item := .Args}} {{if $i}}, {{end}} "{{$item}}" {{end}}];
			go.exit = goExit;
			let mod, inst;
			await WebAssembly.instantiateStreaming(fetch("{{.WASMFile}}"), go.importObject).then((result) => {
				mod = result.module;
				inst = result.instance;
			}).catch((err) => {
				console.error(err);
			});
			await go.run(inst);
			document.getElementById("doneButton").disabled = false;
		})();
	</script>

	<button id="doneButton" disabled>Done</button>
</body>
</html>`
