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
    private $setting = array('host', 'database', 'debug', 'cache', 'template');

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
        return isset(self::get('base')->data) ? self::get('base')->data : DEVER_PATH . 'data/';
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
        $key = $app . '_' . $path;
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
                $config = array(DEVER_PATH, DEVER_PROJECT_PATH, $root);
            } else {
                $config = array(DEVER_PATH, $root);
            }

            $this->cData[$name] = array();

            foreach ($config as $k => $v) {
                $this->assign($name, $v, $path . '/', $name, $k);
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
        if (in_array($name, $this->setting)) {
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
        if ($app != $name) {
            $app = Project::load($app);
            $root = $app['path'];
            $name = $app['name'];
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
            if (isset($_SERVER['SERVER_NAME'])) {
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
        $config[] = DEVER_PROJECT;
        if ($name != 'base') {
            $config[] = $name;
        }

        $env = self::ENV . '/';
        if ($index == 0 && defined('DEVER_ENV_PATH')) {
            $base = DEVER_ENV_PATH;
            $env = '';
        }

        foreach ($config as $k => $v) {
            $this->assign($v, $base, $path . $env . $_SERVER['DEVER_SERVER'] . '/');
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
                if (strpos($v, '/') !== false) {
                    $base = '/' . $v . '/';
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
            $this->merge($config, $key);
        }
    }

    /**
     * merge
     * @param array $config
     * @param string $key
     *
     * @var array
     */
    protected function merge($config, $key)
    {
        if ($key) {
            if (isset($config[$key])) {
                $this->cData[$key] = array_merge($this->cData[$key], $config[$key]);
                $this->setting($config);
            } else {
                $this->cData[$key] = array_merge($this->cData[$key], $config);
            }
        } else {
            $this->cData = array_merge_recursive($this->cData, $config);
        }
    }

    /**
     * setting
     */
    private function setting($config)
    {
        if ($this->cKey == 'base') {
            foreach ($this->setting as $k) {
                if (isset($config[$k])) {
                    if (empty($this->cData[$k])) {
                        $this->cData[$k] = array();
                    }
                    $this->cData[$k] = array_merge($this->cData[$k], $config[$k]);
                }
            }
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
