<?php namespace Dever\Http;

use Dever\Loader\Config;
use Dever\Output\Export;
use Dever\String\Encrypt;
use Dever\Support\Command;
use Dever\Output\Debug;

class Api
{
    /**
     * time
     *
     * @var int
     */
    const TIME = 300;

    /**
     * path 定义api目录
     *
     * @var string
     */
    const PATH =  'api/';

    /**
     * default token
     *
     * @var string
     */
    private static $token = 'dever_api';

    /**
     * log
     *
     * @var string
     */
    private static $log;

    /**
     * load
     * @param  string  $key
     * @param  string  $path
     * 
     * @return string
     */
    static public function load($class, $method, $api, $param = array())
    {
        $config = self::config();
        $key = str_replace($api, '', $method);
        $index = str_replace(DEVER_APP_NAME . '/', '', $class . '.' . $key);
        if (isset($config[$index]) && $config[$index]) {
            if (isset($config[$index]['secure'])) {
                self::check($index, $param);
            }
            return array(true, $key);
        }
        return array(false, $method);
    }

    /**
     * config
     * @param  string  $path
     * 
     * @return string
     */
    static public function config($path = false)
    {
        if (!$path) {
            $path = DEVER_APP_PATH;
        }
        $file = $path . self::PATH . 'main.php';

        if (is_file($file)) {
            return include($file);
        }

        return array();
    }

    /**
     * login
     * @param  string $uid
     *
     * @return string
     */
    public static function login($uid)
    {
        $auth = '';
        $data = array($uid, time());
        if ($data) {
            $auth = base64_encode(Encrypt::encode(implode("\t", $data), self::$token));
        }

        return $auth;
    }

    /**
     * get
     *
     * @return mixed
     */
    public static function get($request)
    {
        $time = time();
        $nonce = self::nonce();
        $signature = self::signature($time, $nonce, $request);

        $request += array
            (
            'time' => $time,
            'nonce' => $nonce,
            'signature' => $signature,
        );

        return $request;
    }

    /**
     * check
     * @param  string  $key
     *
     * @return string
     */
    public static function check($key, $request)
    {
        if (empty($request['signature']) || empty($request['nonce'])) {
            Export::alert('api_signature_exists');
        }

        if (empty($request['time'])) {
            return self::loginResult($request['signature']);
        }
        if (time() - $request['time'] > self::TIME) {
            Export::alert('api_signature_exists');
        }

        $signature = self::signature($request['time'], $request['nonce'], $request);

        if ($request['signature'] != $signature) {
            Export::alert('api_signature_exists');
        }

        return $signature;
    }

    /**
     * loginResult
     *
     * @return mixed
     */
    public static function loginResult($signature)
    {
        $auth = $user = array();
        if ($signature) {
            $auth = explode("\t", Encrypt::decode(base64_decode($signature), self::$token));
        }

        list($uid, $time) = (empty($auth) || count($auth) < 2) ? array(0, '') : $auth;

        if (!empty($uid) && (time() - $time) < 2592000) {
            Config::get('base')->user = array('uid' => $uid, 'time' => $time);
            return $user;
        } else {
            Export::alert('api_signature_exists');
        }
    }

    /**
     * signature
     *
     * @return mixed
     */
    public static function signature($time, $nonce, $request = array())
    {
        $request['token'] = self::token();
        $request['time'] = $time;
        $request['nonce'] = $nonce;
        ksort($request);

        $signature_string = '';
        foreach ($request as $k => $v) {
            $signature_string .= $k . '=' . $v . '&';
        }
        $signature_string = substr($signature_string, 0, -1);
        return sha1($signature_string);
    }

    /**
     * token
     *
     * @return mixed
     */
    public static function token()
    {
        self::$token = Config::get('base')->token ? Config::get('base')->token : self::$token;
        return md5(self::$token);
    }

    /**
     * nonce
     *
     * @return mixed
     */
    public static function nonce()
    {
        return substr(md5(microtime()), rand(10, 15));
    }

    /**
     * doc
     * @param  string  $api
     * @param  array  $request
     * @param  array  $response
     * 
     * @return mixed
     */
    static public function doc($api, $request, $response)
    {
        if (is_array($response) && isset($response[0])) {
            $response = array_shift($response);
        }
        $data['api'] = $api;
        $data['request'] = $request;
        $data['response'] = $response;
        Command::log('auth.api', $data);
    }

    /**
     * doc
     * @param  string  $api
     * @param  array  $request
     * @param  array  $response
     * 
     * @return mixed
     */
    static public function log($api, $request, $response)
    {
        $data['api'] = $api;
        $data['request'] = $request;
        $data['response'] = $response;
        Debug::error($data);
    }
}
