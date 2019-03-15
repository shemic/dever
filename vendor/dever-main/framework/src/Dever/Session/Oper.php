<?php namespace Dever\Session;

use Dever;
use Dever\Loader\Config;
use Dever\String\Encrypt;

# 一系列的session、cookie等基本的操作，前期随意写的。后续将分开。

class Oper
{
    /**
     * key
     *
     * @var string
     */
    private $key = '';

    /**
     * prefix
     *
     * @var string
     */
    private $prefix = 'dever_';

    /**
     * project
     *
     * @var string
     */
    private $project = '';

    /**
     * method
     *
     * @var string
     */
    private $method = 'session';

    /**
     * __construct
     * @param string $key
     * @param string $method
     *
     * @return mixed
     */
    public function __construct($key = false, $method = 'session')
    {
        if (defined('DEVER_DAEMON')) {
            $method = 'cli';
        }
        if ($method != 'cli') {
            @header('P3P: CP="CURa ADMa DEVa PSAo PSDo OUR BUS UNI PUR INT DEM STA PRE COM NAV OTC NOI DSP COR"');
            //
            # 解决post页面返回上一页问题 这个有问题
            /*
            if (Config::get('template')->session_cache) {
                @session_cache_limiter('private');
            }
            */
            $server = Config::get('database')->session;
            if ($server) {
                $link = 'tcp://'.$server['host'].':'.$server['port'];
                if (isset($server['password']) && $server['password']) {
                    $link .= '?auth='.$server['password'];
                }
                @ini_set('session.save_handler', $server['type']);
                @ini_set('session.save_path', $link);
            }
            @session_start();

            # 解决post页面返回上一页问题
            if (Config::get('template')->session_cache) {
                //@header("Cache-control: private");
            }
            
            if (Config::get('host')->cookie) {
                ini_set('session.cookie_domain', Config::get('host')->cookie);
            }
        }

        $this->key = $key ? $key : $this->key;

        $this->method = $method ? $method : $this->method;

        $this->method = ucwords($this->method);

        $this->project = defined('DEVER_PROJECT') ? DEVER_PROJECT : 'default';

        $this->key($this->key);

        return $this;
    }

    /**
     * add
     * @param string $key
     * @param mixed $value
     *
     * @return mixed
     */
    public function add($key, $value, $time = 3600)
    {
        if (is_array($key)) {
            $key = md5(serialize($key));
        }

        if ($this->project != $key) {
            $key = $this->project . '_' . $key;
        }

        $value = Encrypt::encode(base64_encode(serialize($value)), $this->key);

        $method = '_set' . $this->method;

        $this->$method($key, $value, $time);

        return $value;
    }

    /**
     * get
     * @param string $key
     * @param mixed $type
     *
     * @return mixed
     */
    public function get($key, $type = false)
    {
        if (is_array($key)) {
            $key = md5(serialize($key));
        }
        $key = $this->project . '_' . $key;

        $method = '_get' . $this->method;

        $value = $this->$method($key);

        $type == false && $value = Encrypt::decode($value, $this->key);

        $value = unserialize(base64_decode($value));

        return $value;
    }

    /**
     * un
     * @param string $key
     *
     * @return mixed
     */
    public function un($key)
    {
        $key = $this->project . '_' . $key;

        $method = '_unset' . $this->method;

        return $this->$method($key);
    }

    /**
     * key
     * @param string $key
     *
     * @return mixed
     */
    private function key($key)
    {
        $this->key = $this->prefix . '_' . $this->method . '_' . $key;
    }

    /**
     * _setCookie
     * @param string $key
     * @param string $value
     *
     * @return mixed
     */
    private function _setCookie($key, $value, $time = 3600)
    {
        return setCookie($this->prefix . $key, $value, time() + $time, "/", Config::get('host')->cookie);
    }

    /**
     * _getCookie
     * @param string $key
     *
     * @return mixed
     */
    private function _getCookie($key)
    {
        return isset($_COOKIE[$this->prefix . $key]) ? $_COOKIE[$this->prefix . $key] : false;
    }

    /**
     * _unsetCookie
     * @param string $key
     *
     * @return mixed
     */
    private function _unsetCookie($key)
    {
        return setCookie($this->prefix . $key, false, time() - 3600, "/", Config::get('host')->cookie);
    }

    /**
     * _setSession
     * @param string $key
     * @param string $value
     *
     * @return mixed
     */
    private function _setSession($key, $value, $time = 3600)
    {
        return $_SESSION[$this->prefix . $key] = $value;
    }

    /**
     * _getSession
     * @param string $key
     *
     * @return mixed
     */
    private function _getSession($key)
    {
        return (isset($_SESSION[$this->prefix . $key]) && $_SESSION[$this->prefix . $key]) ? $_SESSION[$this->prefix . $key] : false;
    }

    /**
     * _unsetSession
     * @param string $key
     *
     * @return mixed
     */
    private function _unsetSession($key)
    {
        unset($_SESSION[$this->prefix . $key]);

        return true;
    }

    /**
     * _initCli
     *
     * @return mixed
     */
    private function _initCli()
    {
        $this->id = md5($this->key);
        $this->file = Dever::path(Dever::data() . 'session/') . $this->id;

        if (is_file($this->file)) {
            $this->data = unserialize(file_get_contents($this->file));
            return;
        }

        file_put_contents($this->file, null);
    }

    /**
     * _setCli
     * @param string $key
     * @param string $value
     *
     * @return mixed
     */
    private function _setCli($key, $value, $time = 3600)
    {
        $this->_initCli();
        $key = $this->prefix . $key;
        $this->data[$key] = $value;
        file_put_contents($this->file, serialize($this->data));

        return $value;
    }

    /**
     * _getCli
     * @param string $key
     *
     * @return mixed
     */
    private function _getCli($key)
    {
        $this->_initCli();
        $key = $this->prefix . $key;
        return (isset($this->data[$key]) && $this->data[$key]) ? $this->data[$key] : false;
    }

    /**
     * _unsetCli
     * @param string $key
     *
     * @return mixed
     */
    private function _unsetCli($key)
    {
        $this->_initCli();
        $key = $this->prefix . $key;
        unset($this->data[$key]);
        file_put_contents($this->file, serialize($this->data));
        return true;
    }
}
