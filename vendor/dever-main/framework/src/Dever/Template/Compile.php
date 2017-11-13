<?php namespace Dever\Template;

use Dever\Loader\Config;
use Dever\Output\Debug;
use Dever\Support\Path;
use Dever\Http\Url;
use Dever\Routing\Input;

class Compile
{
    /**
     * copyright
     *
     * @var const string
     */
    const COPYRIGHT = '<!--power by dever-->';

    /**
     * file
     *
     * @var string
     */
    protected $file;

    /**
     * template
     *
     * @var string
     */
    protected $template;

    /**
     * parsing
     *
     * @var object
     */
    protected $parsing;

    /**
     * update
     *
     * @var bull
     */
    protected $update = false;

    /**
     * data
     *
     * @var array
     */
    protected $data = null;

    /**
     * index
     *
     * @var int
     */
    protected $index = 0;

    /**
     * load file
     * @param string $file
     * @param string $assets
     * @param string $template
     * @param string $project
     *
     * @return mixed
     */
    public function __construct($file, $assets, $template, $project = false, $path = '', $data = array())
    {
        $this->parsing = new Parsing($path);
        $this->index = 0;
        if ($data) {
            $this->data = $data;
        }
        $this->project($project);
        if (Config::get('template')->publish) {
            $this->update = false;
            $this->file = $this->path($path . $file, 'compile', false) . '.cmp.php';
        } else {
            $this->read($path, $file, $assets, $template);
        }
    }

    /**
     * parsing
     *
     * @return mixed
     */
    public function parsing()
    {
        return $this->parsing;
    }

    /**
     * read file
     * @param string $path
     * @param string $file
     * @param string $assets
     * @param string $template
     * @param bool $local
     *
     * @return mixed
     */
    public function read($path, $file, $assets, $template, $local = false)
    {
        if (strpos($file, 'http://')) {
            $content = file_get_contents($file);
            if ($local == true) {
                $content = $this->local($content);
            }
            $this->template = $content;
        } else {
            $this->template = $assets . $file . '.html';
            if (is_file($this->template)) {
                $this->file = $this->path($path . $file) . '.cmp.php';
                Debug::log($this->file, 'template');
                $is_service = is_file($template);
                $time = is_file($this->file) ? filemtime($this->file) : 0;
                $this->update = defined('DEVER_COMPILE');
                if (filemtime($this->template) > $time || ($is_service && filemtime($template) > $time)) {
                    $this->update = true;
                }
                if (!$this->update && Input::shell(Config::get('template')->shell)) {
                    $this->update = true;
                }
                if ($time == 0 || $this->update == true) {
                    $content = $this->content($assets);
                }
                if (!empty($content) && !$is_service) {
                    return $this->create($this->parsing->content($content));
                }
            } else {
                $this->update = false;
            }
        }
    }

    /**
     * getContent
     *
     * @return string
     */
    public function getContent()
    {
        return $this->template;
    }

    /**
     * content
     *
     * @return string
     */
    public function content($path)
    {
        $content = file_get_contents($this->template);
        if (strpos($content, '@include:') !== false) {
            $temp = explode('@include:', $content);
            $file = end($temp);
            $this->template = $path . $file . '.html';

            $content = file_get_contents($this->template);
        } elseif (strpos($content, '@url:') !== false) {
            $content = str_replace("\n", '', $content);
            $temp = explode('@url:', $content);
            $this->template = end($temp);
            $content = file_get_contents($this->template);
            # 增加资源处理功能 @res:replace 将资源中的路径替换为带有域名的路径　@res:local　本地化
            if (strpos($temp[0], '@res:') !== false) {
                $temp = explode('@res:', $temp[0]);
                $temp = end($temp);
                $content = $this->resources($temp, $content, $this->template, $path);
            }
        }
        $this->template = $content;
        return $content;
    }

    /**
     * resources 资源处理 此处待优化
     *
     * @return string
     */
    private function resources($type, $content, $url, $path)
    {
        $encode = mb_detect_encoding($content, array('GB2312', 'GBK', 'UTF-8'));
        if ($encode == 'GB2312' || $encode == 'GBK' || $encode == 'EUC-CN' || $encode == 'CP936') {
            $content = iconv('GBK', 'UTF-8', $content);
        }
        # 过滤换行
        $content = str_replace(PHP_EOL, '', $content);
        if ($type == 'local') {
            # 规则
            $rule = '<link(.*?)href="(.*?)"';
            preg_match_all('/' . $rule . '/i', $content, $result);
            $rule = '<script src="(.*?)"(.*?)<\/script>';
            preg_match_all('/' . $rule . '/i', $content, $result);
        } elseif (strpos($type, 'include:') !== false) {
            $temp = explode('include:', $type);
            $file = end($temp);
            $include = str_replace("\n", '', file_get_contents($path . $file . '.html'));
            parse_str($include, $param);
            foreach ($param as $k => $v) {
                $content = $this->parsing->replace($k, $v, $content);
            }
        } else {
            parse_str($type, $param);
            foreach ($param as $k => $v) {
                $content = $this->parsing->replace($k, $v, $content);
            }
        }
        return $content;
    }

    /**
     * get file
     *
     * @return string
     */
    public function file()
    {
        return $this->file;
    }

    /**
     * get template or content
     *
     * @return string
     */
    public function template()
    {
        return $this->template;
    }

    /**
     * project
     *
     * @return string
     */
    public function project($project = false)
    {
        $this->project = $project ? $project : DEVER_APP_NAME;

        return $this->project;
    }

    /**
     * path create path
     * @param string $file
     * @param string $project
     *
     * @return string
     */
    public function path($file, $path = 'compile', $create = true)
    {
        $path = Config::data() . $path . DIRECTORY_SEPARATOR;
        $file = DEVER_PROJECT . DIRECTORY_SEPARATOR . $this->project . DIRECTORY_SEPARATOR . $file;
        if ($create) {
            return Path::get($path, $file);
        }
        return $path . $file;
    }

    /**
     * get
     *
     * @return mixed
     */
    public function get()
    {
        if ($this->update == false && is_file($this->file)) {
            ob_start();
            if (isset($this->data) && $this->data && is_array($this->data)) {
                parse_str(http_build_query($this->data));
            }
            require $this->file;
            $content = ob_get_contents();
            if (Config::get('host')->domain) {
                $content = $this->domain($content);
            }
            if (Config::get('host')->uploadRes) {
                $content = Url::uploadRes($content);
            }
            ob_end_clean();
            return $content;
        } elseif ($this->update == false) {
            return '';
        } else {
            return false;
        }
    }

    /**
     * replace domain
     * @param  string $content
     */
    private function domain($content)
    {
        $rule = Config::get('host')->domain['rule'];
        $replace = Config::get('host')->domain['replace'];
        $rule = $rule();
        if ($rule[0] != $rule[1]) {
            foreach ($replace as $k => $v) {
                $source = $rule[0] . str_replace('*', '([a-zA-Z0-9_]+)', $v);
                $desc = $rule[1] . str_replace('*', '$1', $v);
                $source = str_replace('/', '\/', $source);
                $content = preg_replace('/' . $source . '/i', $desc, $content);
            }
        }
        return $content;
    }

    /**
     * create
     * @param string $content
     *
     * @return string
     */
    public function create($content)
    {
        $this->update = false;

        $this->write($this->assets($content));

        return $this->get();
    }

    /**
     * write
     *
     * @return mixed
     */
    public function write($content)
    {
        $content = preg_replace('/<!--(.*?)-->/s', '', $content);

        if (Config::get('host')->merge && strpos($content, Config::get('host')->assets) !== false) {
            $this->merge
                (
                array('<link(.*?)href=[\'|"](.*?)[\'|"](.*?)>', '<script([a-zA-Z\/"\'=\\s]+)src=[\'|"](.*?)[\'|"](.*?)<\/script>'),
                $content
            );
        }

        //$this->file = str_replace('/', DIRECTORY_SEPARATOR, $this->file);

        if (strpos($content, self::COPYRIGHT) === false && strpos($content, '<html') !== false) {
            $content = str_replace('<html', self::COPYRIGHT . '<html', $content);
        }

        file_put_contents($this->file, $content);

        @chmod($this->file, 0755);

        system('chmod -R ' . $this->file . ' 777');
    }

    /**
     * assets
     * @param string $content
     *
     * @return string
     */
    public function assets($content)
    {
        if (Config::get('template')->replace && is_array(Config::get('template')->replace)) {
            foreach (Config::get('template')->replace as $k => $v) {
                if (Config::get('host')->$k) {
                    if (!Config::get('host')->merge && Config::get('template')->domain) {
                        $content = $this->parsing->replace($v, $this->parsing->script('echo Dever::config("host")->' . $k . ''), $content);
                    } else {
                        $content = $this->parsing->replace($v, Config::get('host')->$k, $content);
                    }
                }
            }
        }

        return $content;
    }

    /**
     * merge
     *
     * @return string
     */
    private function merge($rule, &$content)
    {
        foreach ($rule as $k => $v) {
            $v = '/' . $v . '/i';
            preg_match_all($v, $content, $result);
            if (isset($result[2]) && $result[2]) {
                if ($k == 1) {
                    $ext = 'js';
                    $fext = "\r\n;";
                } else {
                    $ext = 'css';
                    $fext = '';
                }
                $file = md5($this->file) . '.' . $ext;
                $host = Config::get('host')->merge . $this->project . DIRECTORY_SEPARATOR . $file . '?v' . DEVER_TIME;
                if (Config::get('template')->domain) {
                    $host = $this->parsing->replace(Config::get('host')->merge, $this->parsing->script('echo Dever::config("host")->merge'), $host);
                }
                $file = $this->path($file, 'assets');
                $assets = '';
                foreach ($result[2] as $fk => $fv) {
                    if ($fv) {
                        $fv = str_replace(Config::get('host')->base, Config::get('base')->workspace, $fv);

                        $fv = str_replace('/', DIRECTORY_SEPARATOR, $fv);

                        $assets .= $this->copy($fv, $file, $ext) . $fext;

                        if (strpos($content, '{{file}}') === false) {
                            $content = str_replace($result[0][$fk], '{{file}}', $content);
                        } else {
                            $content = str_replace($result[0][$fk], '', $content);
                        }
                    }
                }
                if ($assets) {
                    $method = 'zip_' . $ext;
                    //$assets = $this->assets($assets);
                    file_put_contents($file, $this->$method($assets));
                }
                $file = $k == 1 ? '<script type="text/javascript" src="' . $host . '"></script>' : '<link rel="stylesheet" type="text/css" href="' . $host . '" />';

                //$content = $this->zip_css($content);

                $content = str_replace('{{file}}', $file, $content);
            }
        }
    }

    /**
     * copy
     *
     * @return string
     */
    private function copy($file, $copy, $type = 'css')
    {
        $content = file_get_contents($file);

        if ($type == 'css') {
            $rule = '/url\((.*?)\)/i';
            preg_match_all($rule, $content, $result);

            $path = array();
            if (isset($result[1])) {
                foreach ($result[1] as $k => $v) {
                    $temp = $this->getPathByFile($v);
                    $path[$temp] = $temp;

                    $content = str_replace('../', '', $content);
                }
            }

            $boot = $this->getPathByFile($file);
            $copy = $this->getPathByFile($copy);

            if ($path) {
                foreach ($path as $k => $v) {
                    $this->copyDir($boot . $v, $copy, $v);
                }
            }
        }

        return $content;
    }

    /**
     * copyDir
     *
     * @return string
     */
    private function copyDir($src, $dst, $path)
    {
        if (function_exists('system')) {
            system('cp -R ' . $src . ' ' . $dst);
        } else {
            $path = str_replace(array('/', '..'), '', $path);
            $dst = $dst . $path;

            if (!is_dir($dst)) {
                mkdir($dst);
            }

            $dir = opendir($src);

            while (false !== ($file = readdir($dir))) {
                if (($file != '.') && ($file != '..')) {
                    if (is_dir($src . '/' . $file)) {
                        $this->copyDir($src . '/' . $file, $dst . '/' . $file);
                    } else {
                        copy($src . '/' . $file, $dst . '/' . $file);
                    }
                }
            }
            closedir($dir);
        }
    }

    /**
     * getPathByFile
     *
     * @return string
     */
    private function getPathByFile($file)
    {
        $temp = explode('/', $file);
        $count = count($temp) - 1;
        $file = str_replace($temp[$count], '', $file);
        return $file;
    }

    /**
     * zip_css
     * @param string $string
     *
     * @return string
     */
    private function zip_css($string)
    {
        return str_replace(array("\t", "\r\n", "\r", "\n"), '', $string);
    }

    /**
     * zip_js
     * @param string $string
     *
     * @return string
     */
    private function zip_js($string)
    {
        return $string;
        $h1 = 'http://';
        $s1 = '【:??】';
        $h2 = 'https://';
        $s2 = '【s:??】';
        $string = preg_replace('#function include([^}]*)}#isU', '', $string); //include函数体
        $string = preg_replace('#\/\*.*\*\/#isU', '', $string); //块注释
        $string = str_replace($h1, $s1, $string);
        $string = str_replace($h2, $s2, $string);
        $string = preg_replace('#\/\/[^\n]*#', '', $string); //行注释
        $string = str_replace($s1, $h1, $string);
        $string = str_replace($s2, $h2, $string);
        $string = preg_replace('#\s?(=|>=|\?|:|==|\+|\|\||\+=|>|<|\/|\-|,|\()\s?#', '$1', $string); //字符前后多余空格
        $string = $this->zip_css($string);
        $string = trim($string, " ");

        return $string;
    }
}