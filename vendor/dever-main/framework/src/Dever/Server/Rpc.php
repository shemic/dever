<?php namespace Dever\Server;

use Dever\Loader\Import;
use Dever\Output\Export;

class Rpc_Api
{

    public function api($method, $param = array())
    {
    	$data = Import::load($method, $param);
    	$page = Export::page();
    	if ($page) {
    		return array('data' => $data, 'page' => $page);
    	}
        return $data;
    }

    protected function client_can_not_see()
    {

    }
}

class Rpc
{
	static $client = array();
	static public function init()
	{
		self::server();
		die;
	}

	static public function server()
	{
		$service = new \Yar_Server(new Rpc_Api());
		$service->handle();
	}

	static public function client($link)
	{
		if (empty(self::$client[$link])) {
			ini_set("yar.timeout",60000);
			self::$client[$link] = new \Yar_Client($link);
		}
		
		return self::$client[$link];
	}

	static public function api($link, $method, $param = array())
	{
		self::client($link);

		$data = self::$client[$link]->api($method, $param);

		if (isset($data['page']) && $data['page']) {
			Export::page('current', $data['page']);
			return $data['data'];
		}

		return $data;
	}

	static public function loop($link, $method, $param = array())
	{
		Yar_Concurrent_Client::call($link, $method, $param, "callback");
		Yar_Concurrent_Client::call($link, $method, $param, "callback");
		Yar_Concurrent_Client::call($link, $method, $param, "callback");
		Yar_Concurrent_Client::loop(); //send

		return $data;
	}
}