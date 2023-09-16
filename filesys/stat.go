//go:build darwin || linux

package filesys

import (
	"net/http"
	"syscall"
)

func (st *Stat) WriteResponse(fa *Handler, w http.ResponseWriter) {
	s := &syscall.Stat_t{}
	err := syscall.Stat(fixPath(st.Path), s)
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(mapOfStatT(s), w)
}

func (f *Fstat) WriteResponse(fa *Handler, w http.ResponseWriter) {
	s := &syscall.Stat_t{}
	err := syscall.Fstat(f.Fd, s)
	if fa.handleError(w, err, false) {
		return
	}
	fa.okResponse(mapOfStatT(s), w)
}

func (ls *Lstat) WriteResponse(fa *Handler, w http.ResponseWriter) {
	s := &syscall.Stat_t{}
	err := syscall.Lstat(fixPath(ls.Path), s)
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(mapOfStatT(s), w)
}
