<?php

namespace Calc;

class Api
{
    public static function handle(string $key): string
    {
        return Store::get($key);
    }
}
