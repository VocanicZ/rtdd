<?php

use Calc\Calc;
use PHPUnit\Framework\TestCase;

class CalcTest extends TestCase
{
    public function testAdds(): void
    {
        $this->assertSame(3, (new Calc())->add(1, 2));
    }
}
