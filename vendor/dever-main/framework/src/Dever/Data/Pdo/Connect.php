<?php namespace Dever\Data\Pdo;

use Dever\Output\Debug;
use Dever\Output\Export;

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
     * @return Dever\Data\Pdo\Connect;
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
        if ($this->handle) {
            return;
        }
        if (strpos($config['host'], ':') !== false) {
            list($config['host'], $config['port']) = explode(':', $config['host']);
        }

        $dsn['type'] = $config['type'];
        $dsn['host'] = $config['host'];
        $dsn['port'] = $config['port'];
        $dsn['dbname'] = $config['database'];
        $dsn['charset'] = $config['charset'];

        foreach ($dsn as $key => $val) {
            $dsn[$key] = "$key=$val";
        }

        $type = isset($config['pdo_type']) ? $config['pdo_type'] : 'mysql';
        $dsnList = $type . ':' . implode(';', $dsn);

        try {
            $persistent = false;
            if (isset($config['persistent'])) {
                $persistent = $config['persistent'];
            }
            $this->handle = new \PDO($dsnList, $config['username'], $config['password'], array(\PDO::ATTR_PERSISTENT => $persistent));
            $this->handle->setAttribute(\PDO::ATTR_ERRMODE, \PDO::ERRMODE_EXCEPTION);
            //$this->handle->setAttribute(\PDO::ATTR_EMULATE_PREPARES, false);
            $this->handle->setAttribute(\PDO::ATTR_CASE, \PDO::CASE_NATURAL);
            $this->handle->setAttribute(\PDO::ATTR_DEFAULT_FETCH_MODE, \PDO::FETCH_ASSOC);
            //$this->handle->setAttribute(\PDO::MYSQL_ATTR_USE_BUFFERED_QUERY, false);

            Debug::log('db ' . $config['host'] . ' connected', $config['type']);
        } catch (\PDOException $e) {
            if (strstr($e->getMessage(), 'Unknown database')) {
                $method = 'mysql';
                if (function_exists('mysqli_connect')) {
                    $method = 'mysqli';
                }
                $connect = $method . '_connect';
                $query = $method . '_query';
                $close = $method . '_close';
                $link = $connect($config['host'] . ':' . $config['port'], $config['username'], $config['password']);
                if ($link) {
                    if ($method == 'mysql') {
                        $query("CREATE DATABASE `" . $config['database'] . "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", $link);
                    } else {
                        $query($link, "CREATE DATABASE `" . $config['database'] . "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;");
                    }
                    
                    $close($link);
                }
                
                $this->init($config);
            } else {
                Export::alert($e->getMessage());
            }
        }

        //$this->query("set names '".$config['charset']."'");
        //$this->_log('connected mysql:' . $config['host']);
    }

    public function set()
    {
        $this->handle->setAttribute(\PDO::ATTR_EMULATE_PREPARES, false);
    }

    /**
     * __construct
     *
     * @return mixd
     */
    public function __destruct()
    {
        $this->close();
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

    /**
     * prepare
     *
     * @return object
     */
    public function prepare($sql)
    {
        return $this->handle->prepare($sql);
    }

    /**
     * exec
     *
     * @return object
     */
    public function exec($sql)
    {
        return $this->handle->exec($sql);
    }

    /**
     * query
     *
     * @return object
     */
    public function query($sql)
    {
        if ($sql) {
            return $this->handle->query($sql);
        }

        return false;
    }

    /**
     * lastid
     *
     * @return int
     */
    public function id()
    {
        return $this->handle->lastInsertId();
    }

    /**
     * method
     *
     * @return mixed
     */
    public function method($method)
    {
        return $this->handle->$method();
    }
}
