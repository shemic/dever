<?php namespace Dever\Routing;

use Dever;
use Dever\Data\Model\Opt;
use Dever\Loader\Config;
use Dever\Loader\Import;
use Dever\Loader\Project;
use Dever\Output\Debug;
use Dever\Output\Export;
use Dever\Template\View;
use Dever\Http\Url;
use Dever\Cache\Handle as Cache;
use Dever\Data\Model as Db;

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

        if ($uri == 'setup') {
            Export::out(Dever::setup());
            die;
        }

        $state = self::def($uri);

        if (!$state && !self::api($uri)) {
            $file = Uri::file();
            if (isset(Config::get('template')->relation[$file])) {
                $file = array($file, Config::get('template')->relation[$file]);
            }
            $this->content = View::getInstance($file)->runing();
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

            $uri = DEVER_APP_NAME . '/' . $uri;
            if (!Input::$command) {
                $uri .= '_api';
            }

            $this->content = Import::load($uri);
            
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

        if (Debug::init()) {
            Debug::out();
        } elseif ($this->html || Config::get('template')->view) {
            echo Url::https(Url::uploadRes($this->content));
        } else {
            Export::out($this->content);
        }
    }

    public function close()
    {
        Cache::closeAll();
        Db::closeAll();
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
}
