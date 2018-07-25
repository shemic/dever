<?php namespace Dever\String;

class Regular
{
	public static function rule($method, $fix = '/')
    {
        $method = 'rule_' . $method;
        return $fix . self::$method() . $fix;
    }

    public static function rule_mobile()
    {
        return '^(1([3456789][0-9]))\d{8}$';
    }

    public static function rule_email()
    {
        return '^([a-zA-Z0-9]+[_|\_|\.]?)*[a-zA-Z0-9]+@([a-zA-Z0-9]+[_|\_|\.]?)*[a-zA-Z0-9]+\.[a-zA-Z]{2,3}$';
    }
}