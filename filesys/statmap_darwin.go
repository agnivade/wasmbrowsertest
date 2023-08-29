package filesys

import "syscall"

func mapOfStatT(s *syscall.Stat_t) map[string]any {

	toMs := func(ts syscall.Timespec) int64 { return ts.Sec*1000 + ts.Nsec/1e6 }

	// https://github.com/golang/go/blob/c19c4c566c63818dfd059b352e52c4710eecf14d/src/syscall/fs_js.go#L165
	return map[string]any{
		"dev": s.Dev, "ino": s.Ino, "mode": s.Mode,
		"nlink": s.Nlink, "uid": s.Uid, "gid": s.Gid,
		"rdev": s.Rdev, "size": s.Size, "blksize": s.Blksize,
		"blocks": s.Blocks, "atimeMs": toMs(s.Atimespec),
		"mtimeMs": toMs(s.Mtimespec), "ctimeMs": toMs(s.Ctimespec),
	}
}
