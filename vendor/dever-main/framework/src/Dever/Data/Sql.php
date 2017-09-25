<?php namespace Dever\Data;

class Sql
{
    /**
     * prefix
     *
     * @var string
     */
    private $prefix = '';
    
    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * getInstance
     *
     * @return Dever\Data\Sql;
     */
    public static function getInstance()
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
        }

        return self::$instance->init();
    }

    public function setColOrder($order)
    {
        $this->colOrder = $order;
    }

    /**
     * create
     *
     * @return string
     */
    public function create($table, $struct, $type = '', $partition = '')
    {
        $create = $primary = array();

        foreach ($struct as $k => $v) {
            if (isset($v['table']) && $v['table'] != $table) {
                continue;
            }
            
            $primary[$k] = '';
            if (is_array($v)) {
                if (!isset($v['type'])) {
                    continue;
                }
                $com = $v;
                $v = $com['type'];
                if (strpos($com['type'], 'text') !== false) {
                    $primary[$k] .= '';
                } elseif (strpos($com['type'], 'int') === false) {
                    $primary[$k] .= 'not null default \'\'';
                } elseif (!empty($com['default'])) {
                    $default = $com['default'];
                    $com['default'] = '{default}';
                    $primary[$k] .= 'not null default \'' . $com['default'] . '\'';
                } elseif ($k != 'id' && strpos($com['type'], 'int') !== false) {
                    $primary[$k] .= 'not null default 0';
                } else {
                    $primary[$k] .= 'not null';
                }

                if (!empty($com['name'])) {
                    $primary[$k] .= ' comment \'' . $com['name'] . '\'';
                }
            } elseif (is_string($v) && strpos($v, ' ') !== false) {
                $com = explode(' ', $v);
                $v = $com[0];
                if (!empty($com[1])) {
                    $default = $com[1];
                    $com[1] = '{default}';
                    $primary[$k] .= 'not null default \'' . $com[1] . '\'';
                } else {
                    $primary[$k] .= 'not null';
                }

                if (!empty($com[2])) {
                    $primary[$k] .= ' comment \'' . $com[2] . '\'';
                }
            }
            if ($k == 'id') {
                $primary[$k] = 'unsigned auto_increment primary key ' . $primary[$k];
            }

            $v = '`' . $k . '` ' . strtoupper(str_replace('-', '(', $v) . ') ' . $primary[$k] . ''); // not null
            if (strpos($v, '{DEFAULT}') && isset($default)) {
                $v = str_replace('{DEFAULT}', $default, $v);
            }
            $create[] = $v;
        }
        $sql = 'DROP TABLE IF EXISTS `' . $table . '`;CREATE TABLE `' . $table . '`(' . implode(',', $create) . ')';

        //$sql    = 'CREATE TABLE `' . $table . '`(' . implode(',', $create) . ')';

        if ($type) {
            $sql .= ' ENGINE = ' . $type . ';';
        }


        if ($partition) {
            foreach ($partition['value'] as $k => $v) {
                $partition['value'][$k] = 'PARTITION ' . $table .'_' . $k . ' VALUES ' . strtoupper($partition['exp']) . ' (' . $v . ')';
            }
            if (stristr($partition['exp'],'THEN')) {
                $k = $k + 1;
                $v = 'MAXVALUE';
                $partition['value'][$k] = 'PARTITION ' . $table .'_' . $k . ' VALUES ' . strtoupper($partition['exp']) . ' ' . $v . '';
            }
            
            $sql .= 'PARTITION BY ' . strtoupper($partition['type']).'('.$partition['col'].') ) (
                '.implode(',', $partition['value']).'
            );';
        }

        //echo $sql;die;

        return $sql;
    }

    /**
     * alter
     *
     * @return string
     */
    public function alter($table, $config)
    {
        $create = $primary = array();

        $alter = 'ALTER TABLE `' . $table;

        $sql = array();
        foreach ($config as $k => $v) {
            if (isset($v['type'])) {
                $v = array
                    (
                    'add', $k, $k, $v['type'] . ' ' . $v['default'] . ' ' . $v['name'],
                );
            }

            if (empty($v[3])) {
                continue;
            }
            $sql[$k] = '';
            if (isset($v[3]) && strpos($v[3], ' ') !== false) {
                $com = explode(' ', $v[3]);
                $sql[$k] = str_replace('-', '(', $com[0]) . ') ';
                if (isset($com[1]) && $com[1] != '') {
                    $sql[$k] .= 'not null default \'' . $com[1] . '\'';
                } else {
                    $sql[$k] .= 'not null';
                }

                if (!empty($com[2])) {
                    $sql[$k] .= ' comment \'' . $com[2] . '\'';
                }
                $sql[$k] = strtoupper($sql[$k]);
            }

            if ($v[0] == 'add') {
                # 新增字段
                $sql[$k] = $alter . '` ADD `' . $v[1] . '` ' . $sql[$k];
            } elseif ($v[0] == 'delete') {
                # 删除字段
                $sql[$k] = $alter . '` DROP `' . $v[1] . '`';
            } else {
                # 修改字段
                $sql[$k] = $alter . '` CHANGE `' . $v[1] . '` `' . $v[2] . '` ' . $sql[$k];
            }
        }

        $sql = implode(';', $sql);

        return $sql;
    }

    /**
     * col
     *
     * @return string
     */
    private function col($col)
    {
        $result = '';

        if (is_array($col)) {
            $array = array();
            foreach ($col as $k => $v) {
                if (!is_numeric($k)) {
                    $array[] = $this->prefix . $k . ' AS ' . $v;
                } else {
                    $array[] = $this->prefix . $v;
                }
            }
            $result = implode(' ', $array);
        } else {
            $result = $col ? $this->prefix . $col : $this->prefix . '*';
        }

        if (isset($this->col) && $this->col) {
            $result .= ',' . $this->col;
        }

        if (isset($this->score) && $this->score) {
            $result .= ',(' . implode('+', $this->score) . ') as score';
        }

        if (isset($this->as) && $this->as) {
            $result .= ','.implode(',', $this->as);
        }
        return $result;
    }

    /**
     * select
     *
     * @return string
     */
    public function select($table, $col = '')
    {
        $this->table = $table;
        $where = $this->createWhere();

        $join = isset($this->join) ? implode(' ', $this->join) : '';

        if (isset($this->orderBy) && $this->orderBy) {
            $order = $this->orderBy;
        } else {
            $order = $this->order;
        }
        $sql = 'SELECT ' . $this->col($col) . ' FROM `' . $table . '` ' . $join . $where . ' ' . $this->group . ' ' . $order . ' ' . $this->limit;

        $this->init();

        return $sql;
    }

    /**
     * createWhere
     *
     * @return string
     */
    private function createWhere()
    {
        $where = '';
        if ($this->where) {
            if (isset($this->colOrder)) {
                ksort($this->where);
            }

            if (isset($this->between)) {
                $where = 'WHERE ' . $this->between . ' ' . implode(' ', $this->where);
                $this->limit = '';
            } else {
                $where = 'WHERE ' . ltrim(implode(' ', $this->where), 'and');
            }
        }

        return $where;
    }

    /**
     * count
     *
     * @return string
     */
    public function count($table, $col = '')
    {
        $where = $this->createWhere();

        $state = 1;
        if ($col == 'clear') {
            $col = '';
            $state = 2;
        }

        if (!$col || $col == '*') {
            $col = 'count(*) as total';
        }

        $join = isset($this->join) ? implode(' ', $this->join) : '';

        $sql = 'SELECT ' . $col . ' FROM `' . $table . '` ' . $join . $where . ' ' . $this->group . ' ';

        if ($state == 1) {
            $this->init();
        }

        return $sql;
    }

    /**
     * showIndex
     *
     * @return string
     */
    public function showIndex($table)
    {
        $sql = 'SHOW INDEX FROM ' . $table . ' ';

        return $sql;
    }

    /**
     * dropIndex
     *
     * @return string
     */
    public function dropIndex($table, $name)
    {
        $sql = 'ALTER TABLE ' . $table . ' DROP INDEX ' . $name;

        return $sql;
    }

    /**
     * index
     *
     * @return string
     */
    public function index($table, $value)
    {
        $sql = 'ALTER TABLE ' . $table . ' ';

        $max = count($value) - 1;

        $i = 0;

        foreach ($value as $k => $v) {
            $type = 'INDEX';
            if (strpos($v, '.')) {
                $t = explode('.', $v);
                $v = $t[0];
                $type = ucwords($t[1]);
            }
            $sql .= 'ADD ' . $type . ' ' . $k . ' (' . $v . ')';

            if ($i >= $max) {
                $sql .= '';
            } else {
                $sql .= ',';
            }

            $i++;
        }

        return $sql;
    }

    /**
     * insert
     *
     * @return string
     */
    public function insert($table)
    {
        $sql = 'INSERT INTO `' . $table . '` (' . implode(',', $this->col) . ') VALUES (' . implode(',', $this->value) . ')';

        $this->init();

        return $sql;
    }

    /**
     * inserts
     *
     * @return string
     */
    public function inserts($table, $col, $value)
    {
        $sql = 'INSERT INTO `' . $table . '` (' . $col . ') VALUES ';

        $max = count($value) - 1;

        foreach ($value as $k => $v) {
            $sql .= '(' . $v . ')';

            if ($k >= $max) {
                $sql .= '';
            } else {
                $sql .= ',';
            }
        }

        return $sql;
    }

    /**
     * explain
     *
     * @return string
     */
    public function explain($sql)
    {
        $sql = 'EXPLAIN ' . $sql . ' ';

        return $sql;
    }

    /**
     * update
     *
     * @return string
     */
    public function update($table)
    {
        if (!$this->where) {
            return false;
        }

        $where = $this->createWhere();

        $sql = 'UPDATE `' . $table . '` SET ' . implode(',', $this->value) . ' ' . $where;

        //echo $sql;die;

        $this->init();

        return $sql;
    }

    /**
     * delete
     *
     * @return string
     */
    public function delete($table)
    {
        if (!$this->where) {
            return false;
        }

        $where = $this->createWhere();

        if (!$where) {
            return false;
        }

        $sql = 'DELETE FROM `' . $table . '` ' . $where;

        $this->init();

        return $sql;
    }

    /**
     * truncate
     *
     * @return string
     */
    public function truncate($table)
    {
        $sql = 'TRUNCATE TABLE `' . $table . '`';

        return $sql;
    }

    /**
     * opt
     *
     * @return string
     */
    public function opt($table)
    {
        $sql = 'OPTIMIZE TABLE `' . $table . '`';

        return $sql;
    }

    /**
     * sql
     *
     * @return string
     */
    public function sql($sql)
    {
        return $sql;
    }

    /**
     * init
     *
     * @return object
     */
    public function init()
    {
        $this->where = $this->value = $this->col = $this->join = $this->score = $this->as = array();
        $this->order = '';
        $this->orderBy = '';
        $this->group = '';
        $this->limit = '';
        $this->prefix = '';

        return $this;
    }

    /**
     * where
     *
     * @return string
     */
    public function where($param)
    {
        $where = '';
        if (empty($param[2])) {
            $param[2] = '=';
        }

        $value = 1;
        if (strpos($param[2], '|') !== false) {
            $temp = explode('|', $param[2]);
            $param[2] = $temp[0];
            $value = $temp[1];
        }

        if (empty($param[3])) {
            $param[3] = 'and';
        }

        if (strpos($param[3], ')') !== false) {
            $temp = explode(')', $param[3]);
            $param[3] = $temp[0];
            $where = ' )';
        }

        $col = '`s_' . $param[0] . '`';
        if (!strpos($param[0], '.')) {
            $param[0] = $this->prefix . '`' . $param[0] . '`';
        }

        if ($param[2] == 'like') {
            $instr = 'instr(' . $param[0] . ', ' . $param[1] . ')';
            $where = $param[3] . ' ' . $instr . ' > 0' . $where;
            $this->orderBy = 'order by score desc';
            $this->score[] = 'IF('.$instr.', IF(' . $param[0] . '=' . $param[1] . ', 1000*'.$value.', (100-'.$instr.')*'.$value.'), 0)';
            //$this->as[] = ' CONCAT("<em class=\"dever_highlight\">",' . $param[0] . ',"</em>") as ' . $col;
        } else {
            $where = $param[3] . ' ' . $param[0] . ' ' . $param[2] . ' ' . $param[1] . $where;
        }

        if ($param[2] == 'in') {
            $param[1] = str_replace(array('(',')'), '', $param[1]);
            $this->orderBy = 'order by field(' . $param[0] . ', ' . $param[1] . ')';
        }

        if (isset($this->colOrder)) {
            if (isset($this->colOrder[$param[0]])) {
                $this->where[$this->colOrder[$param[0]]] = $where;
            } else {
                $num = count($this->where) + 100;
                $this->where[$num] = $where;
            }
        } else {
            $this->where[] = $where;
        }
    }

    /**
     * order
     *
     * @return string
     */
    public function order($param)
    {
        if (is_array($param[0])) {
            $this->order = 'order by ';
            foreach ($param[0] as $k => $v) {
                $k1 = '';
                if (strpos($k, '.')) {
                    $t = explode('.', $k);
                    $k = $t[1];
                    $k1 = $t[0] . '.';
                }
                $order[] = $k1 . '`' . $k . '` ' . $v;
            }

            $this->order .= implode(',', $order);

            //echo $this->order;die;
        } else {
            if (empty($param[1])) {
                $param[1] = 'desc';
            }

            $this->order = 'order by `' . $param[0] . '` ' . $param[1];
        }
    }

    /**
     * group
     *
     * @return string
     */
    public function group($param)
    {
        if (is_array($param) && isset($param[0])) {
            //$param = trim(implode(',', $param), ',');
            $param = $param[0];

            $this->group = ' group by ' . $param;
        }

        # 去掉id的分组，没用
        if (is_string($param) && $param != 'id') {
            if ($param == 'day') {
                $this->col = 'FROM_UNIXTIME(cdate, "%Y-%m-%d") as day';
            } elseif ($param == 'month') {
                $this->col = 'FROM_UNIXTIME(cdate, "%Y-%m") as month';
            } elseif ($param == 'year') {
                $this->col = 'FROM_UNIXTIME(cdate, "%Y") as year';
            } elseif (strpos($param, ',') === false) {
                $this->col = $param;
            }

            $this->group = ' group by ' . $param;
        }
    }

    /**
     * join 为临时解决方案，不建议用join
     *
     * @return string
     */
    public function join($param)
    {
        $this->prefix = 't_1.';
        $this->join[] = 'as t_1';
        $num = 2;
        foreach ($param as $k => $v) {
            if ($v) {
                $table = 't_' . $num;
                if (strpos($v['on'][0], '.') === false) {
                    $v['on'][0] = 't_1.`' . $v['on'][0] . '`';
                } else {
                    $v['on'][0] = 't_' . $v['on'][0];
                }

                $v['on'][1] = $table . '.`' . $v['on'][1] . '`';

                $v['table'] = str_replace('/', '_', $v['table']);
                if (DEVER_PROJECT != 'default') {
                    $v['table'] = DEVER_PROJECT . '_' . $v['table'];
                }

                $this->join[] = ucwords($v['type']) . ' `' . $v['table'] . '` AS ' . $table . ' ON ' . $v['on'][0] . '=' . $v['on'][1] . ' ';

                if (isset($v['col'])) {
                    $this->col = $v['col'];
                }

                $num++;
            }
        }
    }

    /**
     * limit
     *
     * @return string
     */
    public function limit($param)
    {
        //if(empty($param[1])) $param[1] = 0;

        //$this->limit = 'limit ' . $param[1] . ',' . $param[0];
        $this->limit = 'limit ' . $param[0];

        //$this->between = ' `id` BETWEEN ' . $param[1] . ' AND ' . ($param[1] + $param[0]);
    }

    /**
     * reset limit
     *
     * @return string
     */
    public function reset($param)
    {
        $this->{$param[0]} = '';
    }

    /**
     * add
     *
     * @return string
     */
    public function add($param)
    {
        $this->col[] = '`' . $param[0] . '`';
        $this->value[] = $param[1];
    }

    /**
     * set
     *
     * @return string
     */
    public function set($param)
    {
        if (empty($param[2])) {
            $param[2] = '=';
        }

        $param[0] = '`' . $param[0] . '`';

        if (strpos($param[2], '+=') !== false) {
            $param[2] = '=' . $param[0] . '+';
        }

        if (strpos($param[2], '-=') !== false) {
            $param[2] = '=' . $param[0] . '-';
        }

        $this->value[] = $param[0] . $param[2] . $param[1];
    }
}
