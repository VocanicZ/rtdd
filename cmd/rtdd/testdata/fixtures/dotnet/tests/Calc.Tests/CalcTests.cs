using Xunit;

namespace Calc.Tests;

public class CalcTests
{
    [Fact]
    public void Adds() => Assert.Equal(3, Calculator.Add(1, 2));
}
