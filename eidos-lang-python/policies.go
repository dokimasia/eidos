// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// The lowering policies of the Python target. A config file selects a
// choice for every plan or for one plan. The param dynamic of the python
// directive selects the dynamic type for one declaration. The version
// applies to every file of a plan, so the directive has no param for it.
const (
	// VersionPolicy chooses the version of Python that the backend writes
	// for, from 3.11 to 3.15.
	VersionPolicy plugin.PolicyKey = "python.version"
	// DynamicPolicy chooses the type of a dynamic value: object, the top
	// type, or Any, which turns the checks of a type checker off.
	DynamicPolicy plugin.PolicyKey = "python.dynamic"
)

// The choices of [VersionPolicy]: the versions of Python that the Python
// core developers support.
const (
	Version311 plugin.Choice = "3.11"
	Version312 plugin.Choice = "3.12"
	Version313 plugin.Choice = "3.13"
	Version314 plugin.Choice = "3.14"
	Version315 plugin.Choice = "3.15"
)

// The choices of [DynamicPolicy].
const (
	Object plugin.Choice = "object"
	Any    plugin.Choice = "Any"
)

// versionMajor is the major version that every choice of [VersionPolicy]
// has.
const versionMajor = "3."

// The docs of the policies, which document each policy's key and the param
// of the python directive that overrides it.
const (
	versionDoc = "the version of Python that the written files run on"
	dynamicDoc = "the type of a dynamic value"
)

// Policies returns the specs of the two policies of the target. The default
// version is the oldest version that the Python core developers support, so
// a written file runs on every supported version. The version is a policy
// of the plan, because the render writes the syntax of one version in every
// file. The default dynamic type is object, the top type of Python. The
// backend declares the policies through the backend kit.
//
// # Allocation contract
//
// Policies allocates the list of specs and the list of choices of each
// spec: three allocations.
func Policies() []plugin.PolicySpec {
	return []plugin.PolicySpec{
		{
			Key:     VersionPolicy,
			Choices: []plugin.Choice{Version311, Version312, Version313, Version314, Version315},
			Default: Version311, Doc: versionDoc, Plan: true,
		},
		{Key: DynamicPolicy, Choices: []plugin.Choice{Object, Any}, Default: Object, Doc: dynamicDoc},
	}
}

// Minor returns the minor version of a version of Python 3, as 12 for 3.12,
// and false for a text that is not a version of Python 3. A backend reads
// the minor version of the choice of [VersionPolicy], and compares it with
// the version that adds a construct. The frontend reads the minor version
// of its option. Minor does not allocate.
func Minor(version string) (int, bool) {
	rest, three := strings.CutPrefix(version, versionMajor)
	if !three {
		return 0, false
	}
	minor, err := strconv.ParseUint(rest, 10, 8)
	if err != nil {
		return 0, false
	}
	return int(minor), true
}
