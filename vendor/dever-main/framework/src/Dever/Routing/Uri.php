<?php namespace Dever\Routing;

use Dever\Loader\Config;

class Uri
{
    /**
     * explode
     *
     * @var string
     */
    const EXPLODE = '/';

    /**
     * uri value
     *
     * @var string
     */
    public static $value;

    /**
     * method
     *
     * @var string
     */
    public static $method;

    /**
     * uri type
     *
     * @var string
     */
    public static $type = '?';

    /**
     * url
     *
     * @var string
     */
    public static $url;

    /**
     * get
     *
     * @return string
     */
    public static function get()
    {
        if (self::$value) {
            return self::$value;
        }

        self::info();

        self::request();

        self::method();

        self::command();

        self::defaultValue();

        self::match();

        return self::$value;
    }

    /**
     * default
     *
     * @return mixed
     */
    private static function defaultValue()
    {
        empty(self::$value) && self::$value = 'home';
    }

    /**
     * method
     *
     * @return mixed
     */
    private static function method()
    {
        self::$method = isset($_SERVER['REQUEST_METHOD']) ? $_SERVER['REQUEST_METHOD'] : 'GET';
    }

    /**
     * command
     *
     * @return mixed
     */
    private static function command()
    {
        if (!self::$url && Input::get('send') && Input::$command == true) {
            self::$url = self::$value = str_replace(array('__', '^'), array('?', '&'), Input::get('send'));
        }
    }

    /**
     * info
     *
     * @return mixed
     */
    private static function info()
    {
        if (isset($_SERVER['PATH_INFO'])) {
            self::$type = '';

            self::$value = trim($_SERVER['PATH_INFO'], self::EXPLODE);

            self::$url = preg_replace('/^\//i', '', $_SERVER['REQUEST_URI']);
        }
    }

    /**
     * request
     *
     * @return mixed
     */
    private static function request()
    {
        if (!isset($_SERVER['PATH_INFO']) && isset($_SERVER['REQUEST_URI']) && $_SERVER['REQUEST_URI'] && $_SERVER['REQUEST_URI'] != '/') {
            $entry = defined('DEVER_ENTRY') ? DEVER_ENTRY : 'index.php';
            self::$type = $entry == 'index.php' ? '?' : $entry . '?';
            if ($_SERVER['REQUEST_URI'] != $_SERVER['SCRIPT_NAME']) {
                $script = substr($_SERVER['SCRIPT_NAME'], 0, strpos($_SERVER['SCRIPT_NAME'], $entry));

                if (strpos($_SERVER['REQUEST_URI'], '/' . $entry) !== false) {
                    self::$value = str_replace($_SERVER['SCRIPT_NAME'] . '?', '', $_SERVER['REQUEST_URI']);
                } elseif ($script != $_SERVER['REQUEST_URI']) {
                    self::$value = str_replace($script . '?', '', $_SERVER['REQUEST_URI']);
                }
            } else {
                self::$value = '';
            }

            self::$url = self::$value;
        }
    }

    /**
     * match
     *
     * @return mixed
     */
    public static function match()
    {
        self::input();

        $value = self::$value;

        if (Config::get('route')->cAll && $value) {
            if (Config::get('route')->$value) {
                self::$value = Config::get('route')->$value;
            } else {
                self::grep();
            }
        }
    }

    /**
     * grep
     *
     * @return mixed
     */
    private static function grep()
    {
        foreach (Config::get('route')->cAll as $k => $v) {
            $k = str_replace(':any', '.+', str_replace(':num', '[0-9]+', $k));

            if (preg_match('#^' . $k . '$#', self::$value)) {
                if (strpos($v, '$') !== false && strpos($k, '(') !== false) {
                    $v = preg_replace('#^' . $k . '$#', $v, self::$value);
                }

                self::$value = $v;

                self::$url = self::$value;
            }
        }
        self::input();
    }

    /**
     * input
     *
     * @return mixed
     */
    private static function input()
    {
        if (strpos(self::$value, '?') !== false) {
            $temp = explode('?', self::$value);
            self::$value = $temp[0];
            parse_str($temp[1], $input);
            Input::set('all', $input);
            self::setServer($input);
        }
    }

    /**
     * setServer
     *
     * @return mixed
     */
    private static function setServer($input)
    {
        if (Input::$command == true && isset($input['dever_server'])) {
            $_SERVER['DEVER_SERVER'] = $input['dever_server'];
        }
    }

    /**
     * file
     *
     * @return mixed
     */
    public static function file()
    {
        self::tain();

        if (strpos(self::$value, self::EXPLODE) !== false) {
            $array = explode(self::EXPLODE, self::$value);
            $file = self::parse($array);
        } else {
            $file = self::$value;
        }

        return $file;
    }

    /**
     * url
     *
     * @return mixed
     */
    public static function url()
    {
        return self::$url;
    }

    /**
     * tain
     *
     * @return mixed
     */
    private static function tain()
    {
        if (self::$method == 'GET' && !empty($_SERVER['REQUEST_URI'])) {
            $request_uri = strtoupper(urldecode($_SERVER['REQUEST_URI']));
            if (strpos($request_uri, '<') !== false || strpos($request_uri, '"') !== false || strpos($request_uri, 'CONTENT-TRANSFER-ENCODING') !== false) {
                Export::alert('request_tainting');
            }
            unset($request_uri);
        }
    }

    /**
     * parse
     *
     * @return mixed
     */
    private static function parse(&$array)
    {
        if (isset($array[2]) && empty($array[3])) {
            $file = $array[0] . self::EXPLODE . $array[1] . self::EXPLODE . $array[2];
        } elseif (isset($array[1])) {
            $file = $array[0] . self::EXPLODE . $array[1];
        } elseif (isset($array[0])) {
            $file = $array[0];
        }

        return $file;
    }
}
