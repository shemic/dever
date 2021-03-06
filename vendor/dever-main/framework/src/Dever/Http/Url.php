<?php namespace Dever\Http;

use Dever;
use Dever\Loader\Config;
use Dever\Loader\Project;
use Dever\Routing\Uri;
use Dever\Routing\Input;
use Dever\String\Helper;
use Dever\Loader\Import;

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
            return str_replace('{' . $replace . '}', $value, Config::$global['host']['domains']);
        }

        self::defaultValue($value);

        if (strpos($value, 'http:') !== false || strpos($value, 'https:') !== false) {
            return $value;
        }

        $host = '';
        self::host($host, $value, $project);

        if (strpos($value, Uri::LOAD . '=') === false) {
            $value = Uri::LOAD . '=' . $value;
        }
        
        if ($project) {
            $key = $project . $value;
        } else {
            $key = $value;
        }

        if (isset(self::$config['url']) && isset(self::$config['url'][$key])) {
            return self::$config['url'][$key];
        }

        self::route($value, $project);

        self::workspace($value);

        $url = $host . Uri::$type . $value;
        self::$config['url'][$key] = str_replace('??', '?', $url);

        return self::$config['url'][$key];
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
        if (strstr($file, '{"') || strstr($file, '<img')) {
            $key = '{uploadRes}';

            $file = preg_replace_callback('/'.$key.'(.*?)(\.jpg|\.png|\.gif)/i', function($matches) use ($key, $name)
                {
                    if (isset($matches[1]) && isset($matches[2])) {
                        $matches[1] = str_replace('\/', '/', $matches[1]);
                        $file = Url::upload($key . $matches[1] . $matches[2], $name);
                        return $file;
                    } else {
                        return $matches[0];
                    }
                }, $file);
            return $file;
        } elseif (strpos($file, ',') !== false) {
            $temp = explode(',', $file);
            $file = array();
            foreach ($temp as $k => $v) {
                $file[$k] = self::upload($v, $name);
            }
            return implode(',', $file);
        }

        $file = self::uploadRes($file);

        if ($name && strstr($file, 'http:') && strstr($file, Config::get('host')->uploadRes) && !strstr($file, $name)) {

            if (strstr($name, ',')) {
                $temp = explode(',', $name);
                foreach ($temp as $k => $v) {
                    $file = self::uploadHandle($file, $v);
                }
            } else {
                $file = self::uploadHandle($file, $name);
            }
        }

        $file = self::https($file);

        return $file;
    }

    private function uploadHandle($file, $name)
    {
        $source = $file;
        if (strstr($file, '<img')) {
            return self::html($file, $name);
        }

        if (strstr($file, '.gif')) {
            return $file;
        }

        if (strstr($name, 'wp') && strpos($file, '_wp') !== false) {
            $temp = explode('_wp', $file);
            $temp1 = explode('.', $temp[1]);
            $file = $temp[0] . '_wp' . $name . '.' . $temp1[1];
        } elseif (strstr($name, 't') && strpos($file, '_t') !== false) {
            $temp = explode('_t', $file);
            $temp1 = explode('.', $temp[1]);
            $file = $temp[0] . '_t' . $name . '.' . $temp1[1];
        } elseif (strstr($name, 'c') && strpos($file, '_c') !== false) {
            $temp = explode('_c', $file);
            $temp1 = explode('.', $temp[1]);
            $file = $temp[0] . '_c' . $name . '.' . $temp1[1];
        } elseif (strstr($name, 'p') && strpos($file, '_p') !== false) {
            $temp = explode('_p', $file);
            $temp1 = explode('.', $temp[1]);
            $file = $temp[0] . '_p' . $name . '.' . $temp1[1];
        } else {
            $ext = pathinfo($file, PATHINFO_EXTENSION);
            $file = str_replace('.' . $ext, '_' . $name . '.' . $ext, $file);
        }
        
        $file = Import::load('upload/view.get?file=' . $file);

        if ($file) {
            return $file;
        }
        return $source;
    }

    private static function html($file, $name)
    {
        preg_match_all('/src="(.*?)"/i', $file, $matches);
        if (isset($matches[1])) {
            foreach ($matches[1] as $v) {
                $t = self::upload($v, $name);
                $file = str_replace($v, $t, $file);
            }
        }

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
        if (Input::$command) {
            return $content;
        }
        if (Config::get('host')->uploadRes && $content && is_string($content)) {
            $host = Config::get('host')->uploadRes;
            if (is_array(Config::get('host')->uploadRes)) {
                $index = array_rand(Config::get('host')->uploadRes);
                $host = Config::get('host')->uploadRes[$index];
            }

            if (strpos($content, '{uploadRes}') !== false) {
                $content = str_replace('{uploadRes}', $host, $content);
            } else {
                $data = Config::data() . 'upload/';
                if (strpos($content, $data) !== false) {
                    $content = str_replace($data, $host, $content);
                }
            }
        }

        return $content;
    }

    /**
     * upload
     * @param string $file
     * @param string $name
     *
     * @return array
     */
    public static function local($content)
    {
        if ($content) {
            $path = Config::data() . 'upload/';
            if (!strstr(Config::get('host')->uploadRes, Config::get('host')->base) && strpos($content, '{uploadRes}') !== false) {
                $content = str_replace('{uploadRes}', Config::get('host')->uploadRes, $content);
            } elseif (strpos($content, '{uploadRes}') !== false) {
                $content = str_replace('{uploadRes}', $path, $content);
            } elseif (Config::get('host')->uploadRes && strpos($content, Config::get('host')->uploadRes) !== false) {
                $content = str_replace(Config::get('host')->uploadRes, $path, $content);
            }
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
                    $value = $value . '&' . $arg[1];
                } else {
                    $value = $result;
                }
            } else {
                $index = strpos($value, '?');
                if ($index === false) {
                    $value = preg_replace('/&/', '?', $value, 1);
                }
            }
        } elseif (strpos($value, '?')) {
            $value = str_replace('?', '&', $value);
        }
    }

    private static function callback($value, $str, $route)
    {
        $result = '';

        $value = str_replace(Uri::LOAD . '=', '', $value);

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
     * https 将url中的http替换为https
     * @param string $url
     *
     * @return mixed
     */
    public static function https($url)
    {
        if (Input::$command) {
            return $url;
        }
        if (DEVER_HOST_TYPE == 'https://' && strstr($url, 'http:')) {
            $url = str_replace('http:', 'https:', $url);

            $replace = Config::get('base')->replace;
            if ($replace) {
                foreach ($replace as $k => $v) {
                    if (strstr($url, $k)) {
                        $url = str_replace($k, $v, $url);
                    }
                }
            }
        }

        return $url;
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
            if (!isset($config['url'])) {
                $config['url'] = $config['path'];
            }
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
        if (strpos($value, 'http:') === false && strpos($value, 'https:') === false) {
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
