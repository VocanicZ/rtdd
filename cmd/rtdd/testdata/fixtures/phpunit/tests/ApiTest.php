<?php

use Calc\Api;
use PHPUnit\Framework\TestCase;

class ApiTest extends TestCase
{
    public function testHandle(): void
    {
        $this->assertSame("v:a", Api::handle("a"));
    }
}
