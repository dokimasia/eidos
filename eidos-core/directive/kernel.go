// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package directive

// The six names that belong to the kernel. The registry refuses
// them from any plugin, and the composition registers their
// schemas before any plugin's, so an impersonation is a plain
// duplicate by the time it arrives.
const (
	// KernelMeta overrides metadata at directive authority. Its
	// drop param resolves against the metadata registry and
	// removes a fact or a group.
	KernelMeta Name = "meta"
	// KernelOut redirects a declaration's output; its path param
	// is the override.
	KernelOut Name = "out"
	// KernelDiag suppresses a diagnostic code at a declaration.
	KernelDiag Name = "diag"
	// KernelSkip excludes a declaration from bare and fact-gated
	// rules, or from one plugin.
	KernelSkip Name = "skip"
	// KernelSample states a declaration's two authored values, as
	// text in the source language.
	KernelSample Name = "sample"
	// KernelWitness states a concrete type per type parameter of
	// the declaration it sits on; its schema is open, one key per
	// parameter.
	KernelWitness Name = "witness"
)

// kernelNames lists the reserved names for the registry's check.
var kernelNames = []Name{KernelMeta, KernelOut, KernelDiag, KernelSkip, KernelSample, KernelWitness}

// The kernel schemas' param keys, declared beside their schemas so
// nothing drifts and no generator exists.
const (
	// MetaDrop names the fact or group the meta directive removes.
	MetaDrop ParamKey = "drop"
	// OutPath is the out directive's redirect target.
	OutPath ParamKey = "path"
	// OutTag selects a companion output.
	OutTag ParamKey = "tag"
	// DiagOff names the code the diag directive suppresses.
	DiagOff ParamKey = "off"
	// SkipPlugin narrows a skip to one plugin.
	SkipPlugin ParamKey = "plugin"
	// SampleValue is the sample directive's first authored value.
	SampleValue ParamKey = "value"
	// SampleAlternate is the sample directive's second authored
	// value, one that differs from the first.
	SampleAlternate ParamKey = "alternate"
)

// openKey is the key an open spec is checked under at
// registration, where the instance's own spelling is not yet
// known.
const openKey ParamKey = "<open>"

// Kernel returns the kernel-owned schemas: meta, out, diag, skip,
// sample and witness. Their semantics stay with their owners; what
// registers here is the spelling and its validation.
func Kernel() []Schema {
	return []Schema{
		{
			Name: KernelMeta,
			Params: []ParamSpec{
				{
					Key: MetaDrop, Type: TypeReference, Resolution: ResolveMetadataKey,
					Doc: "the fact or group to remove; the only way to negate a stamped boolean",
				},
			},
			Repeatable: true,
			Doc:        "overrides metadata at directive authority",
		},
		{
			Name: KernelOut,
			Params: []ParamSpec{
				{
					Key: OutPath, Type: TypeString,
					Doc: "the path the declaration's output redirects to",
				},
				{
					Key: OutTag, Type: TypeString,
					Doc: "the companion output the declaration arrives in",
				},
			},
			Doc: "overrides routing for a declaration",
		},
		{
			Name: KernelDiag,
			Params: []ParamSpec{
				{
					Key: DiagOff, Type: TypeString, Required: true,
					Doc: "the diagnostic code suppressed at this declaration",
				},
			},
			Repeatable: true,
			Doc:        "suppresses a diagnostic at a declaration",
		},
		{
			Name: KernelSkip,
			Params: []ParamSpec{
				{
					Key: SkipPlugin, Type: TypeString,
					Doc: "narrows the exclusion to one plugin",
				},
			},
			Doc: "excludes a declaration from bare and fact-gated rules",
		},
		{
			Name: KernelSample,
			Params: []ParamSpec{
				{
					Key: SampleValue, Type: TypeString, Required: true,
					Doc: "the declaration's authored value, as text in the source language",
				},
				{
					Key: SampleAlternate, Type: TypeString,
					Doc: "a second authored value differing from the first",
				},
			},
			Doc: "states the values a declaration's type takes, ahead of any derivation",
		},
		{
			Name: KernelWitness,
			Open: &ParamSpec{
				Type: TypeReference, Resolution: ResolveTypeInScope,
				Doc: "the concrete type the named type parameter runs at",
			},
			Doc: "states a concrete type per type parameter, one key per parameter",
		},
	}
}
