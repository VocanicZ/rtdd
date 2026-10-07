package calc;

import java.util.ArrayList;
import java.util.List;

public class Calc implements Op {
    private final List<Integer> history = new ArrayList<>();

    public Calc() {
        history.clear();
    }

    @Override
    public int apply(int a, int b) {
        int r = a + b;
        history.add(r);
        return r;
    }

    public static <T> List<T> wrap(T x) {
        List<T> out = new ArrayList<>();
        out.add(x);
        return out;
    }

    public int sum(int[] ns) {
        int total = 0;
        for (int n : ns) {
            if (n < 0) {
                continue;
            }
            total = apply(total, n);
        }
        while (total > 100) {
            total -= 100;
        }
        return total;
    }
}
