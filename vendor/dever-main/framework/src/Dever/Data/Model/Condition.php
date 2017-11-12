<?php namespace Dever\Data\Model;

use Dever\Loader\Project;
use Dever\Loader\Config;
use Dever\Routing\Input;
use Dever\Output\Export;

class Condition
{
    /**
     * method
     *
     * @var array
     */
    const METHOD = 'where,option,set,add,order,limit,group,page';

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * load
     *
     * @return mixed
     */
    public static function get()
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
        }

        return self::$instance;
    }

    /**
     * method
     *
     * @return mixd
     */
    public function init($request, $struct, $param, $project, $name, $db)
    {
        $this->request = $request;
        $this->param = $param;
        $this->struct = $struct;
        $this->project = $project;
        $this->name = $name;
        $this->db = $db;
        $this->update = false;

        $this->join();
        $method = explode(',', self::METHOD);
       
        foreach ($method as $one) {
            if (isset($this->request[$one])) {
                if (method_exists($this, $one)) {
                    $this->$one($this->request[$one]);
                } else {
                    if ($one == 'set' || $one == 'add') {
                        $this->update = true;
                    }
                    $this->handle($this->request[$one], $one);
                }
            }
        }
    }

    /**
     * order
     *
     * @return mixd
     */
    protected function method($config, $method)
    {
        if (isset($this->request['input'])) {
            $config = Input::get($method, $config);
        } elseif (isset($this->param[$method])) {
            $config = $this->param[$method];
        }

        if (is_string($config)) {
            $config = array($config, '');
        }

        if(empty($config[0])) $config[0] = $config;
        if(empty($config[1])) $config[1] = '';

        $this->opt($method, $config);

        Database::get($this->db)->$method($config[0], $config[1]);
    }

    /**
     * order
     *
     * @return mixd
     */
    protected function order($config)
    {
        $this->method($config, 'order');
    }

    /**
     * limit
     *
     * @return mixd
     */
    protected function limit($config)
    {
        $this->method($config, 'limit');
    }

    /**
     * group
     *
     * @return mixd
     */
    protected function group($config)
    {
        $this->method($config, 'group');
    }

    /**
     * page
     *
     * @return mixd
     */
    protected function page($config)
    {
        if (is_string($config[1])) {
            $temp[] = $config[1];
            unset($config[1]);
            $config[1] = $temp;
        }

        if (isset($this->param['page']) && $this->param['page'] && $this->param['page'] != $config) {
            $config[0] = $this->param['page'][0];
            unset($this->param['page'][0]);
            if (isset($this->param['page'][1])) {
                $config[1] = array_merge(array(), $this->param['page']);
            }
        }

        if(isset($config[2])) $config[1][2] = $config[2];

        $this->opt('page', $config);

        Database::get($this->db)->page($config[0], $config[1]);
    }

    /**
     * handle
     *
     * @return mixd
     */
    protected function handle($config, $method)
    {
        $send = array();
        foreach ($config as $key => $value) {
            $temp = array();
            if (is_array($value)) {
                $temp = $value;
                $value = $temp[0];
                if ($this->update && empty($this->param[$key])) {
                    $this->param[$key] = $temp[1];
                }
            }

            $input = $this->input($method . '_' . $key, $value, '', $key, $method);

            if ($this->update && !$input) {
                $input = $this->input($key, $value, '', $key, $method);
                if ($method == 'add' && !$input && isset($this->struct[$key]['default']) && $this->struct[$key]['default']) {
                    $input = $this->struct[$key]['default'];
                }
            }

            if ($input || ($input === '0' || $input === 0)) {
                if (is_array($input)) {
                    if (isset($this->struct[$key]) && isset($this->struct[$key]['bit'])) {
                        $vt = 0;
                        foreach ($input as $ki => $vi) {
                            if (isset($this->struct[$key]['bit'][$vi])) {
                                $vt += $this->struct[$key]['bit'][$vi];
                            }
                        }
                        $input = $vt;
                    } elseif (isset($input[0]) && is_array($input[0])) {
                        $input = base64_encode(json_encode($input));
                    } else {
                        $input = str_replace(',0', '', implode(',', $input));
                    }
                }

                if ($input === 'null') {
                    $input = '';
                }

                /*
                if (isset($this->struct[$key]) && isset($this->struct[$key]['type']) && $key != 'id') {
                    if (strpos($this->struct[$key]['type'], 'int') !== false && !is_object($input) && $input > 0) {
                        $input = (int) $input;
                    } else {
                        $input = (string) $input;
                    }
                }
                */

                $result = array($key, $input);
                if (isset($temp[1]) && $temp[1] != $input) {
                    $result[2] = $temp[1];
                }
                if (isset($temp[2])) {
                    $result[3] = $temp[2];
                }
                $send[] = $result;
            }
        }

        if ($send) {
            if ($method == 'option') {
                $method = 'where';
            }

            $this->opt($method, $send);
            Database::get($this->db)->$method($send);
        }
    }

    /**
     * opt
     *
     * @return mixd
     */
    private function opt($method, $param)
    {
        if (Project::load('manage') && Config::get('database')->opt && $this->project . $this->name != 'manageopt') {
            if ($method == 'where') {
                $col = array();
                foreach ($param as $k => $v) {
                    $col[] = $v[0];
                }
            } elseif ($method == 'order') {
                $col = $param[0];
                if (is_string($col)) {
                    if (strpos($col, '`') !== false) {
                        $col = str_replace(array('`', 'desc', 'asc', ' '), '', $col);
                    }

                    $col = explode(',', $col);
                } elseif (is_array($col)) {
                    foreach ($col as $k => $v) {
                        $col[] = $k;
                    }
                }
            }
            
            if (isset($col) && $col) {
                Opt::push($this->project, $this->name, $col);
            }
        }
    }

    /**
     * join
     *
     * @return mixd
     */
    protected function join()
    {
        if (isset($this->request['join']) && $this->request['join']) {
            Database::get($this->db)->join($this->request['join']);
        }
    }

    /**
     * input
     *
     * @return mixd
     */
    private function input($key, $value, $split = '', &$index, $method = '')
    {
        if (isset($this->param[$key])) {
            $request = $this->param[$key];
        }

        if (isset($request) && ($request === '0' || $request === 0)) {
            return 0;
        }

        list($index, $value, $callback, $state) = $this->config($index, $value, $method);

        if (empty($request)) {
            if (isset($this->request['input'])) {
                $request = Input::get($key, $value);
            } else {
                $request = $value;
            }
        }

        if (is_array($request) && isset($request[0]) && !is_array($request[0])) {
            $request = implode(',', $request);
        }

        if (is_string($value) && strpos($value, '/') === 0) {
            $state = preg_match($value, $request);
        } elseif (!empty($request)) {
            if ($callback) {
                $state = $callback($request);
            } elseif (is_string($request) && $split && strpos($request, $split) !== false) {
                $request = explode($split, $request);
            }

            $state = true;
        }

        if ($state) {
            return $this->request($index, $request, $method);
        }

        if ($method != 'option' && !$this->update) {
            if (isset($this->struct[$index]['desc']) && $this->struct[$index]['desc']) {
                Export::alert($this->struct[$index]['desc']);
            } else {
                Export::alert('core_database_request', array($key, ($value ? $value : $callback)));
            }
        }

        return false;
    }

    /**
     * config
     *
     * @return mixd
     */
    private function config($index, $value, $method)
    {
        if ($index && strpos($value, 'yes-') !== false) {
            $temp = explode('-', $value);
            $value = $temp[0];
            $index = $temp[1];
        }

        if ($index && $value == 'yes') {
            $value = 'option';
            /*
            if (strpos($index, '.') !== false) {
                $temp = explode('.', $index);
                $index = $temp[1];
            }*/
            if (isset($this->struct[$index]) && is_array($this->struct[$index]) && isset($this->struct[$index]['match']) && $this->struct[$index]['match']) {
                $value = $this->struct[$index]['match'];
            }
        }
        if (is_array($value) && isset($value[1])) {
            if ($this->update) {
                $value = $value[1];
            } else {
                $value = $value[0];
            }
        }

        $state = false;

        if ($value == 'option') {
            $value = '';
            $state = true;
        }

        $callback = is_string($value) && function_exists($value);
        if ($callback) {
            $callback = $value;
            $value = '';
        }

        return array($index, $value, $callback, $state);
    }

    /**
     * replace
     *
     * @return mixd
     */
    private function replace($content)
    {
        $tags = array(

            "'<iframe[^>]*?>.*?</iframe>'is",

            "'<frame[^>]*?>.*?</frame>'is",

            "'<script[^>]*?>.*?</script>'is",

            "'<head[^>]*?>.*?</head>'is",

            "'<title[^>]*?>.*?</title>'is",

            "'<meta[^>]*?>'is",

            "'<link[^>]*?>'is",
        );

        return preg_replace($tags, "", $content);
    }

    /**
     * request
     *
     * @return mixd
     */
    private function request($index, $request, $method)
    {
        if ($index && isset($this->struct[$index]) && is_array($this->struct[$index]) && isset($this->struct[$index]['callback']) && $this->struct[$index]['callback'] && $request) {
            $callback = $this->struct[$index]['callback'];
            if ($callback == 'maketime') {
                if (is_string($request)) {
                    $request = \Dever::maketime($request);
                }
            } else {
                if (strpos($callback, '.')) {
                    $temp = explode('.', $callback);
                    $callback = $temp[0];
                    $request = $callback($temp[1], $request);
                } else {
                    $request = $callback($request);
                }
            }
        }

        $request = $this->update($index, $request, $method);

        return $request;
    }

    /**
     * update
     *
     * @return mixd
     */
    private function update($index, $request, $method)
    {
        if ($this->update && isset($this->struct[$index]['update'])) {
            if (is_string($request) && empty($this->struct[$index]['strip'])) {
                $request = $this->replace($request);
            }

            if (isset($this->struct[$index]['key']) && Config::get('host')->uploadRes && strpos($request, Config::get('host')->uploadRes) !== false) {
                $request = str_replace(Config::get('host')->uploadRes, '{uploadRes}', $request);
            }
        }

        return $request;
    }
}
