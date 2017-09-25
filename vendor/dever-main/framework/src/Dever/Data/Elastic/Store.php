<?php namespace Dever\Data\Elastic;

use Dever\Data\Store as Base;
use Dever\Routing\Input;

class Store extends Base
{
    /**
     * record
     *
     * @var array
     */
    protected $record;

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
        $this->table = $table;
    }

    /**
     * setTable
     *
     * @return mixd
     */
    public function createTable($struct)
    {
        if ($struct) {
            $properties = $this->getProperties($struct);
            if ($properties) {
                $param['mappings'][$this->alias]['properties'] = $properties;
            }
        }

        if (isset($this->config['setting'])) {
            $param['settings'] = $this->config['setting'];
        }

        $this->value['method'] = '?pretty';
        $this->exe('', 'put', $param, false);
    }

    /**
     * getProperties
     *
     * @return array
     */
    private function getProperties($struct)
    {
        $properties = array();
        foreach ($struct as $k => $v) {
            if (isset($v['type'])) {
                if ($k == 'cdate') {
                    $properties['@timestamp']['type'] = 'date';
                } elseif (strpos($v['type'], 'int') !== false) {
                    $properties[$k]['type'] = 'long';
                } elseif (strpos($v['type'], 'char') !== false) {
                    $properties[$k]['type'] = 'string';
                } elseif (strpos($v['type'], 'text') !== false) {
                    $properties[$k]['type'] = 'text';
                    $properties[$k]['store'] = 'false';
                    if (isset($this->config['analyzer']) && isset($v['search']) && strstr($v['search'], 'fulltext')) {
                        $properties[$k]['term_vector'] = 'with_positions_offsets';
                        $properties[$k]['analyzer'] = $this->config['analyzer'];
                        $properties[$k]['search_analyzer'] = $this->config['analyzer'];
                        $properties[$k]['include_in_all'] = 'true';
                    } else {
                        $properties[$k]['type'] = 'string';
                        $properties[$k]['index'] = 'not_analyzed';
                    }
                }

                if (isset($v['group'])) {
                    $properties[$k]['copy_to'] = $v['group'];
                }
            }
        }

        return $properties;
    }

    /**
     * getIndex
     *
     * @return mixed
     */
    public function getIndex($version, $index)
    {
        return true;
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
    public function query($url = '_status', $type = 'get')
    {
        return $this->exe($url, $type);
    }

    /**
     * exe
     *
     * @return mixed
     */
    public function exe($url = '_status', $type = 'get', $param = array(), $state = true)
    {
        $url = $this->alias . '/' . $url;
        if (isset($this->value['method'])) {
            $url = $this->value['method'];
        }
        return $this->read->handle($url, $type, $param, $state);
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
            if ($v == 'cdate') {
                $v = '@timestamp';
            }
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
        $data = $this->select($col);

        $result = array();

        if ($data && isset($key)) {
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
                } else {
                    $result[] = $v;
                }
            }
        } else {
            $result = $data;
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
        $data = $this->select($col);
        if ($data && isset($data[0]) && $data[0]) {
            return $data[0];
        }
        return array();
    }

    /**
     * count
     *
     * @return array
     */
    public function count($col = 'clear')
    {
        return $this->select($col, true);
    }

    /**
     * insert
     *
     * @return int
     */
    public function insert()
    {
        $id = '';
        if (isset($this->value['add']['id'])) {
            $id = $this->value['add']['id'];
        }
        $state = $this->exe($id, 'post', $this->value['add']);

        $this->log($this->value, 'insert');

        $this->value = array();

        return $state;
    }

    /**
     * update
     *
     * @return int
     */
    public function update()
    {
        $set = $this->value['set'];
        unset($this->value['set']);
        $this->value['set']['script']['inline'] = implode(';', $this->value['script']);
        $this->value['set']['script']['params'] = $set;

        if (isset($this->value['where']['id']) && is_numeric($this->value['where']['id']) && $this->value['where']['id'] > 0) {
            $state = $this->exe($this->value['where']['id'] . '/_update/', 'post', $this->value['set']);
        } else {
            if (isset($this->value['search'])) {
                $this->value['set'] = array_merge($this->value['set'], $this->value['search']);
            }
            $state = $this->exe('_update_by_query/', 'post', $this->value['set']);
        }

        $this->log($this->value, 'update');

        $this->value = array();

        return $state;
    }

    /**
     * delete
     *
     * @return int
     */
    public function delete()
    {
        $id = $this->value['where']['id'];

        $state = $this->exe($id, 'delete');

        $this->log($this->value, 'delete');

        $this->value = array();

        return $state;
    }

    /**
     * select
     *
     * @return array
     */
    private function select($col = '', $record = false)
    {
        if (isset($this->record) && $this->record) {
            return $this->record;
        }

        if ($col && $col != '*' && $col != 'clear') {
            $this->value['search']['_source'] = explode(',', $col);
        }

        if (!isset($this->value['search'])) {
            $this->value['search'] = array();
        } else {
            $this->highlight();
        }

        if (!isset($this->value['search']['sort'])) {
            $this->value['search']['sort'] = array
            (
                '@timestamp' => array('order' => 'desc'),
            );
        }

        $return = $this->exe('_search/?pretty', 'post', $this->value['search']);
        if (!isset($return['data'])) {
            return array();
        }

        $this->record = false;

        $this->log($this->value, 'select');
        if ($col != 'clear') {
            $this->value = array();
        }

        if ($record) {
            $this->record = $return['data'];
            return $return['total'];
        }
        return $return['data'];
    }

    /**
     * highlight
     *
     * @return object
     */
    private function highlight()
    {
        if (isset($this->config['highlight'])) {
            if (is_array($this->config['highlight'])) {
                $this->value['search']['highlight'] = $this->config['highlight'];
            }
        } else {
            //$object = (object) array();
            $object['fragment_size'] = 10000000;
            $object['number_of_fragments'] = 1;
            $this->value['search']['highlight'] = array
            (
                'pre_tags' => array('<em class="dever_highlight">'),
                'post_tags' => array('</em>'),
                'fields' => array('desc' => $object),
            );
        }
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
        if (is_array($param[0]) && $method != 'group') {
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
            $func = 'convert_' . $method;
            if ($param[0] == 'cdate') {
                $param[0] = '@timestamp';
            }
            if (method_exists($this, $func)) {
                $this->$func($param);
            }
            if (isset($param[1])) {
                $this->value[$method][$param[0]] = $param[1];
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
    private function convert_set($param)
    {
        if (empty($param[2])) {
            $param[2] = '=';
        }
        if (strpos($param[2], 'ctx.')) {
            $this->value['script'][] = $param[2];
        } else {
            $this->value['script'][] = 'ctx._source.' . $param[0] . $param[2] . 'params.' . $param[0];
        }
    }

    /**
     * convert_order
     *
     * @return mixed
     */
    private function convert_order($param)
    {
        if ($param[0] == 'id') {
            $param[0] = '_score';
        }
        $this->value['search']['sort'] = array
        (
            $param[0] => array('order' => $param[1]),
        );

        //$this->value['search']['sort'] = array('_doc');
    }

    /**
     * convert_limit
     *
     * @return mixed
     */
    private function convert_limit($param)
    {
        if (is_string($param[0]) && strpos($param[0], ',') !== false) {
            $param = explode(',', $param[0]);
        }

        $this->value['search']['from'] = $param[0];
        $this->value['search']['size'] = $param[1];

        $scroll = Input::get('es_scroll');
        if ($scroll) {
            if ($param[0] == 0) {
                $this->value['method'] = $this->alias . '/_search?scroll=5m';
            } else {
                $oper = new \Dever\Session\Oper(DEVER_PROJECT, 'cookie');
                $scroll = $oper->get('es_scroll');
                $this->value['method'] = '_search/scroll';
                $this->value['search'] = array
                (
                    'scroll' => '5m',
                    'scroll_id' => $scroll,
                );
            }
        }
    }

    /**
     * convert_group
     *
     * @return mixed
     */
    private function convert_group($param)
    {
        //'group' => array('name', array('child' => array('cdate'))),
        $group = $param[0];
        if (is_string($group)) {
            
            $keyword = $this->group_aggs_keyword($group);
            $this->value['search']['aggs'][$group]['terms'] = array
            (
                'field' => $keyword,
                'order' => array('_count' => 'desc'),
                //'min_doc_count' => 2,
                //'execution_hint' => 'map',
            );

            if (isset($this->value['search']['size'])) {
                $this->value['search']['aggs'][$group]['terms']['size'] = $this->value['search']['size'];
            }

            if (isset($param[1]) && $param[1]) {
                if (isset($param[1]['aggs'])) {
                    $this->value['search']['aggs'][$group]['aggs'] = $param[1]['aggs'];
                    unset($param[1]['aggs']);
                } elseif (isset($param[1]['child'])) {
                    $aggs = array();
                    if (isset($param[1]['child_size'])) {
                        $child_size = $param[1]['child_size'];
                        unset($param[1]['child_size']);
                    }
                    foreach ($param[1]['child'] as $k => $v) {
                        if (is_array($v)) {
                            $aggs[$key][$k] = $v;
                        } else {
                            $key = $v;
                            $k = 'terms';
                            $v = $this->group_aggs_keyword($v);
                            $aggs[$key][$k]['field'] = $v;
                            if (isset($child_size)) {
                                $aggs[$key][$k]['size'] = $child_size;
                            }
                        }
                    }
                    $this->value['search']['aggs'][$group]['aggs'] = $aggs;
                    unset($param[1]['child']);
                }
                $this->value['search']['aggs'][$group]['terms'] = array_merge($this->value['search']['aggs'][$group]['terms'], $param[1]);
            }
            //$this->value['search']['size'] = 0;
        } else {
            $this->value['search']['aggs'] = $group;
        }
        
        
        //$map['properties'][$group]['type'] = 'text';
        //$map['properties'][$group]['fielddata'] = true;
        //$this->exe('_mapping', 'put', $map);
        //print_r($this->value);
    }

    /**
     * group_aggs_keyword
     *
     * @return string
     */
    private function group_aggs_keyword($group)
    {
        if (strpos($group, '.')) {
            $keyword = $group;
        } else {
            $keyword = $group;
        }

        return $keyword;
    }

    /**
     * convert_where
     *
     * @return mixed
     */
    private function convert_where($param)
    {
        //print_r($param);die;
        $type = 'bool';
        $method = '';
        if (isset($param[2])) {
            $method = $param[2];
        }

        if (!isset($param[3])) {
            $param[3] = 'and';
        }
        $bool = $this->bool($param[3]);
        if (!isset($this->value['search']['query'][$type][$bool])) {
            $this->value['search']['query'][$type][$bool] = array();
        }
        $index = count($this->value['search']['query'][$type][$bool]);
        $this->value['search']['query'][$type][$bool][$index] = $this->value($param[0], $param[1], $method);
    }

    /**
     * value
     */
    private function value($key, $value, $method)
    {
        if ($key == 'id') {
            $key = '_id';
        }
        $result = array();
        if (!$method) {
            $result['match_phrase'][$key]['query'] = $value;
            //$result['match_phrase'][$key]['analyzer'] = 'not_analyzed';
            $result['match_phrase'][$key]['slop'] = 1;
        } elseif ($method && $method == 'like') {
            $result['query_string']['default_field'] = $key;
            $result['query_string']['query'] = $value;
        } else {
            $method = $this->method($method, $key, $value);
            $result[$method[0]][$key] = $method[1];
        }

        return $result;
    }

    /**
     * bool
     */
    private function bool($value)
    {
        if (stripos($value, 'or') !== false) {
            return 'should';
        } elseif (stripos($value, 'not') !== false) {
            return 'must_not';
        } else {
            return 'must';
        }
    }
    
    /**
     * method
     */
    private function method($method, $key, $value)
    {
        /*
        if ($key == '@timestamp') {
            $value = date('c', $value);
        }
        */
        switch ($method) {
            case 'like':
                $method = 'wildcard';
                $value = $value . '*';
                break;
            case 'in':
                $method = 'terms';
                $value = explode(',', $value);
                break;
            case 'nin':
                $method = 'terms';
                $value = explode(',', $value);
                break;
            case '>':
                $method = 'range';
                $range['gt'] = $value;
                $value = $range;
                break;
            case '>=':
                $method = 'range';
                $range['gte'] = $value;
                $value = $range;
                break;
            case '<':
                $method = 'range';
                $range['lt'] = $value;
                $value = $range;
                break;
            case '<=':
                $method = 'range';
                $range['lte'] = $value;
                $value = $range;
                break;
            case '==':
            case '=':
                $method = 'range';
                $range['gte'] = $value;
                $range['lte'] = $value;
                $value = $range;
                break;
            case '!=':
                $method = 'range';
                $range['gt'] = $value;
                $range['lt'] = $value;
                $value = $range;
                break;
        }
        
        return array($method, $value);
    }
}
