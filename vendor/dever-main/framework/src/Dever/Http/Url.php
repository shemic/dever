<?php namespace Dever\Http;

use Dever\Loader\Config;
use Dever\Loader\Project;
use Dever\Routing\Uri;
use Dever\Routing\Input;
use Dever\String\Helper;

class Url
{
    /**
     * config
     *
     * @var array
     */
    public static $config;

    /**
     * get link
     * @param string $value
     *
     * @return array
     */
    public static function get($value = false, $project = false, $replace = false)
    {
        if ($replace) {
            return str_replace('{' . $replace . '}', $value, Config::$global['host']['domain']);
        }

        self::defaultValue($value);

        if (strpos($value, 'http://') !== false) {
            return $value;
        }

        $host = '';
        self::host($host, $value, $project);

        $key = $value;

        if (isset(self::$config['url']) && isset(self::$config['url'][$key])) {
            return self::$config['url'][$key];
        }

        self::route($value, $project);

        self::workspace($value);

        return self::$config['url'][$key] = $host . Uri::$type . $value;
    }

    /**
     * get Link
     * @param string $url
     * @param array $param
     *
     * @return string
     */
    public static function link($url, $param = array(), $project = 'main')
    {
        if ($param) {
            $send = array();
            foreach ($param as $k => $v) {
                if (is_string($k) && $v) {
                    $send[] = $k . '=' . $v;
                } elseif ($v && $value = Input::get($v)) {
                    $send[] = $v . '=' . $value;
                }
            }
            if ($send) {
                $url .= '?' . implode('&', $send);
            }
        }
        return self::url($url, $project);
    }

    /**
     * upload
     * @param string $file
     * @param string $name
     *
     * @return array
     */
    public static function upload($file, $name = false)
    {
        if (strpos($file, ',') !== false) {
            $temp = explode(',', $file);
            $file = array();
            foreach ($temp as $k => $v) {
                $file[$k] = self::upload($v, $name);
            }
            return implode(',', $file);
        }

        if ($name && strpos($file, '_') !== false) {
            $temp = explode('_', $file);
            $file = $temp[0] . '_' . $name;
        }

        $file = self::uploadRes($file);

        return $file;
    }

    /**
     * upload
     * @param string $file
     * @param string $name
     *
     * @return array
     */
    public static function uploadRes($content)
    {
        if (Config::get('host')->uploadRes && $content && strpos($content, '{uploadRes}') !== false) {
            $host = Config::get('host')->uploadRes;
            if (is_array(Config::get('host')->uploadRes)) {
                $index = array_rand(Config::get('host')->uploadRes);
                $host = Config::get('host')->uploadRes[$index];
            }
            $content = str_replace('{uploadRes}', $host, $content);
        }

        return $content;
    }

    /**
     * workspace
     * @param string $value
     *
     * @return mixed
     */
    private static function workspace(&$value)
    {
        if ($value && Config::get('host')->workspace && strpos($value, Config::get('host')->workspace) !== false) {
            $path = str_replace(Config::get('host')->workspace, '', $value);
            if ($path && strpos($value, $path) !== false) {
                $value = str_replace($path, '', $value);
            }
        }
    }

    /**
     * defaultValue
     * @param string $value
     *
     * @return mixed
     */
    private static function defaultValue(&$value)
    {
        if ($value === false) {
            $value = Uri::$url ? Uri::$url : '';
        }

        if (Config::get('base')->url) {
            if (strpos($value, '?') === false) {
                $value .= '?' . Config::get('base')->url;
            } else {
                $value .= '&' . Config::get('base')->url;
            }
        }
    }

    private static function getArg(&$value)
    {
        $arg = array();

        if (strpos($value, '?') !== false) {
            $arg = explode('?', $value);
            $value = $arg[0];
        }

        return $arg;
    }

    private static function searchRoute(&$value, $route)
    {
        if ($uri = array_search($value, $route)) {
            $value = $uri;
        }
    }

    /**
     * route
     * @param string $value
     *
     * @return mixed
     */
    private static function route(&$value, $project)
    {
        $route = Config::get('route', $project)->cAll;
        if ($route) {

            $arg = self::getArg($value);

            self::searchRoute($value, $route);

            if (isset($arg[1]) && $arg[1]) {

                $out = self::initArg($arg[1], $arg[0]);

                list($str, $pre) = self::createLink($out);

                $result = self::callback($value, $str, $route);

                if (!$result || $result == $value) {
                    $value = $value . '?' . $arg[1];
                } else {
                    $value = $result;
                }
            } else {
                $index = strpos($value, '?');
                if ($index === false) {
                    $value = preg_replace('/&/', '?', $value, 1);
                }
            }
        }
    }

    private static function callback($value, $str, $route)
    {
        $result = '';

        if ($key = array_search($value . '?' . $str, $route)) {
            $result = preg_replace_callback('/\(.*?\)/', 'self::getLink', $key);
        }

        return $result;
    }

    /**
     * initArg
     * @param string $parse
     * @param string $source
     *
     * @return mixed
     */
    private static function initArg($parse, $source)
    {
        parse_str($parse, $out);
        self::$config['link_key'] = self::$config['link_value'] = array();
        self::$config['link_source'] = $source;

        return $out;
    }

    /**
     * createLink
     * @param array $data
     *
     * @return mixed
     */
    private static function createLink($data)
    {
        $str = $pre = '';
        $i = 1;
        foreach ($data as $k => $v) {
            if ($i > 1) {
                $pre = '&';
            }
            $str .= $pre . $k . '=$' . $i;
            $i++;

            self::$config['link_key'][] = $k;
            self::$config['link_value'][] = $v;
        }

        self::$config['link_index'] = 0;

        return array($str, $pre);
    }

    /**
     * route
     * @param string $value
     *
     * @return mixed
     */
    private static function host(&$host, &$value, &$project)
    {
        if ($project) {
            self::setHost($host, $project);
        } elseif (strpos($value, '/') !== false) {
            self::set($host, $value, $project);
        }

        if (!$host) {
            $host = Config::get('host')->base;
        }
    }

    /**
     * route
     * @param string $value
     *
     * @return mixed
     */
    private static function set(&$host, &$value, &$project)
    {
        $temp = explode('/', $value);
        $config = Project::load($temp[0]);
        if ($config) {
            $project = $temp[0];
            unset($temp[0]);
            $value = implode('/', $temp);
            $host = $config['url'];
        }
    }

    /**
     * setHost
     * @param string $host
     * @param string $project
     *
     * @return mixed
     */
    private static function setHost(&$host, $project)
    {
        $config = Project::load($project);
        if ($config) {
            $host = $config['url'];
        }
    }

    /**
     * link
     * @param string $param
     *
     * @return mixed
     */
    private static function getLink($param)
    {
        if (isset($param[0]) && $param[0] && isset(self::$config['link_value']) && isset(self::$config['link_value'][self::$config['link_index']])) {

            self::encode();

            $param[0] = self::$config['link_value'][self::$config['link_index']];
        }
        self::$config['link_index']++;

        return $param[0];
    }

    /**
     * encode
     *
     * @return mixed
     */
    private static function encode()
    {
        if (Config::get('base')->urlEncode) {
            $config = Config::get('base')->urlEncode;
            if ($config) {

                $config = self::filter($config);

                self::replace($config);
            }
        }
    }

    /**
     * replace
     *
     * @return mixed
     */
    private static function replace($config)
    {
        if ($config && is_numeric(self::$config['link_value'][self::$config['link_index']]) && in_array(self::$config['link_key'][self::$config['link_index']], $config)) {
            $method = Config::get('base')->urlEncodeMethod[0];
            
            if (strpos($method, 'Dever::') !== false) {
                $method = str_replace('Dever::', '', $method);
                self::$config['link_value'][self::$config['link_index']] = \Dever\String\Helper::$method(self::$config['link_value'][self::$config['link_index']]);
            } else {
                self::$config['link_value'][self::$config['link_index']] = $method(self::$config['link_value'][self::$config['link_index']]);
            }
        }
    }

    /**
     * filter
     *
     * @return mixed
     */
    private static function filter($config)
    {
        $filter = Config::get('base')->urlEncodeFilter;
        if ($filter && self::$config['link_source']) {
            foreach ($filter as $k => $v) {
                if (strpos(self::$config['link_source'], $v) !== false) {
                    $config = false;
                    break;
                }
            }
        }

        return $config;
    }

    /**
     * location
     * @param string $value
     *
     * @return mixed
     */
    public static function location($value, $type = 1)
    {
        if (strpos($value, 'http://') === false && strpos($value, 'https://') === false) {
            $value = self::get($value);
        }
        switch ($type) {
            case 2:
                $html = '<script>location.href="' . $value . '"</script>';
                Output::abert($html);
                die;
                break;

            default:
                header('HTTP/1.1 301 Moved Permanently');
                header('Location: ' . $value);
                die;
                break;
        }
    }
}
