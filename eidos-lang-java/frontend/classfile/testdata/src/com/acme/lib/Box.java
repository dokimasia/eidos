// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package com.acme.lib;

import java.io.IOException;
import java.util.Collection;
import java.util.List;
import java.util.Map;

/** Box is a generic container: the class-file reader's main fixture. */
@Marker("box")
public class Box<T extends Comparable<T>> extends Base<T> implements Holder<T>, Cloneable {
    public static final int LIMIT = 16;
    public static final long BIG = 1L << 40;
    public static final float RATIO = 1.5f;
    public static final double HALF = 0.5;
    public static final double NAN = Double.NaN;
    public static final char LETTER = 'x';
    public static final boolean ON = true;
    public static final byte SMALL = 7;
    public static final short MID = 300;
    public static final String NAME = "a\"b\\c\né";

    @Marker("n")
    protected List<? extends Number> numbers;
    public Map<String, List<T>> index;
    public List<?> any;
    int hidden;
    private int secret;

    public Box(T value) {
        super(value);
    }

    protected Box() {
        super(null);
    }

    private Box(int x) {
        super(null);
    }

    @Override
    public T get() {
        return null;
    }

    public <U extends T> U conv(List<? super U> sink, int... more) throws IOException {
        return null;
    }

    public static <E extends Exception> void fail() throws E {
    }

    @Deprecated
    public final void done(@Marker String why, final int code) {
    }

    public int[][] grid() {
        return null;
    }

    public Box<T>.Inner inner() {
        return null;
    }

    public void all(String... names) {
    }

    public void fill(int[] values) {
    }

    public <V> void two(V a) {
    }

    public static <E extends Object & Comparable<? super E>> E max(Collection<? extends E> items) {
        return null;
    }

    void pkg() {
    }

    private void priv() {
    }

    public Runnable task() {
        class Local implements Runnable {
            public void run() {
            }
        }
        return new Runnable() {
            public void run() {
                new Local().run();
            }
        };
    }

    /** Inner is bound to a Box, whose field it writes through the synthetic this$0. */
    public class Inner {
        public Inner(String label) {
        }

        /** attach takes a Box first, as Inner's constructor does. */
        @SuppressWarnings("rawtypes")
        public void attach(Box other) {
            hidden = other.hidden;
        }
    }

    /** Nested is bound to no Box, and takes one first. */
    public static class Nested {
        @SuppressWarnings("rawtypes")
        public Nested(Box owner) {
        }
    }

    /** Callback is a member interface, static as every member interface is. */
    protected interface Callback {
        void call();

        Done done();

        /** Done is a member interface of a member interface. */
        interface Done {
        }
    }

    /** Secret has package access, which a signature-only load leaves out. */
    static class Secret {
    }
}
