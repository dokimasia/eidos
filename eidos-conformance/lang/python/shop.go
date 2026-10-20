// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/workspace"
	golang "go.dokimi.dev/eidos/lang/go"
	pythonlang "go.dokimi.dev/eidos/lang/python"
	pythonfrontend "go.dokimi.dev/eidos/lang/python/frontend"
	pythonrules "go.dokimi.dev/eidos/lang/python/rules"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/output"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// Brand is the brand under which the shop fixture stamps its files. A
// carrier of the fixture starts with it, as in #+acme:typescript.
const Brand output.Brand = "acme"

// The module that the edit of the shop fixture changes, the field of Line
// before and after the edit, and the mode of the file that the edit writes.
const (
	modelsFile    = "src/shop/models.py"
	intQuantity   = "quantity: int = 1"
	floatQuantity = "quantity: float = 1"
	modelsMode    = 0o644
)

// ComposeShop returns the composition of the shop fixture over root without
// its plans. The composition has the Python frontend and rules, the Go,
// TypeScript and Python targets, a disk sink at root and the ledger of the
// state directory under root. The Python backend of each plan writes for
// Python 3.15, the version of the syntax of the sources. Build refuses the
// choice of the version for a composition whose plans have no Python
// backend. [ShopPlans] returns the plans that it runs.
func ComposeShop(root string) *workspace.Builder {
	return workspace.New().
		Brand(Brand).
		Frontends(pythonfrontend.New(nil)).
		Rules(pythonrules.New()).
		Targets(golang.Target, typescript.Target, pythonlang.Target).
		Config(workspace.Config{
			Policies: map[plugin.PolicyKey]plugin.Choice{pythonlang.VersionPolicy: pythonlang.Version315},
		}).
		Output(func() (output.Sink, error) { return output.NewDisk(root, Brand) }).
		Ledger(func() (ledger.Ledger, error) { return ledger.OpenDir(root, Brand) })
}

// ShopPlans returns the plans of the shop fixture over one graph, in the
// order python-mirror, go-model and ts-client: [MirrorPlan], [ModelPlan] and
// [ClientPlan]. Each call returns new generators and backends.
func ShopPlans() []workspace.Plan {
	return []workspace.Plan{MirrorPlan(), ModelPlan(), ClientPlan()}
}

// EditShop is the edit of the shop fixture. It makes the field quantity of
// the dataclass Line in src/shop/models.py under root a float. The mirror
// that python-mirror, the first plan, writes changes, and the export of
// python-mirror does not, because an export lists the names of the
// declarations and of their members and not their types. The struct of
// go-model changes too. It reads and writes the one file through an
// [os.Root] of root.
//
// Error modes: the error of a root or a file that does not open, read or
// write, and an error for a module that does not declare the field quantity
// as an int.
func EditShop(root string) error {
	r, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("python: %w", err)
	}
	defer r.Close()
	name := filepath.FromSlash(modelsFile)
	b, err := r.ReadFile(name)
	if err != nil {
		return fmt.Errorf("python: %w", err)
	}
	src := string(b)
	if !strings.Contains(src, intQuantity) {
		return fmt.Errorf("python: %s declares no field %q to make a float", modelsFile, intQuantity)
	}
	if err := r.WriteFile(name, []byte(strings.Replace(src, intQuantity, floatQuantity, 1)), modelsMode); err != nil {
		return fmt.Errorf("python: %w", err)
	}
	return nil
}
