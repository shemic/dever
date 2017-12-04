<?php namespace Dever\Loader;

use Dever\String\Helper;
use Dever\Output\Export;
use Dever\Output\Debug;

class Library
{
    /**
     * DEFAULT_SRC
     *
     * @var string
     */
    const DEFAULT_SRC = 'src';

    /**
     * file
     *
     * @var array
     */
    protected $file;

    /**
     * class
     *
     * @var array
     */
    protected $class;

    /**
     * project
     *
     * @var array
     */
    protected $project;

    /**
     * function
     *
     * @var array
     */
    protected $function;

    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * load
     *
     * @return \Dever\Loader\Library
     */
    public static function get()
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
            spl_autoload_register(array(self::$instance, 'autoload'));
        }

        return self::$instance;
    }

    /**
     * autoload
     *
     * @return mixed
     */
    public function autoload($class)
    {
        if ($class) {
            list($file, $project) = $this->getProject($class);
            if ($file && $project) {
                $state = $this->import($class, $file, $project['path']);

                $project = ucfirst($project['name']);
                if ($state) {
                    return Helper::replace($project . '\\', $project . '\\'.ucfirst(self::DEFAULT_SRC).'\\', $class);
                } else {
                    return $class;
                }
            }
        }
        
        return false;
    }

    public function getClass($class)
    {
        $class = strtolower($class);
        if (strpos($class, '\\') !== false) {
            $class = explode('\\', $class);
        } else {
            $class = explode('/', $class);
        }
        if (isset($class[0]) && isset($class[1])) {
            return $class;
        }
        return false;
    }

    /**
     * getProject
     *
     * @return mixed
     */
    public function getProject($class)
    {
        $class = $this->getClass($class);
        if (!$class) {
            return array(false, false);
        }
        $project = Project::load($class[0]);

        if ($project) {
            $this->loadFunction($project);
            unset($class[0]);
            $count = count($class);
            foreach ($class as $k => $v) {
                if ($k > 1 || $k == $count) {
                    $class[$k] = ucfirst($v);
                }
            }
            $class = implode('/', $class);
            return array($class, $project);
        } else {
            return array($class, false);
        }
        
        
    }

    /**
     * project
     * @param array $array
     *
     * @return array
     */
    public function import($source, $lower, $path)
    {
        $file = $path . $lower . '.php';
        $common = 'common/';
        if (substr($path, 0, strlen($common)) === $common) {
            $include = get_include_path();
            if (strpos($include, ':')) {
                $temp = explode(':', $include);
                $path = $temp[1] . '/' . $path;
            }
        }
        
        if (is_file($file)) {
            $this->includeFile($file);
            return false;
        }

        if (strpos($lower, 'plugin') !== false && defined('DEVER_APP_SETUP')) {
            $file = str_replace($path, DEVER_APP_SETUP, $file);
            if (is_file($file)) {
                $this->includeFile($file);
                return false;
            }
        }

        return $this->includeSrc($source, $lower, $path);
    }

    /**
     * includeSrc
     *
     * @return mixed
     */
    public function includeSrc($source, $lower, $path)
    {
        $file = $path . self::DEFAULT_SRC . '/' . $lower . '.php';
        if (is_file($file)) {
            $this->includeFile($file);
            return true;
        } else {
            $temp = explode('\\', $source);
            unset($temp[0]);
            $source = implode('/', $temp);
            $file = $path . self::DEFAULT_SRC . '/' . $source . '.php';
            if (is_file($file)) {
                $this->includeFile($file);
            } else {
                Export::alert('file_exists', $file);
            }
            return false;
        }
    }

    /**
     * includeFile
     *
     * @return mixed
     */
    public function includeFile($file)
    {
        if (empty($this->file[$file])) {
            $this->file[$file] = true;
            include $file;
        }
    }

    /**
     * loadFunction
     *
     * @return mixed
     */
    public function loadFunction($project)
    {
        if (empty($this->function[$project['name']])) {
            $this->function[$project['name']] = true;
            $file  = $project['path'] . 'common.php';
            if (is_file($file)) {
                include $file;
            }
        }
    }

    /**
     * loadClass
     *
     * @return mixed
     */
    public function loadClass($class)
    {
        $first = substr($class, 0, 1);
        if ($first == '/') {
            $class = DEVER_APP_NAME . $class;
        }
        $class = Helper::replace('/', '\\', $class);
        if (strpos($class, '\\')) {
            $class = implode('\\', array_map('ucfirst', explode('\\', $class)));
        } else {
            $class = ucfirst($class);
        }

        if (empty($this->class[$class])) {
            $className = $this->autoload($class);
            if ($className) {
                $this->class[$class] = new $className();
            } else {
                $this->class[$class] = $class;
            }
        }

        return $this->class[$class];
    }
}
