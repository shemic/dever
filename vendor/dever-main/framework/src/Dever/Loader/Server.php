<?php namespace Dever\Loader;

use Dever;
use Dever\Routing\Input;
use Dever\Http\Curl;
use Dever\Output\Debug;

class Server
{
    /**
     * url
     *
     * @var string
     */
    protected $url;

    /**
     * param
     *
     * @var array
     */
    protected $param;

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
     * @return \Dever\Loader\Server
     */
    public static function get($url, $key, $param)
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
        }

        return self::$instance->load($url, $key, $param);
    }

    /**
     * loadServer
     *
     * @return mixed
     */
    protected function load($url, $key, $param)
    {
        $this->url = $url;
        $this->key = $key;
        $this->param($param);

        $server = 'curl';
        if (Config::get('host')->server && isset(Config::get('host')->server['type'])) {
            $server = Config::get('host')->server['type'];
        }
        return $this->data($server);
    }

    /**
     * tcp
     *
     * @return mixed
     */
    private function data($server)
    {
        if ($this->url && $server == 'tcp' && class_exists('\Swoole\Server')) {
            $data = $this->tcp();
        } elseif ($this->url && $server == 'rpc' && class_exists('\Yar_Server')) {
            $data = $this->rpc();
        } else {
            $data = $this->curl();
        }
        $this->log($data);
        return $this->result($data);
    }

    /**
     * tcp
     *
     * @return mixed
     */
    private function tcp()
    {
        $config = parse_url($this->url);
        $config['port'] = Config::get('host')->server['port'];
        $config['host'] = Config::get('host')->server['host'] ? Config::get('host')->server['host'] : $config['host'];
        $class = \Dever\Server\Swoole::getInstance($config['port'], $config['host']);
        $data = $class->api($this->key, $this->param);
        if (!$data) {
            $data = $this->curl($this->url);
        } else {
            $this->url = $config['host'] . ':' . $config['port'] . '--' . $method;
        }
        
        return $data;
    }

    /**
     * rpc
     *
     * @return mixed
     */
    private function rpc()
    {
        $this->url .= 'rpc.server';
        $data = \Dever\Server\Rpc::api($this->url, $this->key, $this->param);
        $this->url .= '--' . $this->key;
        return $data;
    }

    /**
     * curl
     *
     * @return mixed
     */
    private function curl()
    {
        $this->url .= $this->key;
        $data = Curl::get($this->url, $this->param);
        return $data;
    }

    /**
     * loadServerData
     *
     * @return mixed
     */
    private function result($data)
    {
        if (!is_array($data)) {
            $json = json_decode($data, true);
            if ($json && is_array($json)) {
                $data = $json;
                if (isset($data['page']) && $data['page']) {
                    if (empty($data['page']['current'])) {
                        Dever::$global['page']['current'] = $data['page'];
                    } else {
                        Dever::$global['page'] = $data['page'];
                    }
                }

                if (isset($data['data']) && $data['data']) {
                    $data = $data['data'];
                } elseif(isset($data['code']) || (isset($data['status']) && $data['status'] == 2)) {
                    $data = false;
                }
            }
        }

        return $data;
    }

     /**
     * param
     *
     * @return mixed
     */
    private function param($param)
    {
        $this->param = $param;
        if ($page = Input::get('pg')) {
            $this->param['pg'] = $page;
        }
        if ($total = Input::get('pt')) {
            $this->param['pt'] = $total;
        }
        $this->param['json'] = 1;
        $this->param['cache'] = 1;
    }

    /**
     * log
     *
     * @return log
     */
    protected function log($data = array())
    {
        Debug::log(array('url' => $this->url, 'param' => $this->param, 'data' => $data), 'server');
    }
}
