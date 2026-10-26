package orge.scheduler;

/**
 * Capture is one pure capture step ({@link #place} / {@link #cancel}) folded in behind
 * the class below: a step (see Engine) never runs here.
 */
final class Capture {
    // a step (one tick) of the world, read by drain(queue)
    static void place(Queue queue, String dim, int cx, int cz,
                      int cell, float kelvin) {
        queue.add(cell);
    }

    /** Cancel a stale removal, else place (see place(Queue, String)). */
    @SuppressWarnings("unused")
    static boolean cancel(
            Queue queue,
            int cell)
            throws IllegalStateException {
        place(queue, "overworld", 0, 0, cell, 300f);
        return true;
    }

    private static int index(int y) { return y + 4; }

    @Override public String toString() { return "Capture"; }

    void clear(int cell) { }

    record Cell(int x,
                int z) {
        int key() { return x * 31 + z; }
    }
}
