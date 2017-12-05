<?php namespace Dever\Http;

use Dever\Loader\Config;
use Dever\Output\Debug;
use Dever\Routing\Input;

class Curl
{
    /**
     * handle
     *
     * @var object
     */
    private $handle;

    /**
     * url
     *
     * @var string
     */
    private $url;

    /**
     * param
     *
     * @var array
     */
    private $param;

    /**
     * instance
     *
     * @var object
     */
    protected static $instance;

    /**
     * load
     *
     * @return \Dever\Http\Curl
     */
    public static function getInstance($url, $param = false, $type = 'get', $json = false)
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
        }
        return self::$instance->load($url, $param, $type, $json);
    }

    /**
     * get
     *
     * @return mixed
     */
    public static function get($url, $param = false, $type = 'get')
    {
        return self::getInstance($url, $param, $type)->result();
    }

    /**
     * get
     *
     * @return mixed
     */
    public static function post($url, $param = false, $json = false)
    {
        return self::getInstance($url, $param, $type = 'post', $json)->result();
    }

    /**
     * load
     *
     * @return mixed
     */
    public function load($url, $param = false, $type = '', $json = false)
    {
        $this->init();

        $this->param($param);

        $this->setRequest($type);

        if ($json && $param) {
            $this->setJson($param);
        } elseif ($type == 'post' || $type == 'put') {
            $this->setParam($param);
        } elseif ($param) {
            $url .= '?' . http_build_query($param);
        }

        $this->setUrl($url);

        return $this;
    }

    /**
     * param
     *
     * @return array
     */
    private function param($param)
    {
        if (isset($param[0])) {
            $temp = $param;
            $param = array();
            foreach ($temp as $k => $v) {
                if (is_array($v)) {
                    $param = array_merge($param, $v);
                } else {
                    $param[$k] = $v;
                }
            }
        }
        return $param;
    }

    /**
     * setting
     *
     * @return \Dever\Http\Curl
     */
    public function setting($setting = array())
    {
        if ($setting) {
            $this->init();
            foreach ($setting as $k => $v) {
                $method = 'set' . ucfirst($k);
                $this->$method($v);
            }
        }
        return $this;
    }

    /**
     * result
     *
     * @return \Dever\Http\Curl
     */
    public function result($setting = true)
    {
        if ($setting && $config = Config::get('base')->curl) {
            $this->setting($config);
        }
        $result = curl_exec($this->handle);
        //curl_close($this->handle);
        $data = $result;
        if (!Input::shell('all')) {
            $data = count($data) . ' records';
        }
        Debug::log(array('url' => $this->url, 'param' => $this->param, 'result' => $data), 'curl');
        return $result;
    }

    /**
     * resultCookie
     *
     * @return \Dever\Http\Curl
     */
    public function resultCookie()
    {
        $result = $this->result();
        list($header, $body) = explode("\r\n\r\n", $result, 2);
        preg_match_all("/Set\-Cookie:([^;]*);/", $header, $matches);
        $info['cookie']  = substr($matches[1][0], 1);
        $info['content'] = $body;
        return $info;
    }

    /**
     * setVerbose
     *
     * @return \Dever\Http\Curl
     */
    public function setVerbose($verbose)
    {
        curl_setopt($this->handle, CURLOPT_VERBOSE, $verbose);
        return $this;
    }

    /**
     * setAgent
     *
     * @return \Dever\Http\Curl
     */
    public function setAgent($agent)
    {
        curl_setopt($this->handle, CURLOPT_USERAGENT, $agent);
        return $this;
    }

    /**
     * setUserPWD
     *
     * @return \Dever\Http\Curl
     */
    public function setUserPWD($userpwd)
    {
        curl_setopt($this->handle, CURLOPT_USERPWD, $userpwd);
        return $this;
    }

    /**
     * setTimeOut
     *
     * @return \Dever\Http\Curl
     */
    public function setTimeOut($time)
    {
        curl_setopt($this->handle, CURLOPT_TIMEOUT, $time);
        return $this;
    }

    /**
     * setCookie
     *
     * @return \Dever\Http\Curl
     */
    public function setCookie($cookie)
    {
        curl_setopt($this->handle, CURLOPT_COOKIE, $cookie);
        return $this;
    }

    /**
     * setUrl
     *
     * @return \Dever\Http\Curl
     */
    public function setUrl($url)
    {
        $this->url = $url;
        curl_setopt($this->handle, CURLOPT_URL, $url);
        curl_setopt($this->handle, CURLOPT_RETURNTRANSFER, true);
        return $this;
    }

    /**
     * setParam
     *
     * @return \Dever\Http\Curl
     */
    public function setParam($param)
    {
        $this->param = $param;
        curl_setopt($this->handle, CURLOPT_POSTFIELDS, $param);
        return $this;
    }

    /**
     * setEncode
     *
     * @return \Dever\Http\Curl
     */
    public function setEncode($encode)
    {
        curl_setopt($this->handle, CURLOPT_ENCODING, $encode);
        return $this;
    }

    /**
     * setJson
     *
     * @return \Dever\Http\Curl
     */
    public function setJson($param)
    {
        $string = str_replace("\\/", "/", json_encode($param));
        $search = "#\\\u([0-9a-f]+)#ie";

        if (strpos(strtoupper(PHP_OS), 'WIN') === false) {
            $replace = "iconv('UCS-2BE', 'UTF-8', pack('H4', '\\1'))";
        } else {
            $replace = "iconv('UCS-2', 'UTF-8', pack('H4', '\\1'))";
        }

        $param = preg_replace($search, $replace, $string);
        //$param = json_encode($param, JSON_UNESCAPED_UNICODE);
        $header['Content-Type'] = 'application/json';
        $header['Content-Length'] = strlen($param);
        $this->setHeader($header);
        $this->setParam($param);
        return $this;
    }

    /**
     * setRefer
     *
     * @return \Dever\Http\Curl
     */
    public function setRefer($refer)
    {
        curl_setopt($this->handle, CURLOPT_REFERER, $refer);
        curl_setopt($this->handle, CURLOPT_HEADER, true);
        return $this;
    }

    /**
     * setRefer
     *
     * @return \Dever\Http\Curl
     */
    public function setRequest($type)
    {
        if ($type == 'post') {
            curl_setopt($this->handle, CURLOPT_POST, true);
        } elseif ($type == 'put' || $type == 'delete') {
            curl_setopt($this->handle, CURLOPT_CUSTOMREQUEST, strtoupper($type));
        } else {
            curl_setopt($this->handle, CURLOPT_HTTPGET, true);
        }
        
        return $this;
    }

    /**
     * setIp
     *
     * @return \Dever\Http\Curl
     */
    public function setIp($ip)
    {
        $config['CLIENT-IP'] = $ip;
        $config['X-FORWARDED-FOR'] = $ip;

        $this->setHeader($config);
        curl_setopt($this->handle, CURLOPT_HEADER, true);
        return $this;
    }

    /**
     * setHeader
     *
     * @return \Dever\Http\Curl
     */
    public function setHeader($config)
    {
        $header = array();
        foreach ($config as $k => $v) {
            $header[] = $k . ':' . $v;
        }
        curl_setopt($this->handle, CURLOPT_HTTPHEADER, $header);
        return $this;
    }

    /**
     * init
     *
     * @return mixed
     */
    private function init()
    {
        if (!$this->handle) {
            $this->handle = curl_init();
            curl_setopt($this->handle, CURLOPT_HEADER, false);
        }
    }
}
