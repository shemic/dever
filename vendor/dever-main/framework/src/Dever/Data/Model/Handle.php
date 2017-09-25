<?php namespace Dever\Data\Model;

use Dever\Loader\Config;
use Dever\Loader\Project;
use Dever\Loader\Import;
use Dever\Output\Export;
use Dever\Routing\Input;
use Dever\String\Helper;
use Dever\Data\Model;

class Handle
{
    
    /**
     * param
     *
     * @var array
     */
    protected $param;

    /**
     * request
     *
     * @var array
     */
    protected $request;

    /**
     * config
     *
     * @var array
     */
    protected $config;


    /**
     * update state
     *
     * @var int
     */
    protected $update;

    /**
     * method
     *
     * @var string
     */
    protected $method;

    /**
     * get
     *
     * @return mixed
     */
    public function __construct($method, $config, $param)
    {
        $this->method = $method;
        $this->config = $config;
        $this->param = $param;
        $this->request = isset($config['request']) ? $config['request'][$method] : array();
    }

    /**
     * get
     *
     * @return mixed
     */
    public function get()
    {
        $data = array();
        if ($this->request) {
            $type = $this->request['type'];
            $this->setUpdate($type);
            $this->manage($type);
            $this->hook('start');
            $this->top();

            if ($this->check()) {
                return;
            }

            $this->condition();

            $data = $this->db()->$type($this->request['col']);

            if ($data && $type == 'update' && isset($this->param['where_id'])) {
                $data = $this->param['where_id'];
            }

            $this->after($type, $data);

            $this->hook('end', $data);
        }

        return $data;
    }

    /**
     * manage
     *
     * @return mixd
     */
    private function manage()
    {
        if (Project::load('manage') && DEVER_APP_NAME == 'manage' && $this->config['project']['name'] != 'manage') {
            $admin = Import::load('manage/auth.info');
            if ($admin && $admin['id'] > 0) {
                if ($this->update == 1) {
                    $this->updateManage($admin, 'insert', 'add');
                } elseif($this->update == 2) {
                    $this->updateManage($admin, 'update', 'set');
                } elseif($admin['self'] == 1) {
                    $this->updateManage($admin, 'select', 'where');
                }
            }
        }
    }

    /**
     * updateManage
     *
     * @return mixd
     */
    private function updateManage($admin, $type, $prefix = 'add')
    {
        if (isset($admin['col_' . $type]) && $admin['col_' . $type] && is_array($this->param)) {
            $col = explode(',', $admin['col_' . $type]);
            foreach ($col as $k => $v) {
                if (isset($this->config['struct'][$v]) && empty($this->config['struct'][$v]['value'])) {
                    $this->param[$prefix . '_' . $v] = $admin['id'];
                    if ($prefix == 'where') {
                        $this->param['option_' . $v] = $admin['id'];
                    }
                }
            }
        }
    }

    /**
     * db
     *
     * @return mixd
     */
    private function db()
    {
        return Database::get($this->config['db']);
    }

    /**
     * updateState
     *
     * @return mixd
     */
    protected function setUpdate($type)
    {
        if ($type == 'insert') {
            $this->update = 1;
        } elseif ($type == 'update') {
            $this->update = 2;
        } else {
            $this->update = 0;
        }
    }

    

    /**
     * hook
     * @param string $key
     * @param string $method
     *
     * @return mixd
     */
    private function hook($method = 'start', $data = array())
    {
        if (!$data) {
            $data = $this->param;
        }
        # 不再继续执行hook，就设置这个参数
        if (Config::get('base')->hook) {

        } elseif (isset($this->config[$method][$this->method])) {
            Config::get('base')->hook = true;
            if (isset($this->config['top']) && is_array($this->config['top'])) {
                $this->param = array($data, $this->config['top']);
            }
            if (isset($this->config['auth']) && is_array($this->config['auth'])) {
                $this->param = array($data, $this->config['auth']);
            }
            if (is_array($this->config[$method][$this->method])) {
                foreach ($this->config[$method][$this->method] as $k => $v) {
                    $data = Import::load($v, $this->param);
                    if ($data && is_array($data)) {
                        $this->param[$v] = $data;
                    }
                }
            } else {
                $data = Import::load($this->config[$method][$this->method], $this->param);

                if ($data && is_array($data)) {
                    $this->param[$this->config[$method][$this->method]] = $data;
                }
            }
        }
    }

    /**
     * top
     *
     * @return mixd
     */
    private function top()
    {
        if (isset($this->config['top']) && is_string($this->config['top'])) {
            $value = isset($this->param[$this->config['top']]) ? $this->param[$this->config['top']] : Input::get($this->config['top']);
            if ($value) {
                $top['value'] = $value;
            } else {
                $top = Import::load('manage/auth.getTop', $this->config['top']);
            }
            if ($top) {
                $temp = explode('/', $this->config['top']);
                $this->config['top'] = $temp[1];
                if ($this->update) {
                    $this->setParam($this->config['top'], $top['value']);
                } else {
                    $this->setParam('where_' . $this->config['top'], $top['value']);
                    $this->setParam('option_' . $this->config['top'], $top['value']);
                }
            }
        }
    }


    /**
     * check
     *
     * @return mixd
     */
    private function check()
    {
        $check = false;
        if (isset($this->config['check']) && $this->update) {
            if (is_array($this->config['check'])) {
                foreach ($this->config['check'] as $k => $v) {
                    if ($this->checkCol($v)) {
                        $check = true;
                    }
                }
            } else {
                $check = $this->checkCol($this->config['check']);
            }
        }

        return $check;
    }

    /**
     * checkCol
     *
     * @return mixd
     */
    private function checkCol($col)
    {
        $check = true;
        if (strpos($col, '.option')) {
            $col = str_replace('.option', '', $col);
            $check = false;
        }

        $data = explode(',', $col);
        $id = -1;
        if (isset($this->param['where_id'])) {
            $id = $this->param['where_id'];
        }

        foreach ($data as $k => $v) {
            if (isset($this->config['struct'][$v])) {
                if (isset($this->param['set_' . $v])) {
                    $param['option_' . $v] = $this->param['set_' . $v];
                } elseif (isset($this->param['add_' . $v])) {
                    $param['option_' . $v] = $this->param['add_' . $v];
                } elseif (isset($this->param[$v])) {
                    $param['option_' . $v] = $this->param[$v];
                }

                $temp = explode('-', $this->config['struct'][$v]['name']);
                $name[] = $temp[0];
            }
        }

        if (isset($param) && $param) {
            $info = Model::load($this->config['project']['name'] . '/' . $this->config['name'])->one($param);

            if ($id > 0 && $info && $info['id'] != $id) {
                if ($check == true) {
                    Export::alert(implode(',', $name).'已经存在');
                } else {
                    return true;
                }
            } elseif ($id < 0 && $info) {
                if ($check == true) {
                    Export::alert(implode(',', $name).'已经存在');
                } else {
                    return true;
                }
            }
        }

        return false;
    }

    /**
     * condition
     *
     * @return mixd
     */
    private function condition()
    {
        $this->table();
        $this->col();
        Condition::get()->init($this->request, $this->config['struct'], $this->param, $this->config['project']['name'], $this->config['name'], $this->config['db']);
    }

    /**
     * table
     *
     * @return mixd
     */
    private function table()
    {
        $this->index = '';
        if (isset($this->config['struct'])) {
            if (!isset($this->config['table'])) {
                $this->config['table'] = '';
            }
            $this->db()->table($this->config['project']['name'] . '_' . $this->config['name'], $this->index, true, $this->config['table']);

            $this->create();
        } else {
            $this->db()->table($this->config['name'], $this->index, false);
        }
    }

    /**
     * create
     *
     * @return mixd
     */
    private function create()
    {
        if (isset($this->config['struct'])) {
            $this->config['type'] = isset($this->config['type']) ? $this->config['type'] : 'innodb';
            $this->config['partition'] = isset($this->config['partition']) ? $this->config['partition'] : '';
            $create = $this->db()->create($this->config['struct'], $this->index, $this->config['type'], $this->config['partition']);
            if ($create === true) {
                # 写入默认值
                if (isset($this->config['default'])) {
                    $this->db()->inserts($this->config['default'], $this->index);
                }
            } else {
                if (isset($create['struct'])) {
                    if (count($create['struct']) < count($this->config['struct'])) {
                        $alter = array_diff_key($this->config['struct'], $create['struct']);
                        if ($alter) {
                            $this->db()->alter($alter, $this->config['struct'], $this->index);
                        }
                    }
                }

                if (isset($this->config['alter'])) {
                    $this->db()->alter($this->config['alter'], $this->index);
                }
            }

            if (isset($this->config['index'])) {
                $this->db()->index($this->config['index']);
            }
        }
    }

    /**
     * col
     *
     * @return mixd
     */
    private function col()
    {
        if (empty($this->request['col'])) {
            $this->request['col'] = '*';
        }

        if (Config::get('template')->sql && strpos(Config::get('template')->sql, '*') !== false) {
            $temp = array();
            foreach ($this->config['struct'] as $k => $v) {
                if (isset($v['type'])) {
                    $temp[$k] = '`' . $k . '`';
                }
            }

            $this->request['col'] = str_replace('*', implode(',', $temp), $this->request['col']);
        }
    }

    /**
     * after
     *
     * @return mixd
     */
    protected function after($type, $data)
    {
        if ($this->update) {

            if (Project::load('manage') && isset($this->config['manage']) && isset($this->config['manage']['filter'])) {
                $this->filter($this->config['manage']['filter'], $data);
            }

            if (isset($this->config['sync'])) {
                $this->sync($this->config['sync'], $data);
            }

        } elseif ($data && isset($this->request['relate'])) {
            $this->relate($this->request['relate'], $data, $type);
        }
    }

    /**
     * handle
     *
     * @return mixd
     */
    private function relate($config, &$data, $type)
    {
        if ($type == 'all') {
            foreach ($data as $k => $v) {
                $this->relate($config, $data[$k], 'one');
            }
        } else {
            foreach ($config as $k => $v) {
                foreach ($v as $i => $j) {
                    $v[$i] = $data[$j];
                }
                $data[$k] = Model::load($k, $v);
            }
        }
    }

    /**
     * handle
     *
     * @return mixd
     */
    private function sync($config, $id)
    {
        foreach ($config as $k => $v) {
            $id = $id > 0 ? $id : $this->param['where_id'];

            $info = Model::load($this->config['project']['name'] . '/' . $this->config['name'])->one($id);
            if (empty($info[$v['where'][1]])) {
                break;
            }

            if ($v['type'] == 'delete') {
                Model::load($k)->delete(array
                (
                    'option_' . $v['where'][0] => $info[$v['where'][1]],
                ));
            }
            foreach ($v['update'] as $i => $j) {
                if (strpos($i, '-')) {
                    $t = explode('-', $i);
                    $i = $t[0];
                }
                $value = $info[$j];

                if ($value) {
                    $value = explode(',', $value);
                    foreach ($value as $a => $b) {
                        $method = 'insert';
                        $type = 'add';
                        $param = array();
                        if ($v['type'] != 'delete') {
                            $check = Model::load($k)->one(array
                                (
                                    'option_' . $i => $b,
                                    'option_' . $v['where'][0] => $info[$v['where'][1]],
                                ));

                            if ($check) {
                                $method = 'update';
                                $type = 'set';
                                $param = array
                                (
                                    'where_id' => $check['id'],
                                );
                            }
                        }

                        if ($method) {
                            $param += array
                                (
                                $type . '_' . $i => $b,
                                $type . '_' . $v['where'][0] => $info[$v['where'][1]],
                            );

                            if (isset($v['sync'])) {
                                foreach ($v['sync'] as $c => $d) {
                                    if (isset($info[$d]) && $info[$d]) {
                                        $param[$type . '_' . $c] = $info[$d];
                                    }
                                }
                            }
                            Model::load($k)->$method($param);
                        }
                    }
                }
            }
        }
    }

    /**
     * handle
     *
     * @return mixd
     */
    private function filter($filter, $data)
    {
        if ($this->method == 'update' && isset($this->param['where_id'])) {
            $this->filterText($filter, 'set', $this->param['where_id']);
        } elseif ($this->method == 'insert' && $data > 0) {
            $this->filterText($filter, 'add', $data);
        }
    }

    /**
     * handle
     *
     * @return mixd
     */
    private function filterText($filter, $prefix = 'set', $id)
    {
        $text = '';

        foreach ($filter as $k => $v) {
            if (isset($this->param[$prefix . '_' . $v])) {
                $text .= '&dever_' . $v . '=' . $this->param[$prefix . '_' . $v];
            }
        }

        if ($text) {
            $config['project'] = $this->config['project']['name'];
            $config['table'] = $this->config['name'];
            Import::load('manage/filter.handle', $id, $text, $config);
        }
    }

    /**
     * setParam
     * @param string $key
     *
     * @return mixd
     */
    private function setParam($key, $value)
    {
        if (!isset($this->param[$key])) {
            $this->param[$key] = $value;
        }
    }
}
