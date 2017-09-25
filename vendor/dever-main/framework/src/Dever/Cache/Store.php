<?php namespace Dever\Cache;

interface Store
{
	public function __construct();
	public function connect($config);
	public function get($key);
	public function set($key, $value, $expire);
	public function delete($key);
	public function close();
}