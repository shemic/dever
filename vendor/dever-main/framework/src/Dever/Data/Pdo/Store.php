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
        $sql = $this->sql->showIndex($this->table);

        $handle = $this->update->query($sql);

        $info = $handle->fetchAll();

        if ($info) {
            foreach ($info as $k => $v) {
                if ($v['Key_name'] != 'PRIMARY') {
                    $sql = $this->sql->dropIndex($this->table, $v['Key_name']);
                    $this->update->query($sql);
                }
            }
        }

        $sql = $this->sql->index($this->table, $index[$version]);

        $this->query($sql);

        $this->log($sql, 'index');

        $data['index'] = $version;

        $data['order'] = array_flip(explode(',', array_shift($index[$version])));

        return $data;
    }

    public function exe($sql, $value = array(), $method = '')
    {
        if (stristr($sql, 'select')) {
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

    public function query($sql, $state = true)
    {
        if (empty($this->config['shell'])) {
            if (strpos($this->config['host'], ':') !== false) {
                $temp = explode(':', $this->config['host']);
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
    public function getInserts($value, $data = array())
    {
        $sql = $this->sql->inserts($this->table, $value['col'], $value['value']);

        try {
            $this->update->query($sql);
        } catch (\PDOException $exception) {
            $this->error($exception->getMessage(), $sql);
        }

        $this->log($sql, 'inserts');

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
        $key = false;
        if (strpos($col, '|') !== false) {
            $array = explode('|', $col);
            $key = $array[1];
            $col = $array[0];
        }
        $data = $this->select($col, 'fetchAll');

        if ($data && $key) {
            $result = array();

            foreach ($data as $k => $v) {
                if (isset($v[$key])) {
                    if (isset($array[3]) && isset($v[$array[2]])) {
                        $result[$v[$key]][$v[$array[2]]] = $v;
                    } elseif (isset($array[2]) && isset($v[$array[2]])) {
                        $result[$v[$key]] = $v[$array[2]];
                    } elseif (isset($array[2])) {
                        $result[$v[$key]][] = $v;
                    } else {
                        $result[$v[$key]] = $v;
                    }
                }
            }

            return $result;
        }

        return $data;
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
    public function insert()
    {
        $sql = $this->sql->insert($this->table);

        if ($sql) {
            try {
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
    private function select($col = '', $method = 'fetch', $type = 'select')
    {
        $sql = $this->sql->{$type}($this->table, $col);

        $key = $this->table . '_' . $method . '_' . md5(serialize($this->value) . $sql);

        $data = $this->cache($key);

        if ($data) {
            if ($col != 'clear') {
                $this->value = array();
            }

            return $data;
        }

        if ($type == 'count' && strpos($sql, 'group by `')) {
            $method = 'fetchAll';
        }

        try {
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

        $data = $handle->$method();

        //print_r($data);

        $this->cache($key, $data);

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
        if (is_array($param[0]) && $method != 'order') {
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
            }
        }

        $this->sql->$method($param);
    }
}
