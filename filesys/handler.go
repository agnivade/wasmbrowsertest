package filesys

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
)

// Handler translates json payload data to and from system calls like syscall.Stat
type Handler struct {
	debug         bool
	securityToken string
	logger        *log.Logger
}

func NewHandler(securityToken string, logger *log.Logger) *Handler {
	return &Handler{
		debug:         false,
		securityToken: securityToken,
		logger:        logger,
	}
}

func (fa *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("WBT-Token") != fa.securityToken {
		fa.doError("not implemented", "ENOSYS", w, errors.New("missing WBT-token"))
		return
	}
	switch r.URL.Path {
	case "/fs/stat":
		fa.handle(&Stat{}, w, r)
	case "/fs/fstat":
		fa.handle(&Fstat{}, w, r)
	case "/fs/open":
		fa.handle(&Open{}, w, r)
	case "/fs/write":
		fa.handle(&Write{}, w, r)
	case "/fs/close":
		fa.handle(&Close{}, w, r)
	case "/fs/rename":
		fa.handle(&Rename{}, w, r)
	case "/fs/readdir":
		fa.handle(&Readdir{}, w, r)
	case "/fs/lstat":
		fa.handle(&Lstat{}, w, r)
	case "/fs/read":
		fa.handle(&Read{}, w, r)
	case "/fs/mkdir":
		fa.handle(&Mkdir{}, w, r)
	case "/fs/unlink":
		fa.handle(&Unlink{}, w, r)
	case "/fs/rmdir":
		fa.handle(&Rmdir{}, w, r)
	default:
		fa.doError("not implemented", "ENOSYS", w,
			fmt.Errorf("unsupported api path %q", r.URL.Path))
	}
}

type Responder interface {
	WriteResponse(fa *Handler, w http.ResponseWriter)
}

func (fa *Handler) handle(responder Responder, w http.ResponseWriter, r *http.Request) {
	if err := json.NewDecoder(r.Body).Decode(responder); err != nil {
		fa.logger.Printf("ERROR handle : %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if fa.debug {
		fa.logger.Printf("handle %s %+v\n", r.URL.Path, responder)
	}
	responder.WriteResponse(fa, w)
}

type ErrorCode struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func (fa *Handler) doError(msg, code string, w http.ResponseWriter, err error) {
	if fa.debug {
		fa.logger.Printf("doError %s : %s\n", msg, code)
	}
	if err != nil {
		fa.logger.Printf("Error: %v", err)
	}

	e := &ErrorCode{Error: msg, Code: code}

	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(e); err != nil {
		fa.logger.Printf("Error encoding json: %v", err)
	}
}

func (fa *Handler) okResponse(data any, w http.ResponseWriter) {
	var marshal []byte
	var err error
	if marshal, err = json.Marshal(data); err != nil {
		fa.logger.Println("okResponse json error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if fa.debug {
		fa.logger.Printf("okResponse %s\n", string(marshal))
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(marshal); err != nil {
		fa.logger.Printf("Error writing json: %v", err)
	}
}

func fixPath(path string) string {
	return strings.TrimPrefix(path, "/fs/")
}

type Stat struct {
	Path string `json:"path,omitempty"`
}

type Open struct {
	Path  string `json:"path"`
	Flags int    `json:"flags"`
	Mode  uint32 `json:"mode"`
}

func (o *Open) WriteResponse(fa *Handler, w http.ResponseWriter) {
	fd, err := syscall.Open(fixPath(o.Path), o.Flags, o.Mode)
	if fa.handleError(w, err, true) {
		return
	}
	response := map[string]any{"fd": fd}
	fa.okResponse(response, w)
}

type Fstat struct {
	Fd int `json:"fd"`
}

type Write struct {
	Fd       int    `json:"fd"`
	Buffer   string `json:"buffer"`
	Offset   int    `json:"offset"`
	Length   int    `json:"length"`
	Position *int   `json:"position,omitempty"`
}

func (wr *Write) WriteResponse(fa *Handler, w http.ResponseWriter) {
	if wr.Offset != 0 {
		fa.doError("not implemented", "ENOSYS", w,
			fmt.Errorf("write offset %d not supported", wr.Offset))
		return
	}
	if wr.Position != nil {
		_, err := syscall.Seek(FdType(wr.Fd), int64(*wr.Position), 0)
		if err != nil {
			fa.doError("not implemented", "ENOSYS", w, err)
			return
		}
	}

	bytes, err := base64.StdEncoding.DecodeString(wr.Buffer)
	if err != nil {
		fa.doError("not implemented", "ENOSYS", w, err)
		return
	}

	var written int
	written, err = syscall.Write(FdType(wr.Fd), bytes)
	if err != nil {
		fa.doError("not implemented", "ENOSYS", w, err)
		return
	}

	fa.okResponse(map[string]any{"written": written}, w)
}

type Close struct {
	Fd int `json:"fd"`
}

func (c *Close) WriteResponse(fa *Handler, w http.ResponseWriter) {
	err := syscall.Close(FdType(c.Fd))
	if fa.handleError(w, err, false) {
		return
	}
	fa.okResponse(map[string]any{}, w)
}

type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (r *Rename) WriteResponse(fa *Handler, w http.ResponseWriter) {
	err := syscall.Rename(fixPath(r.From), fixPath(r.To))
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(map[string]any{}, w)
}

type Readdir struct {
	Path string `json:"path"`
}

func (r *Readdir) WriteResponse(fa *Handler, w http.ResponseWriter) {
	entries, err := os.ReadDir(fixPath(r.Path))
	if fa.handleError(w, err, false) {
		return
	}
	stringNames := make([]string, len(entries))
	for i, entry := range entries {
		stringNames[i] = entry.Name()
	}
	fa.okResponse(map[string]any{"entries": stringNames}, w)
}

type Lstat struct {
	Path string `json:"path"`
}

type Read struct {
	Fd       int  `json:"fd"`
	Offset   int  `json:"offset"`
	Length   int  `json:"length"`
	Position *int `json:"position,omitempty"`
}

func (r *Read) WriteResponse(fa *Handler, w http.ResponseWriter) {
	if r.Offset != 0 {
		fa.doError("not implemented", "ENOSYS", w,
			fmt.Errorf("read offset %d not supported", r.Offset))
		return
	}
	if r.Position != nil {
		_, err := syscall.Seek(FdType(r.Fd), int64(*r.Position), 0)
		if err != nil {
			fa.doError("not implemented", "ENOSYS", w, err)
			return
		}
	}

	buffer := make([]byte, r.Length)
	read, err := syscall.Read(FdType(r.Fd), buffer)
	if err != nil {
		fa.doError("not implemented", "ENOSYS", w, err)
		return
	}
	response := map[string]any{
		"read":   read,
		"buffer": base64.StdEncoding.EncodeToString(buffer[:read]),
	}
	fa.okResponse(response, w)

}

type Mkdir struct {
	Path string `json:"path"`
	Perm uint32 `json:"perm"`
}

func (m *Mkdir) WriteResponse(fa *Handler, w http.ResponseWriter) {
	err := syscall.Mkdir(fixPath(m.Path), m.Perm)
	if err != nil {
		fa.doError("not implemented", "ENOSYS", w, err)
		return
	}
	fa.okResponse(map[string]any{}, w)
}

type Unlink struct {
	Path string `json:"path"`
}

func (u *Unlink) WriteResponse(fa *Handler, w http.ResponseWriter) {
	err := syscall.Unlink(fixPath(u.Path))
	if err != nil {
		fa.doError("not implemented", "ENOSYS", w, err)
		return
	}
	fa.okResponse(map[string]any{}, w)
}

type Rmdir struct {
	Path string `json:"path"`
}

func (r *Rmdir) WriteResponse(fa *Handler, w http.ResponseWriter) {
	err := syscall.Rmdir(fixPath(r.Path))
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(map[string]any{}, w)
}

func (fa *Handler) handleError(w http.ResponseWriter, err error, noEnt bool) bool {
	if err == nil {
		return false
	}
	if noEnt && os.IsNotExist(err) {
		// We're not passing the error down for logging here since this is a
		// file not found condition, not an actual error condition.
		fa.doError(syscall.ENOENT.Error(), "ENOENT", w, nil)
	} else {
		fa.doError(syscall.ENOSYS.Error(), "ENOSYS", w, err)
	}
	return true
}
