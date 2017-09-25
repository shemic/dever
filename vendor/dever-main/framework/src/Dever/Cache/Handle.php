<?php namespace Dever\Cache;

use Dever;
use Dever\Cache\Store;
use Dever\Loader\Config;
use Dever\Output\Debug;
use Dever\Routing\Input;

class Handle
{
    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * store
     *
     * @var object
     */
    protected $store;

    /**
     * config
     *
     * @var array
     */
    protected $config;

    /**
     * expire
     *
     * @var int
     */
    protected $expire;

    /**
     * type
     *
     * @var string
     */
    protected $type;

    /**
     * get
     * @param string $key
     * @param array $data
     * @param int $expire
     * @param string $type
     *
     * @return Dever\Cache\Handle;
     */
    public static function load($key = false, $data = false, $expire = 0, $type = 'data')
    {
        $cache = Config::get('cache')->cAll;
        if (empty($cache[$type])) {
            return;
        }
        $handle = self::getInstance($type, $cache[$type]);

        if (is_string($data) && $data == 'delete') {
            return $handle->delete($key);
        } elseif ($data) {
            return $handle->set($key, $data, $expire);
        }
        return $handle->get($key);
    }

    /**
     * getInstance
     *
     * @return Dever\Cache\Handle;
     */
    public static function getInstance($type = 'mysql', $expire = 3600)
    {
        if (empty(self::$instance[$type])) {
            self::$instance[$type] = new self($expire, $type);
        }

        return self::$instance[$type];
    }

    public function __construct($expire = 3600, $type = 'mysql')
    {
        $this->expire = $expire;
        $this->type = $type;
    }

    public function store($key)
    {
        if ($this->store) {
            return;
        }

        if (!$this->config) {
            $this->config = Config::get('cache')->cAll;
        }

        if (isset($this->config['store']) && $this->config['store']) {
            $class = 'Dever\\Cache\\Store\\' . ucfirst($this->config['type']);
            $this->store = new $class();

            $this->log('connect', $this->config['type']);

            foreach ($this->config['store'] as $k => $v) {
                if (empty($v['expire'])) {
                    $v['expire'] = $this->expire;
                }
                $this->store->connect($v);
            }
        }
    }

    /**
     * get
     *
     * @return mixd
     */
    public function get($key)
    {
        $this->store($key);
        if (!$this->store) {
            return false;
        }
        $param = isset($this->config['shell']) ? $this->config['shell'] : 'clearcache';
        if (Input::shell($param)) {
            //$this->delete($key);
            return false;
        }
        $data = $this->store->get($key);
        //$data = json_decode(base64_decode($data), true);
        $this->log('get', $key, $data, $this->expire($key));
        if ($page = $this->store->get('page_' . $key)) {
            Dever::$global['page'] = $page;
        }
        return $data;
    }

    /**
     * set
     *
     * @return mixd
     */
    public function set($key, $value, $expire = 0)
    {
        $this->store($key);
        if (!$this->store) {
            return false;
        }
        $this->init($key, $expire);
        $this->log('set', $key, $value, $expire);
        //$value = base64_encode(json_encode($value));
        if (isset(Dever::$global['page']) && Dever::$global['page']) {
            $this->store->set('page_' . $key, Dever::$global['page'], $expire);
        }
        
        return $this->store->set($key, $value, $expire);
    }

    /**
     * delete
     *
     * @return mixd
     */
    public function delete($key)
    {
        $this->store($key);
        if (!$this->store) {
            return false;
        }
        $this->log('delete', $key, 1);
        return $this->store->delete($key);
    }

    /**
     * init
     *
     * @return mixed
     */
    protected function init($key, &$expire)
    {
        if (isset($this->config[$this->type . 'Key'])) {
            foreach ($this->config[$this->type . 'Key'] as $k => $v) {
                if (strpos($key, $k) !== false) {
                    $expire = $v;
                }
            }
        }

        $this->expire($key, ($expire > 0 ? $expire : $this->expire));
    }

    /**
     * expire
     *
     * @return mixed
     */
    protected function expire($key, $expire = false)
    {
        if (isset($this->config['expire']) && $this->config['expire']) {
            $key .= '_expire';
            if ($expire > 0) {
                //$expire = $expire * 2;
                $this->store->set($key, DEVER_TIME + $expire, $expire);
            } else {
                $expire = $this->store->get($key);
                if ($expire) {
                    $num = $expire - DEVER_TIME;
                    return '将于' . date('Y-m-d H:i:s', $expire) . '失效(' . $num . 's)';
                }
            }
        }
        return false;
    }

    /**
     * log
     *
     * @return mixed
     */
    protected function log($method, $key = false, $value = false, $expire = 0)
    {
        $expire = $expire ? $expire : $this->expire;
        $log = array();
        $log['method'] = $method;
        $log['key'] = $key;
        if ($value) {
            if (!Input::shell('all')) {
                $value = count($value) . ' records';
            }
            $log['value'] = $value;
        }
        if ($expire >= 0) {
            $log['expire'] = $expire;
        }
        Debug::log($log, 'cache');
    }
}
