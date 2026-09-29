namespace Calc;

public static class Api
{
    public static string Fetch(string key)
    {
        return Store.Get(key);
    }
}
