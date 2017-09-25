<?php namespace Dever\Template;

use Dever\Loader\Config;
use Dever\Loader\Import;

class Parsing
{
	/**
     * left
     *
     * @var const string
     */
    const LEFT = '<?php ';

    /**
     * right
     *
     * @var const string
     */
    const RIGHT = ' ?>';

    /**
     * path
     *
     * @var string
     */
    protected $path;

    /**
     * global
     *
     * @var string
     */
    public $global;

    /**
     * __construct
     */
    public function __construct($path = '')
    {
    	$this->path = $path;
    }

    /**
     * load view
     * @param  string $file
     * @param  string $path
     *
     * @return string
     */
    public function load($file, $path = '')
    {
        $desc = '<!--{' . $path . '/' . $file . '}-->';

        $view = View::getInstance($file);

        if ($path) {
            $view->path($path);
        } else {
            $view->path($this->path);
        }

        $view->runing();

        $file = $view->file();

        if ($file) {
            if (strpos($file, '//') !== false) {
                $file = str_replace('//', '/', $file);
            }
            if (Config::get('base')->data) {
                $require = 'Dever::config("base")->data';
                $path = Config::get('base')->data;
            } else {
                $require = 'DEVER_PATH . "data" . DIRECTORY_SEPARATOR';
                $path = DEVER_PATH . 'data' . DIRECTORY_SEPARATOR;
            }
            return $desc . $this->script('require ' . $require . ' . \'' . str_replace($path, '', $file) . '\'') . $desc;
        }
    }

    /**
     * script
     * @param string $string
     *
     * @return string
     */
    public function script($string)
    {
        return self::LEFT . $string . self::RIGHT;
    }

    /**
     * equal
     * @param string $variable
     * @param string $value
     * @param string $key
     *
     * @return string
     */
    public function equal($variable, $value, $key = '')
    {
        if (strpos($key, '$') !== false) {
            $variable .= '[' . $key . ']';
        } elseif (strpos($key, 'Dever') !== false) {
            $variable .= '[\'' . $key . '\']';
        } elseif ($key) {
            $variable .= '[\'' . $key . '\']';
        }

        if (is_array($value)) {
            $value = var_export($value, true);
        } elseif (is_string($value) && strpos($value, '"') !== false && strpos($value, 'Dever::') === false) {
            $value = '\'' . $value . '\'';
        }
        return $this->script('$' . $variable . '=' . $value);
    }

    /**
     * data
     * @param string $data
     *
     * @return mixed
     */
    public function data($data)
    {
        if (is_object($data)) {
            return $data();
        }

        $type = $this->strip($data);
        $global = $this->checkGlobal($type);
        if ($global) {
            return true;
        }

        # include page
        if (strpos($type, '@') !== false) {
            return explode('@', $type);
        }
        # include database|model
        elseif (strpos($type, 'http://') === false && strpos($type, '/') !== false && (strpos($type, '.') !== false || strpos($type, '-') !== false || strpos($type, '!') !== false)) {
            $key = $type;
            if (strpos($type, '|') !== false) {
                if (strpos($type, '!') !== false) {
                    $type = str_replace('!', '$', $type . '');
                }
                $temp = explode('|', $type);
                $type = $temp[0];
                parse_str($temp[1], $param);
                // print_r($param);die;
            }

            $callback = 'Dever::load(\'' . $type . '\')';

            if (isset($param)) {
                $param = var_export($param, true);
                $callback = str_replace(')', ', ' . $param . ')', $callback);
                if (strpos($callback, '$') !== false) {
                    $callback = preg_replace('/\'\$(.*?)\'/', '\$$1', $callback);
                }
                if (strpos($callback, 'Dever::') !== false) {
                    $callback = preg_replace('/\'\Dever::(.*?)\'/', "Dever::$1", $callback);
                }
            }

            $this->push($key, $this->equal('data', $callback, $key));

            return true;
        }

        return $data;
    }

    /**
     * out echo variable
     * @param string $variable
     *
     * @return string
     */
    public function out($variable)
    {
        if (strpos($variable, '$(') !== false) {
            return $variable;
        }
        if (strpos($variable, '$') === 0) {
            return $this->script('echo ' . $variable);
        }
        return $this->script('echo $' . $variable);
    }

    /**
     * each
     * @param string $replace
     * @param string $data
     * @param string $content
     *
     * @return string
     */
    public function each($replace, $method, $content, $loop = false, $key = '', $for = '')
    {
        if ($replace) {
            $strip = $this->strip($replace);
            if ($strip && strpos($replace, $strip) !== false) {
                $replace = $strip;
            }
        }

        //$val = '$data[\'' . $method . '\']';
        $val = $this->val($method);

        if ($key) {
            $content = str_replace($val, $val . '[\'' . $key . '\']', $content);
        }
        if (!$content) {
            $content = $this->out($val);
        }

        $result = '';
        if ($loop == true) {
            if ($this->checkFor($for)) {
                $for = explode('-', $for);
                if (isset($for[1])) {
                    $if = '$i >= '.$for[0].' && $i <= '.$for[1];
                } else {
                    $if = '$i == \''.$for[0].'\'';
                }
                $content = $this->script('if('.$if.'):') . $content . $this->script('endif;');
            }
            $result = $this->script('if(isset('.$val.') && is_array('.$val.')):')
            . $this->equal('t', 'count('.$val.')-1')
            . $this->equal('i', 0)
            . $this->script('foreach('.$val.' as $k => $v):')
            . $content
            . $this->equal('i', '$i+1')
            . $this->script('endforeach;')
            . $this->script('else:')
            . $this->replace($replace, $this->script('echo ' . $val), $content)
            . $this->script('endif;');
        } else {
            $result = $this->script('if('.$val.' && is_array('.$val.')):')
            . $this->replace('$v[', $val.'[', $content)
            //. $this->replace('echo $v', 'echo ' . $val.'', $content)
            . $this->script('else:')
            . $this->replace($replace, $this->script('echo ' . $val), $content)
            . $this->script('endif;');
        }

        return $result;
    }

    private function callback($matches)
    {
        //print_r($matches);die;

        $key = '';

        if ($this->count > 0) {
            $key = $this->count;
        }

        $result = 'if(isset(' . $matches[1] . ') && is_array(' . $matches[1] . ')): foreach(' . $matches[1] . ' as $k' . $key . ' => $v' . $key . '):';

        $this->count++;

        return $result;
    }

    /**
     * content
     * @param string $content
     *
     * @return string
     */
    public function content($content, $data = false)
    {
        if (strpos($content, '<{') === false && strpos($content, '$(') !== false) {
            return $content;
        }

        if ((strpos($content, '$') !== false || strpos($content, 'Dever::') !== false) && strpos($content, '<{') === false && strpos($content, '<?php') === false) {
            $content = '<{' . $content . '}>';
        }
        $echo = ' echo ';

        $content = $this->rule($content);

        $this->count = 0;

        //$content  = preg_replace('/<{\$(.*?)=(.*?)}>/i', self::LEFT.'\$$1=$2' . self::RIGHT, $content);
        $content = preg_replace('/<{\$([a-zA-Z0-9_\'\"\[\]\s]+)=([a-zA-Z0-9_\'\"\[\]\s]+)}>/i', self::LEFT . '\$$1=$2' . self::RIGHT, $content);

        if (strpos($content, 'loop(')) {
            $content = preg_replace_callback('/loop\((.*?)\):/i', array($this, 'callback'), $content);
            $content = $this->replace('endloop', 'endforeach;endif;', $content);
        }

        if (strpos($content, ' = ') && strpos($content, 'var') === false) {
            $content = $this->replace('<{', self::LEFT, $this->replace('}>', self::RIGHT, $content));
        } else {
            $content = $this->replace('<{', self::LEFT . $echo, $this->replace('}>', self::RIGHT, $content));
        }

        $array = array('function', 'foreach', 'endforeach', 'if', 'endif', 'else', 'for', 'endfor', 'highlight_string', 'echo', 'print_r', 'is_array', '$(');
        
        foreach ($array as $k => $v) {
            if (is_numeric($k)) {
                $k = $v;
            }
            if (strpos($content, self::LEFT . $echo . $k) !== false) {
                $content = $this->replace(self::LEFT . $echo . $k, self::LEFT . $v, $content);
            }
        }

        if (strpos($content, '{self}') !== false) {
            $content = $this->replace('{self}', $this->val($data), $content);
        }



        return $content;
    }

    /**
     * logic
     * @param string $logic
     * @param string $string
     *
     * @return string
     */
    public function logic($logic, $string, $index = 1)
    {
        $this->index = $index;
        # 这里暂时这样判断，以后再处理多种逻辑情况的
        if (strpos($logic, '|') !== false) {
            list($handle, $logic) = explode('|', $logic);

            if ($logic == 'foreach') {
                $string = '<{if(isset(' . $handle . ') && ' . $handle . ' && is_array(' . $handle . ')):foreach(' . $handle . ' as $k' . $this->index . ' => $v' . $this->index . '):if($v' . $this->index . '):}>' . $string . '<{endif;endforeach;endif;}>';
            } elseif ($logic == 'if') {
                $string = '<{if(isset(' . $handle . ') && ' . $handle . '):}>' . $string . '<{endif;}>';
            } else {
                $string = '<{' . $handle . '}>' . $string . '<{' . $logic . '}>';
            }
        } else {
            $string = '<{if(isset(' . $logic . ') && ' . $logic . ' && is_array(' . $logic . ')):foreach(' . $logic . ' as $k' . $this->index . ' => $v' . $this->index . '):if($v' . $this->index . '):}>' . $string . '<{endif;endforeach;endif;}>';
        }

        return $this->content($string);
    }

    /**
     * replace
     * @param string $replace
     * @param string $value
     * @param string $content
     *
     * @return string
     */
    public function replace($replace, $value, $content)
    {
        if (!$replace) {
            return $value;
        }

        if (is_string($replace) && strpos($content, $replace) !== false) {
            $content = str_replace($replace, $value, $content);
        }

        return $content;
    }

    /**
     * push
     * @param string $value
     *
     * @return string
     */
    public function push($key, $value)
    {
        $this->global[$key] = $value;
    }

    /**
     * checkGlobal
     * @param string $key
     *
     * @return string
     */
    public function checkGlobal($key)
    {
        if (strpos($key, '$') !== 0) {
            $key = '$' . $key;
        }
        if (isset($this->global[$key]) && $this->global[$key]) {
            return $key;
        }
        return false;
    }

    /**
     * val
     * @param string $key
     *
     * @return string
     */
    public function val($key)
    {
        $global = $this->checkGlobal($key);
        if ($global) {
            return $global;
        } else {
            return '$data[\''.$key.'\']';
        }
    }

    /**
     * set
     * @param string $value
     *
     * @return string
     */
    public function set($key, $value = false)
    {
        if (strpos($key, '$') !== 0 && strpos($key, 'Dever') !== 0) {
            $key = '$' . $key;
        }

        if (!$value) {
            return $this->push($key, $this->script($this->rule($key)));
        }

        if (is_string($value)) {
            if (strpos($value, 'Dever::') === false) {
                $data = Import::load($value);
                if ($data) {
                    $value = 'Dever::load(\'' . $value . '\')';
                } else {
                    $value = var_export($value, true);
                }
            }

            $value = $this->rule($value);
        } elseif (is_object($value)) {
            $value = $value();
        }

        $this->push($key, $this->script($this->rule($key) . '=' . $value . ''));
    }

    /**
     * tag
     * @param string $key
     * @param string $data
     *
     * @return string
     */
    public function tag($key, $data = false)
    {
        if ($data) {
            $result = $data[$key];
        } else {
            $result = $key;
        }
        return $result;
    }

    /**
     * handle
     * @param string $data
     * @param string $content
     * @param string $expression
     *
     * @return string
     */
    public function handle($data, $content, $expression = '', $loop = false, $key = '', $child = false, $for = '')
    {
        $result = '';

        if (is_array($data)) {
            $tags = $this->strip($content);
            foreach ($data as $k => $v) {
                $result .= $this->replace($tags, $v, $content);
            }
        } else {
            $index = false;

            if (is_string($data) && strpos($data, '#') > 0 && strpos($data, '"') === false) {
                list($data, $index) = explode('#', $data);
            }

            $method = $this->data($data);

            if ($method === true) {
                $result = $this->complex($data, $content, $index, $expression, $loop, $key, $child, $for);
            } elseif (is_array($method)) {
                $result = $this->load($method[1], $method[0]);
            } elseif ($method && (is_string($method) || is_numeric($method))) {
                $method = $this->content($method);

                $result = $this->replace($content, $method, $content);

                if ($result == $content) {
                    //$result = $this->replace($this->strip($content), $method, $content);
                }
            } elseif (!$result && (is_string($data) || is_numeric($data))) {
                $data = $this->content($data);

                $result = $this->replace($this->strip($content), $data, $content);
            } else {
                $result = $content;
            }
        }

        return $result;
    }

    /**
     * complex
     * @param string $data
     * @param string $content
     * @param string $index
     * @param string $expression
     *
     * @return string
     */
    public function complex($data, $content, $index = false, $expression = '', $loop = false, $key = '', $child = false, $for = '')
    {
        if ($index) {
            $strip = $this->strip($content);
            $val = $this->val($data);
            if ($this->checkFor($for)) {
                $val .= '[\''.$for.'\']';
            }
            $result = $this->out($val . '[\'' . $index . '\']');
            if ($strip == $content) {
                $result = $this->replace($strip, $result, $content);
            }
        } elseif($child) {
            if ($expression) {
                $content = $this->replace($expression, $this->out('v'), $content);
            }
            $result = $this->each($content, $data, $this->content($content), $loop, $key, $for);
        } elseif (strpos($content, '<?php') !== false) {
            $result = $content;
        } else {
            $strip = $this->strip($content);
            $val = $this->val($data);
            if ($this->checkFor($for)) {
                $val .= '[\''.$for.'\']';
            }
            $result = $this->out($val);
            if ($strip == $content) {
                $result = $this->replace($strip, $result, $content);
            }
        }

        return $result;
    }

    /**
     * rule
     * @param string $content
     *
     * @return string
     */
    public function rule($content)
    {
        if (strpos($content, 'request.') !== false) {
            $content = preg_replace('/request\.([a-zA-Z0-9]+)/', 'Dever::input(\'$1\')', $content);
        }

        if (strpos($content, '$') !== false && strpos($content, '.') !== false) {
            $rule = '\$([a-zA-Z0-9]+)\.([a-zA-Z0-9._]+)';

            $content = preg_replace_callback('/' . $rule . '/i', array($this, 'rule_val'), $content);
        }

        if (strpos($content, '"+') !== false) {
            $content = str_replace(array('"+', '+"'), array('".', '."'), $content);
        }

        if (strpos($content, '++') !== false) {
            $content = str_replace('++', '.', $content);
        }

        if (strpos($content, '<{$') !== false) {
            //$content = str_replace('<{$', '<{echo $', $content);
        }

        return $content;
    }

    /**
     * rule_val
     * @param array $result
     *
     * @return string
     */
    public function rule_val($result)
    {
        if (isset($result[2]) && $result[2]) {
            $result[2] = '$' . $result[1] . '' . preg_replace('/\.([a-zA-Z0-9_]+)/', '[\'$1\']', '.' . $result[2]);

            return $result[2];
        }
        return $result[0];
    }

    /**
     * strip
     * @param string $content
     *
     * @return string
     */
    public static function strip($content)
    {
        if (is_string($content)) {
            return strip_tags($content);
        }
        return $content;
    }

    /**
     * checkFor
     * @param string $for
     *
     * @return bool
     */
    private function checkFor($for)
    {
        if ($for || $for === '0') {
            return true;
        } else {
            return false;
        }
    }
}
