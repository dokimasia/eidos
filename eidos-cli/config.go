// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"go.dokimi.dev/eidos/cli/internal/config"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/output"
)

// configExt is the extension of a config file.
const configExt = ".yaml"

// ConfigError is a fault in a config file: the file, the line and the
// message of the fault. Its Error method returns the fault in the form
// FILE:LINE: MSG, or FILE: MSG for a fault without a line. [Open],
// [Configure] and [Member.Extension] return errors that wrap one
// *ConfigError for each fault of a file.
type ConfigError = config.Error

// ConfigName returns the name of the config file of a brand: the name of
// the state directory of the brand and the extension .yaml, such as
// .acme.yaml.
func ConfigName(brand output.Brand) string {
	return ledger.StateDir(brand) + configExt
}
