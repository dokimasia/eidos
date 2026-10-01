// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

/** Marker is a run-time visible annotation type with defaults. */
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.TYPE, ElementType.PACKAGE, ElementType.FIELD, ElementType.METHOD, ElementType.PARAMETER,
    ElementType.RECORD_COMPONENT})
public @interface Marker {
    String value() default "";

    int[] codes() default {};
}
