<?php namespace Dever\Support;

use Dever\Loader\Import;
use Dever\Loader\Project;
use Dever\Loader\Config;

class Command
{
    /**
     * 运维程序
     *
     * @return string
     */
    public static function daemon($interface, $project = false, $file = 'index.php', $state = false)
    {
        if (strpos($interface, 'http://') !== false) {
            return self::run('curl "' . $interface . '"');
        }
        $project = $project ? $project : DEVER_APP_NAME;
        $project = Project::load($project);
        $path = isset($project['setup']) ? $project['setup'] : $project['path'];
        if (isset($project['entry'])) {
            $file = $project['entry'];
        }

        if ($project) {
            if (strpos($path, 'http://') !== false) {
                //self::curl($path . $interface);
                return self::run('curl "' . $path . $interface . '"');
            } else {
                # ?和&无法解析
                $interface = str_replace(array('?', '&'), array('__', '^'), $interface);
                return self::run('php ' . $path . $file . ' -send ' . $interface, $state);
            }
        }
        return false;
    }

    /**
     * 加入到cron中
     *
     * @return string
     */
    public static function cron($name, $ldate, $interface, $time = 0, $project = false, $update = true)
    {
        if ($ldate > 0) {
            $ldate = date('Y-m-d H:i:s', $ldate);
        }
        $info = Import::load('manage/cron-getOne', array('where_project' => $project, 'where_interface' => $interface));
        if ($info) {
            if ($update) {
                $update = array();
                $update['set_name'] = $name;
                $update['set_ldate'] = $ldate;
                $update['set_interface'] = $interface;
                $update['set_time'] = $time;
                $update['set_project'] = $project;
                $update['set_state'] = 1;
                $update['where_id'] = $info['id'];

                Import::load('manage/cron-update', $update);
            }
        } else {
            $update = array();
            $update['add_name'] = $name;
            $update['add_ldate'] = $ldate;
            $update['add_interface'] = $interface;
            $update['add_time'] = $time;
            $update['add_project'] = $project;

            Import::load('manage/cron-insert', $update);
        }
    }

    /**
     * kill
     *
     * @return string
     */
    public static function kill($command)
    {
        $shell = "ps -ef | grep " . $command . " | grep -v grep | awk '{print $2}' | xargs kill -9";

        self::run($shell, '');
    }

    /**
     * run
     *
     * @return string
     */
    public static function run($shell, $state = false, $method = 'system')
    {
        if ($state === false) {
            $state = ' > /dev/null &';
        }

        //$shell = self::shell($shell);
        $method($shell . $state, $result);
        if (is_file($shell)) {
            unlink($shell);
        }

        return $result;
    }

    /**
     * process
     *
     * @return string
     */
    private static function shell($shell)
    {
        $path = Config::data() . 'shell/';
        $path = Path::get($path);
        $name = md5($shell);
        $file = $path . $name;
        $shell = '#!/bin/sh' . $shell;
        file_put_contents($file, $shell);
        system('chmod +x ' . $file);
        return $file;
    }

    /**
     * process
     *
     * @return string
     */
    public static function process($command, $count = false)
    {
        $shell = "ps -ef | grep " . $command . " | grep -v grep";

        if ($count) {
            $shell .= ' | grep "master" | wc -l';
        }

        $result = self::run($shell, '', 'system');

        if ($result != 1) {
            return $result;
        }

        return false;
    }

    /**
     * log
     *
     * @return string
     */
    public static function log($api, $param = array(), $type = '', $project = 'manage')
    {
        $server = $_SERVER['DEVER_SERVER'];

        //self::daemon($api . '?dever_server=' . $server . '&type='.$type.'&param=' . base64_encode(json_encode($param)), $project);

        Import::load($project . '/' . $api, $param);
    }
}
