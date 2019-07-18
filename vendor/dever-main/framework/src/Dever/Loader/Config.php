<?php namespace Dever\Loader;

class Config
{
    /**
     * env
     *
     * @var const string
     */
    const ENV = 'env';

    /**
     * data
     *
     * @var array
     */
    private $cData;

    /**
     * key
     *
     * @var string
     */
    private $cKey;

    /**
     * setting
     *
     * @var array
     */
    private static $setting = array('base', 'host', 'database', 'debug', 'cache', 'template');

    /**
     * instance
     *
     * @var object
     */
    protected static $instance;

    /**
     * __set
     */
    public function __set($name, $value)
    {
        $this->cData[$this->cKey][$name] = $value;
    }

    /**
     * set
     */
    public function setArray($name, $key, $value)
    {
        if (!isset($this->cData[$this->cKey][$name])) {
            $this->cData[$this->cKey][$name] = array();
        }
        $this->cData[$this->cKey][$name][$key] = $value;
    }

    /**
     * __get
     */
    public function __get($name)
    {
        if (array_key_exists($name, $this->cData[$this->cKey])) {
            return $this->cData[$this->cKey][$name];
        } elseif ($name == 'cAll') {
            return $this->cData[$this->cKey];
        } else {
            return null;
        }
    }

    /**
     * __isset
     */
    public function __isset($name)
    {
        return isset($this->cData[$this->cKey][$name]);
    }

    /**
     * __unset
     */
    public function __unset($name)
    {
        unset($this->cData[$this->cKey][$name]);
    }

    /**
     * init
     *
     * @return mixed
     */
    public static function init()
    {
        $self = self::get('base');

        if (!defined('DEVER_APP_NAME')) {
            $self->defineAppName($self->path);
        }

        unset($self);
    }

    /**
     * data
     */
    public static function data()
    {
        return isset(self::get('base')->data) ? self::get('base')->data : DEVER_PATH . 'data' . DIRECTORY_SEPARATOR;
    }

    /**
     * get
     * @param string $name
     * @param string $app
     * @param string $path
     *
     * @return mixed
     */
    public static function get($name = 'base', $app = '', $path = 'config')
    {
        if (!$app) {
            $app = DEVER_APP_NAME;
        }
        if (in_array($name, self::$setting)) {
            $key = 'base' . '_' . $app . '_' . $path;
        } else {
            $key = $name . '_' . $app . '_' . $path;
        }
        
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self();
        }

        return self::$instance[$key]->load($name, $app, $path);
    }

    /**
     * load
     * @param string $name
     * @param string $app
     * @param string $path
     *
     * @return mixed
     */
    public function load($name, $app, $path)
    {
        $this->cKey = $name;
        if (empty($this->cData[$name])) {

            $name = $this->base($name);
            
            $this->server();

            list($root, $app) = $this->app($app);

            if (defined('DEVER_PROJECT_PATH')) {
                $config = array(DEVER_PATH, DEVER_PROJECT_PATH);
                if ($root != DEVER_PROJECT_PATH) {
                    $config[] = $root;
                }
            } else {
                $config = array(DEVER_PATH, $root);
            }

            $this->cData[$name] = array();

            foreach ($config as $k => $v) {
                $this->assign($name, $v, $path . DIRECTORY_SEPARATOR, $name, $k);
            }
        }

        return $this;
    }

    /**
     * base
     * @param string $name
     *
     * @return mixed
     */
    protected function base($name)
    {
        if (in_array($name, self::$setting)) {
            $name = 'base';
        }

        return $name;
    }

    /**
     * app
     * @param string $app
     *
     * @var mixed
     */
    protected function app($app)
    {
        $root = DEVER_APP_PATH;
        $name = DEVER_APP_NAME;
        if ($app == 'project') {
            $root = DEVER_PROJECT_PATH;
            $name = DEVER_PROJECT;
        } elseif ($app != $name) {
            $app = Project::load($app);
            if (isset($app['setup'])) {
                if (is_dir($app['path'] . 'config')) {
                    $root = $app['path'];
                } else {
                    $root = $app['setup'];
                }
            } else {
                $root = $app['path'];
            }
            $name = $app['name'];
        } elseif (defined('DEVER_APP_SETUP')) {
            if (is_dir(DEVER_APP_SETUP . 'config')) {
                $root = DEVER_APP_SETUP;
            }
        }
        return array($root, $name);
    }

    /**
     * server
     *
     * @var mixed
     */
    protected function server()
    {
        if (empty($_SERVER['DEVER_SERVER'])) {
            if (isset($_SERVER['SERVER_NAME']) && $_SERVER['SERVER_NAME'] != '127.0.0.1') {
                $_SERVER['DEVER_SERVER'] = $_SERVER['SERVER_NAME'];
            } else {
                $_SERVER['DEVER_SERVER'] = 'localhost';
            }
        }

        if (strpos($_SERVER['DEVER_SERVER'], '*.') !== false) {
            $_SERVER['DEVER_SERVER'] = str_replace('*.', '', $_SERVER['DEVER_SERVER']);
        }
    }

    /**
     * env
     * @param string $name
     * @param string $path
     *
     * @var array
     */
    protected function env($name, $base, $path, $key, $index)
    {
        if (DEVER_PROJECT != 'default') {
            $config[] = 'default';
        }
        $config[] = DEVER_PROJECT;
        if ($name != 'base') {
            $config[] = $name;
        }

        $env = self::ENV . DIRECTORY_SEPARATOR;
        if ($index == 0 && defined('DEVER_ENV_PATH')) {
            $base = DEVER_ENV_PATH;
            $env = '';
        }

        foreach ($config as $k => $v) {
            $this->assign($v, $base, $path . $env . $_SERVER['DEVER_SERVER'] . DIRECTORY_SEPARATOR);
        }
    }

    /**
     * assign
     * @param string $name
     * @param string $path
     *
     * @var array
     */
    protected function assign($name, $base, $path, $key = '', $index = -1)
    {
        if (!$base) {
            $base = ini_get('include_path');
            $temp = explode(':', $base);
            foreach ($temp as $k => $v) {
                if (strpos($v, DIRECTORY_SEPARATOR) !== false) {
                    $base = DIRECTORY_SEPARATOR . $v . DIRECTORY_SEPARATOR;
                    break;
                }
            }
        }
        $file = $base . $path . $name . '.php';

        if (is_file($file)) {
            if ($name == 'base') {
                $this->env($name, $base, $path, $key, $index);
            }

            $this->import($file, $key);
        }
    }

    /**
     * import
     * @param string $file
     * @param string $key
     *
     * @var array
     */
    protected function import($file, $key)
    {
        $config = include $file;

        if (is_array($config)) {
            if ($this->cKey == 'base') {
                $this->merge($config, $key);
            } else {
                $this->cData[$this->cKey] = $config;
            }
        }
    }

    /**
     * merge
     * @param array $config
     *
     * @var array
     */
    protected function merge($config, $key = '')
    {
        foreach ($config as $k => $v) {
            $this->mergeOne($config, $k);
        }
    }

    /**
     * merge
     * @param array $config
     *
     * @var array
     */
    protected function mergeOne($config, $key)
    {
        if (isset($this->cData[$key])) {
            $this->cData[$key] = array_merge($this->cData[$key], $config[$key]);
        } else {
            $this->cData[$key] = $config[$key];
        }
    }

    /**
     * defineAppName
     */
    private function defineAppName($path)
    {
        $temp = explode($path, DEVER_APP_PATH);
        define('DEVER_APP_NAME', chop(str_replace(DIRECTORY_SEPARATOR, '_', end($temp)), '_'));
    }
}
