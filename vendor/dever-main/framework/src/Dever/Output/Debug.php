<?php namespace Dever\Output;

use Dever\Loader\Config;
use Dever\Routing\Input;
use Dever\String\Helper;
use Dever\Output\Exceptions;

class Debug
{
    /**
     * data
     *
     * @var array
     */
    private static $data;

    /**
     * trace
     *
     * @var array
     */
    private static $trace;

    /**
     * tool
     *
     * @var array
     */
    private static $tool;

    /**
     * time
     *
     * @var int
     */
    private static $time;

    /**
     * memory
     *
     * @var int
     */
    private static $memory;

    /**
     * init
     */
    public static function init()
    {
        if (Config::get('debug')->request) {
            $request = Input::shell(Config::get('debug')->shell);
            if (is_string(Config::get('debug')->request)
                && strpos(Config::get('debug')->request, Input::ip()) !== false
                && $request) {
                return true;
            } elseif (Config::get('debug')->request === 2 || $request) {
                return true;
            }
        }
        return false;
    }

    /**
     * report
     */
    public static function report()
    {
        ini_set('display_errors', true);
        self::tool();
    }

    /**
     * tool
     */
    private static function tool()
    {
        if (self::$tool) {
            return true;
        }
        if (class_exists('\\Whoops\\Run')) {
            self::$tool = new \Whoops\Run;
            self::$tool->pushHandler(new \Whoops\Handler\PrettyPageHandler);
            if (\Whoops\Util\Misc::isAjaxRequest() || (isset($_SERVER['REQUEST_METHOD']) && $_SERVER['REQUEST_METHOD'] == 'POST')) {
                self::$tool->pushHandler(new \Whoops\Handler\JsonResponseHandler);
            }

            self::$tool->register();
            return true;
        }
        return false;
    }

    /**
     * overtime
     */
    public static function overtime()
    {
        $time = Config::get('debug')->overtime;
        if ($time && self::time() > $time) {
            self::error('Execution time timeout');
        }
    }

    /**
     * log
     * @param string $msg
     * @param string $type
     *
     * @return mixed
     */
    public static function log($msg, $type = 'log')
    {
        if (self::init()) {
            $format = array('msg' => $msg);
            $format = array_merge($format, self::env());
            $msg = Export::format($format);

            self::add($type, $msg);
        }
    }

    /**
     * env
     *
     * @return array
     */
    private static function env()
    {
        $trace = self::trace();
        return array
        (
            'time' => 'current:' . self::time(2) . ' total:' . self::time(),
            'memory' => 'current:' . self::memory(2) . ' total:' . self::memory(),
            'trace' => $trace
        );
    }

    /**
     * error
     */
    public static function error($msg)
    {
        if (Config::get('debug')->log) {
            $data = self::env();
            $data['msg'] = $msg;
            \Dever\Log\Oper::add($msg);
        }
    }

    /**
     * runtime
     */
    public static function runtime()
    {
        if (self::init()) {
            self::add('runtime', Export::format(self::loadfile()), 'Load Files');

            $msg = Export::format(array('time' => self::time(), 'memory' => self::memory()));

            self::add('runtime', $msg, 'Total');
        }
    }

    /**
     * trace
     */
    public static function trace($trace = false)
    {
        $debug = $trace ? $trace : debug_backtrace();
        $trace = array();
        if ($debug) {
            foreach($debug as $k => $v) {
                if (self::checkFile($v)) {
                    $trace = $v['file'] . ':' . $v['line'];
                    self::$trace[$trace] = $v;
                    break;
                }
            }
        }

        return $trace;
    }

    /**
     * checkFile
     */
    private static function checkFile($value)
    {
        if (isset($value['file']) && strpos($value['file'], DEVER_APP_PATH) !== false && isset($value['function']) && $value['function'] != '__callStatic') {
            $config = array('data', 'template', 'assets', 'config', 'daemon', 'route', 'database', 'api', DEVER_ENTRY);
            foreach ($config as $k => $v) {
                if (strpos($value['file'], DEVER_APP_PATH . $v) !== false) {
                    return false;
                }
            }
            return true;
        }
        return false;
    }

    /**
     * time
     */
    public static function time($state = 1)
    {
        $start = self::startTime($state);

        $end = self::endTime();

        return '[' . ($end - $start) . 'S]';
    }

    /**
     * endTime
     */
    public static function endTime()
    {
        self::$time = microtime();

        return self::createTime(self::$time);
    }

    /**
     * startTime
     */
    public static function startTime($state = 1)
    {
        $start = DEVER_START;
        if ($state == 2 && self::$time) {
            $start = self::$time;
        }

        return self::createTime($start);
    }

    /**
     * createTime
     */
    private static function createTime($time)
    {
        list($a, $b) = explode(' ', $time);
        return ((float) $a + (float) $b);
    }

    /**
     * memory
     */
    private static function memory($state = 1)
    {
        $memory = memory_get_usage();
        if ($state == 2 && self::$memory) {
            $memory = $memory - self::$memory;
        }
        self::$memory = $memory;

        return '[' . ($memory / 1024) . 'KB]';
    }

    /**
     * loadfile
     */
    private static function loadfile()
    {
        $files = get_included_files();
        $result = array();
        $path = DEVER_PATH;
        foreach ($files as $k => $v) {
            if (strpos($v, $path) === false) {
                $result[] = $v;
            }
        }
        return $result;
    }

    /**
     * add
     */
    private static function add($method, $msg, $key = false)
    {
        if ($key) {
            self::$data[$method][$key] = $msg;
        } else {
            self::$data[$method][] = $msg;
        }
    }

    /**
     * sql
     */
    public static function sql($key = '')
    {
        if (Config::get('database')->sql) {
            if (is_numeric($key) && isset(Config::get('database')->sql[$key])) {
                return Config::get('database')->sql[$key];
            }
            
            elseif ($key == 'all') {
                return Config::get('database')->sql;
            }

            $num = count(Config::get('database')->sql)-1;
            return self::sql($num);
        }
    }

    /**
     * getTrace
     */
    public static function getTrace()
    {
        return self::$trace ? array_reverse(array_values(self::$trace)) : array();
    }

    /**
     * wait
     */
    public static function wait($msg = '', $notice = 'Dever Data Debug!')
    {
        if ($msg && self::tool()) {
            self::runtime();
            self::error($msg);
            $handler = self::$tool->getHandlers();
            if (self::$data && is_array(self::$data)) {
                foreach (self::$data as $k => $v) {
                    $handler[0]->addDataTable(ucwords($k), $v);
                }
            }
            if (is_array($msg)) {
                $msg = array('array' => Export::format($msg));
            } else {
                $msg = array(gettype($msg) => $msg);
            }
            $handler[0]->addDataTable('Data', $msg);
            $handler[0]->addDataTable('Env', array('time' => self::time(), 'memory' => self::memory(), 'trace' => self::trace()));
            $handler[0]->setPageTitle($notice);
            throw new Exceptions($notice);
        } else {
            print_r($msg);
        }
        die;
    }

    /**
     * data
     */
    public static function data()
    {
        if (self::init()) {
            self::out();
        }
    }

    /**
     * reflection
     */
    public static function reflection($class, $method)
    {
        if (self::init()) {
            # 用此解决debug trace无法跟踪到信息的问题
            $class = new \ReflectionClass($class);
            //$methods = $class->getMethods();
            $trace['file'] = $class->getFileName();
            $trace['line'] = $class->getStartLine();
            $trace['class'] = $class->getName();
            $trace['function'] = $method;
            $content = explode("\n", file_get_contents($trace['file']));
            foreach ($content as $k => $v) {
                if (strpos($v, 'function ' . $method . '(')) {
                    $trace['line'] = $k+1;
                    break;
                }
            }
            $key = $trace['file'] . ':' . $trace['line'];
            self::$trace[$key] = $trace;
        }
    }

    /**
     * out
     */
    public static function out()
    {
        self::runtime();

        if (self::$data && is_array(self::$data)) {
            if (self::tool()) {
                $handler = self::$tool->getHandlers();
                $title = 'Dever Runtime Debug!';
                foreach (self::$data as $k => $v) {
                    $handler[0]->addDataTable(ucwords($k), $v);
                    $handler[0]->setPageTitle($title);
                }

                /*
                $handler[0]->setEditor('sublime');
                $handler[0]->setEditor(function($file, $line) {
                    return "subl://open?file=$file&line=$line";
                });
                */

                throw new Exceptions($title);
            } else {
                echo self::html();
            }
        }
    }

    /**
     * html
     */
    private static function html($show = 'display:none;')
    {
        self::runtime();

        $html = self::createHtml($show);

        if (self::$data) {
            foreach (self::$data as $k => $v) {

                self::createA($html, $k);

                $html .= '<div style="' . $show . '">';
                $html .= '<table border="1" style="width:100%;">';

                foreach ($v as $i => $j) {
                    $html .= '<tr>';

                    $html .= '<td>' . $j . '</td>';

                    $html .= '</tr>';
                }

                $html .= '</table>';

                $html .= '</div>';
            }
        }

        $html .= '</div>';

        return $html;
    }

    /**
     * html
     */
    private static function createHtml($show)
    {
        $fix = 'fixed';
        if (!$show) {
            $fix = '';
        }
        return '<div style="position:' . $fix . ';z-index:10000;bottom:0;background:white;overflow:auto;width:100%;height:auto;">';
    }

    /**
     * html
     */
    private static function createA(&$html, $k)
    {
        $html .= '<a style="font-size:14px;font-weight:bold;margin-left:5px;" href="javascript:;" onclick="var a = $(this).next();if(a.get(0).style.display == \'none\'){a.show();$(this).parent().height(500)}else if(a.get(0).style.display != \'none\'){a.hide();$(this).parent().height(\'auto\')}">' . $k . '</a>';
    }
}
