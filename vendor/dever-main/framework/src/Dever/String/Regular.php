<?php namespace Dever\String;

class Regular
{
	public static function rule($method, $fix = '/', $rule = '')
    {
        $method = 'rule_' . $method;
        return $fix[0] . self::$method($rule) . $fix;
    }

    # 手机号
    public static function rule_mobile($rule)
    {
        return '^(1([3456789][0-9]))\d{8}$';
    }

    # 邮箱
    public static function rule_email($rule)
    {
        return '^([a-zA-Z0-9]+[_|\_|\.]?)*[a-zA-Z0-9]+@([a-zA-Z0-9]+[_|\_|\.]?)*[a-zA-Z0-9]+\.[a-zA-Z]{2,3}$';
    }

    # 中文
    public static function rule_zh($rule)
    {
        $rule = $rule ? $rule : 8;
        return '^([\x{4e00}-\x{9fa5}]){'.$rule.'}$';
    }
}