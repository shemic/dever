<?php namespace Dever\Data;

use Dever;
use Dever\Cache\Handle;
use Dever\Loader\Config;
use Dever\Output\Debug;
use Dever\Pagination\Paginator as Page;
use Dever\Routing\Input;
use Dever\Support\Path;
use Dever\Routing\Uri;

class Store
{
    /**
     * sql
     *
     * @var Dever\Data\Sql
     */
    protected $sql;

    /**
     * read
     *
     * @var Dever\Data\Connect
     */
    protected $read;

    /**
     * update
     *
     * @var Dever\Data\Connect
     */
    protected $update;

    /**
     * config
     *
     * @var array
     */
    protected $config;

    /**
     * table
     *
     * @var string
     */
    protected $table;

    /**
     * alias
     *
     * @var string
     */
    protected $alias;

    /**
     * value
     *
     * @var array
     */
    protected $value = array();

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * getInstance
     *
     * @return Dever\Data\Store;
     */
    public static function getInstance($key, $config)
    {
        if (empty(static::$instance[$key])) {
            static::$instance[$key] = new static($config);
        }

        return static::$instance[$key];
    }

    

    /**
     * __construct
     *
     * @return mixd
     */
    public function __construct($config)
    {
        $this->init();

        $this->config = $config;

        //$this->register($config);
    }

    /**
     * register
     *
     * @return mixd
     */
    protected function register()
    {
        if ($this->read && $this->update) {
            return;
        }
        if (is_array($this->config['host'])) {
            $config = $this->config;
            $host = $this->config['host'];
            if ($host['read'] != $host['update']) {
                $config['host'] = $host['read'];
                $this->read = $this->connect($config);
                $config['host'] = $host['update'];
                $this->update = $this->connect($config);
            } else {
                $config['host'] = $host['read'];
                $this->read = $this->update = $this->connect($config);
            }
        } else {
            $this->read = $this->update = $this->connect($this->config);
        }
        $this->config['link'] = false;
    }

    /**
     * table
     *
     * @return object
     */
    public function table($table, $name = '', $state = true, $alias = '', $prefix = false, $link = false)
    {
        $this->config['link'] = $link;
        $this->alias = $alias ? $alias : $table;
        if ($prefix) {
            $table = $prefix . '_' . $table;
        } elseif (defined('DEVER_DB_PREFIX')) {
            $table = DEVER_DB_PREFIX . '_' . $table;
        } elseif ($state == true && defined('DEVER_PROJECT') && DEVER_PROJECT != 'default') {
            $table = DEVER_PROJECT . '_' . $table;
        }

        $this->setTable($table);

        if (!$link && $state == true && isset($this->sql) && Config::get('database')->sql) {
            $file = $this->file($name);

            if (is_file($file)) {
                $config = include $file;
                if (isset($config['order'])) $this->sql->setColOrder($config['order']);
            }
        }

        return $this;
    }

    /**
     * table
     *
     * @return object
     */
    public function index($index, $name = '')
    {
        if ($this->config['link']) {
            return false;
        }

        if (empty($index)) {
            return false;
        }
        $file = $this->file($name);
        if (!is_file($file)) {
            return false;
        }

        $data = include $file;

        if (isset($index['version'])) {
            $version = $index['version'];

            unset($index['version']);
        } else {
            $version = 1;
        }

        if (empty($data['index']) || (isset($data['index']) && $data['index'] < $version)) {
            $data += $this->getIndex($version, $index);

            file_put_contents($file, '<?php return ' . var_export($data, true) . ';');
        }

        return $this;
    }

    /**
     * alter table
     *
     * @return mixed
     */
    public function alter($alter, $struct = array(), $name = '')
    {
        if ($this->config['link']) {
            return false;
        }

        if (empty($this->sql)) {
            return false;
        }
        if (empty($alter)) {
            return false;
        }
        $file = $this->file($name);

        if (!is_file($file)) {
            return false;
        }

        $data = include $file;
        if (is_array($struct)) {
            $sql = $this->sql->alter($this->table, $alter);

            $this->exe($sql);

            $this->log($sql, 'alter');

            $data['struct'] = array_flip(array_keys($struct));

            file_put_contents($file, '<?php return ' . var_export($data, true) . ';');
        } else {
            if (isset($alter['version'])) {
                $version = $alter['version'];
            } else {
                $version = 1;
            }

            if (isset($alter[$version]) && (empty($data['alter']) || (isset($data['alter']) && $data['alter'] != $version))) {
                $sql = $this->sql->alter($this->table, $alter[$version]);

                $this->exe($sql);

                $this->log($sql, 'alter');

                $data['alter'] = $version;

                file_put_contents($file, '<?php return ' . var_export($data, true) . ';');
            }
        }

        return true;
    }

    /**
     * insert the default value
     *
     * @return mixed
     */
    public function insertDefault($value, $name = '')
    {
        $file = $this->file($name);
        if (!is_file($file)) {
            return false;
        }

        $data = include $file;
        if (isset($value['col']) && isset($value['value'])) {
            $this->truncate();

            $data = $this->insertValues($value, $data);

            file_put_contents($file, '<?php return ' . var_export($data, true) . ';');
        }

        return true;
    }

    /**
     * file
     *
     * @return mixed
     */
    public function file($name = '')
    {
        $path = Config::data();

        if ($name) {
            $name = $this->table . '_' . $name . '.php';
        } else {
            $name = $this->table . '.php';
        }

        $temp = explode('_', $this->table);

        $file = Path::get($path . 'database/', $temp[0] . '/' . $name);
        return $file;
    }

    /**
     * create
     *
     * @return mixed
     */
    public function create($struct, $name = '', $type = 'innodb', $partition = '', $create = false, $auto = 1)
    {
        if ($this->config['link']) {
            return false;
        }

        if (isset($this->sql) && $create > 0) {
            return false;
        }

        if (isset($this->sql) && Config::get('database')->create > 0) {
            return false;
        }

        $create = $create < 0 ? $create : Config::get('database')->create;
        $file = $this->file($name);
        if (is_file($file)) {
            return include $file;
        }

        if (isset($this->sql)) {
            $sql = $this->sql->create($this->table, $struct, $type, $partition, $create, $auto);

            $this->exe($sql);

            $this->log($sql, 'create');
        } elseif (method_exists($this, 'createTable')) {
            $this->createTable($struct);
        }

        $data['time'] = DEVER_TIME;

        $data['table'] = $this->table;

        if (isset($this->sql)) {
            $data['create'] = $sql;

            $data['struct'] = array_flip(array_keys($struct));
        } else {
            $data['create'] = true;

            $this->log($data, 'create');
        }

        file_put_contents($file, '<?php return ' . var_export($data, true) . ';');

        return true;
    }

    private function truncate()
    {
        if (isset($this->sql)) {
            $sql = $this->sql->truncate($this->table);

            $this->exe($sql);

            $this->log($sql, 'truncate');
        }
    }

    /**
     * getPageBySql
     *
     * @return array
     */
    public function getPageBySql($config = array(), $sql = false, $data = array(), Model $model)
    {
        empty($config['template']) && $config['template'] = 'list';

        empty($config['key']) && $config['key'] = 'current';

        empty($config['link']) && $config['link'] = '';

        empty($config['num']) && $config['num'] = 10;

        $page = Page::getInstance($config['key']);

        $page->template($config['template']);

        $page->link($config['link']);

        $total = Input::get('pt', -1);

        if (isset($config['explode']) && isset($config['content'])) {
            $content = explode($config['explode'], $config['content']);
            $page->offset(1);
            $data = $page->data($content, $total);
        } else {
            empty($config['first_num']) && $config['first_num'] = 0;
            $offset = $page->offset($config['num'], $config['first_num']);
            $data = $page->sqlCount($sql, (isset($config['offset']) ? $config['offset'] : $offset), $total, $model, $data);
        }

        Dever::$global['page'][$config['key']] = $page->toArray();
        
        return $data;
    }

    /**
     * page
     *
     * @return object
     */
    public function page($num, $config = array())
    {
        $this->reset('limit');

        empty($config[0]) && $config[0] = 'list';

        empty($config[1]) && $config[1] = 'current';

        empty($config[2]) && $config[2] = '';

        $page = Page::getInstance($config[1]);

        $page->template($config[0]);

        $page->link($config[2]);

        $this->limit($page->offset($num). ',' . $num);

        $total = Input::get('pt', -1);

        if ($total < 0) {
            $total = $this->count('clear');
        }

        $page->total($total);

        Dever::$global['page'][$config[1]] = $page->toArray();
        return $this;
    }

    public function deleteCache($value, $key, $handle)
    {
        return $handle->delete($key);
    }

    /*
    public function cache($key = false, $method = 'get', $data = false)
    {
        $cache = isset($this->config['cache']) ? $this->config['cache'] : Config::get('cache')->cAll;

        if (isset($cache['route']) && $cache['route'] > 0 && $this->table && !isset(Config::get('base')->clearCache['route'])) {
            $handle = Handle::getInstance('route', $cache['route']);
            if ($method == 'put' && $data !== false) {
                $route = Uri::key();
                $value = $handle->hGet($this->table, $route, true);
                if (!$value) {
                    $handle->hSet($this->table, $route, 1);
                }
            } elseif (!$key && $this->table) {
                $value = $handle->hGet($this->table, false, true);
                $handle->delete($this->table);
                if ($value) {
                    array_walk($value, array($this, 'deleteCache'), $handle);
                }
            }
        }

        if (empty($cache['mysql'])) {
            return false;
        }

        $handle = Handle::getInstance('mysql', $cache['mysql']);
        if (!$key && $this->table) {
            $value = $handle->hGet($this->table, false, true);
            if ($value) {
                array_walk($value, array($this, 'deleteCache'), $handle);
            }
        }

        if ($method == 'get') {
            if (DEVER_APP_NAME == 'manage') {
                return false;
            }
            return $handle->get($key);
        }

        if ($method == 'put' && $data !== false) {

            if ($this->table) {
                $value = $handle->hGet($this->table, $key, true);
                if (!$value) {
                    $handle->hSet($this->table, $key, 1);
                }
            }

            return $handle->set($key, $data);
        }

        return false;
    }
    */

    public function cache($key = false, $method = 'get', $data = false)
    {
        $cache = isset($this->config['cache']) ? $this->config['cache'] : Config::get('cache')->cAll;

        if (isset($cache['route']) && $cache['route'] > 0 && $this->table && !isset(Config::get('base')->clearCache['route'])) {
            $handle = Handle::getInstance('route', $cache['route']);
            if ($method == 'put' && $data !== false) {
                $keys = $handle->get($this->table, false);
                $route = Uri::key();
                if (!isset($keys[$route])) {
                    $keys[$route] = 1;
                    $handle->set($this->table, $keys, 0, false);
                }
            } elseif (!$key && $this->table) {
                $keys = $handle->get($this->table, false);
                if ($keys) {
                    array_walk($keys, array($this, 'deleteCache'), $handle);
                }
            }
        }

        if (empty($cache['mysql'])) {
            return false;
        }

        $handle = Handle::getInstance('mysql', $cache['mysql']);
        if (!$key && $this->table) {
            $keys = $handle->get($this->table, false);
            if ($keys) {
                array_walk($keys, array($this, 'deleteCache'), $handle);
            }
        }

        if ($method == 'get') {
            if (DEVER_APP_NAME == 'manage') {
                return false;
            }
            return $handle->get($key);
        }

        if ($method == 'put' && $data !== false) {

            if ($this->table) {
                $keys = $handle->get($this->table, false);
                if (!isset($keys[$key])) {
                    $keys[$key] = 1;
                    $handle->set($this->table, $keys, 0, false);
                }
            }

            return $handle->set($key, $data);
        }

        return false;
    }

    /**
     * error
     *
     * @return error
     */
    public function error($msg, $sql = '')
    {
        if (isset($this->sql)) {
            if ($sql) {
                $msg = array('sql' => $sql, 'error' => $msg);
            } elseif (is_object($msg)) {
                $msg = (array) $msg;
            } elseif (is_string($msg)) {
                $msg = array('sql' => $msg);
            }
            Debug::wait($msg, 'Dever SQL DB Error!');
        } else {
            if (is_string($msg)) {
                $msg = array('value' => $msg);
            }
            Debug::wait($msg, 'Dever NOSQL DB Error!');
        }
    }

    /**
     * log
     *
     * @return log
     */
    public function log($value, $param = array(), $data = array())
    {
        if (isset($this->sql)) {
            $value = $this->replace($value, $param);
            $this->sql($value);
            if (!Input::shell('all') && is_array($data)) {
                $data = count($data) . ' records';
            }
            Debug::log(array('sql' => $value, 'data' => $data), $this->config['type']);
        } else {
            Debug::log(array('value' => $value, 'method' => $param), $this->config['type']);
        }
    }

    /**
     * sql
     *
     * @return sql
     */
    public function replace($value, $param)
    {
        if ($value && is_array($param)) {
            foreach ($param as $k => $v) {
                if (is_string($v)) {
                    if (strpos($v, ',')) {
                        $v = 'in('.$v.')';
                    } else {
                        $v = '"' . $v . '"';
                    }
                }
                $value = str_replace($k, $v, $value);
            }
        }

        return $value;
    }

    /**
     * sql
     *
     * @return mixed
     */
    public function sql($value)
    {
        if (!Config::get('database')->sql) {
            Config::get('database')->sql = array();
        }
        $sql = Config::get('database')->sql;
        array_push($sql, $value);
        Config::get('database')->sql = $sql;
    }

    /**
     * begin
     *
     * @return object
     */
    public function begin()
    {
        return $this;
    }

    /**
     * commit
     *
     * @return object
     */
    public function commit()
    {
        return $this;
    }

    /**
     * rollback
     *
     * @return object
     */
    public function rollback()
    {
        return $this;
    }

    /**
     * fetchAll
     *
     * @return array
     */
    protected function fetchAll($handle, $config = false)
    {
        $rows = function() use ($handle) {
            while ($row = $handle->fetch()) {
                yield $row;
            }
        };
        $result = array();
        $data = $rows();
        if ($data) {
            if ($config) {
                $result = $this->fetchSet($data, $config);
            } else {
                foreach ($data as $row) {
                    $result[] = $row;
                }
            }
        }
        return $result;
    }

    /**
     * fetchSet
     *
     * @return array
     */
    protected function fetchSet($data, $config)
    {
        $result = array();
        $key = $config[1];
        foreach ($data as $row) {
            if (isset($row[$key])) {
                if (isset($config[3]) && isset($row[$config[2]])) {
                    $result[$row[$key]][$row[$config[2]]] = $row;
                } elseif (isset($config[2]) && isset($row[$config[2]])) {
                    $result[$row[$key]] = $row[$config[2]];
                } elseif (isset($config[2])) {
                    $result[$row[$key]][] = $row;
                } else {
                    $result[$row[$key]] = $row;
                }
            }
        }
        return $result;
    }
}
