// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package stagefile_test

import (
	"io/fs"
	"testing"

	"go.dokimi.dev/assert"
	"golang.org/x/sys/unix"

	"go.dokimi.dev/eidos/core/internal/stagefile"
)

// The fixture releases of Linux: a distribution's build of 7.2, the first
// release whose syncfs(2) reports a failed write-back, and the release
// before it.
const (
	releaseFedora = "7.2.7-200.fc44.x86_64"
	releaseFirst  = "5.8.0"
	releaseBefore = "5.7.19"
)

// The Linux barrier calls syncfs(2) for a group that does not fit one
// round of the pool, on a kernel that reports a failed write-back through
// it, over a filesystem whose sync operation makes every file durable.
func TestBarrierLinux(t *testing.T) {
	t.Parallel()

	t.Run("ChoosesSyncfs", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			release string
			fsType  int64
			names   int
			want    bool
		}{
			{
				name: "reports true for a group of Workers names on Linux 7.2 over btrfs", release: releaseFedora,
				fsType: unix.BTRFS_SUPER_MAGIC, names: stagefile.Workers, want: true,
			},
			{
				name: "reports true for Linux 5.8 over ext4", release: releaseFirst,
				fsType: unix.EXT4_SUPER_MAGIC, names: stagefile.Workers, want: true,
			},
			{
				name: "reports true for Linux 6.0 over XFS", release: "6.0",
				fsType: unix.XFS_SUPER_MAGIC, names: stagefile.Workers, want: true,
			},
			{
				name: "reports true for a release candidate of Linux 6.12", release: "6.12-rc1",
				fsType: unix.XFS_SUPER_MAGIC, names: 1000, want: true,
			},
			{
				name: "reports false for a group that fits one round of the pool", release: releaseFedora,
				fsType: unix.BTRFS_SUPER_MAGIC, names: stagefile.Workers - 1,
			},
			{
				name: "reports false for Linux 5.7", release: releaseBefore,
				fsType: unix.EXT4_SUPER_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for Linux 4.19", release: "4.19.0",
				fsType: unix.EXT4_SUPER_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for tmpfs", release: releaseFedora,
				fsType: unix.TMPFS_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for a FUSE filesystem", release: releaseFedora,
				fsType: unix.FUSE_SUPER_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for an empty release", release: "",
				fsType: unix.BTRFS_SUPER_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for a release without a dot", release: "7",
				fsType: unix.BTRFS_SUPER_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for a release whose major is not a number", release: "x.8",
				fsType: unix.BTRFS_SUPER_MAGIC, names: stagefile.Workers,
			},
			{
				name: "reports false for a release whose minor is not a number", release: "5.x",
				fsType: unix.BTRFS_SUPER_MAGIC, names: stagefile.Workers,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, stagefile.ChoosesSyncfs(tt.release, tt.fsType, tt.names), tt.want,
					"the choice of syncfs")
			})
		}
	})

	t.Run("System", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a barrier whose Start fails for a root that does not open", func(t *testing.T) {
			t.Parallel()

			dir, r := rooted(t, nil)
			restricted(t, dir, 0o300)
			g := stagefile.NewGroup(r, stagefile.Reserved, stagefile.System())
			g.Write(target, []byte(body), mode)
			failed, err := g.Commit()
			assert.ErrorIs(t, err, fs.ErrPermission, "the root refuses its read")
			assert.ErrorIs(t, failed[target], stagefile.ErrNotApplied, "and the write is not applied")
		})
	})
}

// ChoosesSyncfs allocates nothing in the ordinary run, which runs no
// benchmark.
func TestBarrierLinuxAllocs(t *testing.T) {
	var chosen bool
	assert.MaxAllocs(t, func() { chosen = stagefile.ChoosesSyncfs(releaseFedora, unix.BTRFS_SUPER_MAGIC, 64) }, 0,
		"ChoosesSyncfs allocates nothing")
	assert.True(t, chosen, "a large group on btrfs calls syncfs")
}
