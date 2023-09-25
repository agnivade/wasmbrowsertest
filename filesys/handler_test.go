package filesys

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

const TOKEN = "test_token"

func TestOpen_Missing(t *testing.T) {
	help := Helper(t)
	o := &Open{Path: help.tempPath("not_found.txt"), Flags: os.O_RDONLY, Mode: 0}
	response := &ErrorCode{}

	help.httpBad(help.req("open", o, response))
	help.errorCode(response.Code, "ENOENT")
}

func TestOpenClose(t *testing.T) {
	help := Helper(t)
	path := help.createFile("found.txt", "some data")
	o := &Open{Path: path, Flags: os.O_RDONLY, Mode: 0}
	c := &Close{}

	help.httpOk(help.req("open", o, c))
	help.true(c.Fd != 0, "no file descriptor returned")

	m := help.newMap()
	help.httpOk(help.req("close", c, &m))
}

func TestStat(t *testing.T) {
	help := Helper(t)
	foundFile := help.createFile("found.txt", "some data")
	payload := &Stat{Path: foundFile}
	m := help.newMap()

	help.httpOk(help.req("stat", payload, &m))
	help.checkStatMap(m, foundFile)
}

func TestStat_Missing(t *testing.T) {
	help := Helper(t)
	notFoundFile := help.tempPath("not_found.txt")
	payload := &Stat{Path: notFoundFile}
	errorCode := &ErrorCode{}

	help.httpBad(help.req("stat", payload, &errorCode))
	help.errorCode(errorCode.Code, "ENOENT")
}

func TestFstat(t *testing.T) {
	help := Helper(t)
	tempFile := help.createFile("exists", "some data")
	fd, closeFd := help.sysOpen(tempFile)
	defer closeFd()

	m := help.newMap()
	fstat := map[string]any{"fd": fd}

	help.httpOk(help.req("fstat", fstat, &m))
	help.checkStatMap(m, tempFile)

	// bad file descriptor test
	fstat = map[string]any{"fd": math.MaxInt64}
	help.httpBad(help.req("fstat", fstat, &m))
}

func TestLstat(t *testing.T) {
	help := Helper(t)
	exists := help.createFile("exists.txt", "some data")
	m := help.newMap()
	payload := &Lstat{Path: exists}

	help.httpOk(help.req("lstat", payload, &m))
	help.checkStatMap(m, exists)

	// test missing file case
	errCode := &ErrorCode{}
	payload = &Lstat{Path: help.tempPath("missing.txt")}

	help.httpBad(help.req("lstat", payload, errCode))
	help.errorCode(errCode.Code, "ENOENT")
}

type readDirResult struct {
	Entries []string `json:"entries"`
}

func TestReaddir(t *testing.T) {
	help := Helper(t)

	help.createFile("exists.txt", "some data")
	r := &readDirResult{}
	payload := &Readdir{Path: help.tmpDir}

	help.httpOk(help.req("readdir", payload, r))
	help.true(len(r.Entries) == 1, "incorrect entries length")

	payload = &Readdir{Path: help.tempPath("badDirectory")}
	help.httpBad(help.req("readdir", payload, r))
}

func TestRename(t *testing.T) {
	help := Helper(t)
	existsFile := help.createFile("exists.txt", "some data")
	missingFile := help.tempPath("missing.txt")
	renameTo := help.tempPath("rename.txt")
	e := &ErrorCode{}
	payload := &Rename{From: existsFile, To: renameTo}

	help.httpOk(help.req("rename", payload, e))
	help.exists(renameTo)

	// test renaming a missing file
	e = &ErrorCode{}
	payload = &Rename{From: missingFile, To: renameTo}
	help.httpBad(help.req("rename", payload, e))
	help.errorCode(e.Code, "ENOENT")
}

func TestWrite(t *testing.T) {
	help := Helper(t)
	writtenFile := help.tempPath("written.txt")
	openMap := help.newMap()
	payload := &Open{Path: writtenFile, Flags: os.O_RDWR | os.O_CREATE | os.O_TRUNC, Mode: 0777}

	help.httpOk(help.req("open", payload, &openMap))
	defer help.deferCloseFd(openMap)

	contents := "some sample file contents"
	buffer := base64.StdEncoding.EncodeToString([]byte(contents))
	w := map[string]any{"fd": openMap["fd"], "buffer": buffer, "length": len(contents)}

	writeResult := help.newMap()
	help.httpOk(help.req("write", w, &writeResult))

	written, ok := writeResult["written"].(float64)
	help.true(ok, "written value in return missing")
	help.true(int(written) == len(contents), "incorrect written length")
}

func TestWrite_with_position(t *testing.T) {
	help := Helper(t)
	writtenFile := help.createFile("writeSeek.txt", "1234567890")
	openMap := help.newMap()
	payload := &Open{Path: writtenFile, Flags: os.O_RDWR, Mode: 0777}

	help.httpOk(help.req("open", payload, &openMap))

	contents := "ZZZ"
	buffer := base64.StdEncoding.EncodeToString([]byte(contents))
	w := map[string]any{"fd": openMap["fd"], "position": 5, "buffer": buffer, "length": len(contents)}

	writeResult := help.newMap()
	help.httpOk(help.req("write", w, &writeResult))

	written, ok := writeResult["written"].(float64)
	help.true(ok, "written value in return missing")
	help.true(int(written) == len(contents), "incorrect written length")

	help.httpOk(help.req("close", openMap, &ErrorCode{}))

	file, err := os.ReadFile(writtenFile)
	help.nilErr(err)
	help.true("12345ZZZ90" == string(file), fmt.Sprintf("expected 12345ZZZ90 but got %q", string(file)))
}

func TestWrite_bad(t *testing.T) {
	help := Helper(t)
	writtenFile := help.tempPath("written.txt")
	openMap := help.newMap()
	payload := &Open{Path: writtenFile, Flags: os.O_RDWR | os.O_CREATE | os.O_TRUNC, Mode: 0777}

	help.httpOk(help.req("open", payload, &openMap))
	defer help.deferCloseFd(openMap)

	//// failing test cases
	//var pos = 5
	//help.httpBad(help.req("write", &Write{Position: &pos}, &ErrorCode{}))
	help.httpBad(help.req("write", &Write{Offset: 1}, &ErrorCode{}))
	help.httpBad(help.req("write", &Write{Buffer: "%%%"}, &ErrorCode{}))
	help.httpBad(help.req("write", &Write{Buffer: ""}, &ErrorCode{}))
}

func TestClose_bad(t *testing.T) {
	help := Helper(t)
	closeMap := map[string]any{"fd": math.MaxInt64}

	// close with bad file descriptor should error
	help.httpBad(help.req("close", closeMap, &ErrorCode{}))
}

func TestMkdir(t *testing.T) {
	help := Helper(t)
	payload := &Mkdir{Path: help.tempPath("nested")}
	help.httpOk(help.req("mkdir", payload, &ErrorCode{}))
	help.exists(payload.Path)

	// mkdir without parent directory should fail
	if runtime.GOOS != "windows" {
		payload = &Mkdir{Path: help.tempPath("nested/levels")}
		help.httpBad(help.req("mkdir", payload, &ErrorCode{}))
	}
}

func TestRmdir(t *testing.T) {
	help := Helper(t)
	payload := &Rmdir{Path: help.tempPath("nested")}
	help.httpOk(help.req("mkdir", payload, &ErrorCode{}))
	help.exists(payload.Path)

	help.httpOk(help.req("rmdir", payload, &ErrorCode{}))
	_, err := os.Stat(payload.Path)
	help.true(os.IsNotExist(err), "directory not removed")

	response := &ErrorCode{}
	payload.Path = help.tempPath("missing")
	help.httpBad(help.req("rmdir", payload, response))

}

func TestUnlink(t *testing.T) {
	help := Helper(t)

	tempFile := help.createFile("delete_me.txt", "to delete")
	payload := &Unlink{Path: tempFile}
	help.httpOk(help.req("unlink", payload, &ErrorCode{}))

	payload = &Unlink{Path: help.tempPath("missing.txt")}
	help.httpBad(help.req("unlink", payload, &ErrorCode{}))
}

type readResult struct {
	Read   int    `json:"read"`
	Buffer string `json:"buffer"`
}

func TestRead(t *testing.T) {
	help := Helper(t)

	content := "some data"
	tmpFile := help.createFile("file.txt", content)
	m := help.newMap()
	help.httpOk(help.req("open", &Open{Path: tmpFile}, &m))
	defer help.deferCloseFd(m)

	readMap := map[string]any{"fd": m["fd"], "offset": 0, "length": len(content)}
	resultMap := &readResult{}
	help.httpOk(help.req("read", readMap, &resultMap))
	help.true(resultMap.Read == len(content), "read length incorrect")

	decodedRead, errDecode := base64.StdEncoding.DecodeString(resultMap.Buffer)
	help.nilErr(errDecode).true(string(decodedRead) == content, "read data did not match")

	readMap = map[string]any{"fd": m["fd"], "offset": 0, "position": 1, "length": len(content) - 1}
	help.httpOk(help.req("read", readMap, &resultMap))
	decodedRead, errDecode = base64.StdEncoding.DecodeString(resultMap.Buffer)
	help.nilErr(errDecode).true(string(decodedRead) == content[1:], "read data did not match")

	readMap = map[string]any{"fd": m["fd"], "offset": 1, "length": len(content) - 1}
	help.httpBad(help.req("read", readMap, &resultMap))

	readMap = map[string]any{"fd": m["fd"], "offset": 0, "position": -1, "length": 1}
	help.httpBad(help.req("read", readMap, &resultMap))

	// read on bad file descriptor
	readMap = map[string]any{"fd": math.MaxInt64, "offset": 0, "length": len(content)}
	help.httpBad(help.req("read", readMap, &resultMap))

}

func Test_handle(t *testing.T) {
	help := Helper(t)

	header := make(http.Header)
	header.Set("WBT-Token", TOKEN)
	u, err := url.Parse("http://localhost:12345/fs/open")
	help.nilErr(err)

	w := &httptest.ResponseRecorder{Body: &bytes.Buffer{}}
	request := &http.Request{URL: u, Header: header, Body: io.NopCloser(bytes.NewBufferString(""))}
	help.handler.handle(&Open{}, w, request)
	help.httpBad(w.Code)
}

func TestToken(t *testing.T) {
	help := Helper(t)

	u, err := url.Parse("http://localhost:12345/fs/open")
	help.nilErr(err)

	w := &httptest.ResponseRecorder{Body: &bytes.Buffer{}}
	request := &http.Request{URL: u, Body: io.NopCloser(bytes.NewBufferString("{}"))}
	help.handler.ServeHTTP(w, request)
	help.httpBad(w.Code)
}

func TestServeDefault(t *testing.T) {
	help := Helper(t)
	u, err := url.Parse("http://localhost:12345/fs/badpath")
	help.nilErr(err)
	header := make(http.Header)
	header.Set("WBT-Token", TOKEN)
	w := &httptest.ResponseRecorder{Body: &bytes.Buffer{}}
	request := &http.Request{URL: u, Header: header, Body: io.NopCloser(bytes.NewBufferString("{}"))}

	help.handler.ServeHTTP(w, request)
	help.httpBad(w.Code)
}

func Test_doError(t *testing.T) {
	help := Helper(t)
	w := &BrokenResponseRecorder{}
	help.handler.doError("msg", "code", w)
}

func Test_okResponse(t *testing.T) {
	help := Helper(t)
	w := &httptest.ResponseRecorder{Body: &bytes.Buffer{}}
	m := help.newMap()
	help.handler.okResponse(m, w)
	help.true(w.Body.String() == "{}", "body string did not match")
	help.true(w.Header().Get("Content-Type") == "application/json", "bad content type header")

	// test case for bad serialization
	w = &httptest.ResponseRecorder{Body: &bytes.Buffer{}}
	help.handler.okResponse(&badJson{}, w)
	help.true(w.Body.String() == "", "body should be empty")
	help.true(w.Code == http.StatusInternalServerError, "bad http code")
}

type badJson struct {
	Broken func() `json:"broken"`
}

// END TESTS

type BrokenResponseRecorder struct {
	httptest.ResponseRecorder
}

func (rw *BrokenResponseRecorder) Write(buf []byte) (int, error) {
	return 0, fmt.Errorf("broken pipe for data %q", string(buf))
}

// Helper provides convenience methods for testing the handler.
func Helper(t *testing.T) *helperApi {
	help := &helperApi{
		t:      t,
		m:      &sync.Mutex{},
		tmpDir: t.TempDir(),
	}
	var logger *log.Logger
	if os.Getenv("DEBUG_FS_HANDLER") != "" {
		logger = log.New(os.Stderr, "[wasmbrowsertest]: ", log.LstdFlags|log.Lshortfile)
	} else {
		logger = log.New(io.Discard, "", 0)
	}
	help.handler = NewHandler(TOKEN, logger)
	help.handler.debug = true
	help.tmpDir = t.TempDir()
	return help
}

type helperApi struct {
	t       *testing.T
	m       *sync.Mutex
	handler *Handler
	tmpDir  string
}

func (h *helperApi) req(path string, payload any, response any) (code int) {
	h.t.Helper()

	u, err := url.Parse("http://localhost:12345/fs/" + path)
	h.nilErr(err)
	header := make(http.Header)
	header.Set("WBT-Token", TOKEN)

	body, err := json.Marshal(payload)
	h.nilErr(err)

	req := &http.Request{URL: u, Header: header, Body: io.NopCloser(bytes.NewReader(body))}
	w := &httptest.ResponseRecorder{Body: &bytes.Buffer{}}
	h.handler.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), response)
	h.nilErr(err)
	return w.Code
}

func (h *helperApi) tempPath(path string) string {
	h.t.Helper()
	return filepath.Join(h.tmpDir, path)
}

func (h *helperApi) createFile(path string, contents string) string {
	h.t.Helper()
	filePath := h.tempPath(path)
	h.nilErr(os.WriteFile(filePath, []byte(contents), 0644))
	return filePath
}

func (h *helperApi) nilErr(err error) *helperApi {
	h.t.Helper()
	if err != nil {
		h.t.Fatal(err)
	}
	return h
}

func (h *helperApi) exists(path string) {
	h.t.Helper()
	_, err := os.Stat(path)
	h.true(err == nil, fmt.Sprintf("path %s does not exist", path))
}

func (h *helperApi) httpOk(code int) *helperApi {
	h.t.Helper()
	if code != http.StatusOK {
		h.t.Fatalf("incorrect http code %d - expected 200", code)
	}
	return h
}

func (h *helperApi) httpBad(code int) *helperApi {
	h.t.Helper()
	if code != http.StatusBadRequest {
		h.t.Fatalf("incorrect http code %d - expected 400", code)
	}
	return h
}

func (h *helperApi) errorCode(actual, expected string) *helperApi {
	h.t.Helper()
	if actual != expected {
		h.t.Fatalf("incorrect error code %q - expected %q", actual, expected)
	}
	return h
}

func (h *helperApi) true(condition bool, message string) *helperApi {
	h.t.Helper()
	if !condition {
		h.t.Fatal(message)
	}
	return h
}

func (h *helperApi) newMap() map[string]any {
	return map[string]any{}
}

func (h *helperApi) checkStatMap(m map[string]any, path string) {
	h.t.Helper()

	stat, err := os.Stat(path)
	h.nilErr(err)

	if size, ok := m["size"].(int64); ok {
		if size != stat.Size() {
			h.t.Fatal("got incorrect size")
		}
	}
	if mTime, ok := m["mtimeMs"].(int64); ok {
		if mTime != stat.ModTime().UnixMilli() {
			h.t.Fatal("got incorrect mtimeMs")
		}
	}
	if mode, ok := m["mode"].(int64); ok {
		if mode != int64(stat.Mode()) {
			h.t.Fatal("got incorrect mode")
		}
	}
}

// sysOpen returns a file descriptor for an existing file and a function to close it.
// The return type is any to support both int and windows descriptors.
func (h *helperApi) sysOpen(path string) (result any, deferClose func()) {
	h.t.Helper()
	fd, err := syscall.Open(path, 0, 0)
	h.nilErr(err)
	return fd, func() {
		errClose := syscall.Close(fd)
		if errClose != nil {
			h.t.Fatal(errClose)
		}
	}
}

func (h *helperApi) deferCloseFd(m map[string]any) {
	h.t.Helper()
	h.httpOk(h.req("close", m, &ErrorCode{}))
}
