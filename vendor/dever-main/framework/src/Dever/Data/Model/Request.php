<?php namespace Dever\Data\Model;

use Dever\Loader\Config;

class Request
{
    /**
     * config
     *
     * @var array
     */
    protected $config;

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
    public static function get($key, $method, $struct, $search)
    {
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self();
        }

        return self::$instance[$key]->call($method, $struct, $search);
    }

    /**
     * call
     *
     * @return mixd
     */
    public function call($method, $struct, $search)
    {
        if (empty($this->config[$method])) {
            $call = '_' . $method;
            if(strpos($call, 'update_')) {
                $this->config[$method] = $this->auto($method);
            } else {
                $this->config[$method] = $this->$call();
            }
            $this->setting($method, $struct, $search);
        }
        return $this->config[$method];
    }

    /**
     * setting
     * @param string $method
     * @param array $struct
     *
     * @return mixd
     */
    protected function setting($method, $struct, $search)
    {
        $state = $this->state($method);
        if ($state) {
            foreach ($struct as $key => $value) {
                if (isset($value['match'])) {
                    $this->settingValue($state, $method, $key, $value, $search);
                }
            }
        }
    }

    /**
     * settingValue
     *
     * @return mixd
     */
    protected function settingValue($state, $method, $key, $value, $search)
    {
        if (is_array($value['match'])) {
            $value['match'] = $value['match'][0];
            if (isset($value['insert']) && $state == 'add') {
                $value['match'] = 'yes';
            } elseif(empty($value['insert']) && ($state == 'add' || $state == 'set')) {
                $value['match'] = 'yes';
            }
        }
        if ($state == 'option') {
            $this->settingOption($method, $key, $value);
            if (isset($value['search']) && strpos($value['search'], 'fulltext') !== false && $search) {
                $this->config[$method][$state][$key] = array($value['match'], $this->searchType($search));
            }
        }

        if (empty($this->config[$method][$state][$key]) && isset($value['match'])) {
            $this->config[$method][$state][$key] = $value['match'];
        }
    }

    /**
     * settingValue
     *
     * @return mixd
     */
    protected function settingOption($method, $key, $value)
    {
        if (isset($value['bit'])) {
            $this->config[$method]['option'][$key] = array('option', '&');
        }

        if (isset($value['order']) && isset($this->config[$method]['order'])) {
            $this->settingOrder($method, $key, $value);
        }

        if (isset($value['search'])) {
            $this->settingSearch($method, $key, $value);
        }

        if (isset($value['in'])) {
            $this->config[$method]['option'][$key] = array('option', 'in');
        }
    }

    /**
     * settingValue
     *
     * @return mixd
     */
    protected function settingOrder($method, $key, $value)
    {
        if (isset($this->config[$method]['order']['id']) && $this->config[$method]['order']['id'] != $key) {
            $this->config[$method]['order'][$key] = is_string($value['order']) ? $value['order'] : 'desc';

            $this->config[$method]['order'] = array_reverse($this->config[$method]['order']);
        }
    }

    /**
     * settingSearch
     *
     * @return mixd
     */
    protected function settingSearch($method, $key, $value)
    {
        if (strpos($value['search'], 'time') !== false || strpos($value['search'], 'date') !== false) {
            $this->config[$method]['option']['start_' . $key] = array('yes-' . $key, '>=');
            $this->config[$method]['option']['end_' . $key] = array('yes-' . $key, '<=');
        } elseif (strpos($value['search'], 'mul') !== false) {
            $this->config[$method]['option'][$k] = array('option', 'like');
        }
    }

    /**
     * searchType
     *
     * @return mixd
     */
    private function searchType($method)
    {
        switch($method)
        {
            case 1:
                $method = '=';
                break;
            case 2:
                $method = 'like';
                break;
            case 3:
                $method = '>';
                break;
            case 4:
                $method = '>=';
                break;
            case 5:
                $method = '<';
                break;
            case 6:
                $method = '<=';
                break;
        }
        
        return $method;
    }

    /**
     * state
     *
     * @return mixd
     */
    protected function state($method)
    {
        $config = array('option', 'set', 'add');
        foreach ($config as $value) {
            if (isset($this->config[$method][$value])) {
                return $value;
            }
        }
        return false;
    }

    /**
     * auto
     *
     * @return mixd
     */
    protected function auto($method)
    {
        $config = $this->_update();
        $method = str_replace('update_', '', $method);
        $config['set'][$method] = 'yes';
        return $config;
    }

    /**
     * _one
     *
     * @return mixd
     */
    protected function _one()
    {
        return array
        (
            'type' => 'one',
            'option' => array(),
        );
    }

    /**
     * _list
     *
     * @return mixd
     */
    protected function _list()
    {
        $page = $this->_all();
        $num = 15;
        if (Config::get('base')->excel) {
            return $page;
        }
        return array_merge($page, array('page' => array($num, 'list')));
    }

    /**
     * _state
     *
     * @return mixd
     */
    protected function _state()
    {
        return array_merge($this->_all(), array('where' => array('state' => 1)));
    }

    /**
     * _all
     *
     * @return mixd
     */
    protected function _all()
    {
        return array
        (
            'type' => 'all',
            'order' => array('id' => 'desc'),
            //'group' => 'id',
            //'limit' => '0,10',
            'col' => '*|id',
            'option' => array(),
        );
    }

    /**
     * _total
     *
     * @return mixd
     */
    protected function _total()
    {
        return array
        (
            'type' => 'count',
            'col' => 'count(1) as total',
        );
    }

    /**
     * _update
     *
     * @return mixd
     */
    protected function _update()
    {
        return array
        (
            'type' => 'update',
            'set' => array(),
            'where' => array('id' => 'yes'),
        );
    }

    /**
     * _updatemul
     *
     * @return mixd
     */
    protected function _updatemul()
    {
        $config = $this->_update();

        if (Config::get('base')->mul_type) {
            if (Config::get('base')->mul_type == 2) {
                unset($config['where']);
                $config['option'] = array();
            } else {
                $config['where']['id'] = array('yes', 'in');
            }
        }

        return $config;
    }

    /**
     * _delete
     *
     * @return mixd
     */
    protected function _delete()
    {
        return array
        (
            'type' => 'delete',
            'option' => array(),
            //'where' => array('id' => 'yes'),
        );
    }

    /**
     * _insert
     *
     * @return mixd
     */
    protected function _insert()
    {
        return array
        (
            'type' => 'insert',
            'add' => array(),
        );
    }
}
