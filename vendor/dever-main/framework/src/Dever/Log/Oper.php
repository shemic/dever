<?php namespace Dever\Log;

use Dever\Loader\Config;

class Oper
{
    public static function add($msg)
    {
        return;
        $log = '';
        if (is_array($msg)) {
            foreach($msg as $k => $v)
            {
                $log .= " ^".ucfirst($k).":" . self::filter($v);
            }
        } else {
            $log = $msg;
        }

        self::push($log);
        
    }

    private static function push($log)
    {
        if (is_array(Config::get('debug')->log) && isset(Config::get('debug')->log['type']) && Config::get('debug')->log['type'] == 'udp') {
            $method = Config::get('debug')->log['type'];
            $class = '\\Dever\\Server\\' . ucfirst($method);
            $class = new $class;
            $class->client(Config::get('debug')->log['host'], Config::get('debug')->log['port']);
            $class->push($log);
        } else {
            openlog(DEVER_APP_NAME,LOG_PID,LOG_LOCAL3);
            syslog(LOG_DEBUG,$log);
        }
    }

    public static function filter($string)
    {
        return str_replace(array("\t","\n","\r"),array(",",",",","),$string);
    }
}
