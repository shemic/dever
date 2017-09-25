<?php namespace Dever\Data\Mongo;

use Dever\Output\Debug;

class Connect
{
    /**
     * handle
     *
     * @var object
     */
    private $handle;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * getInstance
     *
     * @return Dever\Data\Mongo\Connect;
     */
    public static function getInstance($config)
    {
        $key = $config['host'] . $config['database'];
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self();
            self::$instance[$key]->init($config);
        }

        return self::$instance[$key];
    }

    /**
     * init
     *
     * @return mixd
     */
    private function init($config)
    {
        if (strpos($config['host'], ':') !== false) {
            list($config['host'], $config['port']) = explode(':', $config['host']);
        }

        try
        {
            if (!isset($config['timeout'])) {
                $config['timeout'] = 1000;
            }
            $mongo = new \Mongo('mongodb://' . $config['host'] . ':' . $config['port'], array("connectTimeoutMS" => $config['timeout']));

            $this->handle = $mongo->selectDB($config['database']);

            Debug::log('mongodb ' . $config['host'] . ' connected', $config['type']);
        } catch (\PDOException $e) {
            echo $e->getMessage();die;
        }
    }

    /**
     * __destruct
     *
     * @return mixd
     */
    public function __destruct()
    {
        $this->close();
    }

    /**
     * table
     *
     * @return mixd
     */
    public function table($table)
    {
        return $this->handle->selectCollection($table);
    }

    /**
     * handle
     *
     * @return object
     */
    public function handle()
    {
        return $this->handle;
    }

    /**
     * close
     *
     * @return mixd
     */
    public function close()
    {
        $this->handle = null;
    }
}
