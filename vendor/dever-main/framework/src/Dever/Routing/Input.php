<?php namespace Dever\Routing;

use Dever\Loader\Config;
use Dever\String\Helper;
use Dever\Support\Env;
use Dever\Output\Export;

class Input
{
    /**
     * request
     *
     * @var array
     */
    private static $request = array();

    /**
     * param
     *
     * @var array
     */
    private static $param = '';

    /**
     * command
     *
     * @var bool
     */
    public static $command = false;

    /**
     * method
     *
     * @var bool
     */
    public static $method = false;

    /**
     * init status
     *
     * @var bool
     */
    public static $init = false;

    /**
     * init
     *
     * @return array
     */
    public static function init()
    {
        if (self::$init == false) {
            global $argc, $argv;
            if ($argc > 1) {
                self::requestCommand($argc, $argv);
            } else {
                self::requestHttp();
            }
            self::$init = true;
        }

        if (isset(self::$request['l'])) {
            unset(self::$request['l']);
        }

        if (isset(self::$request['h'])) {
            unset(self::$request['h']);
        }
    }

    /**
     * method
     *
     * @return mixed
     */
    public static function method()
    {
        if (!self::$method) {
            self::$method = isset($_SERVER['REQUEST_METHOD']) ? $_SERVER['REQUEST_METHOD'] : 'GET';
        }
        
        return self::$method;
    }

    /**
     * ip
     *
     * @return mixed
     */
    public static function ip()
    {
        return Env::ip();
    }

    /**
     * requestHttp
     *
     * @return mixed
     */
    protected static function requestHttp()
    {
        self::$request = $_GET;
        $method = self::method();
        if ($method && $method == 'POST' && isset($_POST) && $_POST) {
            self::$request = array_merge(self::$request, $_POST);
        }

        if (isset($_FILES) && $_FILES) {
            self::$request = array_merge(self::$request, $_FILES);
        }

        self::$param = http_build_query(self::$request);

        if (!isset(self::$request['h'])) {
            $header = Env::header();
            if (isset($header['cookie'])) {
                unset($header['cookie']);
            }
            self::$request = array_merge(self::$request, $header);
        }
        
        //unset($_GET);unset($_POST);unset($_FILES);
        self::$command = false;
    }

    /**
     * requestCommand
     * @param int $argc
     * @param array $argv
     *
     * @return mixed
     */
    protected static function requestCommand($argc, $argv)
    {
        $command = array();
        $key = '';
        for ($i = 1; $i < $argc; $i++) {
            if (substr($argv[$i], 0, 1) == '-' || substr($argv[$i], 0, 1) == '^') {
                if ($key != '') {
                    $command[$key] = $key;
                }
                $key = substr($argv[$i], 1);
                continue;
            }
            if ($key != '') {
                $command[$key] = $argv[$i];
                $key = '';
            } else {
                $command[$argv[$i]] = $argv[$i];
            }
        }

        self::$request = $command;
        self::$command = true;
        unset($command);
    }

    /**
     * get
     * @param string $name
     * @param string $value
     *
     * @return mixed
     */
    public static function get($name = false, $value = '', $condition = '', $alert = '')
    {
        self::init();
        if ($name == 'dever_uri') {
            return self::$param;
        }
        if (!$name) {
            return self::$request;
        }

        if (is_array($name)) {
            foreach ($name as $key) {
                $value[] = self::get($key);
            }
            return $value;
        }

        if (is_string($name) && isset(self::$request[$name]) && self::$request[$name]) {
            $value = Helper::xss(self::$request[$name]);
        }

        self::getEncode($name, $value);

        if ($condition) {
            if (!$value) {
                Export::alert($alert);
            }
            $state = false;
            $test = '$state = ' . $value . $condition . ';';
            eval($test);
            if (!$state) {
                Export::alert($alert);
            }
        }

        return $value;
    }

    /**
     * getEncode
     * @param string $name
     * @param string $value
     *
     * @return array
     */
    protected static function getEncode($name, &$value)
    {
        if (Config::get('base')->urlEncode) {
            $config = Config::get('base')->urlEncode;
            if ($config && in_array($name, $config) && is_string($value)) {
                $method = Config::get('base')->urlEncodeMethod[1];
                if (strpos($method, 'Dever::') !== false) {
                    $method = str_replace('Dever::', '', $method);
                    $value = \Dever\String\Helper::$method($value);
                } else {
                    $value = $method($value);
                }
            }
        }
    }

    /**
     * prefix
     * @param string $name
     *
     * @return array
     */
    public static function prefix($name)
    {
        self::init();
        $key = 'prefix_' . $name;
        if (isset(self::$request[$key])) {
            return self::$request[$key];
        }
        self::$request[$key] = array();

        foreach (self::$request as $k => $v) {
            if (strpos($k, $name) === 0) {
                self::$request[$key][$k] = Helper::xss($v);
            }
        }

        return self::$request[$key];
    }

    /**
     * set
     * @param string $name
     *
     * @return array
     */
    public static function set($name, $value = '', $prefix = false)
    {
        self::init();
        if ($name == 'all' && is_array($value)) {
            self::$request = array_merge(self::$request, $value);
        } else {
            self::$request[$name] = $value;
        }

        if ($prefix) {
            $prefix = 'prefix_' . $prefix;
            if (isset(self::$request[$prefix])) {
                self::$request[$prefix][$name] = $value;
            }
        }

        return $value;
    }

    /**
     * shell
     *
     * @return mixed
     */
    public static function shell($value = '')
    {
        if (!Config::get('base')->shell) {
            $shell = self::get('shell');
            if ($shell) {
                $shell = explode(',', $shell);
                Config::get('base')->shell = $shell;
            }
        }
        if ($value && Config::get('base')->shell && in_array($value, Config::get('base')->shell)) {
            return true;
        }
        return false;
    }
}
