<?php namespace Dever\Log;

use Dever;
use Dever\Loader\Config;
use Dever\Support\Path;

class Oper
{
    # 1为debug 2为notice 3为info 其他是自定义，只有当配置不为syslog时自定义才生效
    public static function add($msg, $type = 1)
    {
        if (!Config::get('debug')->log) {
            return;
        }
        $log = 'dever';
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
        return $class->push($log);
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
        } else {
            $type = LOG_INFO;
            $name = LOG_LOCAL4;
        }

        openlog(DEVER_APP_NAME, LOG_PID, $name);
        $state = syslog($type, $log);
        closelog();
        return $state;
    }

    private static function push_file($log, $type)
    {
        $now = Dever::udate('Y-m-d'.'\T'.'H:i:s.u+08:00');
        $project = DEVER_PROJECT;
        $app = DEVER_APP_NAME;
        $log = $now . ' ' . $project . ' ' . $app . ' ' . $log . "\r\n";
        $file = self::getFileName($type);
        $file = Dever::path($file[0], $file[1] . $file[2]);

        $size = isset(Config::get('debug')->log['size']) ? Config::get('debug')->log['size'] : 5242880;//默认5M

        if (file_exists($file) && $size <= filesize($file)) {
            rename($file, $file . '.' . date('H_i_s') . '.bak');
            @chmod($file, 0755);
            //@system('chmod -R 777 ' . $file);
        }
        
        $state = error_log($log, 3, $file);
        return $state;
    }

    public static function get($day, $type = 1)
    {
        if (is_array(Config::get('debug')->log)) {
            $method = Config::get('debug')->log['type'];
        } else {
            $method = 'syslog';
        }
        $method = 'get_' . $method;
        return self::$method($day, $type);
    }

    private static function get_http($day, $type)
    {
        return false;
    }

    private static function get_udp($day, $type)
    {
        return false;
    }

    private static function get_syslog($day, $type)
    {
        return false;
    }

    private static function get_file($day, $type)
    {
        $file = self::getFileName($type, $day);
        $content = '';
        $path = $file[0] . $file[1];
        if (is_dir($path)) {
            $dir = scandir($path);
            foreach ($dir as $k => $v) {
                if (strstr($v, $file[2])) {
                    $content .= file_get_contents($path . $v);
                }
            }
        }
        if ($content) {
            return explode("\n", $content); 
        }
        return array();    
    }

    public static function filter($string)
    {
        if (is_array($string)) {
            $string = json_encode($string);
        }
        return str_replace(array("\t","\n","\r"),array(",",",",","),$string);
    }

    private static function getFileName($type, $day = '')
    {
        $file = $day ? $day : date('Y_m_d_H');
        if (strstr($file, '-')) {
            $file = str_replace('-', '_', $file);
        }
        $root = Path::day('logs', true, $file);

        if ($type == 1) {
            $prefix = 'debug';
        } elseif ($type == 2) {
            $prefix = 'notice';
        } elseif ($type == 3) {
            $prefix = 'info';
        } else {
            $prefix = $type;
        }

        if (strstr($type, '/')) {
            $path = rtrim($prefix, '/') . '/';
            $file = $file . '.log';
        } else {
            $path = '';
            $file = $prefix . '_' . $file . '.log';
        }
        return array($root, $path, $file);
    }
}
