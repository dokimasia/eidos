// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"encoding/xml"
	"fmt"
)

// coordinateSeparator joins a module's group and artifact, as in
// com.acme:store.
const coordinateSeparator = ":"

// pom is the part of a pom.xml the frontend reads: the module's group
// and artifact, and the group the parent states, which a module without
// a group of its own inherits.
type pom struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Parent     struct {
		GroupID string `xml:"groupId"`
	} `xml:"parent"`
}

// parsePOM decodes a pom.xml.
func parsePOM(data []byte) (*pom, error) {
	var p pom
	if err := xml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("frontend: %w", err)
	}
	return &p, nil
}

// module returns the module's coordinates, group:artifact, the group the
// parent's where the module states none, and empty for a module without
// an artifact.
func (p *pom) module() string {
	if p.ArtifactID == "" {
		return ""
	}
	group := p.GroupID
	if group == "" {
		group = p.Parent.GroupID
	}
	return group + coordinateSeparator + p.ArtifactID
}
