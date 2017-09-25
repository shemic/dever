<?php namespace Dever\Server;

use Dever\Support\Path;
use Dever\Support\Command;
use Dever\Loader\Import;
use Dever\Output\Export;
use Dever\Routing\Input;
use Dever\Loader\Config;
use Dever\Output\Debug;
use swoole_server as Server;
use swoole_client as Client;

class Swoole
{
    protected $log = '/var/log/dever/swoole_';
    protected $client = array();
    protected $callback;
    protected $ip;
    protected $port;
    static protected $instance;

    /**
     * load file
     * @param string $file
     * @param string $path
     * 
     * @return \Dever\Template\View
     */
    static public function getInstance($port = 30000, $ip = '0.0.0.0')
    {
        $key = $ip . ':' . $port;
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self($port, $ip);
        }

        return self::$instance[$key];
    }

    /**
     * __construct
     * @param int $this->port
     * @param string $this->ip
     *
     * @return mixed
     */
    public function __construct($port = 30000, $ip = '0.0.0.0')
    {
        $this->setPort($port);
        $this->setIp($ip);
    }

    /**
     * set ip
     * @param string $this->ip
     *
     * @return mixed
     */
    private function setIp($ip)
    {
        $this->ip = $ip;
    }

    /**
     * set port
     * @param int $this->port
     *
     * @return mixed
     */
    private function setPort($port)
    {
        $this->port = $port;
    }

    /**
     * create path
     *
     * @return mixed
     */
    private function path()
    {
        $path = DEVER_APP_PATH;

        return Path::get($path . 'process/');
    }

    /**
     * reload server
     * @param string $config
     *
     * @return mixed
     */
    public function reload($config = false, $callback = array())
    {
        $content_server = '';
        if ($config) {
            $_SERVER['DEVER_SERVER'] = $config;
            $content_server = "\$_SERVER['DEVER_SERVER'] = '".$config."';";
        }
        if ($this->ip && $this->port) {
            $server = $this->port;
            $entry = defined('DEVER_ENTRY') ? DEVER_ENTRY : 'index.php';
            $content = "<?php

            define('DEVER_DAEMON', true);".$content_server."

            include(dirname(__FILE__) . DIRECTORY_SEPARATOR . '../".$entry."');

            \$ip = '".$this->ip."';

            \$port = '".$this->port."';

            \$callback = ".var_export($callback, true).";

            Dever::tcp(\$port, \$ip)->server(\$callback);";
                        
            $file = $this->path() . $server . '.php';
            
            file_put_contents($file, $content);
            
            Command::kill($file);
                
            return Command::run('php ' . $file, '');
        }

        return false;
    }

    /**
     * daemon
     *
     * @return mixed
     */
    public function daemon()
    {
        /*
        $cron = 'count=`ps -fe |grep "tcp.start" | grep -v "grep" | grep "master" | wc -l`

        echo $count
        if [ $count -lt 1 ]; then
        ps -eaf |grep "tcp.start" | grep -v "grep"| awk \'{print $2}\'|xargs kill -9
        sleep 2
        ulimit -c unlimited
        php /data/webroot/server.php
        echo "restart";
        echo $(date +%Y-%m-%d_%H:%M:%S) >'.self::$log.'restart.log
        fi';
        */

        $process = $this->path();
        $path = scandir($process);
        foreach ($path as $k => $v) {
            if (strpos($v, '.php')) {
                $v = $process . $v;
                $state = Command::process($v);
                if (!$state) {
                    Command::run('php ' . $v);
                }
            }
        }
    }

    /**
     * start server
     *
     * @return mixed
     */
    public function server($callback = false)
    {
        $server = new Server($this->ip, $this->port);
        $config = array
        (
            'worker_num' => 8,
            'daemonize' => true,
            'max_request' => 0,
            'dispatch_mode' => 2,
            'log_file' => $this->log . 'run.log',

            # 启用task
            //'task_worker_num' => 1,
            //'task_max_request' => 0,

            # 确定数据完整性
            //'open_length_check' => true,
            //'open_eof_check' => true,
            //'open_eof_split' => true,
            //'package_eof' => '"eof":1}',
            
            # 确定心跳,确定死链接 ,每30秒检测一次心跳,如果60秒内没有数据发送,则切断链接
            //'open_tcp_keepalive' => true,
            //'heartbeat_check_interval' => 30,
            //'heartbeat_idle_time' => 60,

            # debug开启
            //'debug_mode'=> 1
        );
        if (Config::get('host')->apiServer) {
            $config = array_merge($config, Config::get('host')->apiServer);
        }

        $this->callback = $callback;
        if ($this->callback) {
            $this->callback['ip'] = $this->ip;
            $this->callback['port'] = $this->port;
        }

        $server->set($config);

        $server->on('start', array($this, 'server_start'));
        $server->on('connect', array($this, 'server_connect'));
        $server->on('receive', array($this, 'server_receive'));
        $server->on('close', array($this, 'server_close'));
        //$server->on('task', array($this, 'server_task'));
        //$server->on('finish', array($this, 'server_finish'));
        $server->on('shutdown', array($this, 'server_shutdown'));

        $server->start();
    }

    public function server_start($server)
    {
        //echo "Service:Start...";
        if (isset($this->callback['start'])) {
            Import::load($this->callback['start'], array($server, $this->callback));
        }
    }

    public function server_shutdown($server)
    {
        if (isset($this->callback['shutdown'])) {
            Import::load($this->callback['shutdown'], array($server, $this->callback));
        }
        //echo "Service:Shutdown...";
    }

    public function server_connect($server, $fd)
    {
        if (isset($this->callback['connect'])) {
            Import::load($this->callback['connect'], array($server, $fd, $this->callback));
        }
        //echo "Client:Connect.\n";
    }

    public function server_receive($server, $fd, $from_id, $data)
    {
        $data = json_decode($data, true);
        # 直接转发给客户端
        if ($data['method'] == 'send' && isset($data['param']['tcp_id']) && isset($data['param']['send'])) {
            if (is_array($data['param']['send'])) {
                $data['param']['send']['tcp_id'] = $data['param']['tcp_id'];
                $data['param']['send'] = json_encode($data['param']['send']);
            }
            $server->send($data['param']['tcp_id'], $data['param']['send']);
            unset($data['param']['tcp_id']);
        } else {
            if (isset($this->callback['receive'])) {
                $state = Import::load($this->callback['receive'], array($server, $fd, $data, $this->callback));

                if (!$state) {
                    $server->close($fd);
                    return;
                }
            }
            if (isset($data['param']['token'])) {
                $data['param']['tcp_id'] = $fd;
            }

            if (isset(Config::get('host')->apiServer['backend'])) {
                list($project, $interface) = explode('/', $data['method']);
                $send = Command::daemon($interface . '?' . http_build_query($data['param']), $project, 'index.php');

                $send = $send[0];
            } else {
                Input::set('pg', 0);
                Input::set('pt', 0);
            	$send = Import::load($data['method'], $data['param']);
                if (is_array($send)) {
                    $page = Export::page('current', false, false);
                    if ($page) {
                        $send = array('data' => $send, 'page' => $page);
                    }
                    //$send['eof'] = 1;
                    $send = json_encode($send);
                }
            }

            $server->send($fd, $send);
        }

        if (isset(Config::get('host')->apiServer['pconnect'])) {
            $data['param']['tcp_id'] = 1;
        }
        
        if (empty($data['param']['tcp_id'])) {
            $server->close($fd);
        }
    }

    public function server_close($server, $fd)
    {
        if (isset($this->callback['close'])) {
            Import::load($this->callback['close'], array($server, $fd, $this->callback));
        }
        //echo "Client: Close.\n";
    }

    private function client($time = 0, $callback = false)
    {
    	$time = $time > 0 ? $time : 500;
        if (!$callback) {
            $client = new Client(SWOOLE_SOCK_TCP, SWOOLE_SOCK_SYNC);
            $state = $client->connect($this->ip, $this->port, $time);
            if (!$state) {
                return false;
            }
        } else {
        	$this->callback = $callback;
            if ($this->callback) {
                $this->callback['ip'] = $this->ip;
                $this->callback['port'] = $this->port;
            }
            $client = new Client(SWOOLE_SOCK_TCP, SWOOLE_SOCK_ASYNC);
            $client->on('connect', array($this, 'client_connect'));
            $client->on('receive', array($this, 'client_receive'));
            $client->on('error', array($this, 'client_error'));
            $client->on('close', array($this, 'client_close'));
            $client->connect($this->ip, $this->port, $time);
        }

        return $client;
    }

    public function client_connect($client)
    {
        if (isset($this->callback['connect'])) {
            Import::load($this->callback['connect'], array($client, $this->callback));
        }
    }

    public function client_receive($client, $data)
    {
        if (isset($this->callback['receive'])) {
            return Import::load($this->callback['receive'], array($client, $data, $this->callback));
        }
        return false;
    }

    public function client_error($client)
    {
        if (isset($this->callback['error'])) {
            Import::load($this->callback['error'], array($client, $this->callback));
        }
    }

    public function client_close($client)
    {
        if (isset($this->callback['close'])) {
            Import::load($this->callback['close'], array($client, $this->callback));
        }
    }

    public function api($method, $param = array(), $time = 0, $size = 65535, $flag = true)
    {
        $client = $this->client($time);

        if (!$client) {
            return false;
        }

        $send = array('method' => $method, 'param' => $param);
        $client->send(json_encode($send));
        $data = $client->recv($size, $flag);
        if ($data && is_string($data) && strpos('{', $data) !== false) {
            $data = json_decode($data, true);
        }
        
        $client->close();
        
        if (isset($data['page']) && $data['page']) {
            Export::page('current', $data['page']);
            return $data['data'];
        }

        return $data;
    }
}