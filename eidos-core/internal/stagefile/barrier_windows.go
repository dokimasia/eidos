// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stagefile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The flags of NtFlushBuffersFileEx that the barrier passes. flushSync
// writes the data and the metadata of a file and synchronizes the storage
// to flush its cache. flushNoSync, FLUSH_FLAGS_NO_SYNC in ntifs.h, writes
// the data and the metadata and leaves the cache of the storage.
const (
	flushSync   = 0
	flushNoSync = 2
)

// The file systems for which the documentation of NtFlushBuffersFileEx
// lists FLUSH_FLAGS_NO_SYNC, as GetVolumeInformationByHandleW names them.
const (
	fileSystemNTFS  = "NTFS"
	fileSystemFAT   = "FAT"
	fileSystemFAT32 = "FAT32"
	fileSystemExFAT = "exFAT"
)

// finalPathFlags asks GetFinalPathNameByHandleW for the normalized path
// with the volume's drive letter: FILE_NAME_NORMALIZED and VOLUME_NAME_DOS,
// which are both zero.
const finalPathFlags = 0

// fileSystemNameLen is the length in UTF-16 units of the buffer that
// receives the name of a file system.
const fileSystemNameLen = windows.MAX_PATH + 1

// ntFlushBuffersFileEx is NtFlushBuffersFileEx of ntdll.dll, which
// golang.org/x/sys/windows does not wrap. Windows provides it since
// Windows 8.
var ntFlushBuffersFileEx = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtFlushBuffersFileEx")

// windowsBarrier is the barrier of Windows. A flush needs a handle with
// write or append access, and a directory handle that [os.Root] opens has
// read access alone, so the barrier opens the root and each directory
// itself, by its path below the final path of the root.
type windowsBarrier struct {
	// path is the final path of the root.
	path string
	// dir is the root, open for writing from Start to Close.
	dir windows.Handle
	// serial is the serial number of the root's volume.
	serial uint32
	// noSync reports whether the root's file system takes
	// FLUSH_FLAGS_NO_SYNC.
	noSync bool
}

// System returns a new barrier of Windows. File flushes each staging file
// with NtFlushBuffersFileEx and FLUSH_FLAGS_NO_SYNC, which writes its data
// and its metadata to the device without a flush of the device's cache.
// The order barrier flushes the root with flags 0, which also flushes that
// cache, so the data that the earlier flushes wrote is on stable storage
// when it returns. The durability barrier flushes each changed directory
// with FLUSH_FLAGS_NO_SYNC, then the root with flags 0. On a file system
// that [NoSyncFlush] does not list, such as ReFS, and for a file or a
// directory on another volume than the root, each flush takes flags 0. It
// allocates the barrier.
func System() Barrier { return &windowsBarrier{} }

// NoSyncFlush reports whether the documentation of NtFlushBuffersFileEx
// lists FLUSH_FLAGS_NO_SYNC for the file system that
// GetVolumeInformationByHandleW names fileSystem: NTFS, FAT, FAT32 and
// exFAT. It does not allocate.
func NoSyncFlush(fileSystem string) bool {
	switch fileSystem {
	case fileSystemNTFS, fileSystemFAT, fileSystemFAT32, fileSystemExFAT:
		return true
	default:
		return false
	}
}

// Start loads NtFlushBuffersFileEx, reads the final path of the root, and
// opens the root for writing. It reads the serial number of the root's
// volume and the name of its file system.
//
// Error modes: a procedure that does not load, a root that does not open
// or whose path does not read, a root that does not open for writing,
// and a volume that does not read, each wrapped.
func (b *windowsBarrier) Start(r *os.Root, _ int) error {
	if err := ntFlushBuffersFileEx.Find(); err != nil {
		return fmt.Errorf("stagefile: load NtFlushBuffersFileEx: %w", err)
	}
	root, err := r.Open(".")
	if err != nil {
		return fmt.Errorf("stagefile: open the root: %w", err)
	}
	path, err := finalPath(windows.Handle(root.Fd()))
	if err = errors.Join(err, root.Close()); err != nil {
		return fmt.Errorf("stagefile: read the path of the root: %w", err)
	}
	dir, err := openDir(path)
	if err != nil {
		return err
	}
	var (
		serial uint32
		name   [fileSystemNameLen]uint16
	)
	err = windows.GetVolumeInformationByHandle(dir, nil, 0, &serial, nil, nil, &name[0], fileSystemNameLen)
	if err != nil {
		return errors.Join(fmt.Errorf("stagefile: read the volume of the root: %w", err), windows.CloseHandle(dir))
	}
	b.path, b.dir, b.serial, b.noSync = path, dir, serial, NoSyncFlush(windows.UTF16ToString(name[:]))
	return nil
}

// File flushes f with FLUSH_FLAGS_NO_SYNC where f is on the root's volume
// and the file system takes the flag, and with flags 0 otherwise.
//
// Error modes: a volume that does not read and a flush that fails,
// wrapped.
func (b *windowsBarrier) File(f *os.File) error { return b.flush(windows.Handle(f.Fd()), f.Name()) }

// Order flushes the root with flags 0, which flushes the cache of the
// storage.
//
// Error modes: the status of the flush, wrapped.
func (b *windowsBarrier) Order() error { return ntFlush(b.dir, flushSync, b.path) }

// Durable opens each directory of dirs for writing and flushes it as File
// flushes a file, on the pool, then flushes the root with flags 0.
//
// Error modes: the error of each directory that does not open, flush or
// close, and the status of the last flush, joined.
func (b *windowsBarrier) Durable(dirs []string) error {
	errs := make([]error, len(dirs), len(dirs)+1)
	Each(len(dirs), func(i int) {
		path := filepath.Join(b.path, filepath.FromSlash(dirs[i]))
		h, err := openDir(path)
		if err != nil {
			errs[i] = err
			return
		}
		errs[i] = errors.Join(b.flush(h, path), windows.CloseHandle(h))
	})
	return errors.Join(append(errs, b.Order())...)
}

// Close closes the root.
//
// Error modes: the error of the close, wrapped.
func (b *windowsBarrier) Close() error {
	if err := windows.CloseHandle(b.dir); err != nil {
		return fmt.Errorf("stagefile: close the root: %w", err)
	}
	return nil
}

// flush flushes one file or directory: with FLUSH_FLAGS_NO_SYNC where it is
// on the root's volume and the file system takes the flag, and with flags
// 0 otherwise. name is its path for the errors.
//
// Error modes: a volume that does not read and a flush that fails,
// wrapped.
func (b *windowsBarrier) flush(h windows.Handle, name string) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return fmt.Errorf("stagefile: read the volume of %s: %w", name, err)
	}
	flags := uintptr(flushSync)
	if b.noSync && info.VolumeSerialNumber == b.serial {
		flags = flushNoSync
	}
	return ntFlush(h, flags, name)
}

// finalPath returns the final path of the file or directory that h refers
// to, as GetFinalPathNameByHandleW returns it.
//
// Error modes: the error of GetFinalPathNameByHandleW.
func finalPath(h windows.Handle) (string, error) {
	size := uint32(windows.MAX_PATH)
	for {
		buf := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(h, &buf[0], size, finalPathFlags)
		if err != nil {
			return "", fmt.Errorf("stagefile: %w", err)
		}
		if n < size {
			return windows.UTF16ToString(buf[:n]), nil
		}
		size = n
	}
}

// openDir opens the directory at path for writing, which a flush needs.
// The handle shares reading, writing and deletion, so the commit's own
// handles keep working.
//
// Error modes: a path that does not convert to UTF-16 and a directory that
// does not open, each wrapped.
func openDir(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("stagefile: %s: %w", path, err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, fmt.Errorf("stagefile: open %s for writing: %w", path, err)
	}
	return h, nil
}

// ntFlush calls NtFlushBuffersFileEx on h with flags. name is the path of
// h for the error.
//
// Error modes: the NTSTATUS of a flush that fails, wrapped.
func ntFlush(h windows.Handle, flags uintptr, name string) error {
	var status windows.IO_STATUS_BLOCK
	code, _, _ := ntFlushBuffersFileEx.Call(uintptr(h), flags, 0, 0, uintptr(unsafe.Pointer(&status)))
	if code != 0 {
		//nolint:gosec // An NTSTATUS is 32 bits wide, and the call returns it in a register.
		return fmt.Errorf("stagefile: flush %s: %w", name, windows.NTStatus(code))
	}
	return nil
}
