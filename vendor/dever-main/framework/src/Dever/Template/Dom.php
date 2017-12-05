<?php namespace Dever\Template;

use Dever\Loader\Config;
use Dever\Loader\Lang;
use Dever\Output\Debug;
use Dever\Template\Compile;
use Dever\Template\Parse;
use Sunra\PhpSimple\HtmlDomParser;
use Dever\Http\Url;

class Dom
{
    /**
     * current
     *
     * @var object
     */
    protected $current;

    /**
     * data
     *
     * @var string
     */
    protected $data;

    /**
     * for
     *
     * @var string
     */
    protected $for;

    /**
     * expression
     *
     * @var string
     */
    protected $expression;

    /**
     * loop
     *
     * @var bool
     */
    protected $loop = false;

    /**
     * key
     *
     * @var string
     */
    protected $key;

    /**
     * judge
     *
     * @var object
     */
    protected $judgeDom;

    /**
     * outertext
     *
     * @var string
     */
    protected $outertext = '';

    /**
     * attr
     *
     * @var string
     */
    protected $attr = 'outertext';

    /**
     * parsing
     *
     * @var \Dever\Template\Parsing
     */
    protected $parsing;

    /**
     * dom
     *
     * @var \Sunra\PhpSimple\HtmlDomParser
     */
    protected $dom;

    /**
     * temp dom
     *
     * @var \Sunra\PhpSimple\HtmlDomParser
     */
    protected $temp;

    /**
     * __construct
     *
     * @return mixed
     */
    public function __construct($value, Parsing $parsing)
    {
        $this->parsing = $parsing;

        $this->load($value);

        $this->filter();
        $this->import('include');
        $this->import('.include');
        $this->link();
    }

    /**
     * load file or string
     * @param string $file
     *
     * @return mixed
     */
    public function load($value)
    {
        $strip = Config::get('template')->strip;
        if (is_file($value)) {
            $this->dom = HtmlDomParser::file_get_html($value,
                $use_include_path   = false, 
                $context            = null, 
                $offset             = -1, 
                $maxLen             = -1, 
                $lowercase          = true, 
                $forceTagsClosed    = true, 
                $target_charset     = DEFAULT_TARGET_CHARSET, 
                $stripRN            = $strip, 
                $defaultBRText      = DEFAULT_BR_TEXT, 
                $defaultSpanText    = DEFAULT_SPAN_TEXT);
        } else {
            $this->dom = HtmlDomParser::str_get_html($value,
                $lowercase          = true, 
                $forceTagsClosed    = true, 
                $target_charset     = DEFAULT_TARGET_CHARSET, 
                $stripRN            = $strip, 
                $defaultBRText      = DEFAULT_BR_TEXT, 
                $defaultSpanText    = DEFAULT_SPAN_TEXT);
        }
    }

    /**
     * fetch
     * @param array $param
     *
     * @return mixed
     */
    public function fetch($param)
    {
        $this->data($param[1]);

        $this->current($param[0]);

        $this->loop = $this->child = false;

        if (isset($param[2]) && $param[2]) {
            $this->fetchAttr($param[2]);
        }

        $this->handle();
    }

    /**
     * fetchAttr
     * @param array $param
     *
     * @return mixed
     */
    private function fetchAttr($param)
    {
        if ($this->current && is_string($this->data)) {
            $value = $this->parsing->checkGlobal($this->data);
            if (!$value && strpos($this->data, 'Dever::') === false && (strpos($this->data, '.') || strpos($this->data, '-'))) {
                $value = '<{Dever::load(\'' . $this->data . '\')}>';
            }

            if (!$value) {
                $value = $this->data;
            }
            if (is_string($param)) {
                $this->command($param, $value);
            } else {
                $this->child($param);
            }
        }
    }

    /**
     * command
     * @param array $param
     *
     * @return mixed
     */
    private function command($command, $value)
    {
        switch ($command) {
            case 'none':
                $value = str_replace(array('<{', '}>'), array('<{if(!(', ')):}>'), $value);
                $this->current->style = $this->parsing->content($value . 'display:none;<{endif;}>');
                break;
            case 'remove':
                $content = str_replace(array('<{', '}>'), array('<{if((', ')):}>'), $value);
                $this->current->outertext = $this->parsing->content($content . $value . '<{endif;}>');
                break;
            default:
                $this->child($command);
                break;
        }
    }

    /**
     * render
     * @param array $param
     *
     * @return mixed
     */
    public function render($param)
    {
        $this->loop = $this->child = false;
        if (is_array($param[1])) {
            $send[1] = $param[0];
            foreach ($param[1] as $k => $v) {
                $send[0] = $k;
                $send[2] = $v;
                $this->fetch($send);
            }
        } else {
            $temp = $param;
            $param[0] = $temp[1];
            $param[1] = $temp[0];
            $this->fetch($param);
        }
    }

    /**
     * loop
     * @param array $param
     *
     * @return mixed
     */
    public function loop($param)
    {
        $this->data($param[1]);

        $this->current($param[0]);

        $this->loop = $this->child = true;

        if (isset($param[2]) && $param[2]) {
            $this->child($param[2]);
        }

        $this->handle();
    }

    /**
     * jq
     * @param array $param
     *
     * @return mixed
     */
    public function jq($param)
    {
        echo 'error';die;
    }

    /**
     * data
     *
     * @return string
     */
    public function data($value)
    {
        $this->for = '';
        if (strpos($value, '[') !== false && strpos($value, '["') === false) {
            preg_match('/\[(.*?)\]/i', $value, $matches);
            if (isset($matches[1])) {
                $value = str_replace($matches[0], '', $value);
                $this->for = $matches[1];
            }
        }
        $this->data = $value;
        $this->judgeDom = false;
    }

    /**
     * get
     *
     * @return string
     */
    public function get()
    {
        $this->layout();
        $content = $this->dom->save();

        if ($this->parsing->global) {
            $content = implode(PHP_EOL, $this->parsing->global) . PHP_EOL . $content;
        }
        return $content;
    }

    /**
     * child
     * @param array $child
     *
     * @return array
     */
    public function child($child)
    {
        $this->child = true;
        if ($this->current) {
            $judge = '';
            if (is_string($child) || (is_array($child) && isset($child[0]))) {
                $this->setting($this->current, $this->attr, $child);
            } elseif (is_array($child)) {
                $this->expression = '';
                $this->attr = 'outertext';
                foreach ($child as $k => $v) {
                    if (strpos($k, 'if') !== false || strpos($k, 'else') !== false) {
                        $state = false;
                        $judge .= $this->judge($k, $v);
                        if ($k == 'endif' && $v == 1) {
                            $this->find('judge', 1, $judge);
                            $judge = '';
                        }
                    } else {
                        if ($k == 'self') {
                            $this->attribute($v, $this->current);
                        } elseif ($k == 'parent') {
                            $this->parent($v);
                        } else {
                            $this->find($k, $v, $judge);
                        }
                        $judge = '';
                    }
                }
            }
        }
    }

    /**
     * parent
     * @param array $value
     *
     * @return mixed
     */
    private function parent($value, $judge = '')
    {
        $parent = $this->current->parent();
        if (isset($value['number']) && $value['number'] >= 2) {
            for ($i = 2; $i <= $value['number']; $i++) {
                $parent = $parent->parent();
            }
        }

        $this->attribute($value, $parent);
    }

    /**
     * find
     * @param string $key
     * @param array $value
     *
     * @return mixed
     */
    private function find($key, $value, $judge = '')
    {
        if (strpos($key, '|') !== false) {
            $temp = explode('|', $key);
            list($key, $index) = $temp;
            if (!is_numeric($index)) {
                $this->each($temp, $key, $value, $index, $this->current, $judge);
                return;
            }
        } else {
            $index = 0;
            if (isset($value['key'])) {
                $index = $value['key'];unset($value['key']);
            }
        }

        $dom = $this->current->find($key, $index);
        if ($dom) {
            $this->attribute($value, $dom);
        } else {
            $this->setting($this->current, $key, $value, $index);
        }

        if ($judge) {
            $this->judgeDom->outertext = $judge;
        }
    }

    /**
     * judge
     * @param string $key
     * @param array $value
     *
     * @return array
     */
    private function judge(&$key, $value = '')
    {
        if (!$this->judgeDom) {
            $this->judgeDom =& $this->current;
        }
        if (strpos($key, '|')) {
            $temp = explode('|', $key);
            $key = $temp[0];
        }
        if ($key == 'endif') {
            $content = $this->parsing->content('<{' . $key . ';}>');
        } else {
            if (is_array($value)) {
                foreach ($value as $k => $v) {
                    $this->find($k, $v);
                    $value = $this->judgeDom->outertext;
                }
                //$value = implode('', $value);
            } else {
                $value = $this->parsing->content('<{' . $value . '}>');
            }
            //$value = '{content}';
            $content = $this->parsing->content('<{' . $key . ':}>') . $value;
        }
        return $content;
    }

    /**
     * each
     * @param string $temp
     * @param string $k
     * @param array $v
     * @param string $index
     * @param object $dom
     *
     * @return array
     */
    private function each($temp, $key, $value, $index, $dom, $judge = '')
    {
        $temp[2] = isset($temp[2]) ? $temp[2] : 0;

        if ($key == 'self') {
            $child = $dom;
        } else {
            $dom = $dom->find($key);

            foreach ($dom as $i => $j) {
                if ($i == $temp[2]) {
                    $child = $j;
                } else {
                    $j->outertext = '';
                }
            }
        }
        
        if (isset($child)) {
            $this->attribute($value, $child);

            $num = 1;
            $temp = str_replace('$v', '', $index);
            $temp = explode('.', $temp);
            if ($temp[0]) {
                $num = $temp[0] + 1;
            }

            $child->outertext = $this->parsing->logic($index, $child->outertext, $num);
            if ($this->judgeDom) {
                $this->judgeDom =& $child;
            }
            if ($judge) {
                $child->outertext = $judge . $child->outertext;
            }
        }
    }

    /**
     * attribute
     * @param array $value
     * @param object $dom
     *
     * @return mixed
     */
    private function attribute($value, $dom, $key = 0)
    {
        $judge = '';
        $data  = '';
        $setting = '';
        if (is_array($value)) {
            foreach ($value as $k => $v) {
                if ($k == 'html') {
                    $data = $v;
                } elseif (strpos($k, 'if') !== false || strpos($k, 'else') !== false) {
                    $judge .= $this->judge($k, $v);
                } else {
                    $index = 0;
                    if (strpos($k, '|') !== false) {
                        $temp = explode('|', $k);
                        list($k, $index) = $temp;
                        if (!is_numeric($index)) {
                            $this->each($temp, $k, $v, $index, $dom);

                            continue;
                        }
                    }
                    $setting .= $this->setting($dom, $k, $v, $index);

                }
            }

            if ($this->temp) {
                $this->temp = null;
            }

        } else {
            $data = $value;
        }

        if ($setting) {
            preg_match_all('/<'.$dom->tag.'(.*?)">(.*?)<\/'.$dom->tag.'>/i', $dom->outertext, $matches);
            if ($matches[0]) {
                $attr = '';
                if (isset($matches[1][0]) && $matches[1][0]) {
                    $attr = $matches[1][0] . '"';
                }
                $dom->outertext = '<' . $dom->tag . $attr .'>' . $setting . '</'.$dom->tag.'>';
            }
        } elseif ($data) {
            $dom->innertext = $this->parsing->content($data);
        }

        if ($judge) {
            if (strpos($judge, $dom->innertext) === false) {
                $dom->innertext = $judge . $dom->innertext;
            } else {
                $dom->innertext = $judge;
            }
            
        }
    }

    /**
     * filter
     *
     * @return mixed
     */
    private function filter()
    {
        $dom = $this->dom->find('filter');

        foreach ($dom as $k => $v) {
            $dom[$k]->outertext = '';
        }

        $dom = $this->dom->find('dever');

        foreach ($dom as $k => $v) {
            $dom[$k]->outertext = '';
        }
    }

    /**
     * import
     *
     * @return mixed
     */
    private function import($name)
    {
        $dom = $this->dom->find($name);
        if ($dom) {
            foreach ($dom as $k => $v) {
                if (isset($v->path)) {
                    $v->file = $v->path . $v->file;
                }

                $v->outertext = $this->parsing->load($v->file, $v->system);
            }
        }
    }

    /**
     * link
     *
     * @return mixed
     */
    private function link()
    {
        $dom = $this->dom->find('a');

        foreach ($dom as $k => $v) {
            if (isset($v->src) && $v->src) {
                $v->src = Url::get($v->src);
            }
        }
    }

    /**
     * layout
     *
     * @return mixed
     */
    private function layout()
    {
        if (Config::get('template')->layout) {
            $dom = $this->dom->find(Config::get('template')->layout);
            if (isset($dom[0]) && $dom[0]->outertext) {
                foreach ($dom as $k => $v) {
                    $v->innertext = $this->parsing->content('<{endif;}>' . $v->innertext . '<{if(isset($_SERVER["HTTP_X_PJAX"])):}><{else:}>');
                }

                $html = $this->dom->find('html');
                if (isset($html[0]) && $html[0]->outertext) {
                    $html[0]->outertext = $this->parsing->content('<{if(isset($_SERVER["HTTP_X_PJAX"])):}><{else:}>' . $html[0]->outertext . '<{endif;}>');
                }
            }
        }
    }

    /**
     * setting
     * @param object $dom
     * @param string $attribute
     * @param string $value
     * @param int $index
     *
     * @return mixed
     */
    private function setting($dom, $attribute, $value, $index = 0)
    {
        if (is_array($value) && isset($value[0])) {
            $this->child = false;
            $param = $value;
            $key = $this->parsing->val($this->data);
            if ($this->col) {
                $key .= '[\''.$this->col.'\']';
            }
            if ($param[1] && $param[1] == 'none') {
                $dom->style = $this->parsing->content('<{if(!'.$key.'):}>display:none;<{endif;}>');
            } elseif ($param[1] && $param[1] == 'remove') {
                $dom->outertext = $this->parsing->content('<{if('.$key.'):}>'.$dom->innertext.'<{endif;}>');
            }
            $value = '<{if('.$key.'):}>'.$value[0].'<{else:}>'.$value[1].'<{endif;}>';
        } elseif (is_array($value)) {
            $child = $dom->find($attribute, $index);

            if (!$child) {
                if (!$this->temp) {
                    # 这里因为dom类对append支持的不好，所以只能重新读取一次
                    if ($index > 0) {
                        # 当有一个节点时，后续的所有节点均使用第一个节点为模板
                        $this->temp = HtmlDomParser::str_get_html($dom->innertext);
                    } else {
                        # 清空原有父节点的全部内容
                        $dom->innertext = '';
                        # 当没有节点时，直接创建
                        $this->temp = HtmlDomParser::str_get_html("<$attribute></$attribute>");
                    }
                }

                $child = $this->temp->find($attribute, 0);
            }
            $this->attribute($value, $child);

            if ($this->temp) {
                $dom->innertext .= $this->temp->innertext;
            }

            return $child->outertext;
        }

        if ($attribute == 'html') {
            $attribute = 'innertext';
        }
        # modal
        elseif ($attribute == 'modal') {
            $dom->{'data-am-modal'} = '{target: \'#DEVER_modal\', closeViaDimmer: 0}';
            $dom->{'href'} = '#DEVER_modal';
            $dom->{'data-toggle'} = 'modal';

            $attribute = 'onclick';

            if (strpos($value, '|') !== false) {
                $temp = explode('|', $value);
                $dom->{'data-modal-title'} = $temp[0];
                $dom->{'data-modal-content'} = $temp[1];
            } else {
                $dom->{'data-modal-title'} = '提醒您';
                $dom->{'data-modal-content'} = $value;
            }
            $value = '$(\'#DEVER_modal_title\').html($(this).attr(\'data-modal-title\'));$(\'#DEVER_modal_body\').html($(this).attr(\'data-modal-content\'))';
        }

        $value = $this->parsing->content($value, $this->data);

        if (strpos($attribute, '++') !== false) {
            $attribute = str_replace('++', '', $attribute);

            if (!strstr($dom->$attribute, $value)) {
                $dom->$attribute = $dom->$attribute . $value;
            }
        } elseif (strpos($attribute, '--') !== false) {
            $attribute = str_replace('--', '', $attribute);

            if (strpos($dom->$attribute, $value) !== false) {
                $dom->$attribute = str_replace($value, '', $dom->$attribute);
            }
        } else {
            $dom->$attribute = $value;
        }
        return '';
    }

    /**
     * parse
     * @param string $parse
     *
     * @return mixed
     */
    private function parse(&$parse)
    {
        if (strpos($parse, '@') !== false) {
            $temp = explode('@', $parse);
            $parse = $temp[0];
            $array = array
            (
                'html' => 'innertext',
            );
            $this->attr = isset($array[$temp[1]]) ? $array[$temp[1]] : $temp[1];
        }
    }

    /**
     * current
     * @param array $parse
     *
     * @return mixed
     */
    private function current($parse)
    {
        //$this->attr = 'outertext';
        $this->col = '';
        $this->attr = 'innertext';
        if (is_array($parse)) {
            $this->parse($parse[0]);
            $this->current = $this->dom->find($parse[0], $parse[1]);
        } else {
            $index = 0;
            if (strpos($parse, '|') !== false) {
                $temp = explode('|', $parse);
                $parse = $temp[0];
                $index = $temp[1];
                if (!is_numeric($index) && strpos($index, '.')) {
                    $temp = explode('.', $index);
                    $this->col = $temp[1];
                }
            }
            $this->parse($parse);
            $dom = $this->dom->find($parse);

            if ($dom) {
                foreach ($dom as $k => $v) {
                    if ($k == $index) {
                        $this->current = $v;
                    } else {
                        $dom[$k]->outertext = '';
                    }
                }
            } else {
                Debug::log(Lang::get('dom_exists', $parse));
            }
        }

        if (!$this->current) {
            Debug::log(Lang::get('dom_exists', $parse));
        } else {
            $this->expression = $this->current->innertext;
        }

        return $this;
    }

    /**
     * handle
     *
     * @return array
     */
    private function handle()
    {
        if ($this->current) {
            $this->current->{$this->attr} = $this->parsing->handle($this->data, $this->current->{$this->attr}, $this->expression, $this->loop, $this->col, $this->child, $this->for);
        }
    }
}
