// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package descriptor

import (
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"

	"go.dokimi.dev/eidos/sdk/position"
)

// The field numbers of descriptor.proto that a path of source code info
// follows, by the message that declares each field.
const (
	filePackage    = 2
	fileDependency = 3
	fileMessage    = 4
	fileEnum       = 5
	fileService    = 6
	fileExtension  = 7
	fileSyntax     = 12
	fileEdition    = 14

	fileOptionDependency = 15
	fileOptions          = 8

	messageExtensionRange = 5
	messageOptions        = 7
	fieldOptions          = 8
	oneofOptions          = 2
	enumOptions           = 3
	enumValueOptions      = 3
	serviceOptions        = 3
	methodOptions         = 4
	rangeOptions          = 3

	messageField     = 2
	messageNested    = 3
	messageEnum      = 4
	messageExtension = 6
	messageOneof     = 8

	fieldExtendee = 2
	fieldTypeName = 6
	fieldScalar   = 5

	enumValue = 2

	serviceMethod = 2

	methodInput  = 2
	methodOutput = 3
)

// commentMark opens each line of a comment that the lowering hands the
// unit. protoc removes the marks from the text of a comment.
const commentMark = "//"

// locations indexes the source code info of one file by path. A path is
// encoded as a string of its numbers, so a lookup allocates nothing.
type locations struct {
	file   string
	byPath map[string]*descriptorpb.SourceCodeInfo_Location
	// blocks are the locations of the extend blocks, by the path of the
	// list of extensions that they declare: the extensions of the file or
	// of one message. protoc gives each block the path of the list.
	blocks map[string][]*descriptorpb.SourceCodeInfo_Location
}

// newLocations indexes the source code info of a file whose path is file.
// A file without source code info has an empty index.
func newLocations(file string, info *descriptorpb.SourceCodeInfo) *locations {
	l := &locations{file: file, byPath: map[string]*descriptorpb.SourceCodeInfo_Location{}}
	for _, loc := range info.GetLocation() {
		key := pathKey(loc.GetPath())
		if n := len(loc.GetPath()); n > 0 && isExtensionList(loc.GetPath()) {
			if l.blocks == nil {
				l.blocks = map[string][]*descriptorpb.SourceCodeInfo_Location{}
			}
			l.blocks[key] = append(l.blocks[key], loc)
			continue
		}
		if _, held := l.byPath[key]; !held {
			l.byPath[key] = loc
		}
	}
	return l
}

// isExtensionList reports whether a path ends at the list of extensions of
// the file or of a message, which is the path of an extend block. The path
// of a message is a message of the file followed by nested messages, so a
// path of a field that ends in the number of its type name is no list.
func isExtensionList(path []int32) bool {
	n := len(path)
	switch {
	case n == 1:
		return path[0] == fileExtension
	case n >= 3 && n%2 == 1 && path[0] == fileMessage && path[n-1] == messageExtension:
		for k := 2; k < n-1; k += 2 {
			if path[k] != messageNested {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// pathKey encodes a path as a string of its numbers.
func pathKey(path []int32) string {
	var b strings.Builder
	for _, n := range path {
		b.WriteRune(n + 1)
	}
	return b.String()
}

// under reports whether a location exists below a path, at a longer path
// that starts with it. It reads every location, so the lowering asks it
// only for an option of a message type without a location of its own.
func (l *locations) under(path []int32) bool {
	prefix := pathKey(path)
	for key := range l.byPath {
		if len(key) > len(prefix) && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// at returns the location of a path, and nil for a path without one.
func (l *locations) at(path []int32) *descriptorpb.SourceCodeInfo_Location {
	return l.byPath[pathKey(path)]
}

// start returns the position of the first byte of a location. A span of
// source code info counts lines and columns from zero, and a position
// counts them from one. A nil location is at the start of the file.
func (l *locations) start(loc *descriptorpb.SourceCodeInfo_Location) position.Pos {
	span := loc.GetSpan()
	if len(span) < 3 {
		return position.Pos{File: l.file, Line: 1, Col: 1}
	}
	return position.Pos{File: l.file, Line: int(span[0]) + 1, Col: int(span[1]) + 1}
}

// endLine returns the line of the last byte of a location, from one.
func endLine(loc *descriptorpb.SourceCodeInfo_Location) int {
	span := loc.GetSpan()
	switch len(span) {
	case 3:
		return int(span[0]) + 1
	case 4:
		return int(span[2]) + 1
	default:
		return 1
	}
}

// contains reports whether the span of outer contains the start of inner.
func contains(outer, inner *descriptorpb.SourceCodeInfo_Location) bool {
	o, i := outer.GetSpan(), inner.GetSpan()
	if len(o) < 3 || len(i) < 3 {
		return false
	}
	startLine, startCol := o[0], o[1]
	endLine, endCol := o[0], o[2]
	if len(o) == 4 {
		endLine, endCol = o[2], o[3]
	}
	afterStart := i[0] > startLine || i[0] == startLine && i[1] >= startCol
	beforeEnd := i[0] < endLine || i[0] == endLine && i[1] <= endCol
	return afterStart && beforeEnd
}

// rawComment returns the text of a comment of source code info with a
// comment mark at each line, which the unit splits as it splits a comment
// of the source. It returns the empty string for an empty comment.
func rawComment(text string) string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = commentMark + line
	}
	return strings.Join(lines, "\n")
}

// lineCount returns the number of lines of a comment of source code info.
func lineCount(text string) int {
	return strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
}
