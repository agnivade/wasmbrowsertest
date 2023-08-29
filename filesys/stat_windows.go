//go:build windows

package filesys

import (
	"io/fs"
	"net/http"
	"os"
	"syscall"
)

func (st *Stat) WriteResponse(fa *FsHandler, w http.ResponseWriter) {
	stat, err := os.Stat(fixPath(st.Path))
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(mapOfFileInfo(stat), w)
}

func (f *Fstat) WriteResponse(fa *FsHandler, w http.ResponseWriter) {
	fileInfo := &syscall.ByHandleFileInformation{}
	err := syscall.GetFileInformationByHandle(FdType(f.Fd), fileInfo)
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(mapOfByHandleFileInformation(fileInfo), w)
}

func (ls *Lstat) WriteResponse(fa *FsHandler, w http.ResponseWriter) {
	stat, err := os.Stat(fixPath(ls.Path))
	if fa.handleError(w, err, true) {
		return
	}
	fa.okResponse(mapOfFileInfo(stat), w)
}

func mapOfFileInfo(s os.FileInfo) map[string]any {
	mode := s.Mode() & fs.ModePerm
	if s.IsDir() {
		mode |= 1 << 14
	}
	return map[string]any{
		"dev": 0, "ino": 0, "mode": mode,
		"nlink": 0, "uid": 1000, "gid": 1000,
		"rdev": 0, "size": s.Size(), "blksize": 0,
		"blocks": 0, "atimeMs": s.ModTime().UnixMilli(),
		"mtimeMs": s.ModTime().UnixMilli(), "ctimeMs": s.ModTime().UnixMilli(),
	}
}

func mapOfByHandleFileInformation(s *syscall.ByHandleFileInformation) map[string]any {
	size := int64(s.FileSizeHigh)<<32 + int64(s.FileSizeLow)
	var mode os.FileMode
	if s.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		mode |= 0444
	} else {
		mode |= 0666
	}
	if s.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		mode |= 1 << 14
	}

	nsToMs := func(ft syscall.Filetime) int64 {
		return ft.Nanoseconds() / 1e6
	}
	return map[string]any{
		"dev": 0, "ino": 0, "mode": mode,
		"nlink": 0, "uid": 1000, "gid": 1000,
		"rdev": 0, "size": size, "blksize": 0,
		"blocks": 0, "atimeMs": nsToMs(s.LastAccessTime),
		"mtimeMs": nsToMs(s.LastWriteTime), "ctimeMs": nsToMs(s.CreationTime),
	}
}
