// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

import java.lang.annotation.RetentionPolicy;

/** Specced is annotated with Spec, which states a value of every element value kind. */
@Spec(b = 1, c = 'q', d = 2.5, f = 1.25f, i = 42, j = 7L, s = 3, z = true, str = "s\t", e = RetentionPolicy.SOURCE,
    cls = String[].class, ann = @Marker(value = "m", codes = {1, 2}), arr = {})
public class Specced {
}
