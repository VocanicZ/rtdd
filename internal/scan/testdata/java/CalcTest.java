package calc;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

class CalcTest {
    @Test
    void testApply() {
        assertEquals(3, new Calc().apply(1, 2));
    }

    @Test
    void testSum() {
        Calc c = new Calc();
        assertEquals(6, c.sum(new int[] {1, 2, 3}));
    }
}
