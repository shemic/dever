<?php namespace Dever\Server;

class Udp
{
    public function push($msg)
    {
        if (is_array($msg)) {
            $msg = json_encode($msg);
        }
        $this->send($msg);
        $this->close();
        return true;
    }

    public function start($port, $callback)
    {
        $this->bind($port);
        $this->recv();
    }

    public function client($host, $port)
    {
        $this->config($host, $port);
        $this->create();
    }

    public function server($host, $port)
    {
        ob_implicit_flush();
        $this->config($host, $port);
        $this->create();
    }

    private function recv()
    {
        while (true) {
            $from = "";
            $port = 0;
            socket_recvfrom($this->handle, $data, 1024, 0, $from, $port);
            echo $data;
            usleep(1000);
        }
    }

    private function create()
    {
        $this->handle = socket_create(AF_INET, SOCK_DGRAM, SOL_UDP);
        if ($this->handle === false) {
            echo "socket_create() failed:reason:" . socket_strerror(socket_last_error()) . "\n";
            die;
        }
    }

    private function bind($port)
    {
        $this->config($port);
        $ok = socket_bind($this->handle, $this->host, $this->port);
        if ($ok === false) {
            echo "socket_bind() failed:reason:" . socket_strerror( socket_last_error($this->handle));
            die;
        }
    }

    private function send($msg)
    {
        $this->config();
        $len = strlen($msg);
        $input = Input::getInput('debug');
        if ($input) {
            echo $msg;
        }
        try {
            socket_sendto($this->handle, $msg, $len, 0, $this->host, $this->port);
        } catch(Exception $e) {
            echo 'Udp Error: ' .$e->getMessage();
        }
    }

    private function close()
    {
        socket_close($this->handle);
    }

    private function config($host, $port)
    {
        $this->host = $host;
        $this->port = $port;
    }
}
