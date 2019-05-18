<?php namespace Dever\Cache;

# 2019-04-23更新，为了新需求比较乱，后续优化
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
            return false;
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
     * increment
     * @param string $key
     * @param array $data
     * @param int $expire
     * @param string $type
     *
     * @return Dever\Cache\Handle;
     */
    public static function increment($key = false, $data = 1)
    {
        $cache = Config::get('cache')->cAll;
        if (empty($cache[$type])) {
            return false;
        }
        $handle = self::getInstance($type, $cache[$type]);

        $data = $handle->get($key);
        if (!$data) {
            return $handle->set($key, $data);
        }
        return $handle->incr($key, $data);
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

    /**
     * closeAll
     *
     * @return mixed
     */
    public static function closeAll()
    {
        return;
        if (self::$instance) {
            foreach (self::$instance as $k => $v) {
                self::$instance[$k]->close();
                self::$instance[$k] = null;
                unset(self::$instance[$k]);
            }
        }
    }

    public function __construct($expire = 3600, $type = 'mysql')
    {
        $this->expire = $expire;
        $this->type = $type;

        if (!$this->config) {
            $this->config = Config::get('cache')->cAll;
        }
    }

    public function close()
    {
        if ($this->store) {
            $this->store->close();
        }
    }

    public function store($key)
    {
        if ($this->store) {
            return true;
        }

        if ($this->none($key)) {
            return false;
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

            return true;
        }

        return false;
    }

    /**
     * get
     *
     * @return mixd
     */
    public function get($key, $page = true)
    {
        $param = isset($this->config['shell']) ? $this->config['shell'] : 'clearcache';
        if (Input::shell($param)) {
            //$this->delete($key);
            return false;
        }

        if ($page) {
            if (!$this->init($key)) {
                return false;
            }
        }

        if (!$this->store($key)) {
            return false;
        }

        $data = $this->store->get($key);
        //$data = json_decode(base64_decode($data), true);
        if (strstr($data, 'serialize_')) {
            $data = unserialize($data);
        }
        
        $this->log('get', $key, $data, $this->expire($key));

        $page_key = 'page_' . $key;
        if ($page && $page_data = $this->store->get($page_key)) {
            $page_data = unserialize($page_data);
            Dever::$global['page'] = $page_data;
            $this->log('get', $page_key, $page_data, $this->expire($page_key));
        }

        return $data;
    }

    /**
     * set
     *
     * @return mixd
     */
    public function set($key, $value, $expire = 0, $page = true)
    {
        $state = true;
        if ($page) {
            $state = $this->init($key);
            if (!$state) {
                return false;
            }
        }

        if (!$this->store($key)) {
            return false;
        }

        if ($expire == 0) {
            if ($state > 1) {
                $expire = $state;
            } else {
                $expire = $this->expire;
            }
        }

        $this->expire($key, $expire);
        $this->log('set', $key, $value, $expire);
        //$value = base64_encode(json_encode($value));
        if (is_array($value)) {
            $value = 'serialize_' . serialize($value);
        }

        if ($page && isset(Dever::$global['page']) && Dever::$global['page']) {
            $page_key = 'page_' . $key;
            $this->store->set($page_key, serialize(Dever::$global['page']), $expire);
            $this->log('set', $page_key, Dever::$global['page'], $expire);
        }
        
        return $this->store->set($key, $value, $expire);
    }

    /**
     * incr
     *
     * @return mixd
     */
    public function incr($key, $value)
    {
        $state = true;
        if ($page) {
            $state = $this->init($key);
            if (!$state) {
                return false;
            }
        }

        if (!$this->store($key)) {
            return false;
        }
        
        return $this->store->incr($key, $value);
    }

    /**
     * hGet
     *
     * @return mixd
     */
    public function hGet($key, $hkey = '', $exists = false)
    {
        $param = isset($this->config['shell']) ? $this->config['shell'] : 'clearcache';
        if (Input::shell($param)) {
            //$this->delete($key);
            return false;
        }

        if (!$this->init($key)) {
            return false;
        }

        if (!$this->store($key)) {
            return false;
        }

        if ($hkey) {
            if ($exists) {
                $data = $this->store->hExists($key, $hkey);
                return $data;
            } else {
                $data = $this->store->hGet($key, $hkey);
            }
        } else {
            if ($exists) {
                $data = $this->store->hKeys($key);
                return $data;
            }
            $data = $this->store->hGet($key);
        }

        if ($data === false) {
            $data = false;
        } else {
            $data = unserialize($data);
        }

        $this->log('hGet', $key . ':' . $hkey, $data, $this->expire($key));

        return $data;
    }

    /**
     * hSet
     *
     * @return mixd
     */
    public function hSet($key, $hkey, $value, $expire = 0)
    {
        $state = $this->init($key);
        if (!$state) {
            return false;
        }

        if (!$this->store($key)) {
            return false;
        }

        if ($expire == 0) {
            if ($state > 1) {
                $expire = $state;
            } else {
                $expire = $this->expire;
            }
        }

        $this->expire($key, $expire);
        $this->log('hSet', $key . ':' . $hkey, $value, $expire);
        $value = serialize($value);
        
        return $this->store->hSet($key, $hkey, $value, $expire);
    }

    /**
     * delete
     *
     * @return mixd
     */
    public function delete($key, $hkey = false)
    {
        $state = $this->store($key);
        if (!$state) {
            return false;
        }
        $this->log('delete', $key, 1);
        if ($hkey) {
            return $this->store->hDel($key, $hkey);
        }
        return $this->store->delete($key);
    }

    /**
     * init
     *
     * @return mixed
     */
    protected function init($key)
    {
        if ($this->type == 'route' && isset(Dever::config('base')->clearCache[$this->type])) {
            return false;
        }
        $state = 1;

        if (isset($this->config[$this->type . 'Key'])) {
            foreach ($this->config[$this->type . 'Key'] as $k => $v) {
                if (strpos($key, $k) !== false) {
                    $state = $v;
                }
            }
        }

        if (!$state && strstr($key, 'route_')) {
            Dever::config('base')->clearCache = array($this->type => 1);
        }

        return $state;
    }

    /**
     * none
     *
     * @return mixed
     */
    protected function none($key)
    {
        if (isset($this->config[$this->type . 'None'])) {
            foreach ($this->config[$this->type . 'None'] as $k => $v) {
                if (strpos($key, $v) !== false) {
                    return true;
                }
            }
        }

        return false;
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
            if (!Input::shell('all') && is_array($value)) {
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
