<?php namespace Dever\Data\Mongo;

use Dever\Data\Store as Base;
use MongoDB\Driver\Command;
use MongoDB\Driver\BulkWrite;
use MongoDB\Driver\Query;
use MongoDB\BSON\Regex;
use MongoDB\BSON\ObjectId;

class Store extends Base
{
    /**
     * init
     *
     * @return mixd
     */
    public function init()
    {
        return;
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
        $this->register();
        $this->table = $this->read->db . '.' . $table;
        $this->connect = $this->read->table($table);
    }

    /**
     * getIndex
     *
     * @return mixed
     */
    public function getIndex($version, $index)
    {
        $return = $this->connect->ensureIndex($index[$version], array('name' => ''));

        $this->log($index[$version], 'index');

        $data['index'] = $version;

        return $data;
    }

    /**
     * alter table
     *
     * @return mixed
     */
    public function alter($alter, $struct = array(), $name = '')
    {
        return true;
    }

    /**
     * query table
     *
     * @return mixed
     */
    public function query($sql, $state = true)
    {
        return true;
    }

    /**
     * exe table
     *
     * @return mixed
     */
    public function exe($sql, $value = array(), $method = 'fetchAll')
    {
        return true;
    }

    /**
     * insert the default value
     *
     * @return mixed
     */
    public function getInserts($value)
    {
        $col = explode(',', $value['col']);
        $value = explode(',', $value['value']);

        foreach ($col as $k => $v) {
            $this->value['add'][$v] = $value[$k];
            $this->insert();
        }

        $this->log($value, 'inserts');

        $data = include $file;

        $data['insert'] = $value;

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
        $data = $this->select($col, 'find');

        $result = array();

        if ($data) {
            foreach ($data as $k => $v) {
                $v = (array)$v;
                $v['id'] = (array) $v['_id'];
                $v['id'] = $v['id']['oid'];
                unset($v['_id']);
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
                } else {
                    $result[] = $v;
                }
            }
        }

        return $result;
    }

    /**
     * one
     *
     * @return array
     */
    public function one($col = '')
    {
        $data = $this->select($col, 'findOne');
        $result = array();

        if ($data) {
            foreach ($data as $k => $v) {
                $v = (array)$v;
                $v['id'] = (array) $v['_id'];
                $v['id'] = $v['id']['oid'];
                unset($v['_id']);
                $result = $v;
            }
        }

        return $result;
    }

    /**
     * count
     *
     * @return array
     */
    public function count($col = 'clear')
    {
        return $this->select($col, 'count');
    }

    /**
     * insert
     *
     * @return int
     */
    public function insert($num = 1)
    {
        $insert = array();
        foreach ($this->value['add'] as $k => $v) {
            if (is_numeric($v)) {
                $v = (float) $v;
            }
            $insert[$k] = $v;
        }
        $bulk = new BulkWrite;
        if ($num > 1) {
            $data = $insert;
            $insert = array();
            foreach ($i = 0; $i < $num; $i++) {
                $insert[] = $data;
            }
        }
        $id = $bulk->insert($insert);
        $result = $this->connect->executeBulkWrite($this->table, $bulk);
        $this->log($this->value, 'insert');
        $this->value = array();
        if ($result->getInsertedCount() >= 1) {
            return (string) $id;
        }
        return false;
    }

    /**
     * update
     *
     * @return int
     */
    public function update()
    {
        $update = array();
        foreach ($this->value['set'] as $k => $v) {
            if (is_numeric($v)) {
                $v = (float) $v;
            }
            $update[$k] = $v;
        }
        $update = array('$set' => $update);
        $param = array('multi' => true, 'upsert' => false);
        $bulk = new BulkWrite;
        $bulk->update($this->value['where'], $update, $param);
        $result = $this->connect->executeBulkWrite($this->table, $bulk);
        $this->log($this->value, 'update');
        $this->value = array();
        return $result->getModifiedCount();
    }

    /**
     * delete
     *
     * @return int
     */
    public function delete()
    {
        $bulk = new BulkWrite;
        $bulk->delete($this->value['where']);
        $result = $this->connect->executeBulkWrite($this->table, $bulk);
        $this->log($this->value, 'delete');
        $this->value = array();
        return $result->getDeletedCount();
    }

    /**
     * command
     *
     * @return mixd
     */
    public function command(array $param) {
        return $this->connect->executeCommand($this->table, new Command($param));
    }

    /**
     * select
     *
     * @return array
     */
    private function select($col = '', $method = 'find')
    {
        $filter = array();
        $options = array();
        if (isset($this->value['where'])) {
            $filter = $this->value['where'];
        }
        if (isset($filter['state'])) {
            $filter['state'] = (string) $filter['state'];
        }
        
        if ($method == 'count') {
            $query = new Query($filter, $options);
            $cursor = $this->connect->executeQuery($this->table, $query);
            $result = count($cursor->toArray());
        } else {
            if (isset($this->value['order'])) {
                $options['sort'] = $this->value['order'];
            }
            if (isset($this->value['limit'])) {
                foreach ($this->value['limit'] as $k => $v) {
                    $options['skip'] = $k;
                    $options['limit'] = $v;
                }
            }
            if ($col && $col != '*' && $col != 'clear') {
                if (is_string($col)) {
                    $temp = explode(',', $col);
                    $col = array();
                    foreach ($temp as $k => $v) {
                        $options['projection'][$v] = true;
                    }
                }
            }

            $query = new Query($filter, $options);
            $result = $this->connect->executeQuery($this->table, $query);
        }

        $this->log($this->value, 'select');

        if ($col != 'clear') {
            $this->value = array();
        }

        return $result;
    }

    /**
     * join
     *
     * @return object
     */
    public function join($param)
    {
        return $this;
    }

    /**
     * __call
     *
     * @return object
     */
    public function __call($method, $param)
    {
        if (is_array($param[0])) {
            foreach ($param[0] as $k => $v) {
                if ($method == 'order') {
                    $this->call($method, array($k, $v));
                } else {
                    $this->call($method, $v);
                }
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
        if (is_array($param) && isset($param[0])) {
            if ($param[0] == 'id') {
                $param[0] = '_id';
            }

            $func = 'convert_' . $method;
            if (method_exists($this, $func)) {
                $this->$func($param);
            }
            if (is_array($param) && isset($param[1])) {
                $this->value[$method][$param[0]] = $param[1];
            } else {
                $this->value[$method] = $param;
            }
        } else {
            $this->value[$method] = $param;
        }
    }

    /**
     * convert_order
     *
     * @return mixed
     */
    private function convert_order(&$param)
    {
        switch ($param[1]) {
            case 'desc':
                $param[1] = -1;
                break;
            case 'asc':
                $param[1] = 1;
                break;
        }
    }

    /**
     * convert_limit
     *
     * @return mixed
     */
    private function convert_limit(&$param)
    {
        $param = explode(',', $param[0]);
    }

    /**
     * convert_group
     *
     * @return mixed
     */
    private function convert_group(&$param)
    {
        print_r($param);die;
    }

    /**
     * convert_where
     *
     * @return mixed
     */
    private function convert_where(&$param)
    {
        if (is_numeric($param[1])) {
            $param[1] = (int) $param[1];
            return;
        }
        if (isset($param[2])) {
            $state = true;
            switch ($param[2]) {
                case 'like':
                    # 模糊查询
                    $param[1] = (string) $param[1];
                    if (strpos($param[1], '%') !== false) {
                        $param[1] = str_replace('%', '(.*?)', $param[1]);
                        $param[1] = new Regex($param[1], 'i');
                    } else {
                        $param[1] = new Regex($param[1] . '(.*?)', 'i');
                    }
                    $state = false;
                    break;

                case 'in':
                case 'nin':
                    # in查询
                    $param[1] = explode(',', $param[1]);
                    if ($param[0] == '_id') {
                        foreach ($param[1] as $k => $v) {
                            $param[1][$k] = new ObjectId($v);
                        }
                    }
                    $param[2] = '$' . $param[2];
                    break;

                case '>':
                    $param[2] = '$gt';
                    break;
                case '>=':
                    $param[2] = '$gte';
                    break;
                case '<':
                    $param[2] = '$lt';
                    break;
                case '<=':
                    $param[2] = '$lte';
                    break;
                case '!=':
                    $param[2] = '$ne';
                    break;
                case '%':
                    $param[2] = '$mod';
                    break;
                case 'bt':
                    $state = false;
                    $param[1] = array('gt' => $param[1][0], 'lt' => $param[1][1]);
                    break;
                case 'bte':
                    $state = false;
                    $param[1] = array('gte' => $param[1][0], 'lte' => $param[1][1]);
                    break;
                default:
                    $param[2] = '$' . $param[2];
                    break;
            }
            if ($state == true) {
                $param[1] = array($param[2] => $param[1]);
            }
        }

        if ($param[0] == '_id' && is_string($param[1])) {
            $param[1] = new ObjectId($param[1]);
        }
    }
}
