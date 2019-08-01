<?php namespace Dever\Loader;

use Dever\Support\Path;
use Dever\Http\Url;

class Project
{
    /**
     * content
     *
     * @var array
     */
    protected static $content;

    /**
     * get
     * @param string $key
     * @param string $param
     *
     * @return string
     */
    public static function content($update = false)
    {
        $file = Path::get(Config::data() . 'project' . DIRECTORY_SEPARATOR, DEVER_PROJECT . '.php');

        if (self::$content && $update == false) {
            return $file;
        }

        if (is_file($file)) {
            require $file;

            self::$content = $project;
        }

        return $file;
    }

    /**
     * import
     * @param string $name
     *
     * @return string
     */
    public static function import($name)
    {
        if (isset(self::$content[$name])) {
            Library::get()->loadFunction(self::$content[$name]);
        }
    }

    /**
     * register
     *
     * @return string
     */
    public static function register()
    {
        $file = self::content();

        if (empty(self::$content[DEVER_APP_NAME])) {
            self::initFile($file);
        } elseif (isset(self::$content[DEVER_APP_NAME])) {
            self::updateFile();
        }
    }

    /**
     * updateFile
     *
     * @return string
     */
    private static function updateFile()
    {
        # 是否同步更新不同的path和url
        if (Config::get('host')->sync_path && self::$content[DEVER_APP_NAME]['path'] != DEVER_APP_PATH) {
            self::update(DEVER_APP_NAME, 'path', DEVER_APP_PATH);
        }

        if (defined('DEVER_APP_SETUP') && Config::get('host')->sync_path && self::$content[DEVER_APP_NAME]['setup'] != DEVER_APP_SETUP) {
            self::update(DEVER_APP_NAME, 'setup', DEVER_APP_SETUP);
        }

        if (Config::get('host')->sync_url && Config::get('host')->base && self::$content[DEVER_APP_NAME]['url'] != Config::get('host')->base) {
            self::update(DEVER_APP_NAME, 'url', Config::get('host')->base);
        }
    }

    /**
     * initFile
     * @param string $file
     *
     * @return string
     */
    private static function initFile($file)
    {
        self::init();
        $this->updateContent($file);
        if (self::load('manage') && Import::load('manage/auth.data')) {
            Import::load('manage/menu.load');
        }
    }

    /**
     * init
     *
     * @return string
     */
    private static function init()
    {
        $config = array();

        self::initConfig($config);

        self::setConfig($config);

        self::setIncludePath($config);

        self::$content[DEVER_APP_NAME] = $config;

        unset($config);
    }

    /**
     * initConfig
     * @param  array $config
     *
     * @return mixed
     */
    private static function initConfig(&$config)
    {
        $url = Config::get('host')->base ? Config::get('host')->base : DEVER_APP_HOST;
        $config = array(
            'name' => DEVER_APP_NAME,
            'path' => DEVER_APP_PATH,
            'url' => $url,
            'lang' => DEVER_APP_NAME,
            'order' => 1,
            'icon' => '',
            'entry' => defined('DEVER_ENTRY') ? DEVER_ENTRY : 'index.php',
        );
    }

    /**
     * setIncludePath
     * @param  array $config
     *
     * @return mixed
     */
    private static function setIncludePath(&$config)
    {
        if (defined('DEVER_INCLUDE_PATH')) {
            $config['base'] = DEVER_INCLUDE_PATH;

            $path = DEVER_APP_PATH;
            $base = DEVER_INCLUDE_PATH;
            if (strstr(DEVER_APP_PATH, DEVER_PATH)) {
                $path = str_replace(DEVER_PATH, '', DEVER_APP_PATH);
            } elseif (strstr(DEVER_APP_PATH, Config::get('base')->path)) {
                $temp = explode(Config::get('base')->path, DEVER_APP_PATH);
                $base = $temp[0];
                $path = $temp[1];
            }
            $config['rel'] = $path;
        }
    }

    /**
     * setConfig
     * @param  array $config
     *
     * @return mixed
     */
    private static function setConfig(&$config)
    {
        if (defined('DEVER_APP_LANG')) {
            $config['lang'] = DEVER_APP_LANG;
        }

        if (defined('DEVER_MANAGE_ICON')) {
            $config['icon'] = DEVER_MANAGE_ICON;
        }

        if (defined('DEVER_MANAGE_ORDER')) {
            $config['order'] = DEVER_MANAGE_ORDER;
        }

        if (defined('DEVER_APP_SETUP')) {
            $config['setup'] = DEVER_APP_SETUP;
        }

        if (defined('DEVER_APP_LIB')) {
            $config['lib'] = DEVER_APP_LIB;
        }
    }

    /**
     * update
     * @param  string $key
     * @param  string $index
     * @param  string $value
     *
     * @return mixed
     */
    public static function update($key, $index, $value)
    {
        $file = self::content();

        if (isset(self::$content[$key])) {
            self::$content[$key][$index] = $value;

            self::updateContent($file);
        }
    }

    /**
     * update
     * @param  string $key
     * @param  string $index
     * @param  string $value
     *
     * @return mixed
     */
    public static function updateContent($file)
    {
        file_put_contents($file, '<?php $project = ' . var_export(self::$content, true) . ';');
    }

    /**
     * read
     *
     * @return mixed
     */
    public static function read()
    {
        return self::$content;
    }

    /**
     * load
     * @param string $project
     *
     * @return array
     */
    public static function load($project)
    {
        $config = false;

        if (is_array($project)) {
            foreach ($project as $one) {
                $config = self::load($one);
                if ($config) {
                    break;
                }
            }
        } else {
            if (isset(self::$content[$project])) {
                $config = self::$content[$project];
            }

            if (isset(Config::get('host')->project[$project])) {
                if ($config) {
                    $config = array_merge($config, Config::get('host')->project[$project]);
                } else {
                    $config = Config::get('host')->project[$project];
                    $config['name'] = $project;
                }
            }
        }

        return $config;
    }
}
