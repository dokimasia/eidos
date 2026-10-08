// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"

	"go.yaml.in/yaml/v3"
)

// List is a config file that lists workspaces. A command runs the
// workspaces of a list in list order.
type List struct {
	Version    Version `yaml:"version"    schema:"required" doc:"The version of the file format. It must be 1."`
	Workspaces []Entry `yaml:"workspaces" schema:"required" doc:"The workspaces, in the order that a command runs them."`
}

// Entry is one workspace of a list. Both paths use slashes.
type Entry struct {
	Root   string `yaml:"root"   schema:"required" doc:"The workspace root. A relative path is relative to the directory of the list."`
	Config string `yaml:"config"                   doc:"The config file of the workspace. A relative path is relative to the directory of the list. The default is the config file of the brand in the root."`
}

// check returns an [Error] for a list without workspaces, and one Error for
// each entry without a root. n is the node of the workspaces key, and each
// Error has the line of the list or of the entry.
func (l *List) check(name string, n *yaml.Node) error {
	if len(l.Workspaces) == 0 {
		return &Error{File: name, Line: n.Line, Msg: "the list has no workspaces"}
	}
	var errs []error
	for i, e := range l.Workspaces {
		if e.Root == "" {
			errs = append(errs, &Error{File: name, Line: n.Content[i].Line, Msg: "the workspace has no root"})
		}
	}
	return errors.Join(errs...)
}
