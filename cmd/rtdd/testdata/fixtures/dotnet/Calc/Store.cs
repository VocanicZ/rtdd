namespace Calc;

public static class Store
{
    public static string Get(string key)
    {
        return "v:" + key;
    }
}
