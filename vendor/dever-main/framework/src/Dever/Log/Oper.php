<?php namespace Dever\Log;

use Dever\Loader\Config;

class Oper
{
    # 1为debug 2为notice 3为info
    public static function add($msg, $type = 1)
    {
        $log = '';
        if (is_array($msg)) {
            foreach($msg as $k => $v)
            {
                $log .= "&".$k."=" . self::filter($v);
            }
        } else {
            $log = $msg;
        }

        self::push($log, $type);
        
    }

    private static function push($log, $type = 1)
    {
        if (is_array(Config::get('debug')->log) && isset(Config::get('debug')->log['type']) && Config::get('debug')->log['type'] == 'udp') {
            $method = Config::get('debug')->log['type'];
            $class = '\\Dever\\Server\\' . ucfirst($method);
            $class = new $class;
            $class->client(Config::get('debug')->log['host'], Config::get('debug')->log['port']);
            $class->push($log);
        } else {
            if ($type == 1) {
                $type = LOG_DEBUG;
                $name = LOG_LOCAL1;
            } elseif ($type == 2) {
                $type = LOG_NOTICE;
                $name = LOG_LOCAL2;
            } elseif ($type == 3) {
                $type = LOG_INFO;
                $name = LOG_LOCAL3;
            }

            openlog(DEVER_APP_NAME, LOG_PID, $name);
            $state = syslog($type, $log);
            closelog();
        }
    }

    public static function filter($string)
    {
        return str_replace(array("\t","\n","\r"),array(",",",",","),$string);
    }
}
