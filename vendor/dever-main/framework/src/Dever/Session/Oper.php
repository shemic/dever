<?php namespace Dever\Session;

use Dever\Loader\Config;
use Dever\String\Encrypt;

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
        @header('P3P: CP="CURa ADMa DEVa PSAo PSDo OUR BUS UNI PUR INT DEM STA PRE COM NAV OTC NOI DSP COR"');
        @session_start();
        if (Config::get('host')->cookie) {
            ini_set('session.cookie_domain', Config::get('host')->cookie);
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

        $key = $this->project . '_' . $key;

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
}
