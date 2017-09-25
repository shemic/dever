<?php namespace Dever\Data\Model;

use Dever\Data\Sql;

class Link
{
    /**
     * table
     *
     * @var string
     */
    protected $table;

    /**
     * col
     *
     * @var string
     */
    protected $col;

    /**
     * bind
     *
     * @var array
     */
    protected $bind;

    /**
     * where
     *
     * @var array
     */
    protected $where;

    /**
     * order
     *
     * @var array
     */
    protected $order;

    /**
     * group
     *
     * @var array
     */
    protected $group;

    /**
     * limit
     *
     * @var array
     */
    protected $limit;

    /**
     * page
     *
     * @var array
     */
    protected $page;

    /**
     * cache
     *
     * @var string
     */
    protected $cache;

    /**
     * db
     *
     * @var Dever\Data\Model
     */
    protected $db;

    /**
     * sql
     *
     * @var Dever\Data\Sql
     */
    protected $sql;

    /**
     * __construct
     *
     * @return mixed
     */
    public function __construct($db)
    {
        $this->db = $db;
        $this->sql = Sql::getInstance();
    }

    /**
     * fetch
     *
     * @return mixd
     */
    public function fetch($method = 'fetch')
    {
        $sql = $this->sql->select($this->table, $this->col);
        echo $sql;die;
        return $this->db->$method($sql, $this->bind, $this->page, $this->cache);
    }

    /**
     * fetchAll
     *
     * @return mixd
     */
    public function fetchAll()
    {
        return $this->fetch('fetchAll');
    }

    /**
     * table
     *
     * @return mixd
     */
    public function table($name)
    {
        $this->table = $name;
        return $this;
    }

    /**
     * col
     *
     * @return mixd
     */
    public function col($col)
    {
        $this->col = $col;
        return $this;
    }

    /**
     * order
     *
     * @return mixd
     */
    public function order()
    {
        return $this;
    }

    /**
     * group
     *
     * @return mixd
     */
    public function group($group)
    {
        $this->sql->group($group);
        return $this;
    }

    /**
     * limit
     *
     * @return mixd
     */
    public function limit($limit)
    {
        $this->sql->limit(array($limit));
        return $this;
    }

    /**
     * page
     *
     * @return mixd
     */
    public function page($page)
    {
        $this->page = $page;
        return $this;
    }

    /**
     * cache
     *
     * @return mixd
     */
    public function cache($cache)
    {
        $this->cache = $cache;
        return $this;
    }

    /**
     * where
     *
     * @return mixd
     */
    public function where($param)
    {
        $this->param($param);
        if ($this->where) {
            foreach ($this->where as $k => $v) {
                $this->sql->where($v);
            }
        }
        
        return $this;
    }

    /**
     * param
     *
     * @return mixd
     */
    public function param($param)
    {
        $this->where = array();
        foreach ($param as $k => $v) {
            if (strpos($k, 'where_') !== false || strpos($k, 'option_') !== false) {
                $k = str_replace(array('where_', 'option_'), '', $k);
                $key = ':' . count($this->where);
                if (is_array($v) && isset($v[0])) {
                    $this->bind[$key] = $v[0];
                    $v[0] = $key;
                    array_unshift($v, $k);
                    $this->where[$k] = $v;
                } else {
                    $this->bind[$key] = $v;
                    $this->where[$k] = array($k, $key);
                }
            } elseif ($k == 'limit' || $k == 'order' || $k == 'group') {
                $this->$k = $v;
            } else {
                $k = str_replace(array('add_', 'set_'), '', $k);
                $this->update[$k] = $v;
            }
        }
    }
}
