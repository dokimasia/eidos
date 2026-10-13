// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/workspace"
)

// Configure applies the config file of the brand of b in root to b, as
// [Open] configures a workspace: the name, the workers, the ignored
// directives, the parse memo, the refinement of each plan, the policies and
// the options. The file is the config file of the brand in root, such as
// .acme.yaml, and Configure searches no other directory. It leaves b as it
// is where root has no such file, and it adds no sink and no ledger. It
// skips the keys of the options of the command line.
//
// Error modes: Configure returns an error for a composition without a valid
// brand, for a file that does not read or decode, for a file that lists
// workspaces, and for a plugin directory of the file that does not load.
// The error of a file that does not decode contains the path of the file
// relative to root.
func Configure(b *workspace.Builder, root string) error {
	brand := b.BrandName()
	if !brand.Valid() {
		return fmt.Errorf("cli: the composition has the brand %q, which names no config file", brand)
	}
	name := ConfigName(brand)
	data, err := os.ReadFile(filepath.Join(root, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cli: read the config file: %w", err)
	}
	decoded, err := config.Decode(name, data)
	if err != nil {
		return err
	}
	if decoded.List != nil {
		return fmt.Errorf("cli: the config file %s lists workspaces, and a composition configures one", name)
	}
	return decoded.Document.Apply(b, root)
}
