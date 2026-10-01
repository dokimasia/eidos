// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

/** Holder is an interface with an abstract, a default, a static and a private method. */
public interface Holder<T> {
    int CONSTANT = 3;

    T get();

    default boolean present() {
        return helper();
    }

    static <T> Holder<T> empty() {
        return null;
    }

    private boolean helper() {
        return get() != null;
    }
}
