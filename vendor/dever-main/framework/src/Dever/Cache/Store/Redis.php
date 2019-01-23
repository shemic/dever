<?php namespace Dever\Cache\Store;

use Dever\Cache\Store;

class Redis implements Store
{
    private $redis;
    private $expire;

    public function __construct()
    {
        $this->redis = new \Redis;
    }

    public function connect($config)
    {
        if (isset($config["host"])) {
            $this->expire = $config['expire'];

            $this->redis->connect($config["host"], $config["port"]);
            if (isset($config['password'])) {
                $this->redis->auth($config['password']);
            }
        }
    }

    public function getRedis()
    {
        return $this->redis;
    }

    public function get($key)
    {
        if (!$this->redis) {
            return false;
        }

        $key = $this->key($key);

        $result = $this->redis->get($key);

        return $result;
    }

    public function set($key, $value, $expire = 0)
    {
        if (!$this->redis) {
            return false;
        }

        if (!is_string($key)) {
            return false;
        }

        $key = $this->key($key);

        $expire = $expire > 0 ? $expire : $this->expire;

        $result = $this->redis->set($key, $value, $expire);

        return $result;
    }

    public function delete($key)
    {
        if (!$this->redis) {
            return false;
        }

        $key = $this->key($key);

        if ($this->redis->delete($key, 0)) {
            return true;
        }
        return false;
    }

    public function close()
    {
        if (!$this->redis) {
            return false;
        }

        if ($this->redis->close()) {
            return true;
        }
        return false;
    }

    private function key($key)
    {
        return '_' . $key;
        return DEVER_APP_NAME . '_' . $key;
    }
}
