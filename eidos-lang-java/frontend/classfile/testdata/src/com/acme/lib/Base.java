// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

/** Base is the generic superclass whose erased get makes javac write a bridge in Box. */
public abstract class Base<T> {
    protected Base(T value) {
    }

    public abstract T get();
}
