<?php namespace Dever\Loader;

use Dever;

class Lang
{
    private static $data = array();

    /**
     * get
     * @param string $key
     * @param array $param
     *
     * @return string
     */
    public static function get($key = 'host', $param = array())
    {
        $lang = Dever::input('dever-lang');
        if (!$lang) {
            $lang = Dever::session('dever_lang');
            if (!$lang) {
                $lang = Config::get('base')->lang;
            }
        } else {
            Dever::session('dever_lang', $lang, 86400 * 365);
        }

        $name = 'lang/' . $lang;

        if (!isset(self::$data[$name])) {
            self::$data[$name] = Config::get($name)->cAll;
        }

        if (isset(self::$data[$name][$key])) {
            self::$data[$name][$key] = self::replace(self::$data[$name][$key], $param);
            return self::$data[$name][$key];
        }

        return $key;
    }

    /**
     * replace
     * @param string $param
     *
     * @return array
     */
    private static function replace($value, &$param)
    {
        if ($param) {
            $param = self::param($param);

            foreach ($param as $k => $v) {
                self::set($value, $k, $v);
            }
        }

        return $value;
    }

    /**
     * param
     * @param string $param
     *
     * @return array
     */
    private static function param($param)
    {
        if (is_string($param)) {
            $param = array($param);
        }

        return $param;
    }

    /**
     * set
     * @param string $value
     * @param string $k
     * @param string $v
     *
     * @return array
     */
    private static function set(&$value, $k, $v)
    {
        $k = '{' . $k . '}';
        if (strpos($value, $k) !== false) {
            $value = str_replace($k, $v, $value);
        }
    }
}
