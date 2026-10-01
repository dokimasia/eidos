// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

import java.lang.annotation.RetentionPolicy;

/** Spec is a run-time invisible annotation type with an element of every element value kind. */
public @interface Spec {
    byte b();

    char c();

    double d();

    float f();

    int i();

    long j();

    short s();

    boolean z();

    String str();

    RetentionPolicy e();

    Class<?> cls();

    Marker ann();

    int[] arr();
}
