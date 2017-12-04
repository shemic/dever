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
        'db' => array('Data\\Model', 'load'),

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

        'rule' => array('String\\Helper', 'rule'),
        'idtostr' => array('String\\Helper', 'idtostr'),
        'strtoid' => array('String\\Helper', 'strtoid'),
        'cut' => array('String\\Helper', 'cut'),
        'code' => array('String\\Helper', 'code'),
        'rand' => array('String\\Helper', 'rand'),

        'cache' => array('Cache\\Handle', 'load'),
        'clearHeaderCache' => array('Routing\\Route', 'clearHeaderCache'),

        'tcp' => array('Server\\Swoole', 'getInstance'),

        'path' => array('Support\\Path', 'get'),
        'mobile' => array('Support\\Env', 'mobile'),
        'weixin' => array('Support\\Env', 'weixin'),
        'ua' => array('Support\\Env', 'ua'),
        'ip' => array('Support\\Env', 'ip'),
        
        'daemon' => array('Support\\Command', 'daemon'),
        'cron' => array('Support\\Command', 'cron'),
        'run' => array('Support\\Command', 'run'),
        'kill' => array('Support\\Command', 'kill'),

        'csv' => array('Support\\Csv', 'export'),
        'csvRead' => array('Support\\Csv', 'read'),
    );

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
        }
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
     * 获取_param
     * @param string $value
     * @param string $index
     *
     * @return array
     */
    public static function param($name)
    {
        if (isset(Dever::$global['base']['_param'])) {
            if (isset(Dever::$global['base']['_param']['add_' . $name])) {
                return Dever::$global['base']['_param']['add_' . $name];
            }

            if (isset(Dever::$global['base']['_param']['set_' . $name])) {
                return Dever::$global['base']['_param']['set_' . $name];
            }
        }

        return false;
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

        $v = mktime($s[0], $s[1], $s[2], $t[1], $t[2], $t[0]);

        return $v;
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
        $script = 'var config={};config.init=false;config.host="' . self::url('') . '";config.type="' . Dever\Routing\Uri::$type . '";config.current="' . self::url() . '";config.upload="' . self::config('host')->upload . '";config.assets="' . self::config('host')->assets . '";';

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

                Dever::curl($url);

                echo $url . ' 安装成功！<br />' . "\r\n";
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
     * submit 处理重复提交功能 此处暂时有问题
     * @param string $type
     * @param int $value
     *
     * @return mixed
     */
    public static function submit($type = 'update', $value = 1)
    {
        if (empty(self::$save)) {
            self::$save = new \Dever\Session\Oper(DEVER_APP_NAME, 'cookie');
        }

        if ($type == 'check') {
            $submit = self::$save->get('submit');

            if ($submit == 2) {
                Dever::abert('请不要重复提交');
            }
        }

        if ($value) {
            self::$save->add('submit', $value);
        }
    }
}
