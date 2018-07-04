<?php namespace Dever\String;

use Dever\Loader\Config;

class Encrypt
{
    /**
     * key
     *
     * @var string
     */
    private static $key = 'qwertyuiop12345asdfghjkl67890zxcvbnm';

    /**
     * encode
     * @param  string  $string
     * @param  string  $key
     *
     * @return string
     */
    public static function encode($string, $key = "")
    {
        $ckey_length = 5;

        if (!$key) {
            $key = Config::get('base')->token;
            if ($key) {
                $key = sha1($key);
            } else {
                $key = sha1(self::$key);
            }
        }

        $keya = md5(substr($key, 0, 16));
        $keyb = md5(substr($key, 16, 16));
        $keyc = $ckey_length ? substr(md5(microtime()), -$ckey_length) : ''; //md5串后4位，每次不一样

        $cryptkey = $keya . md5($keya . $keyc); //两个md5串
        $key_length = strlen($cryptkey); //64

        $string = sprintf('%010d', time()) . substr(md5($string . $keyb), 0, 16) . $string;
        $string_length = strlen($string);

        $result = '';
        $box = range(0, 255);

        $rndkey = array();
        for ($i = 0; $i <= 255; $i++) {
            $rndkey[$i] = ord($cryptkey[$i % $key_length]); //生成一个255个元素的数组
        }

        for ($j = $i = 0; $i < 256; $i++) {
            //将$box数组转换为无序并且个数不变的数据
            $j = ($j + $box[$i] + $rndkey[$i]) % 256;
            $tmp = $box[$i];
            $box[$i] = $box[$j];
            $box[$j] = $tmp;
        }

        for ($a = $j = $i = 0; $i < $string_length; $i++) {
            $a = ($a + 1) % 256;
            $j = ($j + $box[$a]) % 256;
            $tmp = $box[$a];
            $box[$a] = $box[$j];
            $box[$j] = $tmp;
            $result .= chr(ord($string[$i]) ^ ($box[($box[$a] + $box[$j]) % 256]));
        }

        return $keyc . str_replace('=', '', self::base64_encode($result));

    }

    /**
     * decode
     * @param  string  $string
     * @param  string  $key
     *
     * @return string
     */
    public static function decode($string, $key = "")
    {
        $ckey_length = 5;

        if (!$key) {
            $key = Config::get('base')->token;
            if ($key) {
                $key = sha1($key);
            } else {
                $key = sha1(self::$key);
            }
        }

        $keya = md5(substr($key, 0, 16));
        $keyb = md5(substr($key, 16, 16));
        $keyc = $ckey_length ? substr($string, 0, $ckey_length) : ''; //和encrypt时的$keyc一样

        $cryptkey = $keya . md5($keya . $keyc);
        $key_length = strlen($cryptkey);

        $string = self::base64_decode(substr($string, $ckey_length));
        $string_length = strlen($string);

        $result = '';
        $box = range(0, 255);

        $rndkey = array();
        for ($i = 0; $i <= 255; $i++) {
            $rndkey[$i] = ord($cryptkey[$i % $key_length]);
        }

        for ($j = $i = 0; $i < 256; $i++) {
            //和encrypt时的$box一样
            $j = ($j + $box[$i] + $rndkey[$i]) % 256;
            $tmp = $box[$i];
            $box[$i] = $box[$j];
            $box[$j] = $tmp;
        }

        for ($a = $j = $i = 0; $i < $string_length; $i++) {
            //核心操作，解密
            $a = ($a + 1) % 256;
            $j = ($j + $box[$a]) % 256;
            $tmp = $box[$a];
            $box[$a] = $box[$j];
            $box[$j] = $tmp;
            $result .= chr(ord($string[$i]) ^ ($box[($box[$a] + $box[$j]) % 256]));
        }

        if (substr($result, 10, 16) == substr(md5(substr($result, 26) . $keyb), 0, 16)) {
            return substr($result, 26);
        } else {
            return '';
        }

    }

    /**
     * base64_encode
     * @param  string  $string
     *
     * @return string
     */
    public static function base64_encode($string)
    {
        if (!$string) {
            return false;
        }
        $encodestr = base64_encode($string);
        $encodestr = str_replace(array('+', '/'), array('-', '_'), $encodestr);
        return $encodestr;
    }

    /**
     * base64_decode
     * @param  string  $string
     *
     * @return string
     */
    public static function base64_decode($string)
    {
        if (!$string) {
            return false;
        }
        $string = str_replace(array('-', '_'), array('+', '/'), $string);
        $decodestr = base64_decode($string);
        return $decodestr;
    }
}
