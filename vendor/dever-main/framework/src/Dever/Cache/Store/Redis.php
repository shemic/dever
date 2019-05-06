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

            $this->redis->pconnect($config["host"], $config["port"]);
            if (isset($config['password']) && $config['password']) {
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

    public function hGet($key, $hkey = false)
    {
        if (!$this->redis) {
            return false;
        }

        $key = $this->key($key);

        if ($hkey) {
            $result = $this->redis->hGet($key, $hkey);
        } else {
            $result = $this->redis->hGetAll($key);
        }
        return $result;
    }

    public function hDel($key, $hkey)
    {
        if (!$this->redis) {
            return false;
        }

        $key = $this->key($key);

        $result = $this->redis->hDel($key, $hkey);
        return $result;
    }

    public function hExists($key, $hkey)
    {
        if (!$this->redis) {
            return false;
        }

        $key = $this->key($key);

        $result = $this->redis->hExists($key, $hkey);
        return $result;
    }

    public function hKeys($key)
    {
        if (!$this->redis) {
            return false;
        }

        $key = $this->key($key);

        $result = $this->redis->hKeys($key);
        return $result;
    }

    public function hSet($key, $hkey, $value, $expire = 0)
    {
        if (!$this->redis) {
            return false;
        }

        if (!is_string($key)) {
            return false;
        }

        $key = $this->key($key);

        $expire = $expire > 0 ? $expire : $this->expire;

        $result = $this->redis->hSet($key, $hkey, $value);

        return $result;
    }

    public function delete($key)
    {
        if (!$this->redis) {
            return false;
        }

        if (!is_array($key)) {
            $key = $this->key($key);
        }

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
