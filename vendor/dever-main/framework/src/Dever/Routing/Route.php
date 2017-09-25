<?php namespace Dever\Routing;

use Dever\Data\Model\Opt;
use Dever\Loader\Config;
use Dever\Loader\Import;
use Dever\Loader\Project;
use Dever\Output\Debug;
use Dever\Output\Export;
use Dever\Template\View;

class Route
{
    /**
     * html
     *
     * @var bool
     */
    protected $html = false;

    /**
     * runing
     *
     * @return Dever\Routing\Route
     */
    public function __construct()
    {
        if (Config::get('base')->clearHeaderCache) {
            self::clearHeaderCache();
        }
    }

    /**
     * runing
     *
     * @return Dever\Routing\Route
     */
    public function runing()
    {
        $uri = Uri::get();

        $state = self::def($uri);

        if (!$state && !self::api($uri)) {
            $this->content = View::getInstance(Uri::file())->runing();
            $this->html = true;
        }

        return $this;
    }

    /**
     * def
     * @param string $uri
     *
     * @return bool
     */
    private function def($uri)
    {
        if ($uri == 'tcp.deamon') {
            \Dever\Server\Swoole::daemon();
            return true;
        } elseif ($uri == 'rpc.server') {
            \Dever\Server\Rpc::init();
            return true;
        }

        return false;
    }

    /**
     * api
     * @param string $uri
     *
     * @var bool
     */
    public function api($uri)
    {
        if (strpos($uri, '.') !== false) {
            $this->content = Import::load(DEVER_APP_NAME . '/' . $uri . '_api');

            $this->html = false;

            return true;
        }

        return false;
    }

    public function output()
    {
        if (!isset($this->content)) {
            return;
        }

        if (!$this->content) {
            Export::alert('error_page');
        }

        if (Project::load('manage')) {
            if (Config::get('base')->cron && DEVER_TIME % 2 == 0) {
                Import::load('manage/project.cron');
            }

            if (Config::get('database')->opt) {
                Opt::record();
            }
        }

        $this->debug();
    }

    private function debug()
    {
        Debug::overtime();

        if ($this->https() && $this->content) {
            $this->content = str_replace('http://', 'https://', $this->content);
        }

        if (Debug::init()) {
            Debug::out();
        } elseif ($this->html || Config::get('template')->view) {
            echo $this->content;
        } else {
            Export::out($this->content);
        }
        die;
    }

    /**
     * clearHeaderCache
     */
    public static function clearHeaderCache()
    {
        header("Expires: -1");
        header("Cache-Control: no-store, private, post-check=0, pre-check=0, max-age=0", false);
        header("Pragma: no-cache");
    }

    /**
     * https
     *
     * @return bool
     */
    private function https()
    {
        $state = false;
        if (isset($_SERVER['HTTP_X_REQUEST_PROTOCOL']) && $_SERVER['HTTP_X_REQUEST_PROTOCOL'] == 'https') {
            $state = true;
        }
        return $state;
    }
}
