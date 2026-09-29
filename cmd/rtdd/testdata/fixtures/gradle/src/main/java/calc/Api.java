package calc;

public final class Api {
    public static String fetch(String key) {
        return Store.get(key);
    }
}
