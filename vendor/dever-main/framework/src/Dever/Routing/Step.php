<?php namespace Dever\Routing;

use Dever\Config\Load as Config;
use Dever\Http\Input;
use Dever\Http\Output;
use Dever\Security\Api;
use Dever\Session\Save;

class Step
{
    protected static $step;
    protected static $key;
    protected static $method;
    protected static $save;
    protected static $check = 'step_signature';
    protected static $time = 500;

    public static function init($method, $step, $key, $class)
    {
        self::$save = new Save();
        self::$step = $step;
        self::$key = $key;
        self::$method = $method . $key;
        if (self::$step == 1) {
            self::create();
        } else {
            self::check();
            self::create();
        }
    }

    private static function create()
    {
        $input = Input::prefix('step_');
        $param = $input + array('method' => self::$method . (self::$step + 1));

        Config::$global['step'][self::$step] = Api::get($param);
        self::$save->add(self::$check, Config::$global['step'][self::$step]['signature'], self::$time);
    }

    private static function check()
    {
        $input = Input::prefix('step_');
        $param = $input + array('method' => self::$method . self::$step);

        $signature = Api::result($param);

        $save = self::$save->get(self::$check);

        if ($signature != $save) {
            Output::abert('api_signature_exists');die;
        } else {
            self::$save->un(self::$check);
        }
    }

    public static function set($value)
    {
        self::$save->add('data_' . (self::$step + 1), $value, self::$time);
    }

    public static function get()
    {
        return self::$save->get('data_' . self::$step);
    }
}
