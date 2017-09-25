<?php namespace Dever\Data\Mysql;

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
     * @return Dever\Data\Mysql\Connect;
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
        $this->handle = @mysql_connect($config['host'], $config['username'], $config['password'], true);

        if (!$this->handle) {
            die('Could not connect: ' . mysql_error());
        }

        Debug::log('db ' . $config['host'] . ' connected', $config['type']);

        if (!mysql_select_db($config['database'], $this->handle)) {
            $this->query("CREATE DATABASE `" . $config['database'] . "` DEFAULT CHARACTER SET utf8 COLLATE utf8_general_ci");

            if (!mysql_select_db($config['database'], $this->handle)) {
                die("Can\'t use " . $config['database'] . " : " . mysql_error());
            }
        }

        $this->query("set names '" . $config['charset'] . "'");
        //$this->_log('connected mysql:' . $config['host']);
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
        @mysql_close($this->handle);
        $this->handle = null;
    }

    /**
     * fetchAll
     *
     * @return object
     */
    public function fetchAll($sql, $method = MYSQL_ASSOC)
    {
        $handle = $this->exec($sql);

        $result = array();

        while ($row = mysql_fetch_array($handle, $method)) {
            $result[] = $row;
        }

        return $result;
    }

    /**
     * fetch
     *
     * @return object
     */
    public function fetch($sql, $method = MYSQL_ASSOC)
    {
        $handle = $this->exec($sql);

        $result = mysql_fetch_array($handle, $method);

        return $result;
    }

    /**
     * exec
     *
     * @return object
     */
    public function exec($sql)
    {
        # 同步执行
        if (strpos($sql, ';')) {
            $temp = explode(';', $sql);
            foreach ($temp as $k => $v) {
                $this->exec($v);
            }

            return true;
        } else {
            return mysql_query($sql, $this->handle);
        }
    }

    /**
     * query
     *
     * @return object
     */
    public function query($sql)
    {
        $this->exec($sql);

        return $this;
    }

    /**
     * rowCount
     *
     * @return object
     */
    public function rowCount()
    {
        return mysql_affected_rows();
    }

    /**
     * fetchColumn
     *
     * @return object
     */
    public function fetchColumn($sql)
    {
        $handle = $this->exec($sql);

        $result = mysql_fetch_row($handle);

        return $result[0];
    }

    /**
     * lastid
     *
     * @return int
     */
    public function id()
    {
        return mysql_insert_id($this->handle);
    }

    /**
     *
     * @desc 释放结果内存
     * @param  $query SQL语句
     * @return  Boolean
     * @author alfa  2011-2-17
     */
    public function freeResult($query)
    {
        return @mysql_free_result($query);
    }

    /**
     * @desc 得到错误编号
     * @return (int)错误编号
     * @author alfa  2011-2-17
     */
    public function getErrno()
    {
        return mysql_errno();
    }

    /**
     *
     * @desc 得到错误消息
     * @return (string)错误消息
     * @author alfa  2011-2-17
     */
    public function getError()
    {
        return mysql_error();
    }
}
