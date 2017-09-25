<?php namespace Dever\Data;

use Dever;
use Dever\Loader\Config;
use Dever\Loader\Project;
use Dever\Loader\Import;
use Dever\Output\Export;
use Dever\Routing\Input;
use Dever\String\Helper;

class Model
{
    /**
     * config
     *
     * @var array
     */
    public $config;

    /**
     * data
     *
     * @var array
     */
    protected $data;

    /**
     * index
     *
     * @var string
     */
    protected $index;

    /**
     * table
     *
     * @var string
     */
    protected $table;

    /**
     * API
     *
     * @var string
     */
    const DATABASE = 'database/';

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * key
     *
     * @var string
     */
    protected static $key;

    /**
     * load
     *
     * @return mixed
     */
    public static function load($table = '', $param = '')
    {
        list($table, $index, $method) = self::table($table);
        $key = $table . '_' . $index;
        if (empty(static::$instance[$key])) {
            static::$instance[$key] = new static($table, $index);
        }

        if ($method) {
            return static::$instance[$key]->$method($param);
        }

        return static::$instance[$key];
    }


    /**
     * table
     * @param array $param
     *
     * @return array
     */
    public static function table($table)
    {
        $index = $method = '';
        if (!$table) {
            $table = self::$key;
        } elseif (strpos($table, '/') === false) {
            $index = $table;
            $table = self::$key;
        }
        self::$key = $table;
        if (strpos($table, '-')) {
            list($table, $method) = explode('-', $table);
        }
        if (strpos($table, ':')) {
            list($table, $index) = explode(':', $table);
        }

        return array($table, $index, $method);
    }

    /**
     * __construct
     *
     * @return mixed
     */
    public function __construct($table, $index = '')
    {
        $this->init($table, $index);
    }

    /**
     * init
     *
     * @return mixd
     */
    protected function init($table, $index = '')
    {
        $this->table = $table;
        $this->index = $index;
        if (!$this->table) {
            $this->config['name'] = 'table';
            $this->config['project'] = '';
            $this->config['db'] = '';
        } else {
            $this->loadConfig();
        }
    }

    /**
     * loadConfig
     *
     * @return mixd
     */
    protected function loadConfig()
    {
        list($projectName, $tableName) = explode('/', $this->table);
        $projectInfo = Project::load($projectName);

        $path = $projectInfo['path'] . self::DATABASE;
        if ($tableName == 'col') {
                //print_r($this->table);die;
            }
        if ($this->index) {
            $path .= $this->index . '/';
            $this->config['db'] = $this->index;
        } else {
            $this->config['db'] = $projectInfo['name'];
        }

        $file = $path . $tableName . '.php';
        if (is_file($file)) {
            $config = include $file;
            $this->config = array_merge($this->config, $config);
        } else {
            $this->config['name'] = $tableName;
        }
        $this->config['project'] = $projectInfo;
    }

    /**
     * db
     *
     * @return mixd
     */
    private function db()
    {
        return Model\Database::get($this->config['db']);
    }

    /**
     * query
     *
     * @return mixd
     */
    public function query($sql, $data = array(), $method = '')
    {
        $this->sql = Helper::replace('{table}', $this->getTableName(), $sql);
        return $this->db()->exe($this->sql, $data, $method);
    }

    /**
     * getTableName
     *
     * @return mixd
     */
    private function getTableName()
    {
        if (isset($this->config['struct'])) {
            return $this->config['project']['name'] . '_' . $this->config['name'];
        } else {
            return $this->config['name'];
        }
    }

    /**
     * fetch
     *
     * @return mixd
     */
    public function fetch($sql, $data = array(), $cache = false)
    {
        return $this->fetchAll($sql, $data, false, $cache, 'fetch');
    }

    /**
     * setTable
     *
     * @return mixd
     */
    private function setTable($sql)
    {
        $table = $sql;
        //$this->db()->setTable($table);
    }

    /**
     * fetchAll
     *
     * @return mixd
     */
    public function fetchAll($sql, $data = array(), $page = array(), $cache = false, $method = 'fetchAll')
    {
        $this->setTable($sql);

        if (!$cache && !empty(Config::get('cache')->mysql)) {
            $cache = 'getSql_' . md5($sql . serialize($data));
            if ($page && $p = Input::get('page')) {
                $cache .= '_p' . $p;
            }
        }

        if ($cache) {
            $result = $this->db()->cache($cache);
            if ($result) {
                return $result;
            }
        }

        if ($page) {
            $result = $this->page($page, $sql, $data);
        } else {
            $result = $this->query($sql, $data)->$method();
        }
        
        if ($cache) {
            $this->db()->cache($cache, $result);
        } else {
            $this->db()->log($this->sql, $data, $result);
        }

        return $result;
    }

    /**
     * rowCount
     *
     * @return mixd
     */
    public function rowCount($sql, $data = array())
    {
        return $result = $this->query($sql, $data)->rowcount();
    }

    /**
     * lastId
     *
     * @return mixd
     */
    public function lastId($sql, $data = array())
    {
        return $result = $this->query($sql, $data)->id();
    }

    /**
     * page
     *
     * @return array
     */
    public function page($config = array(), $sql = false, $data = array())
    {
        return $this->db()->getPageBySql($config, $sql, $data, $this);
    }

    /**
     * index
     *
     * @return array
     */
    public function index($index)
    {
        return $this->db()->index($index);
    }

    /**
     * transaction begin
     *
     * @return array
     */
    public function begin()
    {
        return $this->db()->begin();
    }

    /**
     * transaction commit
     *
     * @return array
     */
    public function commit()
    {
        return $this->db()->commit();
    }

    /**
     * transaction rollback
     *
     * @return array
     */
    public function rollback()
    {
        return $this->db()->rollback();
    }

    /**
     * __call
     *
     * @return mixd
     */
    public function __call($method, $param)
    {
        if (isset($param[0][0]) && is_array($param[0][0])) {
            $result = array();
            foreach ($param[0] as $k => $v) {
                $result[] = $this->$method($v);
            }
            
            return $result;
        }

        $param = $param ? $this->initParam($param[0], $method) : array();

        $key = $this->table . $method . md5(serialize($param));

        $this->compatible($key, $method, $param);

        if (!isset($this->data[$key])) {
            $this->request($method, $param);
            $this->data[$key] = $this->callData($method, $param);
        }

        return $this->data[$key];

    }

    /**
     * getData
     *
     * @return mixd
     */
    private function callData($method, $param)
    {
        $param = $this->search($method, $param);
        $handle = new Model\Handle($method, $this->config, $param);
        return $handle->get();
    }

    /**
     * compatible
     *
     * @return mixd
     */
    protected function compatible($key, $method, $param)
    {
        if (Config::get('database')->compatible && isset($this->config['project'])) {
            $file = $this->config['project']['path'] . 'database/' . ucfirst(Config::get('database')->compatible) . '/' . ucfirst($this->config['name']) . '.php';
            if (is_file($file)) {
                $class = ucfirst($this->config['project']['name']) . '\\Database\\' . ucfirst(Config::get('database')->compatible) . '\\' . ucfirst($this->config['name']);
                if (class_exists($class) && method_exists($class, $method)) {
                    $this->data[$key] = $class::$method($param);
                }
            }
        }
    }

    /**
     * request
     *
     * @return mixd
     */
    protected function request($method, $param)
    {
        if (empty($this->config['request'][$method]) && isset($this->config['struct'])) {
            $search = '';
            if (isset($param['search_type'])) {
                $search = $param['search_type'];
            }

            $this->config['request'][$method] = Model\Request::get($this->table, $method, $this->config['struct'], $search);

            if ($method == 'all' && isset($param['option'])) {
                foreach ($param['option'] as $k => $v) {
                    $this->config['request'][$method]['option'][$k] = $v;
                }
            }

            if ($method == 'list' && isset($this->config['manage']['list_type']) && $this->config['manage']['list_type']) {
                $this->config['request']['list']['page'][0] = 50000;
            }
        }
    }

    /**
     * search
     *
     * @return mixd
     */
    protected function search($method, $param)
    {
        if (isset($param['search_type']) && $param['search_type'] == 2) {
            $join = array();
            $i = 2;
            foreach ($param as $k => $v) {
                if (strpos($k, '-') !== false) {
                    $k = str_replace(array('option_', 'where_'), '', $k);
                    if (isset($this->config['struct'][$k]) && isset($this->config['struct'][$k]['sync'])) {
                        $temp = explode('-', $k);
                        $join[] = array
                        (
                            'table' => $temp[0] . '/' . $temp[1],
                            'type' => 'left join',
                            'on' => $this->config['struct'][$k]['sync'],
                        );

                        $t = 't_' . $i . '.' . $temp[2];
                        $this->config['request'][$method]['option'][$t] = $this->config['request'][$method]['option'][$k];
                        $param['option_' . $t] = $param['option_' . $k];
                        unset($this->config['request'][$method]['option'][$k]);
                        unset($param['option_' . $k]);
                        $i++;
                    }
                }
            }
            if ($join) {
                $this->config['request'][$method]['join'] = $join;
            }
        }

        return $param;
    }

    protected function searchFulltext($k, $param)
    {

    }

    /**
     * initParam
     *
     * @return mixd
     */
    private function initParam($param, $method)
    {
        if ($param && !is_array($param)) {
            $param = array('where_id' => $param, 'option_id' => $param);
        }
        return $param;
    }
}
