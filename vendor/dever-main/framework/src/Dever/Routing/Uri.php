<?php namespace Dever\Routing;

use Dever;
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
     * load
     *
     * @var string
     */
    const LOAD = 'l';

    /**
     * uri value
     *
     * @var string
     */
    public static $value;

    /**
     * pathinfo value
     *
     * @var string
     */
    public static $pathinfo;

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
     * key
     *
     * @var string
     */
    public static $key;

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
     * key
     *
     * @return mixed
     */
    public static function key()
    {
        if (!self::$key) {
            if (self::$method == 'GET') {
                $url = self::$url;
            } else {
                $url = Input::get('dever_uri');
            }

            if (defined('DEVER_SESSION')) {
                Dever::session_start();
                $key = 'dever_' . DEVER_PROJECT . '_passport';
                if (isset($_SESSION[$key])) {
                    $url .= '&u=' . $_SESSION[$key];
                } elseif (isset($_COOKIE[$key])) {
                    $url .= '&u=' . $_COOKIE[$key];
                }
            }
            
            $uri = DEVER_APP_NAME . '_' . self::get();

            $param = Config::get('cache')->routeNoParam;
            if ($param) {
                foreach ($param as $k => $v) {
                    if (strpos($url, $k)) {
                        foreach ($v as $k1 => $v1) {
                            if (strpos($uri, $v1)) {
                                $url = preg_replace('/[&|?]'.$k.'=([a-zA-Z0-9_%=]+)/', '', $url);
                                break;
                            }
                        }
                    }
                }
            }

            self::$key = 'route_' . $uri . '_' . sha1($url) . '_v2';
        }
        return self::$key;
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
        self::$method = Input::method();

        if (isset($_SERVER['DEVER_URITYPE']) && $_SERVER['DEVER_URITYPE']) {
            self::$type = $_SERVER['DEVER_URITYPE'];
        }
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

        if (strpos(self::$url, 'shell')) {
            self::$url = preg_replace('/[&|?]shell=(.*)/', '', self::$url);
        }
    }

    /**
     * info
     *
     * @return mixed
     */
    private static function info()
    {
        self::$pathinfo = -1;
        if (isset($_SERVER['PATH_INFO'])) {
            self::$pathinfo = $_SERVER['PATH_INFO'];
        } elseif (isset($_SERVER['ORIG_PATH_INFO'])) {
            self::$pathinfo = $_SERVER['ORIG_PATH_INFO'];
        }
        if (self::$pathinfo != -1) {
            if (strstr($_SERVER['REQUEST_URI'], 'index.php/')) {
                self::$type = 'index.php/';
            } else {
                self::$type = '';
            }

            self::$value = trim(self::$pathinfo, self::EXPLODE);

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
        $request = $_REQUEST;
        if (isset($request['l'])) {
            self::$value = $request['l'];
            self::$url = $_SERVER['QUERY_STRING'];
        } else {
            self::request_uri();
        }
    }

    /**
     * request_uri
     *
     * @return mixed
     */
    private static function request_uri()
    {
        if (self::$pathinfo == -1 && isset($_SERVER['REQUEST_URI']) && $_SERVER['REQUEST_URI'] && $_SERVER['REQUEST_URI'] != '/') {
            $entry = defined('DEVER_ENTRY') ? DEVER_ENTRY : 'index.php';
            self::$type = $entry == 'index.php' ? '?' : $entry . '?';
            if ($_SERVER['REQUEST_URI'] != $_SERVER['SCRIPT_NAME']) {
                $script = substr($_SERVER['SCRIPT_NAME'], 0, strpos($_SERVER['SCRIPT_NAME'], $entry));
                $uri = $_SERVER['REQUEST_URI'];
                if (strpos($_SERVER['QUERY_STRING'], '?') !== false) {
                    $uri = $_SERVER['PHP_SELF'] . '?' . $_SERVER['QUERY_STRING'];
                }

                if (strpos($uri, '/' . $entry) !== false) {
                    self::$value = str_replace($_SERVER['SCRIPT_NAME'] . '?', '', $uri);
                } elseif ($script != $uri) {
                    self::$value = str_replace($script, '', $uri);
                    self::$value = ltrim(self::$value, '?');
                }

                if (strpos(self::$value, '?') === false) {
                    self::$value = preg_replace('/&/', '?', self::$value, 1);
                }
                self::$value = ltrim(self::$value, '&');
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

            self::$value = str_replace(self::LOAD . '=', '', self::$value);
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

                self::$value = self::LOAD . '=' . $v;

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
        } elseif (strpos(self::$value, '&') !== false) {
            parse_str(self::$value, $input);
            if (isset($input[self::LOAD])) {
                self::$value = $input[self::LOAD];
            } else {
                $temp = explode('&', self::$value);
                self::$value = $temp[0];
            }
        }

        if (isset($input)) {
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
