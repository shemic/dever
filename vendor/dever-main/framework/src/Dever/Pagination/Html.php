<?php namespace Dever\Pagination;

/*
# 定义父节点的类型、属性等（整个page的节点）
$html->parent = array('div', 'class="half-bg-bottom page margin-bottom-30"');
# 定义子节点的类型、属性等（每个page的节点）
$html->child = array();
# 定义上一页的名称、样式
$html->prev = array('&lt;', 'pre');
# 定义下一页的名称、样式
$html->next = array('&gt;', 'next');
# 定义每个页数的样式，当前页的样式 样式写在哪 是否和旧样式共用
$html->page = array('page', 'current', 'current', true);
# 定义开始页
$html->start = false;
# 定义结束页
$html->end = false;
# 定义跳转页
$html->jump = false;
# 定义扩展信息
$html->ext = '';
# 生成
$html->create();
 */
class Html
{
    /**
     * parent
     *
     * @var array
     */
    public $parent;

    /**
     * child
     *
     * @var array
     */
    public $child;

    /**
     * prev
     *
     * @var array
     */
    public $prev = array('prev', 'prev');

    /**
     * next
     *
     * @var array
     */
    public $next = array('next', 'next');

    /**
     * page
     *
     * @var array
     */
    public $page = array('page', 'current', '', false);

    /**
     * start
     *
     * @var array
     */
    public $start = false;

    /**
     * end
     *
     * @var array
     */
    public $end = false;

    /**
     * jump
     *
     * @var array
     */
    public $jump = false;

    /**
     * ext
     *
     * @var string
     */
    public $ext = '';

    /**
     * html
     *
     * @var string
     */
    public $html = '';

    /**
     * paginator
     *
     * @var object
     */
    private $paginator;

    /**
     * __construct
     *
     * @return mixd
     */
    public function __construct(Paginator $paginator)
    {
        $this->paginator = $paginator;
        $this->html = '';
    }

    /**
     * get
     */
    public function get()
    {
        return $this->html;
    }

    /**
     * create
     */
    public function create()
    {
        /*
        if ($ext) {
            $this->ext = $ext;
        }
        */

        $html = '';

        if (empty($current[2])) {
            $current[2] = '';
        }

        $this->start()
            ->prev()
            ->page()
            ->next()
            ->end()
            ->jump();

        $this->html = $this->tag($this->parent, $this->html);
    }

    /**
     * start
     */
    private function start()
    {
        if ($this->start && $this->paginator->current > 1) {
            $this->html .= $this->set($this->start[1], 1, $this->start[0], $this->page[2]);
        }
        return $this;
    }

    /**
     * prev
     */
    private function prev()
    {
        if ($this->prev) {
            $this->html .= $this->set($this->prev[1], $this->paginator->prev, $this->prev[0], $this->page[2]);
        } elseif (isset($prev[2])) {
            $this->html .= $this->set($this->prev[1], $this->paginator->current, $this->prev[0], $this->page[2]);
        }
        return $this;
    }

    /**
     * page
     */
    private function page()
    {
        if ($this->page[1]) {
            $i = $this->paginator->start;

            for ($i; $i <= $this->paginator->end; $i++) {
                $this->html .= $this->set($this->getPageClass($i), $i, $i, $this->page[2]);
            }
        }
        return $this;
    }

    /**
     * getPageClass
     */
    private function getPageClass($index)
    {
        $class = $this->page[0];
        if ($index == $this->paginator->current) {
            if (isset($this->page[3]) && $this->page[3] == true) {
                $class = $this->page[1];
            } else {
                if ($class) {
                    $class .= ' ';
                }

                $class .= $this->page[1];
            }
        }

        return $class;
    }

    /**
     * end
     */
    private function end()
    {
        if ($this->end && $this->paginator->current < $this->paginator->page) {
            $this->html .= $this->set($this->end[1], $this->paginator->page, $this->end[0], $this->page[2]);
        }
        return $this;
    }

    /**
     * next
     */
    private function next()
    {
        if ($this->next) {
            $this->html .= $this->set($this->next[1], $this->paginator->next, $this->next[0], $this->page[2]);
        } elseif (isset($next[2])) {
            $this->html .= $this->set($this->next[1], $this->paginator->end, $this->next[0], $this->page[2]);
        }
        return $this;
    }

    /**
     * jump
     */
    private function jump()
    {
        if ($this->jump) {
            $click = 'onclick="var link=\'' . $this->paginator->href('{1}') . '\';location.href=link.replace(\'{1}\', document.getElementById(\'dever_page\').value)"';
            $this->html .= str_replace('{click}', $click, $this->jump);
        }
        return $this;
    }

    /**
     * set
     *
     * @return string
     */
    public function set($class, $num, $name, $type = '')
    {
        if ($type == 'parent') {
            $this->child[1] = 'class="' . $class . '"';
            $class = '';
        }
        return $this->tag($this->child, $this->getContent($class, $num, $name));
    }

    /**
     * getContent
     *
     * @return string
     */
    private function getContent($class, $num, $name)
    {
        if ($this->child[0] == 'a') {
            $this->child[1] = $this->attr($class, $this->paginator->href($num));
            $content = $name;
        } else {
            $content = $this->tag(array('a', $this->attr($class, $this->paginator->href($num))), $name);
        }

        return $content;
    }

    /**
     * tag
     *
     * @return string
     */
    public function tag($tag, $content)
    {
        if (!$tag) {
            return $content;
        }
        $attr = '';
        if (is_array($tag)) {
            $temp = $tag;unset($tag);
            $tag = $temp[0];
            $attr = $temp[1];
        }
        return '<' . $tag . ' ' . $attr . '>' . $content . '</' . $tag . '>';
    }

    /**
     * attr
     *
     * @return string
     */
    public function attr($class, $href)
    {
        return ' class="' . $class . '" href="' . $href . '" ';
    }
}
