package calc;

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

class ApiTest {
    @Test
    void fetches() {
        assertEquals("v:k", Api.fetch("k"));
    }
}
