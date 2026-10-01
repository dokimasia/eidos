// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

/** Color is an annotated enum whose second constant has a body, an anonymous class. */
@Marker("color")
public enum Color implements Holder<String> {
    RED("r"),
    GREEN("g") {
        @Override
        public String get() {
            return "G";
        }
    },
    @Deprecated
    BLUE("b");

    /** FIRST is a field of the enum that is no constant of it. */
    public static final Color FIRST = RED;

    private final String code;

    Color(String code) {
        this.code = code;
    }

    @Override
    public String get() {
        return code;
    }
}
