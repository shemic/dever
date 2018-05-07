<?php namespace Dever\Log;

use Dever;
use Dever\Loader\Config;
use Dever\Support\Path;

class Oper
{
    # 1为debug 2为notice 3为info
    public static function add($msg, $type = 1)
    {
        if (!Config::get('debug')->log) {
            return;
        }
        $log = '';
        if (is_array($msg)) {
            foreach ($msg as $k => $v) {
                if (is_array($v)) {
                    foreach ($v as $k1 => $v1) {
                        $log .= "&".$k . "_" . $k1 ."=" . self::filter($v1);
                    }
                } else {
                    $log .= "&".$k."=" . self::filter($v);
                }
            }
        } else {
            $log = $msg;
        }

        return self::push($log, $type);
    }

    private static function push($log, $type = 1)
    {
        if (is_array(Config::get('debug')->log)) {
            $method = Config::get('debug')->log['type'];
        } else {
            $method = 'syslog';
        }
        $method = 'push_' . $method;
        return self::$method($log, $type);
    }

    private static function push_http($log, $type)
    {
        return false;
    }

    private static function push_udp($log, $type)
    {
        $class = '\\Dever\\Server\\Udp';
        $class = new $class;
        $class->client(Config::get('debug')->log['host'], Config::get('debug')->log['port']);
        $class->push($log);
    }

    private static function push_syslog($log, $type)
    {
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
        return $state;
    }

    private static function push_file($log, $type)
    {
        $size = isset(Config::get('debug')->log['size']) ? Config::get('debug')->log['size'] : 5242880;//默认5M
        $date = explode('-', date("Y-m-d"));
        $path = Path::get(Config::data() . 'logs' . DIRECTORY_SEPARATOR , DEVER_PROJECT . DIRECTORY_SEPARATOR . $date[0] . DIRECTORY_SEPARATOR . $date[1] . DIRECTORY_SEPARATOR);
        $now = Dever::udate('Y-m-d'.'\T'.'H:i:s.u+08:00');
        $project = DEVER_PROJECT;
        $app = DEVER_APP_NAME;
        $log = $now . ' ' . $project . ' ' . $app . ' ' . $log . "\r\n";
        $file = date('Y_m_d');
        if ($type == 1) {
            $file = 'debug_' . $file . '.log';
        } elseif ($type == 2) {
            $file = 'notice_' . $file . '.log';
        } elseif ($type == 3) {
            $file = 'info_' . $file . '.log';
        }
        $file = $path . $file;

        if (file_exists($file) && $size <= filesize($file)) {
            rename($file, $file . '_bak');
        }
        
        return error_log($log, 3, $file);
    }

    public static function filter($string)
    {
        return str_replace(array("\t","\n","\r"),array(",",",",","),$string);
    }
}
