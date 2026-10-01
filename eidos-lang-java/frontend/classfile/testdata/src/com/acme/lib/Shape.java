// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

/** Shape is a sealed interface that permits two classes. */
public sealed interface Shape permits Circle, Square {
}
