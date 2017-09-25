<?php namespace Dever\Cache\Store;

use Dever\Cache\Store;

class Memcache implements Store
{
    private $class;
    private $expire;

    public function __construct()
    {
        if (class_exists('\Memcached', false)) {
            $this->class = new \Memcached;
        } else {
            $this->class = new \Memcache;
        }
    }

    public function connect($config)
    {
        if (isset($config["host"])) {
            $this->expire = $config['expire'];

            $this->class->addServer($config["host"], $config["port"], $config["weight"]);
        }
    }

    public function get($key)
    {
        if (!$this->class) {
            return false;
        }

        $key = $this->key($key);

        $result = $this->class->get($key);

        return $result;
    }

    public function set($key, $value, $expire = 0)
    {
        if (!$this->class) {
            return false;
        }

        if (!is_string($key)) {
            return false;
        }

        $key = $this->key($key);

        $expire = $expire > 0 ? $expire : $this->expire;

        if (!class_exists('\Memcached', false)) {
            $result = $this->class->set($key, $value, MEMCACHE_COMPRESSED, $expire);
        } else {
            $result = $this->class->set($key, $value, $expire);
        }

        return $result;
    }

    public function delete($key)
    {
        if (!$this->class) {
            return false;
        }

        $key = $this->key($key);

        if ($this->class->delete($key, 0)) {
            return true;
        }
        return false;
    }

    public function close()
    {
        if (!$this->class) {
            return false;
        }

        if ($this->class->close()) {
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
