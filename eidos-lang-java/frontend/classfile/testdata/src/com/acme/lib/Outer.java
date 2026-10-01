// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

/** Outer declares a member record, which is static as every member record is. */
public class Outer {
    /** Pair is the member record, generic in its left component. */
    public record Pair<L>(L left, int right) {
    }
}
