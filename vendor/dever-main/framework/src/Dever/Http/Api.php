<?php namespace Dever\Http;

use Dever\Loader\Config;
use Dever\Output\Export;
use Dever\String\Encrypt;
use Dever\Support\Command;
use Dever\Output\Debug;
use Dever\Loader\Project;
use Dever\Support\Path;
use Dever\Data\Model;

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
                self::check($param, $config[$index]);
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
    public static function get($request, $key = false)
    {
        if ($key) {
            Config::get('base')->token = $key;
        }
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
    public static function check($request, $param = array(), $key = false)
    {
        if (empty($request['signature']) || empty($request['nonce'])) {
            Export::alert('api_signature_exists');
        }

        if ($key) {
            Config::get('base')->token = $key;
        }

        self::checkSignatureExits($request['signature']);

        if (empty($request['time'])) {
            return self::checkLogin($request['signature']);
        }
        if (time() - $request['time'] > self::TIME) {
            Export::alert('api_signature_exists');
        }

        $signature_check = $request['signature'];

        if ($param && isset($param['request'])) {
            foreach ($param['request'] as $k => $v) {
                if (isset($request[$k])) {
                    $param['request'][$k] = $request[$k];
                }
            }
            $temp = $param['request'];
            $temp['token'] = self::token();
            $temp['time'] = $request['time'];
            $temp['nonce'] = $request['nonce'];
            $request = $temp;
        }

        $signature = self::signature($request['time'], $request['nonce'], $request);

        if ($signature_check != $signature) {
            Export::alert('api_signature_exists');
        }

        return $signature;
    }

    /**
     * checkLogin
     *
     * @return mixed
     */
    public static function checkLogin($signature, $state = true, $max = false)
    {
        if (is_numeric($signature)) {
            return Config::get('base')->user = array('uid' => $signature, 'time' => time());
        }
        $auth = $user = array();
        if ($signature) {
            $auth = explode("\t", Encrypt::decode(base64_decode($signature), self::$token));
        }

        list($uid, $time) = (empty($auth) || count($auth) < 2) ? array(0, '') : $auth;
        Config::get('base')->user = array('uid' => $uid, 'time' => $time);

        if (!empty($uid)) {
            if ($max == false || ($max > 0 && (time() - $time) < $max)) {
                return Config::get('base')->user;
            }
        }
        if ($state) {
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
        if (isset($request['signature'])) {
            unset($request['signature']);
        }
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
        return sha1(self::$token);
    }

    /**
     * nonce
     *
     * @return mixed
     */
    public static function nonce()
    {
        return substr(sha1(microtime()), rand(10, 15));
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

    /**
     * checkSignatureExits
     * @param  string  $api
     * @param  array  $request
     * @param  array  $response
     * 
     * @return mixed
     */
    static public function checkSignatureExits($signature)
    {
        $type = Config::get('base')->apiSignature;
        if (!$type) {
            return;
        }
        $type = 'file';
        if (Project::load('manage') && $type == 'db') {
            $where['signature'] = $signature;
            $info = Model::load('manage/signature')->one($where);
            if ($info) {
                Export::alert('api_signature_repeat');
            } else {
                Model::load('manage/signature')->insert($where);
            }
        } else {
            $path = Path::month('signature');
            $file = $path . 'signature_' . date('Y_m_d');
            $config = array();
            if (is_file($file)) {
                $config = include($file);
                if (isset($config[$signature])) {
                    Export::alert('api_signature_repeat');
                }
            }
            $config[$signature] = 1;
            $content = '<?php return ' . var_export($config, true) . ';';
            file_put_contents($file, $content);
        }
    }
}
