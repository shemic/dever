<?php namespace Dever\Output;

use RuntimeException;

class Exceptions extends RuntimeException
{
    public function __construct($message, $code = 0)
    {
        parent::__construct($message, $code);
    }

    public function getTraces()
    {
    	$data = Debug::getTrace();
    	//$data = $this->getTrace();
    	//print_r($data);die;
    	return $data;
    }
}
