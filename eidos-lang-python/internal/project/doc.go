// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package project places the modules of a Python workspace: it reads the
// source roots that the build backend of a pyproject.toml states, and it
// derives the package of a module from its source root.
//
// [Read] decodes one project file into a [Project]. [Place] returns the
// source root of a file: a root that the table of its build backend
// states, src below the directory of a project that states none, or the
// directory above the run of package directories that contain the file.
// [Package] returns the package of a module below its root, as the import
// system of Python names it.
//
// # Build backends
//
// Read reads the table of the backend that the table build-system names:
//
//   - setuptools: the key "" of package-dir, and each where of
//     packages.find, below [tool.setuptools]
//   - hatchling: the sources of [tool.hatch.build.targets.wheel], and the
//     parent directory of each of its packages
//   - poetry-core: the from of each entry of the packages of [tool.poetry]
//   - uv_build: module-root of [tool.uv.build-backend]
//   - pdm-backend: package-dir of [tool.pdm.build]
//   - maturin: python-source of [tool.maturin]
//
// A project file without a table build-system builds with setuptools.
//
// # Dependency position
//
// internal/project imports github.com/pelletier/go-toml/v2, lang/python and
// the Go stdlib.
package project
