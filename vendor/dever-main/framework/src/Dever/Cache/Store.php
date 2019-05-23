<?php namespace Dever\Cache;

/*
interface StoreInterFace
{
	public function __construct();
	public function connect($config);
	public function get($key);
	public function set($key, $value, $expire);
	public function delete($key);
	public function close();
}
*/

class Store
{
	/**
     * instance
     *
     * @var string
     */
    protected static $instance;

	/**
     * getInstance
     *
     * @return Dever\Cache\Handle;
     */
    public static function getInstance($type = 'redis', $handle)
    {
        if (empty(self::$instance[$type])) {
            $class = 'Dever\\Cache\\Store\\' . ucfirst($type);
            self::$instance[$type] = new $class();
            $handle->log('connect', $type);
        }

        return self::$instance[$type];
    }
}