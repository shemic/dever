<?php namespace Dever\Data\Pdo;

use Dever\Data\Sql;
use Dever\Data\Store as Base;

class Store extends Base
{
    /**
     * init
     *
     * @return mixd
     */
    public function init()
    {
        $this->sql = Sql::getInstance();
    }

    /**
     * connect
     *
     * @return mixd
     */
    public function connect($config)
    {
        return Connect::getInstance($config);
    }

    /**
     * setTable
     *
     * @return mixd
     */
    public function setTable($table)
    {
        $this->table = $table;
    }

    /**
     * getIndex
     *
     * @return mixed
     */
    public function getIndex($version, $index)
    {
        $this->register();
        $sql = $this->sql->showIndex($this->table);

        $handle = $this->update->query($sql);

        $info = $handle->fetchAll();

        $this->dropIndex($info);

        $sql = $this->sql->index($this->table, $index[$version]);

        $this->query($sql);

        $this->log($sql, 'index');

        $data['index'] = $version;

        $data['order'] = array_flip(explode(',', array_shift($index[$version])));

        return $data;
    }

    private function dropIndex($info)
    {
        if ($info) {
            $this->register();
            $index = array();
            foreach ($info as $k => $v) {
                if ($v['Key_name'] != 'PRIMARY' && !isset($index[$v['Key_name']])) {
                    $sql = $this->sql->dropIndex($this->table, $v['Key_name']);
                    $this->update->query($sql);
                    $index[$v['Key_name']] = true;
                }
            }
        }
    }

    public function exe($sql, $value = array(), $method = '')
    {
        $create = false;
        if (stristr($sql, 'create')) {
            $create = true;
        }
        $this->register($create);
        
        if ($create && isset($this->create)) {
            $db = $this->create;
        } elseif (stristr($sql, 'select')) {
            $db = $this->read;
        } else {
            $db = $this->update;
        }

        if (!is_array($value)) {
            $value = array(':0' => $value);
        }

        try {
            if ($value) {
                $handle = $db->prepare($sql);
                $handle->execute($value);
            } else {
                $handle = $db->query($sql);
            }
        } catch (\PDOException $exception) {
            $this->error($exception->getMessage(), $sql);
        }

        if ($method) {
            $data = $handle->$method();
            $this->log($sql, $value, $data);
            return $data;
        } else {
            return $handle;
        }
    }

    public function query($sql, $state = false)
    {
        if (empty($this->config['shell'])) {
            if (is_string($this->config['host']) && strpos($this->config['host'], ':') !== false) {
                $temp = explode(':', $this->config['host']);
                $this->config['host'] = $temp[0];
                $this->config['port'] = $temp[1];
            } elseif (isset($this->config['host']['read']) && strpos($this->config['host']['read'], ':') !== false) {
                $temp = explode(':', $this->config['host']['read']);
                $this->config['host'] = $temp[0];
                $this->config['port'] = $temp[1];
            }

            $this->config['shell'] = 'mysql -u' . $this->config['username'] . ' -p' . $this->config['password'] . ' ' . $this->config['database'] . ' -h' . $this->config['host'] . ' -P' . $this->config['port'] . ' -e ';
        }

        if ($state == true) {
            # 异步执行
            \Dever::run($this->config['shell'] . '"' . $sql . '"');
        } else {
            try {
                $this->register();
                # 同步执行
                if (strpos($sql, ';')) {
                    $temp = explode(';', $sql);
                    foreach ($temp as $k => $v) {
                        $this->update->query($v);
                    }
                } else {
                    $this->update->query($sql);
                }
            } catch (\PDOException $exception) {
                $this->error($exception->getMessage(), $sql);
            }
        }
    }

    /**
     * insert the default value
     *
     * @return mixed
     */
    public function insertValues($value, $data = array())
    {
        $this->register();
        $sql = $this->sql->insertValues($this->table, $value['col'], $value['value']);

        try {
            $this->update->query($sql);
        } catch (\PDOException $exception) {
            $this->error($exception->getMessage(), $sql);
        }

        $this->log($sql, 'insertValues');

        $data['insert'] = $sql;

        return $data;
    }

    /**
     * all
     *
     * @return array
     */
    public function all($col)
    {
        $config = false;
        if (strpos($col, '|') !== false) {
            $config = explode('|', $col);
            $col = $config[0];
        }
        return $this->select($col, 'fetchAll', 'select', $config);
    }

    /**
     * one
     *
     * @return array
     */
    public function one($col)
    {
        return $this->select($col);
    }

    /**
     * count
     *
     * @return array
     */
    public function count($col = '')
    {
        return $this->select($col, 'fetchColumn', 'count');
    }

    /**
     * insert
     *
     * @return int
     */
    public function insert($num = 1)
    {
        $sql = $this->sql->insert($this->table, $num);

        if ($sql) {
            try {
                $this->register();

                $handle = $this->update->prepare($sql);

                $handle->execute($this->value);
            } catch (\PDOException $exception) {
                $this->error($exception->getMessage(), $sql);
            }

            $id = $this->update->id();

            $this->log($sql, $this->value);

            $this->cache();
        }

        $this->value = array();

        return $id;
    }

    /**
     * update
     *
     * @return int
     */
    public function update()
    {
        $sql = $this->sql->update($this->table);

        $result = false;

        if ($sql) {
            try {
                $this->register();

                $handle = $this->update->prepare($sql);

                $handle->execute($this->value);
            } catch (\PDOException $exception) {
                $this->error($exception->getMessage(), $sql);
            }

            $result = $handle->rowCount();
            //$result = $this->update->id();

            $this->log($sql, $this->value);

            $this->cache();
        }

        $this->value = array();

        return $result;
    }

    /**
     * delete
     *
     * @return int
     */
    public function delete()
    {
        $sql = $this->sql->delete($this->table);

        $result = false;

        if ($sql) {
            try {
                $this->register();

                $handle = $this->update->prepare($sql);

                $handle->execute($this->value);
            } catch (\PDOException $exception) {
                $this->error($exception->getMessage(), $sql);
            }

            $result = $handle->rowCount();

            $this->log($sql, $this->value);

            $this->cache();
        }

        $this->value = array();

        return $result;
    }

    /**
     * select
     *
     * @return array
     */
    private function select($col = '', $method = 'fetch', $type = 'select', $config = false)
    {
        $sql = $this->sql->{$type}($this->table, $col);

        $key = $this->table . '_' . $method . '_' . md5(serialize($this->value) . $sql);

        $data = $this->cache($key);

        if ($data !== false) {
            if ($col != 'clear') {
                $this->value = array();
            }

            return $data;
        }

        if ($type == 'count' && strpos($sql, 'group by `')) {
            $method = 'fetchAll';
        }

        try {
            $this->register();

            if ($this->value) {
                $handle = $this->read->prepare($sql);
                //print_r($this->value);
                $handle->execute($this->value);
            } else {
                $handle = $this->read->query($sql);
            }
        } catch (\PDOException $exception) {
            $this->error($exception->getMessage(), $sql);
        }

        if ($method == 'fetchAll') {
            $data = $this->fetchAll($handle, $config);
            
        } else {
            $data = $handle->$method();
        }

        //print_r($data);die;
        $state = $this->cache($key, 'put', $data);
        $this->log($sql, $this->value, $data);
        if ($col != 'clear') {
            $this->value = array();
        }
        return $data;
    }

    /**
     * join
     *
     * @return object
     */
    public function join($param)
    {
        $this->sql->join($param);

        return $this;
    }

    /**
     * begin
     *
     * @return object
     */
    public function begin()
    {
        $this->register();
        $this->update->method('beginTransaction');

        return $this;
    }

    /**
     * commit
     *
     * @return object
     */
    public function commit()
    {
        $this->register();
        $this->update->method('commit');

        return $this;
    }

    /**
     * rollback
     *
     * @return object
     */
    public function rollback()
    {
        $this->register();
        $this->update->method('rollBack');

        return $this;
    }

    /**
     * __call
     *
     * @return object
     */
    public function __call($method, $param)
    {
        if (isset($param[0]) && is_array($param[0]) && $method != 'order') {
            foreach ($param[0] as $k => $v) {
                $this->call($method, $v);
            }
        } else {
            $this->call($method, $param);
        }

        return $this;
    }

    /**
     * call
     *
     * @return mixd
     */
    private function call($method, $param)
    {
        if ($method == 'where' || $method == 'set' || $method == 'add') {
            # 特殊处理in
            if (isset($param[2]) && $param[2] == 'in') {
                if (!is_array($param[1])) {
                    $param[1] = explode(',', $param[1]);
                }

                $prefix = 'in_';
                $temp = $param[0];
                foreach ($param[1] as $k => $v) {
                    if (strpos($temp, '.')) {
                        $temp = str_replace('.', '_', $temp);
                    }
                    $k = ':' . $temp . '_' . $prefix . $k;
                    $key[] = $k;
                    $this->value[$k] = $v;
                }

                $param[1] = '(' . implode(',', $key) . ')';
            } else {
                $key = ':' . count($this->value);

                $this->value[$key] = $param[1];

                $param[1] = $key;

                if (isset($param[2]) && $param[2] == 'like') {
                    $param[2] = 'like^' . $this->value[$key];
                    //$this->value[$key] = trim($this->value[$key], ',');
                }
            }
        }

        $this->sql->$method($param);
    }
}
