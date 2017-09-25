<?php namespace Dever\Loader;

class Lang
{
    /**
     * get
     * @param string $key
     * @param array $param
     *
     * @return string
     */
    public static function get($key = 'host', $param = array())
    {
        $name = 'lang/' . Config::get('base')->lang;

        if (Config::get($name)->$key) {
            Config::get($name)->$key = self::replace(Config::get($name)->$key, $param);
            return Config::get($name)->$key;
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
