// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build windows

package local

import (
	"syscall"
	"unsafe"
)

// See https://msdn.microsoft.com/en-us/library/windows/desktop/aa365240(v=vs.85).aspx

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procMoveFileExW = kernel32.NewProc("MoveFileExW")
)

const (
	moveFileReplaceExisting = 1
	moveFileWriteThrough    = 8
)

func moveFileEx(source, target *uint16, flags uint32) error {
	ret, _, err := procMoveFileExW.Call(uintptr(unsafe.Pointer(source)), uintptr(unsafe.Pointer(target)), uintptr(flags))
	if ret == 0 {
		if err != nil {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

func atomicRename(source, target string) error {
	lpReplacedFileName, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	lpReplacementFileName, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	return moveFileEx(lpReplacementFileName, lpReplacedFileName, moveFileReplaceExisting|moveFileWriteThrough)
}
