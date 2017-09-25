<?php namespace Dever\Template;

use Dever\Loader\Config;
use Dever\Loader\Project;
use Dever\Session\Oper;
use Dever\Routing\Input;
use Dever\Cache\Handle as Cache;
use Dever\Support\Path;
use Dever\Routing\Uri;
use Dever\Support\Env;

class View
{
    /**
     * assets path
     *
     * @var string
     */
    const ASSETS = 'html';

    /**
     * template path
     *
     * @var string
     */
    const TEMPLATE = 'template';

    /**
     * assets
     *
     * @var string
     */
    protected $assets;

    /**
     * name
     *
     * @var string
     */
    protected $name;

    /**
     * template
     *
     * @var string
     */
    protected $template;

    /**
     * fetch
     *
     * @var array
     */
    protected $fetch;

    /**
     * content
     *
     * @var string
     */
    protected $content;

    /**
     * path
     *
     * @var string
     */
    protected $path;

    /**
     * dom
     *
     * @var object
     */
    protected $dom;

    /**
     * file
     *
     * @var string
     */
    protected $file;

    /**
     * data
     *
     * @var array
     */
    protected $data = null;

    /**
     * pjax
     *
     * @var bool
     */
    protected $pjax = null;

    /**
     * project
     *
     * @var string
     */
    protected $project;

    /**
     * root
     *
     * @var string
     */
    protected $root;

    /**
     * compile
     *
     * @var \Dever\Template\Compile
     */
    protected $compile;

    /**
     * node
     *
     * @var array
     */
    protected $node;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * load file
     * @param string $file
     * @param string $path
     *
     * @return \Dever\Template\View
     */
    public static function getInstance($file, $path = '', $project = false)
    {
        $key = $path . DIRECTORY_SEPARATOR . $file;
        if (empty(self::$instance[$key])) {
            self::$instance[$key] = new self($file, $path);

            self::$instance[$key]->project($project);
        }

        return self::$instance[$key];
    }

    /**
     * load service
     * @param string $service
     *
     * @return \Dever\Template\View
     */
    public static function get($file, $path = '', $project = false)
    {
        $class = new self($file, $path);

        $class->project($project);

        return $class->runing();
    }

    /**
     * html 主流模板引擎的方式
     * @param $file 文件名
     * @param $data 数据
     *
     * @return mixed
     */
    public static function html($file = false, $data = array(), $project = false)
    {
        $class = new self($file, '');

        $class->parse = false;

        $class->project($project, $data);

        return $class->runing();
    }

    /**
     * page
     * @param string $service
     *
     * @return \Dever\Template\View
     */
    public function page($file, $path = '')
    {
        echo $this->load($file, $path);
    }

    /**
     * load service
     * @param string $service
     *
     * @return \Dever\Template\View
     */
    public function load($file, $path = '', $project = false)
    {
        $view = View::getInstance($file, $path, $project);

        return $view->runing();
    }

    /**
     * __construct
     * @param string $file
     * @param string $path
     *
     * @return mixed
     */
    public function __construct($file, $path = '')
    {
        $this->file = $file;

        $this->path($path);

        $this->content = '';

        Config::get('template')->view = true;
    }


    /**
     * project
     * @param string $project
     * @param array $data
     *
     * @return mixed
     */
    public function project($project, $data = array())
    {
        $this->root = Config::get('base')->assets;
        $this->template = DEVER_APP_PATH;
        if ($data) {
            $this->data = $data;
        }

        $this->project = $project;

        if ($this->project) {
            $project = Project::load($this->project);
            $this->root = str_replace(DEVER_APP_PATH, $project['path'], $this->root);
            $this->template = $project['path'];
        }

        $this->template .= self::TEMPLATE . DIRECTORY_SEPARATOR;
    }

    /**
     * path
     * @param string $path
     *
     * @return \Dever\Template\View
     */
    public function path($path = false)
    {
        if (Config::get('template')->assets) {
            $template = $this->session();
        }

        if (isset($template) && $template) {
            $this->path = $template . DIRECTORY_SEPARATOR;
        } elseif ($path) {
            if ($path == 'manage') {
                Config::get('template')->template = 'manage';
            }
            $this->path = $path . DIRECTORY_SEPARATOR;
        } elseif (Config::get('template')->assets) {
            if (is_array(Config::get('template')->assets)) {
                $this->path = Env::mobile() ? Config::get('template')->assets[1] : Config::get('template')->assets[0];
            } else {
                $this->path = Config::get('template')->assets;
            }
            $this->path .= DIRECTORY_SEPARATOR;
        } else {
            $temp = explode(DIRECTORY_SEPARATOR, DEVER_APP_PATH);
            $this->path = $temp[count($temp) - 2] . DIRECTORY_SEPARATOR;
        }

        return $this;
    }

    /**
     * session
     *
     * @return string
     */
    private function session()
    {
        $template = '';
        if (Config::get('template')->name) {
            $save = new Oper();
            $template = Input::get(Config::get('template')->name);
            if ($template) {
                if ($template == 'none') {
                    $save->un(Config::get('template')->name);
                } else {
                    $save->add(Config::get('template')->name, $template);
                }
            }

            $template = $save->get(Config::get('template')->name);
        }
        return $template;
    }

    /**
     * runing
     *
     * @return \Dever\Template\View
     */
    public function runing()
    {
        $key = Uri::url();
        $this->content = $this->cache($key);
        if ($this->content) {
            return $this->content;
        }

        $this->assets()->compile();
        $this->cache($key, $this->content);

        return $this->content;
    }

    /**
     * cache
     *
     * @return mixed
     */
    private function cache($key, $data = false)
    {
        return Cache::load($key, $data, 0, 'html');
    }

    /**
     * assets
     *
     * @return \Dever\Template\View
     */
    public function assets($state = 1)
    {
        if ($this->path && strpos($this->root, DIRECTORY_SEPARATOR . $this->path) !== false) {
            $this->path = '';
        }

        $html = Config::get('template')->path ? Config::get('template')->path . DIRECTORY_SEPARATOR : self::ASSETS . DIRECTORY_SEPARATOR;

        $this->assets = $this->root . $this->path . $html;

        if ($state == 1 && !is_file($this->assets . $this->file . '.html') && Config::get('template')->assets && is_array(Config::get('template')->assets) && isset(Config::get('template')->assets[0])) {
            $this->path(Config::get('template')->assets[0] . DIRECTORY_SEPARATOR);

            return $this->assets(2);
        }

        if (Config::get('template')->template) {
            if (is_array(Config::get('template')->template)) {
                $this->name = Env::mobile() ? Config::get('template')->template[1] : Config::get('template')->template[0];
            } else {
                $this->name = Config::get('template')->template;
            }
            $this->name .= DIRECTORY_SEPARATOR;
        } else {
            $this->name = $this->path;
        }

        $this->template = $this->template . $this->name . $this->file . '.php';

        $this->replace();

        return $this;
    }

    /**
     * replace
     *
     * @return \Dever\Template\View
     */
    private function replace()
    {
        if ($this->path && !Config::get('host', $this->project)->replace) {
            if (Config::get('template', $this->project)->replace && is_array(Config::get('template', $this->project)->replace)) {
                foreach (Config::get('template', $this->project)->replace as $k => $v) {
                    $this->replaceOne($k);
                }
            }

            Config::get('host', $this->project)->replace = true;
        }
    }

    /**
     * replaceOne
     *
     * @return \Dever\Template\View
     */
    private function replaceOne($k)
    {
        if ($k == 'public') {
            return;
        }
        $key = 'assets/';

        if (Config::get('template', $this->project)->template && is_array(Config::get('template', $this->project)->template)) {
            foreach (Config::get('template', $this->project)->template as $v) {
                if (Config::get('host', $this->project)->$k && strpos(Config::get('host', $this->project)->$k, $key . $v) !== false) {
                    return;
                }
            }
        }

        if ($k != 'manage' && Config::get('host', $this->project)->$k) {
            Config::get('host', $this->project)->$k = str_replace($key, $key . $this->path, Config::get('host', $this->project)->$k);
        }
    }

    /**
     * dom
     *
     * @return \Dever\Template\View
     */
    public function dom()
    {
        $this->dom = new Dom($this->compile->template(), $this->compile->parsing());

        return $this;
    }

    /**
     * file
     *
     * @return string
     */
    public function file()
    {
        if (empty($this->compile)) {
            return '';
        }
        return $this->compile->file();
    }

    /**
     * compile
     *
     * @return \Dever\Template\View
     */
    public function compile()
    {
        $this->layout();

        $this->compile = new Compile($this->file, $this->assets, $this->template, $this->project, $this->path, $this->data);

        $this->content = $this->compile->get();
        
        if ($this->content || $this->content !== false) {
            $this->front($this->content, true);
            return $this->content;
        }
        $this->front($this->compile->getContent(), false);

        return $this->template();
    }

    /**
     * front
     *
     * @return mixed
     */
    private function front($content, $display = true)
    {
        if (strpos($content, '<dever') !== false && strpos($content, '</dever>') !== false) {
            $tag = Common::tag($content);
            if ($tag) {
                foreach ($tag as $k => $v) {
                    $method = $v['method'];
                    call_user_func_array(array($this, $method), $v['param']);
                }
                if ($display == true) {
                    $this->display();
                }
            }
        }
    }

    /**
     * layout
     *
     * @return mixed
     */
    public function layout()
    {
        if (Config::get('template')->layout && empty($this->pjax)) {
            if (array_key_exists('HTTP_X_PJAX', $_SERVER) && $_SERVER['HTTP_X_PJAX']) {
                $this->pjax = true;
            } else {
                $this->pjax = false;
            }
        }
    }

    /**
     * template
     *
     * @return \Dever\Template\View
     */
    public function template()
    {
        if (!is_file($this->template)) {
            return $this->display();
        }
        $view = $this;

        require $this->template;

        return $this->content;
    }

    /**
     * __call
     *
     * @return \Dever\Template\View
     */
    public function __call($method, $param)
    {
        $name = $desc = '';
        if (strpos($method, '_')) {
            $temp = explode('_', $method);
            $method = $temp[0];
            $name = $temp[1];
        } elseif (isset($param[0]) && is_string($param[0]) && strpos($param[0], ':')) {
            $temp = explode(':', $param[0]);
            $param[0] = $temp[1];
            $name = $temp[0];
            if (isset($temp[2])) {
                $desc = $temp[2];
            }
        }
        $this->fetch[] = array($method, $param);
        if ($name) {
            $send = array
            (
                'name' => $desc,
                'type' => $method,
                'param' => $param
            );
            $this->node[$name] = $send;
        }
        return $this;
    }

    /**
     * set　设置变量
     * @param $key 变量名
     * @param $value 变量的值
     *
     * @return \Dever\Template\View
     */
    public function set($key, $value = false)
    {
        $this->compile->parsing()->set($key, $value);

        return $this;
    }

    /**
     * call
     * @param $key
     *
     * @return \Dever\Template\View
     */
    public function call($key)
    {
        $temp = explode(':', $key);
        $node = $this->readNode($temp[0]);
        if ($node && isset($node[$temp[1]])) {
            $method = $node[$temp[1]]['type'];
            call_user_func_array(array($this, $method), $node[$temp[1]]['param']);
        }

        return $this;
    }

    /**
     * hook
     * @param $key
     *
     * @return \Dever\Template\View
     */
    public function hook($key, $value)
    {
        $this->compile->set('Dever::get(\'hook\')->$key', $value);

        $value = '<{function(){echo 1;}}>';
        $this->fetch($key, $value);

        return $this;
    }

    /**
     * display
     * @param $file 文件名
     * @param $local 是否本地化 默认不抓取文件到本地
     *
     * @return mixed
     */
    public function display($file = false, $local = false)
    {
        if ($file) {
            $this->compile->read($file, $this->assets, $this->template, $local);
        }

        if ($this->fetch) {
            if (!$this->dom) {
                $this->dom();
            }

            if (is_object($this->dom)) {
                $this->pushNode();
                foreach ($this->fetch as $k => $v) {
                    $k = $v[0];
                    $this->dom->$k($v[1]);
                }
                $this->content = $this->compile->create($this->dom->get());
            }
        } else {
            $this->content = $this->compile->create($this->content);
        }

        return $this->content;
    }

    private function nodePath($project)
    {
        $project = $project ? $project : DEVER_APP_NAME;
        return Path::get(Config::data() . 'compile' . DIRECTORY_SEPARATOR , DEVER_PROJECT . DIRECTORY_SEPARATOR . $project . DIRECTORY_SEPARATOR . 'node' . DIRECTORY_SEPARATOR);
    }

    private function pushNode()
    {
        if (!$this->node) {
            return;
        }
        $path = $this->nodePath($this->project);
        $file = Path::get($path, $this->name . $this->file . '.php');
        file_put_contents($file, '<?php return ' . var_export($this->node, true) . ';');
    }

    private function readNode($file)
    {
        $temp = explode('/', $file);
        $path = $this->nodePath($temp[0]);
        unset($temp[0]);
        $file = Path::get($path, $this->name . implode('/', $temp) . '.php');
        if (is_file($file)) {
            $node = include $file;
            return $node;
        }
        
        return false;
    }

    /**
     * result 获取当前视图模板的基本信息
     *
     * @return array
     */
    public function result()
    {
        $result['service'] = $this->template;
        $result['template'] = $this->assets . $this->file . '.html';
        $result['cmp'] = $this->compile->file();

        //$content = file_get_contents($result['template']);

        //$rule = '<script class="include"(.*?)path="(.*?)"(.*?)file="(.*?)"(.*?)><\/script>';

        //preg_match_all(DIRECTORY_SEPARATOR . $rule . '/i', $content, $result);

        return $result;
    }
}
