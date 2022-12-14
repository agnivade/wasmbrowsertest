package main

import (
	_ "embed"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//go:embed index.html
var indexHTML string

type wasmServer struct {
	indexTmpl    *template.Template
	wasmFile     string
	wasmExecJS   []byte
	args         []string
	coverageFile string
	envMap       map[string]string
	logger       *log.Logger
}

func NewWASMServer(wasmFile string, args []string, coverageFile string, l *log.Logger) (http.Handler, error) {
	var err error
	srv := &wasmServer{
		wasmFile:     wasmFile,
		args:         args,
		coverageFile: coverageFile,
		logger:       l,
		envMap:       make(map[string]string),
	}

	for _, env := range os.Environ() {
		vars := strings.SplitN(env, "=", 2)
		srv.envMap[vars[0]] = vars[1]
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
			WASMFile     string
			Args         []string
			CoverageFile string
			EnvMap       map[string]string
		}{
			WASMFile:     filepath.Base(ws.wasmFile),
			Args:         ws.args,
			CoverageFile: ws.coverageFile,
			EnvMap:       ws.envMap,
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
