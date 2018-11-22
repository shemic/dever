<?php

class Dever
{
    private static $register = array
    (
        'page' => array('Pagination\\Paginator', 'getPage'),
        'total' => array('Pagination\\Paginator', 'getTotal'),
        'html' => array('Pagination\\Paginator', 'getHtml'),

        'url' => array('Http\\Url', 'get'),
        'location' => array('Http\\Url', 'location'),
        'upload' => array('Http\\Url', 'upload'),
        'pic' => array('Http\\Url', 'upload'),
        'uploadRes' => array('Http\\Url', 'uploadRes'),
        'https' => array('Http\\Url', 'https'),
        'local' => array('Http\\Url', 'local'),
        'link' => array('Http\\Url', 'link'),
        'curl' => array('Http\\Curl', 'get'),

        'input' => array('Routing\\Input', 'get'),
        'preInput' => array('Routing\\Input', 'prefix'),
        'setInput' => array('Routing\\Input', 'set'),
        'shell' => array('Routing\\Input', 'shell'),

        'alert' => array('Output\\Export', 'alert'),
        'out' => array('Output\\Export', 'out'),
        'outDiy' => array('Output\\Export', 'diy'),
        'pageInfo' => array('Output\\Export', 'page'),
        'debug' => array('Output\\Debug', 'wait'),
        'sql' => array('Output\\Debug', 'sql'),
        'error' => array('Output\\Debug', 'error'),

        'load' => array('Loader\\Import', 'load'),
        'import' => array('Loader\\Project', 'import'),
        'apply' => array('Loader\\Import', 'apply'),
        'lang' => array('Loader\\Lang', 'get'),
        'db' => array('Data\\Model', 'load'),
        'upinto' => array('Data\\Model', 'upinto'),

        'setStep' => array('Routing\\Step', 'set'),
        'getStep' => array('Routing\\Step', 'get'),

        'config' => array('Loader\\Config', 'get'),
        'data' => array('Loader\\Config', 'data'),
        'project' => array('Loader\\Project', 'load'),

        'view' => array('Template\\View', 'get'),
        'render' => array('Template\\View', 'html'),

        'lace' => array('Template\\Common', 'lace'),
        'first' => array('Template\\Common', 'first'),
        'last' => array('Template\\Common', 'last'),
        'other' => array('Template\\Common', 'other'),
        'sort' => array('Template\\Common', 'sort'),
        'a' => array('Template\\Common', 'a'),
        'img' => array('Template\\Common', 'img'),
        'assets' => array('Template\\Common', 'assets'),
        'table' => array('Template\\Common', 'table'),
        'tbody' => array('Template\\Common', 'tbody'),
        'dom' => array('Template\\Common', 'dom'),

        'token' => array('Http\\Api', 'get'),
        'login' => array('Http\\Api', 'login'),
        'loginResult' => array('Http\\Api', 'loginResult'),
        'nonce' => array('Http\\Api', 'nonce'),

        'rule' => array('String\\Regular', 'rule'),
        'id' => array('String\\Helper', 'id'),
        'idtostr' => array('String\\Helper', 'idtostr'),
        'strtoid' => array('String\\Helper', 'strtoid'),
        'uuid' => array('String\\Helper', 'uuid'),
        'order' => array('String\\Helper', 'order'),
        'hide' => array('String\\Helper', 'hide'),
        'cut' => array('String\\Helper', 'cut'),
        'code' => array('String\\Helper', 'code'),
        'uid' => array('String\\Helper', 'uid'),
        'rand' => array('String\\Helper', 'rand'),
        'filter' => array('String\\Helper', 'filter'),

        'qqvideo' => array('String\\Helper', 'qqvideo'),
        'ishtml' => array('String\\Helper', 'ishtml'),
        'replace' => array('String\\Helper', 'replace'),
        'strlen' => array('String\\Helper', 'strlen'),
        'addstr' => array('String\\Helper', 'addstr'),
        'encode' => array('String\\Encrypt', 'encode'),
        'decode' => array('String\\Encrypt', 'decode'),

        'cache' => array('Cache\\Handle', 'load'),
        'clearHeaderCache' => array('Routing\\Route', 'clearHeaderCache'),

        'tcp' => array('Server\\Swoole', 'getInstance'),

        'path' => array('Support\\Path', 'get'),
        'pathDay' => array('Support\\Path', 'day'),
        'mobile' => array('Support\\Env', 'mobile'),
        'weixin' => array('Support\\Env', 'weixin'),
        'zero' => array('Support\\Env', 'zero'),
        'header' => array('Support\\Env', 'header'),
        'ua' => array('Support\\Env', 'ua'),
        'ip' => array('Support\\Env', 'ip'),
        'os' => array('Support\\Env', 'os'),
        'browser' => array('Support\\Env', 'browser'),
        
        'daemon' => array('Support\\Command', 'daemon'),
        'cron' => array('Support\\Command', 'cron'),
        'run' => array('Support\\Command', 'run'),
        'kill' => array('Support\\Command', 'kill'),

        'excelExport' => array('Support\\Excel', 'export'),
        'excelImport' => array('Support\\Excel', 'import'),

        'log' => array('Log\\Oper', 'add'),
    );

    private static $define = array();

    public static $markdown;

    public static $save;

    public static $global;

    /**
     * register
     * @param string $name
     * @param array $param
     *
     * @return mixed
     */
    public static function __callStatic($name, $param = array())
    {
        if (isset(self::$register[$name])) {
            $class = 'Dever\\' . self::$register[$name][0];
            $method = self::$register[$name][1];
            return call_user_func_array(array($class, $method), $param);
        } elseif (isset(self::$define[$name])) {
            if (is_array(self::$define[$name])) {
                $class = self::$define[$name][0];
                $method = self::$define[$name][1];
            } else {
                 $class = self::$define[$name];
                 $method = $name;
            }
            
            return call_user_func_array(array($class, $method), $param);
        }
    }

    /**
     * 注册方法
     * @param string $method
     * @param array $function
     *
     * @return array
     */
    public static function reg($method, $function)
    {
        self::$define[$method] = $function;
    }

    /**
     * 判断是否包含
     * @param string $value
     * @param string $source
     *
     * @return array
     */
    public static function in($value, $index)
    {
        $key = $value . '_' . $index;
        if (isset(self::$global['in'][$key])) {
            return self::$global['in'][$key];
        }
        if (is_array($index)) {
            $index = implode(',', $index);
        }
        $index .= ',';
        if (strpos($value, ',')) {
            $temp = explode(',', $value);
            foreach ($temp as $k => $v) {
                $state = self::in($v, $index);
                if ($state == true) {
                    break;
                }
            }

            return self::$global['in'][$key] = $state;
        } else {
            if (strpos($index, $value . ',') !== false) {
                return self::$global['in'][$key] = true;
            }
            return self::$global['in'][$key] = false;
        }
    }

    /**
     * 创建临时文件
     * @param string $name
     * @param string $content
     *
     * @return array
     */
    public static function tmp($name, $content)
    {
        $name = tempnam(Dever::path(Dever::data() . 'tmp'), $name);
        file_put_contents($name, $content);
        ob_start();
        require $name;
        $content = ob_get_contents();
        ob_end_clean();
        unlink($name);
        return $content;
    }

    /**
     * 过滤emoji
     * @param string $str
     *
     * @return array
     */
    public static function emoji($str)
    {
        if (function_exists('mb_convert_encoding')) {
            //$str = mb_convert_encoding($str, 'UTF-8');
        }
       
        $str = preg_replace_callback(
            '/./u',
            function (array $match) {
                return strlen($match[0]) >= 4 ? '' : $match[0];
            },
            $str
        );

        return $str;
    }

    /**
     * 获取_param
     * @param string $name
     * @param string $param
     *
     * @return array
     */
    public static function param($name, $param)
    {
        if (isset($param['add_' . $name])) {
            return $param['add_' . $name];
        } elseif (isset($param['set_' . $name])) {
            return $param['set_' . $name];
        } elseif(isset($param[$name])) {
            return $param[$name];
        }

        return Dever::input($name, false);
    }

    public static function mdate($num, $type = 1)
    {
        $num = time() - $num;

        if ($num <= 0) {
            if ($type == 2) {
                return '1秒前';
            } else {
                return '1S';
            }
        }

        $config = array
            (
            array(31536000, 'Y', '年'),
            array(2592000, 'T', '个月'),
            array(604800, 'W', '星期'),
            array(86400, 'D', '天'),
            array(3600, 'H', '小时'),
            array(60, 'M', '分钟'),
            array(1, 'S', '秒'),
        );

        if ($type == 2) {
            foreach ($config as $k => $v) {
                $value = intval($num / $v[0]);

                if ($value != 0) {
                    return $value . $v[2] . '前';
                }
            }

            return '';
        } else {
            $result = '';

            foreach ($config as $k => $v) {
                if ($num > $v[0]) {
                    $value = intval($num / $v[0]);
                    $num = $num - $v[0] * $value;
                    $result .= $value . $v[1] . ' ';
                }
            }

            return $result;
        }
    }

    public static function maketime($v)
    {
        if (!$v) {
            return '';
        }

        if (is_numeric($v)) {
            return $v;
        }

        if (is_array($v)) {
            $v = $v[1];
        }

        if (strstr($v, ' ')) {
            $t = explode(' ', $v);
            $v = $t[0];
            $s = explode(':', $t[1]);
        } else {
            $s = array(0, 0, 0);
        }

        if (!isset($s[1])) {
            $s[1] = 0;
        }

        if (!isset($s[2])) {
            $s[2] = 0;
        }

        if (strstr($v, '-')) {
            $t = explode('-', $v);
        } elseif (strstr($v, '/')) {
            $u = explode('/', $v);
            $t[0] = $u[2];
            $t[1] = $u[0];
            $t[2] = $u[1];
        }

        if (!isset($t)) {
            $t = array(0, 0, 0);
        }

        if (!isset($t[1])) {
            $t[1] = 0;
        }

        if (!isset($t[2])) {
            $t[2] = 0;
        }

        $v = mktime($s[0], $s[1], $s[2], $t[1], $t[2], $t[0]);

        return $v;
    }

    public static function udate($format = 'u', $utimestamp = null)
    {
        if (is_null($utimestamp)) {
            $utimestamp = microtime(true);
        }
        $timestamp = floor($utimestamp);
        $milliseconds = round(($utimestamp - $timestamp) * 1000000);
        return date(preg_replace('`(?<!\\\\)u`', $milliseconds, $format), $timestamp);
    }

    public static function proxy($method = false, $param = false)
    {
        if ($method) {
            if (Dever::config('host')->proxy) {
                $method = urlencode($method);
                return Dever::url(Dever::config('host')->proxy . 'proxy_method=' . $method . '&' . $param);
            }
            return self::url($method . '?' . $param);
        }
        $data = self::input();

        foreach ($data as $k => $v) {
            self::setInput($k, $v);
        }

        return self::load(urldecode($data['proxy_method']), array('CHECK' => true));
    }

    public static function script()
    {
        $script = 'var config={};config.init=false;config.host="' . self::url('') . '";config.type="' . Dever\Routing\Uri::$type . '";config.current="' . self::url() . '";config.upload="' . self::config('host')->upload . '";config.assets="' . self::config('host')->assets . '";config.script="' . self::config('host')->script . '";';

        if (self::config('host')->css) {
            $script .= 'config.css="' . self::config('host')->css . '";';
        }
        if (self::config('host')->js) {
            $script .= 'config.js="' . self::config('host')->js . '";';
        }
        if (self::config('host')->images) {
            $script .= 'config.images="' . self::config('host')->images . '";';
        }
        if (self::config('host')->proxy) {
            $script .= 'config.proxy="' . self::config('host')->proxy . '";';
        }
        if (self::config('host')->manage) {
            $script .= 'config.manage_assets="' . self::config('host')->manage . '";';
        }
        if (self::config('host')->base) {
            $script .= 'config.workspace="' . self::config('host')->base . '";';
        }
        if (self::config('template')->layout) {
            $script .= 'config.layout = "' . self::config('template')->layout . '";$(document).ready(function()
                        {
                            $(document).pjax("a", "' . self::config('template')->layout . '", {"timeout":8000});

                            $(document).on("submit", "#form1", function (event) {event.preventDefault();$.pjax.submit(event, "' . self::config('template')->layout . '", {"push": true, "replace": false, timeout:8000, "scrollTo": 0, maxCacheLength: 0});});

                            $(document).on("pjax:start", function()
                            {
                                NProgress.start();
                            });
                            $(document).on("pjax:end", function()
                            {
                                NProgress.done();
                            });
                        });';
        }
        return $script;
    }

    /**
     * 加载分享前端组件
     * @param string $key
     *
     * @return string
     */
    public static function share($project, $uid, $share_url, $title, $img, $desc, $link = false, $button = false)
    {
        if (!$uid) {
            $uid = -1;
        }

        $host = self::config('host')->script;
        $html = '<script src="'.$host.'lib/share/weixin.js" ></script><script src="'.$host.'dever/share.js" ></script>';

        $html .= '<script type="text/javascript">';
        $html .= '$(function()';
        $html .= '{';
        $html .= 'var uid = ' . $uid . ';';
        $html .= 'var project = ' . $project . ';';
        $html .= 'var url = "' . $share_url . '?api.";';
        $html .= 'var param = {};';
        $html .= 'param.title = "' . $title . '";';
        $html .= 'param.img = "' . $img . '";';
        $html .= 'param.desc = "' . $desc . '";';

        if ($link) {
            $html .= 'param.url = "' . $link . '";';
        } else {
            $html .= 'param.url = location.href;';
        }

        if ($button) {
            $html .= 'var button = "' . $link . '";';
        } else {
            $html .= 'var button = false;';
        }

        $html .= 'Dever_Share.Init(uid, project, url, param, button);';
        $html .= '})';
        $html .= '</script>';
        return $html;
    }

    /**
     * queue队列，需要队列组件 dever package queue
     * @param string $key
     *
     * @return string
     */
    public static function queue($key = 'default', $value = false)
    {
        self::import('queue');
        if ($value) {
            if ($value == 'len') {
                $result = self::len($key);
            } else {
                $result = self::push($value, $key);
            }
        } else {
            $result = self::pop($key);
        }
        
        return $result;
    }

    /**
     * 通过Redis队列来实现的锁操作
     * @param string $key
     *
     * @return string
     */
    public static function lock($key = 'default', $total = false)
    {
        self::import('queue');
        if ($total && $total > 0) {
            # 写入
            $len = self::len($key);
            if ($len < $total) {
                $count = $total - $len;
                for($i=1; $i <= $count; $i++) {
                    self::push($i, $key);
                }
            } else {
                return false;
            }
        } else {
            $state = self::pop($key);
            if (!$state) {
                return false;
            }
        }
        
        return true;
    }

    /**
     * json_encode编码
     * @param string $value
     *
     * @return string
     */
    public static function json_encode($value)
    {
        //$value = json_encode($value, JSON_UNESCAPED_UNICODE + JSON_FORCE_OBJECT);
        $value = json_encode($value, JSON_UNESCAPED_UNICODE);
        if (strpos($value, '<null>')) {
            $value = str_replace('<null>', '', $value);
        }
        $webp = self::input('webp', -1);
        if ($webp > 0) {
            $value = self::upload($value, 'wp' . $webp);
        } else {
            $value = self::uploadRes($value);
            $value = self::https($value);
        }

        return $value;
    }

    /**
     * json_decode解码
     * @param string $value
     *
     * @return string
     */
    public static function json_decode($value)
    {
        return json_decode($value, true);
    }

    /**
     * 数据库数组编码
     * @param string $value
     *
     * @return string
     */
    public static function array_encode($value)
    {
        return base64_encode(self::json_encode($value));
    }

    /**
     * 数据库数组解码
     * @param string $value
     *
     * @return string
     */
    public static function array_decode($value)
    {
        return self::json_decode(base64_decode($value, true));
    }

    /**
     * 获取文件后缀
     * @param string $file
     *
     * @return string
     */
    public static function ext($file)
    {
        return pathinfo($file, PATHINFO_EXTENSION);
    }

    /**
     * markdown解析
     * @param string $value
     *
     * @return string
     */
    public static function markdown($value)
    {
        self::$markdown = self::$markdown ? self::$markdown : new \Parsedown();

        # 一个回车就换行
        self::$markdown->setBreaksEnabled(true);

        # 过滤掉所有特殊代码
        return self::$markdown->text($value);
    }

    /**
     * setup 安装引导程序
     * @param string $value
     *
     * @return string
     */
    public static function setup()
    {
        $host = DEVER_APP_HOST;

        $path = DEVER_APP_PATH . '../';

        $dir = scandir($path);

        foreach ($dir as $k => $v) {
            if ($v != '.' && $v != '..' && is_dir($path . $v)) {
                $url = str_replace(array('www', 'main'), $v, $host);
                if (is_file($path . $v . '/index.php')) {
                    Dever::curl($url);

                    echo $url . ' 安装成功！<br />' . "\r\n";
                }
            }
        }

        return '感谢您使用Dever！';
    }

    /**
     * dyna 动态解析功能
     * @param string $key
     * @param array $data
     *
     * @return mixed
     */
    public static function odyna($key = false, $data = array(), $type = '')
    {
        return (object) self::dyna($key, $data, $type);
    }

    /**
     * dyna 动态解析功能
     * @param string $key
     * @param array $data
     * @param string $type
     *
     * @return mixed
     */
    public static function dyna($key = false, $data = array(), $type = '')
    {
        if (isset(self::$global['dyna'][$key])) {
            return self::$global['dyna'][$key];
        }
        $config = self::config('dyna')->cAll;

        if ($type && isset($config[$type])) {
            $config = $config[$type];
        }

        if (empty($config[$key])) {
            return $key;
        }
        if (!$data) {
            return $config[$key];
        }

        if (is_array($config[$key])) {
            foreach ($config[$key] as $k => $v) {
                $v = '$value = "' . $v . '";';
                eval($v);
                if (isset($value)) {
                    $config[$key][$k] = $value;
                }
            }
        } else {
            $v = '$value = "' . $config[$key] . '";';
            eval($v);
            if (isset($value)) {
                $config[$key] = $value;
            }
        }

        self::$global['dyna'][$key] = $config[$key];

        return $config[$key];
    }

    /**
     * session 使用session，默认为cookie
     * @param string $key
     * @param string $value
     * @param string $type
     *
     * @return mixed
     */
    public static function session($key, $value = false, $timeout = 3600, $type = 'cookie')
    {
        if (empty(self::$save)) {
            self::$save = new \Dever\Session\Oper(DEVER_APP_NAME, $type);
        }

        if ($value) {
            return self::$save->add($key, $value, $timeout);
        } else {
            return self::$save->get($key);
        }
    }

    /**
     * submit 处理重复提交功能 此处暂时有问题
     * @param string $type
     * @param int $value
     *
     * @return mixed
     */
    public static function submit($type = 'update', $value = 1)
    {
        if ($type == 'check') {
            $submit = self::session('submit');

            if ($submit == 2) {
                Dever::abert('请不要重复提交');
            }
        } else {
            self::session('submit', $value);
        }
    }

    /**
     * defaultValue 过滤默认值
     * @param string $value
     *
     * @return mixed
     */
    public static function defaultValue($v)
    {
        if (!$v) {
            return $v;
        }
        $v = str_replace('-1', '', $v);
        $v = str_replace(',,,', ',', $v);
        $v = str_replace(',,', ',', $v);
        if ($v == ',') {
            $v = '';
        } elseif (!strstr($v, ',')) {
            $v .= ',';
        }
        return $v;
    }
}
