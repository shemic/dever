<?php namespace Dever\Output;

use Dever;
use Dever\Loader\Lang;
use Dever\Pagination\Paginator;
use Dever\Routing\Input;

class Export
{
    /**
     * state
     *
     * @var boot
     */
    private static $state = false;

    /**
     * alert
     * @param string $msg
     * @param array $param
     *
     * @return string
     */
    public static function alert($msg, $param = array(), $debug = false)
    {
        $msg = self::result($msg, $param);
        if (Debug::init()) {
            Debug::wait($msg);
        } elseif (self::$state) {
            echo $msg;
        } elseif ($debug) {
            echo self::format($msg);
        } else {
            self::html($msg);
        }
        die;
    }

    /**
     * out
     * @param string $msg
     * @param array $param
     *
     * @return string
     */
    public static function out($msg, $param = array(), $return = false)
    {
        if (is_array($msg)) {
            Input::set('json', Input::get('json', 1));
        }
        $result = self::result($msg, $param, 1);
        if ($return) {
            return $result;
        }
        if (!self::$state) {
            print_r($msg);
        } else {
            print_r($result);
        }
    }

    /**
     * diy
     * @param string $msg
     * @param array $param
     *
     * @return string
     */
    public static function diy($msg, $param = array())
    {
        if (is_array($msg)) {
            Input::set('json', Input::get('json', 1));
        }
        $result = self::result($msg, $param, 1, false);
        if (!self::$state) {
            print_r($msg);
        } else {
            print_r($result);
        }
        die;
    }

    /**
     * debug
     * @param string $msg
     * @param array $param
     *
     * @return string
     */
    public static function debug($msg, $param = array())
    {
        self::alert($msg, $param, true);
    }

    /**
     * format
     * @param string $msg
     *
     * @return string
     */
    public static function format($msg)
    {
        $content = "<pre>\n";
        $content .= htmlspecialchars(print_r($msg, true));
        $content .= "\n</pre>\n";
        return $content;
    }

    /**
     * html
     * @param string $msg
     *
     * @return string
     */
    public static function html($msg)
    {
        $html = new Html;
        $html->out($msg);
    }

    /**
     * result
     * @param string $msg
     *
     * @return string
     */
    public static function result($msg, $param = array(), $status = 2, $state = true)
    {
        if ($state) {
            $result = self::msg($msg, $param, $status);
        } else {
            $result = $msg;
        }

        self::json($result, 2);

        self::callback($result);

        self::func($result);

        return $result;
    }

    /**
     * msg
     * @param string $msg
     *
     * @return string
     */
    public static function msg($msg, $param = array(), $status = 2)
    {
        $result = array();

        $result['status'] = $status;

        self::code($msg, $param, $result);

        if (is_string($msg)) {
            $msg = Lang::get($msg, $param);
        }

        if ($status == 1) {
            $result['msg'] = 'success';
            $result['data'] = $msg;
        } else {
            $result['msg'] = $msg;
        }

        self::success($status, $result);

        return $result;
    }

    /**
     * callback
     * @param string $msg
     *
     * @return string
     */
    private static function json(&$msg, $state = 2)
    {
        $json = Input::get('json', $state);

        if ($json != 2 && is_array($msg)) {
            if (!$msg) {
                $msg = (object) $msg;
            }
            $msg = json_encode($msg);
            self::$state = true;
        }
    }

    /**
     * callback
     * @param string $msg
     *
     * @return string
     */
    private static function callback(&$msg)
    {
        $callback = Input::get('callback');

        if ($callback) {
            self::json($msg, 1);
            $msg = $callback . '(' . $msg . ')';
        }
    }

    /**
     * func
     * @param string $msg
     *
     * @return string
     */
    private static function func(&$msg)
    {
        $function = Input::get('function');

        if ($function) {
            self::json($msg, 1);
            $msg = '<script>parent.' . $function . '(' . $msg . ')' . '</script>';
        }
    }

    /**
     * code
     * @param string $msg
     * @param array $param
     * @param array $result
     *
     * @return mixed
     */
    private static function code($msg, $param, &$result)
    {
        if (is_numeric($msg)) {
            $result['code'] = $msg;
        } elseif (is_numeric($param)) {
            $result['code'] = $param;
            $param = array();
        } elseif (is_array($msg) && isset($msg['code'])) {
            $result['code'] = $msg['code'];
        } else {
            $result['code'] = 1;
        }

        if ($param) {
            $result['param'] = $param;
        }
    }

    /**
     * success
     * @param string $status
     * @param array $result
     *
     * @return mixed
     */
    private static function success($status, &$result)
    {
        if ($status == 1) {
            $result['code'] = 1;
            if ($page = self::page()) {
                $result['page'] = $page;
            }
        }
    }

    /**
     * page
     * @param string $msg
     *
     * @return string
     */
    public static function page($method = 'current')
    {
        if (isset(Dever::$global['page'][$method])) {
            return Dever::$global['page'][$method];
        }
        return Paginator::getInstance($method)->toArray();
    }
}
