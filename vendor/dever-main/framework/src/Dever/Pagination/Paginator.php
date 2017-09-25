<?php namespace Dever\Pagination;

use Dever\Http\Url;
use Dever\Routing\Input;
use Dever\Routing\Uri;

class Paginator
{
    /**
     * PAGE
     *
     * @var string
     */
    const PAGE = 'page/';

    /**
     * total
     *
     * @var int
     */
    public $total;

    /**
     * html
     *
     * @var string
     */
    public $html = '';

    /**
     * start
     *
     * @var int
     */
    public $start;

    /**
     * end
     *
     * @var int
     */
    public $end;

    /**
     * num
     *
     * @var int
     */
    public $num;

    /**
     * first_num
     *
     * @var int
     */
    public $first_num = 0;

    /**
     * maxpage
     *
     * @var int
     */
    public $maxpage = 10;

    /**
     * page
     *
     * @var int
     */
    public $page;

    /**
     * prev
     *
     * @var int
     */
    public $prev;

    /**
     * next
     *
     * @var int
     */
    public $next;

    /**
     * current
     *
     * @var int
     */
    public $current;

    /**
     * template
     *
     * @var string
     */
    public $template;

    /**
     * link
     *
     * @var string
     */
    public $link;

    /**
     * array
     *
     * @var array
     */
    public $array;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * getInstance
     *
     * @return Dever\Pagination\Paginator
     */
    public static function getInstance($key = 'current')
    {
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self();
        }

        return self::$instance[$key];
    }

    /**
     * getPage
     *
     * @return Dever\Pagination\Paginator;
     */
    public static function getPage($key, $total = false, $template = '')
    {
        if (is_numeric($total) && $total > 0) {
            self::getInstance($key)->total($total);
            self::getInstance($key)->offset(1);
            self::getInstance($key)->template($key);
            self::getInstance($key)->link();
        }

        $result = self::getInstance($key)->handle($template);

        if ($total == 'NEXT') {
            if (self::getInstance($key)->next) {
                $result = self::getInstance($key)->href(self::getInstance($key)->next);
            } else {
                $result = '';
            }
        }
        return $result;
    }

    /**
     * getTotal
     *
     * @return Dever\Pagination\Paginator;
     */
    public static function getTotal($key)
    {
        return self::getInstance($key)->total();
    }

    /**
     * getHtml
     *
     * @return Dever\Pagination\Paginator;
     */
    public static function getHtml($key, $template = '')
    {
        return self::getPage($key, false, $template);
    }

    /**
     * toArray
     *
     * @return array
     */
    public function toArray()
    {
        if (empty($this->array)) {
            $total = $this->total();
            if ($total > 0) {
                $current = $this->current();
                $this->setArray($total, $current);
            }
        }

        return $this->array;
    }

    /**
     * setArray
     *
     * @return array
     */
    private function setArray($total, $current)
    {
        $totalPage = $this->getAllPage();
        $status = 1;
        if ($current >= $totalPage) {
            $status = 0;
        }
        $this->array = array
        (
            'total' => $total,
            'current_page' => $current,
            'total_page' => $totalPage,
            'next_page' => $this->next(),
            'prev_page' => $this->prev(),
            'html' => $this->handle(),
            'status' => $status,
        );
    }

    /**
     * getCurrent
     *
     * @return int
     */
    public function current($name = 'pg')
    {
        $this->current = Input::get($name, 1);
        if ($this->total && $this->current > $this->total) {
            $this->current = $this->total;
        }

        return $this->current;
    }

    /**
     * setMaxPage
     *
     * @return mixd
     */
    public function setMaxPage($maxpage = 10)
    {
        $this->maxpage = $maxpage;
    }

    /**
     * getPage
     *
     * @return int
     */
    public function getAllPage()
    {
        if (!$this->total) {
            return 0;
        }

        if ($this->first_num > 0) {
            $this->page = ceil(($this->total - $this->first_num) / $this->num) + 1;
        } else {
            $this->page = ceil($this->total / $this->num);
        }

        return $this->page;
    }

    /**
     * offset
     *
     * @return int
     */
    public function offset($num, $first_num = 0)
    {
        $this->num = $num;
        $current = $this->current();

        if ($first_num > 0) {
            $this->first_num = $first_num;
            if ($current == 1) {
                $this->num = $first_num;
                $first_num = 0;
            } else {
                $current = $current - 1;
            }
        }

        $offset = $first_num + $this->num * ($current-1);
        
        return $offset;
    }

    /**
     * total
     *
     * @return mixd
     */
    public function total($total = false)
    {
        if (!$total) {
            return $this->total ? $this->total : 0;
        }

        $this->total = $total;
    }

    /**
     * maxpage
     *
     * @return mixd
     */
    public function maxpage($maxpage)
    {
        $this->maxpage = $maxpage;
    }

    /**
     * link
     *
     * @return mixd
     */
    public function link($link = '')
    {
        if (!$link) {
            $link = Uri::$url;
        }

        $this->linkReplaceStr($link, 'pg');

        $this->linkReplaceStr($link, 'pt');

        $this->linkSearch($link);

        return $this->link = $link;
    }

    /**
     * linkSearch
     *
     * @return mixd
     */
    private function linkSearch(&$link)
    {
        $search = Input::prefix('search_');

        if ($search) {
            foreach ($search as $k => $v) {
                if ($v && strpos($link, $k) === false) {
                    $link .= '&' . $k . '=' . $v;
                }
            }
        }
    }

    /**
     * linkReplaceStr
     *
     * @return mixd
     */
    private function linkReplaceStr(&$link, $string)
    {
        if (strpos($link, $string . '=') !== false) {
            $link = preg_replace('/[?|&]' . $string . '=(\d+)/i', '', $link);
        }
    }

    /**
     * template
     *
     * @return mixd
     */
    public function template($template)
    {
        $this->template = $template;
    }

    /**
     * data
     *
     * @return mixd
     */
    public function data($content, $total = 0)
    {
        $this->total = $total;
        if ($content) {
            if ($this->total <= 0) {
                $this->total = count($content);
            }

            $this->current();

            $data = isset($content[$this->current - 1]) ? $content[$this->current - 1] : array();

            return $data;
        }

        return array();
    }

    /**
     * sqlCount
     *
     * @return mixd
     */
    public function sqlCount($sql, $offset, $total = 0, $db, $data)
    {
        $this->total = $total;

        if ($this->total <= 0) {
            $this->sqlCountQuery($sql, $db, $data);
        }

        $data_sql = $sql . ' LIMIT ' . $offset . ', ' . $this->num;
        $result = $db->query($data_sql, $data)->fetchAll();

        return $result;
    }

    /**
     * sqlCountQuery
     *
     * @return mixd
     */
    private function sqlCountQuery($sql, $db, $data)
    {
        $sql = mb_strtolower($sql);
        if (strstr($sql, ' from ')) {
            $temp = explode(' from ', $sql);
        }

        if (isset($temp[1])) {
            if (strstr($temp[1], ' order ')) {
                $temp = explode(' order ', $temp[1]);
                $sql = $temp[0];
            } else {
                $sql = $temp[1];
            }

            $sql = 'SELECT count(1) as num FROM ' . $sql;
            $this->total = $db->query($sql, $data)->fetchColumn();
        }
    }

    /**
     * handle
     *
     * @return mixd
     */
    public function handle($template = '')
    {
        if ($this->html) {
            return $this->html;
        }

        if ($this->total < 1) {
            return '';
        }

        $this->getAllPage();

        if ($this->page < $this->current) {
            $this->current = $this->page;
        }

        if ($this->page <= 1) {
            return '';
        }

        if ($this->total > $this->num) {
            if ($this->current > 1) {
                $this->prev = $this->current - 1;
            }

            if ($this->current < $this->page) {
                $this->next = $this->current + 1;
            }

            if ($this->page <= $this->maxpage) {
                $this->start = 1;
                $this->end = $this->page;
            } else {
                $page = intval($this->maxpage / 2);
                if ($this->current < $page) {
                    $this->start = 1;
                } elseif ($this->current <= ($this->page - $this->maxpage)) {
                    $this->start = $this->current - $page;
                } elseif ($this->current > $this->page - $this->maxpage && $this->current <= $this->page - $page) {
                    $this->start = $this->current - $page;
                } elseif ($this->current > $this->page - $page) {
                    $this->start = $this->page - $this->maxpage + 1;
                }
                $this->end = $this->start + $this->maxpage - 1;

                if ($this->start < 1) {
                    $this->end = $this->current + 1 - $this->start;
                    $this->start = 1;
                    if (($this->end - $this->start) < $this->maxpage) {
                        $this->end = $this->maxpage;
                    }
                } elseif ($this->end > $this->page) {
                    $this->start = $this->page - $this->maxpage + 1;
                    $this->end = $this->page;
                }
            }
        }

        return $this->getTemplate($template);
    }

    /**
     * get
     *
     * @return string
     */
    public function getTemplate($template = '')
    {
        if ($this->html) {
            return $this->html;
        }

        $template = $template ? $template : $this->template;
        $file = DEVER_APP_PATH . self::PAGE . $template . '.php';

        if (is_file($file)) {
            $html = new Html($this);
            $page = $this;

            include $file;

            return $this->html = $html->get();
        }

        return '';
    }

    /**
     * next
     *
     * @return string
     */
    public function next()
    {
        return $this->current < $this->page ? $this->current + 1 : $this->page;
    }

    /**
     * next
     *
     * @return string
     */
    public function prev()
    {
        return $this->current > 1 ? $this->current - 1 : 1;
    }

    /**
     * href
     *
     * @return string
     */
    public function href($page = false)
    {
        $page = $page ? $page : $this->current;
        //return Url::get($this->link . '&pg=' . $page . '&pt=' . $this->total) . $this->ext;
        return Url::get($this->link . '&pg=' . $page, DEVER_APP_NAME);
    }
}
