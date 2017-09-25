<?php namespace Dever\Data;

use Dever;

class Migration
{
    protected $num;
    protected $class;
    protected $method;
    protected $type;

    public function __construct($num = 1000)
    {
        $this->num = $num;
    }

    public function action($class, $method, $type, $num = false)
    {
        $limit = 0;
        $this->num = $num > 0 ? $num : $this->num;
        $this->class = $class;
        $this->method = $method;
        $this->type = $type;
        while($this->handle($limit, $this->num))
        {
            $limit += $num;
        }

        return $limit;
    }

    private function handle($limit, $num)
    {
        $param['limit'] = array($num, $limit);

        $data = Dever::load($this->type, $param);

        if(!$data)
        {
            return false;
        }

        $method = $this->method;
        foreach($data as $k => $v)
        {
            $this->class->$method($v);
        }

        return true;
    }
}