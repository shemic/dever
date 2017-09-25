<?php namespace Dever\Support;

class Path
{
    /**
     * get
     * @param string $path
     * @param string $file
     *
     * @return array
     */
    public static function get($path, $file = '')
    {
        self::createPath($path);
        self::createFile($file);

        if ($file && strpos($file, DIRECTORY_SEPARATOR) !== false) {
            self::create($path, $file);
        } else {
            $path .= $file;
        }

        return $path;
    }

    /**
     * create
     * @param string $path
     * @param string $file
     *
     * @return mixed
     */
    private static function create(&$path, $file)
    {
        $array = explode(DIRECTORY_SEPARATOR, $file);
        $count = count($array) - 2;
        for ($i = 0; $i <= $count; $i++) {
            $path .= $array[$i] . DIRECTORY_SEPARATOR;
            self::createPath($path);
        }
        $path .= $array[$i];
    }

    /**
     * createPath
     * @param string $path
     *
     * @return mixed
     */
    private static function createPath($path)
    {
        if (!is_dir($path)) {
            mkdir($path);
            @chmod($path, 0755);
            @system('chmod -R 777 ' . $path);
        }
    }

    /**
     * createFile
     * @param string $file
     *
     * @return mixed
     */
    private static function createFile(&$file)
    {
        $file = str_replace('/', DIRECTORY_SEPARATOR, $file);
    }
}
