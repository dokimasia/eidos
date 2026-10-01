// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

/** Range is a record without type arguments whose compact canonical constructor is declared. */
public record Range(int lo, int hi) {
    public Range {
        if (lo > hi) {
            throw new IllegalArgumentException();
        }
    }
}
