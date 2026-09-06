using Xunit;

namespace Calc.Tests;

public class CalcTests
{
    [Fact]
    public void Adds() => Assert.Equal(3, Calc.Add(1, 2));
}
