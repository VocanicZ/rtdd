<?php

namespace Calc;

class Store
{
    public static function get(string $key): string
    {
        return "v:" . $key;
    }
}
