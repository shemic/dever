<?php namespace Dever\Data\Model;

use Dever\Support\Command;
use Dever\Output\Debug;

class Opt
{
    protected static $data;

    protected static $instance;

    /**
     * push
     *
     * @return mixd
     */
    public static function push($project, $table, $col)
    {
        if ($col) {
            $key = $project . '.' . $table;
            foreach ($col as $k => $v) {
                self::$data[$key][$v] = $v;
            }

            $time = Debug::time();
            self::$data[$key]['time'] = $time;
        }
    }

    /**
     * record
     *
     * @return mixd
     */
    public static function record()
    {
        if (self::$data) {
            Command::log('auth.opt', self::$data);
        }
    }
}
