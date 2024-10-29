package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/agnivade/wasmbrowsertest/filesys"
)

//go:embed index.html
var indexHTML string

type wasmServer struct {
	indexTmpl     *template.Template
	wasmFile      string
	wasmExecJS    []byte
	args          []string
	envMap        map[string]string
	logger        *log.Logger
	fsHandler     *filesys.Handler
	securityToken string
}

var wasmLocations = []string{
	"misc/wasm/wasm_exec.js",
	"lib/wasm/wasm_exec.js",
}

func NewWASMServer(wasmFile string, args []string, coverageFile string, l *log.Logger) (http.Handler, error) {
	var err error
	srv := &wasmServer{
		wasmFile: wasmFile,
		args:     args,
		logger:   l,
		envMap:   make(map[string]string),
	}

	// try for some security on an api capable of
	// reads and writes to the file system
	srv.securityToken, err = generateToken()
	if err != nil {
		return nil, err
	}
	srv.fsHandler = filesys.NewHandler(srv.securityToken, l)

	for _, env := range os.Environ() {
		vars := strings.SplitN(env, "=", 2)
		srv.envMap[vars[0]] = vars[1]
	}

	var buf []byte
	for _, loc := range wasmLocations {
		buf, err = os.ReadFile(filepath.Join(runtime.GOROOT(), loc))
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if err != nil {
		var perr *os.PathError
		if errors.As(err, &perr) {
			if strings.Contains(perr.Path, filepath.Join("golang.org", "toolchain")) {
				return nil, fmt.Errorf("The Go toolchain does not include the WebAssembly exec helper before Go 1.24. Please copy wasm_exec.js to %s", filepath.Join(runtime.GOROOT(), "misc", "wasm", "wasm_exec.js"))
			}
		}
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
			WASMFile      string
			Args          []string
			EnvMap        map[string]string
			SecurityToken string
			Pid           int
			Ppid          int
		}{
			WASMFile:      filepath.Base(ws.wasmFile),
			Args:          ws.args,
			EnvMap:        ws.envMap,
			SecurityToken: ws.securityToken,
			Pid:           os.Getpid(),
			Ppid:          os.Getppid(),
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
	default:
		if strings.HasPrefix(r.URL.Path, "/fs/") {
			ws.fsHandler.ServeHTTP(w, r)
		}
	}
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
