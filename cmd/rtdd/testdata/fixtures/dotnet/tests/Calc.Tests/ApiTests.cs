using Xunit;

namespace Calc.Tests;

public class ApiTests
{
    [Fact]
    public void Fetches() => Assert.Equal("v:k", Api.Fetch("k"));
}
