// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package stagefile

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
)

// syncfsMajor and syncfsMinor are the first release of Linux whose
// syncfs(2) reports a failed write-back.
const (
	syncfsMajor = 5
	syncfsMinor = 8
)

// release is the release of the running kernel as uname(2) spells it,
// read once in a process. A uname(2) that fails reads as the empty
// string, which no release check passes.
var release = sync.OnceValue(func() string {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return ""
	}
	return unix.ByteSliceToString(u.Release[:])
})

// linuxBarrier is the barrier of Linux. Start chooses syncfs(2) by
// [ChoosesSyncfs], and the calls of fileSync otherwise.
type linuxBarrier struct {
	fileSync
	// release is the release of the kernel that System read.
	release string
	// dir is the root from Start to Close while the group calls
	// syncfs(2), and nil where the group syncs each file.
	dir *os.File
	// dev is the device of the root while dir is open.
	dev uint64
}

// System returns a new barrier of Linux. A group of [Workers] names or
// more on Linux 5.8 or newer, whose root is on ext4, XFS or btrfs, calls
// syncfs(2) on the root at each barrier, as [ChoosesSyncfs] states. Every
// other group syncs each staging file and each directory with fsync(2).
// A staging file or a directory on another device than the root gets an
// fsync(2) of its own in either case.
//
// syncfs(2) reports each failed write-back since the descriptor that it
// receives was opened, so Start opens the root before the first write.
// syncfs(2) runs the sync operation of the filesystem, and FUSE forwards
// that operation to a server only for virtiofs, so the barrier calls
// syncfs(2) on three filesystems alone. syncfs(2) also writes back the
// dirty data of every other process on the filesystem. A group that fits
// one round of the pool syncs each file, which does not wait for that
// data.
//
// It allocates the barrier.
func System() Barrier { return &linuxBarrier{release: release()} }

// ChoosesSyncfs reports whether a group of names writes and removals
// calls syncfs(2) at its barriers, on a kernel of release as uname(2)
// spells it, over a root on the filesystem whose magic number statfs(2)
// reports as fsType. It reports true for [Workers] names or more on Linux
// 5.8 or newer over ext4, XFS or btrfs, and false for a release that it
// does not read. It does not allocate.
func ChoosesSyncfs(release string, fsType int64, names int) bool {
	if names < Workers {
		return false
	}
	switch fsType {
	case unix.EXT4_SUPER_MAGIC, unix.XFS_SUPER_MAGIC, unix.BTRFS_SUPER_MAGIC:
	default:
		return false
	}
	majorText, rest, dotted := strings.Cut(release, ".")
	if !dotted {
		return false
	}
	end := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		end = len(rest)
	}
	major, majorErr := strconv.Atoi(majorText)
	if majorErr != nil {
		return false
	}
	minor, minorErr := strconv.Atoi(rest[:end])
	if minorErr != nil {
		return false
	}
	return major > syncfsMajor || major == syncfsMajor && minor >= syncfsMinor
}

// Start keeps the root, and opens it where the group calls syncfs(2). It
// reads the type of the root's filesystem from fstatfs(2), and the root's
// device from fstat(2).
//
// Error modes: a root that does not open, an fstatfs(2) or an fstat(2)
// that fails, and a close of the root that fails, each wrapped.
func (b *linuxBarrier) Start(r *os.Root, n int) error {
	b.root = r
	dir, err := r.Open(".")
	if err != nil {
		return fmt.Errorf("stagefile: open the root: %w", err)
	}
	var fsStat unix.Statfs_t
	if err := unix.Fstatfs(int(dir.Fd()), &fsStat); err != nil {
		return errors.Join(fmt.Errorf("stagefile: fstatfs the root: %w", err), dir.Close())
	}
	magic := int64(fsStat.Type) //nolint:unconvert // Statfs_t.Type is an int32 or a uint32 on some architectures.
	if !ChoosesSyncfs(b.release, magic, n) {
		if err := dir.Close(); err != nil {
			return fmt.Errorf("stagefile: close the root: %w", err)
		}
		return nil
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(dir.Fd()), &st); err != nil {
		return errors.Join(fmt.Errorf("stagefile: fstat the root: %w", err), dir.Close())
	}
	b.dir, b.dev = dir, uint64(st.Dev) //nolint:unconvert // Stat_t.Dev is a uint32 on some architectures.
	return nil
}

// File syncs f with fsync(2) where the group syncs each file, and where
// f is on another device than the root. It does nothing for a file on the
// root's device while the group calls syncfs(2).
//
// Error modes: an fstat(2) or a sync that fails, wrapped.
func (b *linuxBarrier) File(f *os.File) error {
	if b.dir == nil {
		return b.fileSync.File(f)
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return fmt.Errorf("stagefile: fstat %s: %w", f.Name(), err)
	}
	if dev := uint64(st.Dev); dev != b.dev { //nolint:unconvert // Stat_t.Dev is a uint32 on some architectures.
		return b.fileSync.File(f)
	}
	return nil
}

// Order calls syncfs(2) on the root while the group calls syncfs(2), and
// returns nil where File synced each staging file.
//
// Error modes: the error of syncfs(2), wrapped.
func (b *linuxBarrier) Order() error {
	if b.dir == nil {
		return b.fileSync.Order()
	}
	return syncfs(b.dir)
}

// Durable syncs each directory of dirs with fsync(2) where the group
// syncs each file. While the group calls syncfs(2), it syncs each
// directory on another device than the root with fsync(2), then calls
// syncfs(2) on the root. It stats each directory relative to the root.
//
// Error modes: the error of each stat and each sync that fails, and the
// error of syncfs(2), joined.
func (b *linuxBarrier) Durable(dirs []string) error {
	if b.dir == nil {
		return b.fileSync.Durable(dirs)
	}
	var (
		other []string
		errs  []error
	)
	for _, dir := range dirs {
		var st unix.Stat_t
		if err := unix.Fstatat(int(b.dir.Fd()), dir, &st, 0); err != nil {
			errs = append(errs, fmt.Errorf("stagefile: stat %s: %w", dir, err))
			continue
		}
		if dev := uint64(st.Dev); dev != b.dev { //nolint:unconvert // Stat_t.Dev is a uint32 on some architectures.
			other = append(other, dir)
		}
	}
	return errors.Join(append(errs, syncDirs(b.root, other), syncfs(b.dir))...)
}

// Close closes the root where Start opened it.
//
// Error modes: the error of the close, wrapped.
func (b *linuxBarrier) Close() error {
	if b.dir == nil {
		return b.fileSync.Close()
	}
	if err := b.dir.Close(); err != nil {
		return fmt.Errorf("stagefile: close the root: %w", err)
	}
	return nil
}

// syncfs calls syncfs(2) on the filesystem of dir.
//
// Error modes: the error of syncfs(2), wrapped.
func syncfs(dir *os.File) error {
	if err := unix.Syncfs(int(dir.Fd())); err != nil {
		return fmt.Errorf("stagefile: syncfs %s: %w", dir.Name(), err)
	}
	return nil
}
