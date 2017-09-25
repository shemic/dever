<?php namespace Dever\Data\Model;

use Dever\Loader\Config;
use Dever\Output\Export;

class Database
{
    /**
     * config
     *
     * @var array
     */
    protected $config;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * get
     *
     * @return mixed
     */
    public static function get($key = '')
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
        }

        return self::$instance->store($key);
    }

    /**
     * __construct
     *
     * @return mixed
     */
    public function __construct()
    {
        $this->config = Config::get('database')->cAll;
    }

    /**
     * store
     *
     * @return mixd
     */
    protected function store($key = '')
    {
        if ($key && isset($this->config[$key])) {
            $class = 'Dever\\Data\\' . ucwords($this->config[$key]['type']) . '\\Store';
            return $class::getInstance($key, $this->config[$key]);
        } elseif(isset($this->config['default'])) {
            $class = 'Dever\\Data\\' . ucwords($this->config['default']['type']) . '\\Store';
            return $class::getInstance('default', $this->config['default']);
        } elseif(isset($this->config['type'])) {
            $class = 'Dever\\Data\\' . ucwords($this->config['type']) . '\\Store';
            return $class::getInstance($key, $this->config);
        } else {
            Export::alert('database_config_exists', $key);
        }
    }
}
