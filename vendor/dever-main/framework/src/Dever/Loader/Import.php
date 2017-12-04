<?php namespace Dever\Loader;

use Dever\Http\Api;
use Dever\Output\Export;
use Dever\Routing\Input;
use Dever\String\Helper;
use Dever\Data\Model;
use Dever\Output\Debug;
use Dever\Cache\Handle as Cache;

class Import
{
    /**
     * API
     *
     * @var string
     */
    const API = '_api';

    /**
     * SECURE
     *
     * @var string
     */
    const SECURE_API = '_secure_api';

    /**
     * COMMIT
     *
     * @var string
     */
    const COMMIT = '_commit';

    /**
     * MAIN
     *
     * @var string
     */
    const MAIN = 'main';

    /**
     * api
     *
     * @var bool
     */
    protected $api = false;

    /**
     * param
     *
     * @var array
     */
    protected $param;

    /**
     * class
     *
     * @var array
     */
    protected $class;

    /**
     * data
     *
     * @var array
     */
    protected $data;

    /**
     * key
     *
     * @var string
     */
    protected $key;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * load
     *
     * @param  string  $method
     * @param  array  $param
     * @return \Dever\Loader\Import
     */
    public static function load()
    {
        list($key, $attr, $param) = self::argc(func_get_args());

        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self($key);
        }

        return self::$instance[$key]->get($param, $attr);
    }

    /**
     * argc
     * @param array $param
     *
     * @return array
     */
    public static function argc($argv)
    {
        if (isset($argv[0])) {
            $method = $argv[0];
        } else {
            Export::alert('api_param_exists', 'method');
        }

        $param = array();

        if (strpos($method, self::API) && empty($argv[1])) {
            $param = Input::get();
        } elseif (isset($argv[1]) && $argv[1]) {
            unset($argv[0]);
            $param = array_values($argv);
        }

        if (strpos($method, '?')) {
            $temp = explode('?', $method);
            $method = $temp[0];
            $param[0] = array();
            parse_str($temp[1], $param[0]);
        }

        list($method, $attr) = self::attr($method);

        return array($method, $attr, $param);
    }

    /**
     * attr
     *
     * @return mixed
     */
    public static function attr($key)
    {
        $attr = '';
        if (strpos($key, '#') !== false) {
            $temp = explode('#', $key);
            $key = $temp[0];
            $attr = $temp[1];
        }

        return array($key, $attr);
    }

    /**
     * __construct
     *
     * @return mixed
     */
    protected function __construct($key)
    {
        $this->key = $key;
    }

    /**
     * get
     *
     * @return mixed
     */
    protected function get($param = array(), $attr = '')
    {
        $this->data($this->status($param));

        if ($attr) {
            if (isset($this->data[$attr])) {
                return $this->data[$attr];
            }
            return false;
        }

        return $this->data;
    }

    /**
     * data
     *
     * @return mixed
     */
    protected function data($param)
    {
        $cache = $this->cache();
        if ($cache) {
            $this->data = $cache;
        } else {
            $this->getData($param);
            $this->cache($this->data);
        }
    }

    /**
     * getData
     *
     * @return mixed
     */
    protected function getData($status)
    {
        if (strpos($this->key, 'http://') !== false) {
            $this->loadServer($this->key);
        }

        if ($status == true || empty($this->data)) {
            $this->data = array();
            $this->loadClass();
        }

        $state = false;
    }

    /**
     * loadServer
     *
     * @return mixed
     */
    protected function loadServer($key = '', $url = '')
    {
        $this->data = $this->cache(false, 'curl');
        if (!$this->data) {
            $this->data = Server::get($url, $key, $this->param);
            $this->cache($this->data, 'curl');
        }
    }

    /**
     * manage
     *
     * @return mixed
     */
    protected function manage()
    {
        if (strpos($this->key, 'manage/') !== false && strpos($this->key, 'manage/auth') === false) {
            self::load('manage/auth.init');
        }
    }

    /**
     * loadClass
     *
     * @return mixed
     */
    protected function loadClass()
    {
        $this->manage();

        if (strpos($this->key, '!')) {
            $this->key = str_replace('!', '.' . self::MAIN, $this->key);
        }

        if (strpos($this->key, '.')) {
            $this->loadMethod();
        } elseif (strpos($this->key, '-')) {
            $this->loadModel();
        } else {
            $this->class = Library::get()->loadClass($this->key);
            $this->data =& $this->class;
        }
    }

    /**
     * loadModel
     *
     * @return mixed
     */
    protected function loadModel()
    {
        if (isset($this->param[0])) {
            $param = $this->param[0];
        } else {
            $param = $this->param;
        }
        $this->data = Model::load($this->key, $param);
    }

    /**
     * loadMethod
     *
     * @return mixed
     */
    protected function loadMethod()
    {
        list($class, $method) = $this->demerge();
        list($key, $project) = Library::get()->getProject($class);
        $this->ai($project);
        if ($this->data) {
            return;
        }
        if ($project && strpos($project['path'], 'http://') === 0) {
            $this->loadServer(strtolower($key) . '.' . $method, $project['path']);
            return;
        }

        $this->class = Library::get()->loadClass($class);
        $method = $this->api($class, $method);
        $commit = $this->commit($method);
        if ($commit) {
            $db = Model::load(DEVER_APP_NAME . '/commit');
            try {
                $db->begin();
                $this->call($method, $project);
                $db->commit();
            } catch (\Exception $e) {
                $db->rollBack();
                //$this->data = -1;
                $data = $e->getTrace();
                Export::alert(implode(' ', $data[1]['args']));
            }
        } else {
            $this->call($method, $project);
        }
    }

    /**
     * ai
     *
     * @return mixed
     */
    protected function ai($project)
    {
        if (DEVER_APP_NAME == 'ai' && !$project && Config::get('base')->ai && isset(Config::get('base')->ai[$this->key])) {
            $this->data = Import::load('ai/data.get', $this->key, Config::get('base')->ai[$this->key]);
        }
    }

    /**
     * demerge
     *
     * @return mixed
     */
    protected function demerge()
    {
        list($class, $method) = explode('.', $this->key);
        $main = array(self::MAIN . self::API, self::MAIN . self::SECURE_API, self::MAIN . self::COMMIT);
        if (in_array($method, $main)) {
            $method = str_replace(self::API, '', $method);
        }
        return array($class, $method);
    }

    /**
     * call
     *
     * @return mixed
     */
    protected function call($method, $project)
    {
        if (is_array($method)) {
            foreach ($method as $one) {
                $this->call($one, $project);
                $this->setCall();
            }
        } else {
            $plugin = Config::get('plugin')->{$this->key};
            //$this->step($method, $this->class);
            $param = $this->getParam($method);
            if ($plugin && isset($plugin['start'])) {
                $param = Import::load($project['name'] . '/plugin/' . $plugin['start'], $param);
            }

            if ($plugin && isset($plugin['cover'])) {
                $this->data = Import::load($project['name'] . '/plugin/' . $plugin['cover'], $param);
            } else {
                if (is_array($param) && $param) {
                    $this->data = call_user_func_array(array($this->class, $method), $param);
                } else {
                    $this->data = call_user_func(array($this->class, $method), $param);
                }
            }

            if ($plugin && isset($plugin['end'])) {
                Import::load($project['name'] . '/plugin/' . $plugin['end'], $this->data);
            }
            
            Debug::reflection($this->class, $method);

            if ($this->api) {
                $this->apiRecord();
            }
        }
    }

    /**
     * getParam
     *
     * @return mixed
     */
    protected function getParam($method)
    {
        if (!isset($this->param[0])) {
            $reflectionMethod = new \ReflectionMethod($this->class, $method);
            $param = $reflectionMethod->getParameters();
            $result = array();
            foreach ($param as $k => $v) {
                $name = $v->name;
                if (isset($this->param[$name])) {
                    $result[$name] = $this->param[$name];
                }
            }
            return $result;
        } else {
            return $this->param;
        }
    }

    /**
     * setCall
     *
     * @return mixed
     */
    protected function setCall()
    {
        if (!$this->data && isset($this->param['callback'])) {
            $this->data = $this->param['callback'];
        }
        $this->param['callback'] = $this->data;
    }

    /**
     * apiRecord
     *
     * @return mixed
     */
    protected function apiRecord()
    {
        if (Config::get('base')->apiDoc) {
            Api::doc($this->key, $this->param, $this->data);
        }

        if (Config::get('base')->apiLog) {
            Api::log($this->key, $this->param, $this->data);
        }
    }

    /**
     * api
     *
     * @return mixed
     */
    protected function api($class, $method)
    {
        $this->api = false;
        if (strpos($method, self::API) !== false) {
            $this->api = true;
            $method = $this->secure($method);
            $map = Helper::replace(array(self::API, self::SECURE_API), '', $method);
            if (method_exists($this->class, $map) && method_exists($this->class, $method)) {
                return array($map, $method);
            }
        }

        if (Config::get('base')->apiConfig) {
            list($api, $method) = Api::load($class, $method, self::API, $this->param);
            if ($api) {
                $this->api = $api;
            }
        }

        if (!method_exists($this->class, $method)) {
            if (Config::get('base')->apiOpenPath) {
                $className = get_class($this->class);
                if (stripos($className, '\\' . Config::get('base')->apiOpenPath)) {
                    if (strpos($method, self::API) !== false) {
                        $temp = explode(self::API, $method);
                        $method = $temp[0];
                    }
                    return $method;
                }
            }

            if ($this->api == false) {
                $newMethod = $method . self::API;
                if (method_exists($this->class, $newMethod)) {
                    return $newMethod;
                }
            }
            Export::alert('method_exists', array($class, $method));
        }
        return $method;
    }

    /**
     * secure
     *
     * @return mixed
     */
    protected function secure($method)
    {
        if (strpos($method, self::SECURE_API) !== false) {
            Api::check($this->key, $this->param);
        } else {
            $api = Helper::replace(self::API, self::SECURE_API, $method);
            if (method_exists($this->class, $api)) {
                Api::check($this->key, $this->param);
                return $api;
            }
        }

        return $method;
    }

    /**
     * commit
     *
     * @return mixed
     */
    protected function commit($method)
    {
        if (is_array($method)) {
            foreach ($method as $one) {
                $state = $this->commit($one);
                if ($state) {
                    return true;
                }
            }
        }
        elseif (strpos($method, self::COMMIT) !== false) {
            return true;
        }
        return false;
    }

    /**
     * cache
     *
     * @return mixed
     */
    protected function cache($data = false, $type = 'load')
    {
        if (isset($this->param) && $this->param) {
            $key = $this->key . '_' . md5(serialize($this->param));
        } else {
            $param = Input::get();
            $key = $this->key . '_' . md5(serialize($param));
        }

        if ($page = Input::get('page')) {
            $key .= '_p' . $page;
        }

        return Cache::load($key, $data, 0, $type);
    }

    /**
     * status
     *
     * @return mixed
     */
    protected function status($param)
    {
        $status = false;
        if (isset($param['cache']) && $param['cache']) {
            $status = true;
        } elseif (isset($this->param) && $this->param != $param) {
            $status = true;
        }
        $this->param = $param;
        //$this->unsetParam();
        
        return $status;
    }

    /**
     * unsetParam
     *
     * @return mixed
     */
    protected function unsetParam()
    {
        $config = array('shell', 'json', 'callback', 'function');
        foreach ($config as $k => $v) {
            if (isset($this->param[$v])) {
                unset($this->param[$v]);
            }
        }
    }

    /**
     * step
     *
     * @return mixed
     */
    protected function step($method, $class)
    {
        $key = '_step_';
        if (strpos($method, $key) != false) {
            $config = explode($key, $method);

            Step::init($config[0], $config[1], $key, $class);
        }
    }

    /**
     * log
     *
     * @return log
     */
    protected function log($method, $data = array())
    {
        Debug::log(array('method' => $method, 'param' => $this->param, 'data' => $data), 'load');
    }
}
