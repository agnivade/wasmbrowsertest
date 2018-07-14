package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

type wasmServer struct {
	wasmExec, targetWASM []byte
	indexHTML            *template.Template
	wasmFile             string
	assetFolder          string
	args                 []string
}

func (ws *wasmServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		err := ws.indexHTML.Execute(w, data)
		if err != nil {
			log.Println(err)
		}
	case "/" + filepath.Base(ws.wasmFile):
		f, err := os.Open(ws.wasmFile)
		if err != nil {
			log.Println(err)
			return
		}
		defer f.Close()
		http.ServeContent(w, r, r.URL.Path, time.Now(), f)
	default:
		http.ServeFile(w, r, path.Join(ws.assetFolder, r.URL.Path))
	}
}

func getHandler(assetFolder, wasmFile string, args []string) (http.Handler, error) {
	var err error
	srv := &wasmServer{
		wasmFile:    wasmFile,
		assetFolder: assetFolder,
		args:        args,
	}

	// Read index.html and store as template.
	indexBuf, err := ioutil.ReadFile(path.Join(assetFolder, "index.html"))
	if err != nil {
		return srv, err
	}
	srv.indexHTML, err = template.New("index").Parse(string(indexBuf))
	if err != nil {
		return srv, err
	}
	return srv, nil
}
