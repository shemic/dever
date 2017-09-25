<?php namespace Dever\Output;

use Dever\Http\Url;

class Html
{

    /**
     * out
     * @param array $msg
     *
     * @return string
     */
    public function out($msg)
    {
        $html = '' . $msg['msg'];
        $host = Url::get('');

        $status = $msg['code'] > 1 ? $msg['code'] : '404';

        $this->import($status, $html, $host);
    }

    /**
     * _import
     * @param string $name
     *
     * @return string
     */
    private function import($name, $html, $host)
    {
        header("HTTP/1.1 404 Not Found");
        header("Status: 404 Not Found");
        $file = DEVER_APP_PATH . 'config/html/' . $name . '.html';
        if (is_file($file)) {
            include $file;
        } else {
            include DEVER_PATH . 'config/html/index.html';
        }
    }
}
