// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

import java.util.List;

/** Point is a record with a compact canonical constructor, an annotated component, a field and a member type. */
public record Point(int x, @Marker("y") int y, List<String> tags) implements Comparable<Point> {
    /** ORIGIN is a field of the record that is no component of it. */
    public static final Point ORIGIN = new Point(0, 0, List.of());

    public Point {
        if (x < 0) {
            throw new IllegalArgumentException();
        }
    }

    @Override
    public int compareTo(Point o) {
        return 0;
    }

    /** Axis is a member type of the record. */
    public enum Axis {
        X,
        Y
    }
}
