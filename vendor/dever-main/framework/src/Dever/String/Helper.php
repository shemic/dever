<?php namespace Dever\String;

use Dever\Http\Curl;
use Dever;
class Helper
{
    /**
     * order 生成uuid
     *
     * @return mixed
     */
    public static function uuid()
    {
        mt_srand((double)microtime() * 10000);
        $charid = strtoupper(self::id()); 
        $hyphen = chr(45);        
        $uuid   = chr(123)            
                 .substr($charid, 0, 8).$hyphen               
                 .substr($charid, 8, 4).$hyphen            
                 .substr($charid,12, 4).$hyphen            
                 .substr($charid,16, 4).$hyphen            
                 .substr($charid,20,12)            
                 .chr(125);
        return $uuid;
    }

    /**
     * order 生成订单号
     * @param string $prefix 前缀
     *
     * @return mixed
     */
    public static function order($prefix = '')
    {
        if ($prefix) {
            $prefix = substr(strtoupper($prefix), 0, 2);
        }
        return $prefix . (strtotime(date('YmdHis', time()))) . substr(microtime(), 2, 6) . sprintf('%03d', rand(0, 999));
    }

    /**
     * hide 将字符串中的某几个字符隐藏
     * @param string $string
     *
     * @return mixed
     */
    public static function hide($string, $start = 3, $len = 4, $hide = '****')
    {
        return substr_replace($string, $hide, $start, $len);
    }

    /**
     * code 一般用于生成验证码
     * @param int $num
     *
     * @return mixed
     */
    public static function code($num = 4)
    {
        $code = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
        $rand = $code[rand(0,25)]
            .strtoupper(dechex(date('m')))
            .date('d')
            .substr(time(),-5)
            .substr(microtime(),2,5)
            .sprintf('%02d',rand(0,99));
        for(
            $a = md5( $rand, true ),
            $s = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ',
            $d = '',
            $f = 0;
            $f < $num;
            $g = ord( $a[ $f ] ),
            $d .= $s[ ( $g ^ ord( $a[ $f + 8 ] ) ) - $g & 0x1F ],
            $f++
        );
        return $d;
    }

    /**
     * uid 根据uid生成码
     * @param int $uid
     *
     * @return mixed
     */
    function uid($uid, $type = 'encode')
    {
        static $source_string = 'E5FCDG3HQA4B1NOPIJ2RSTUV67MWX89KLYZ';

        if ($type == 'encode') {
            $code = '';
            $num = $uid;
            while ($num > 0) {
                $mod = $num % 35;
                $num = ($num - $mod) / 35;
                $code = $source_string[$mod].$code;
            }
            if (empty($code[3])) {
                $code = str_pad($code,4,'0',STR_PAD_LEFT);
            }
            return $code;
        } else {
            $code = $uid;
            if (strrpos($code, '0') !== false)
            $code = substr($code, strrpos($code, '0')+1);
            $len = strlen($code);
            $code = strrev($code);
            $num = 0;
            for ($i=0; $i < $len; $i++) {
                $num += strpos($source_string, $code[$i]) * pow(35, $i);
            }
            return $num;
        }
    }

    /**
     * id 生成唯一id
     * @param int $num
     *
     * @return mixed
     */
    public static function id()
    {
        $charid = strtoupper(md5(uniqid(mt_rand(), true)));
        return substr($charid, 0, 8) . substr($charid, 8, 4) . substr($charid, 12, 4) . substr($charid, 16, 4) . substr($charid, 20, 12);
    }

    /**
     * 生成随机数
     * @param int $len 长度
     * @param int $type 类型
     *
     * @return mixed
     */
    public static function rand($len, $type = 4)
    {
        $source = array("0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z");

        $config = array
            (
            0 => array("min" => 0, "max" => 9), /// 全数字
            1 => array("min" => 10, "max" => 35), /// 全小写
            2 => array("min" => 36, "max" => 61), /// 全大写
            3 => array("min" => 10, "max" => 61), /// 大小写
            4 => array("min" => 0, "max" => 61), /// 数字+大小写
        );
        if (!isset($config[$type])) {
            $type = 4;
        }

        $rand = "";
        for ($i = 0; $i < $len; $i++) {
            $rand .= $source[rand($config[$type]["min"], $config[$type]["max"])];
        }

        return $rand;
    }

    public static function xss($data)
    {
        if (!is_string($data)) {
            return $data;
        }
        $data = str_replace(array('&amp;', '&lt;', '&gt;'), array('&amp;amp;', '&amp;lt;', '&amp;gt;'), $data);
        $data = preg_replace('/(&#*\w+)[\x00-\x20]+;/u', '$1;', $data);
        $data = preg_replace('/(&#x*[0-9A-F]+);*/iu', '$1;', $data);
        $data = html_entity_decode($data, ENT_COMPAT, 'UTF-8');

        // Remove any attribute starting with "on" or xmlns
        $data = preg_replace('#(<[^>]+?[\x00-\x20"\'])(?:on|xmlns)[^>]*+>#iu', '$1>', $data);

        // Remove javascript: and vbscript: protocols
        $data = preg_replace('#([a-z]*)[\x00-\x20]*=[\x00-\x20]*([`\'"]*)[\x00-\x20]*j[\x00-\x20]*a[\x00-\x20]*v[\x00-\x20]*a[\x00-\x20]*s[\x00-\x20]*c[\x00-\x20]*r[\x00-\x20]*i[\x00-\x20]*p[\x00-\x20]*t[\x00-\x20]*:#iu', '$1=$2nojavascript...', $data);
        $data = preg_replace('#([a-z]*)[\x00-\x20]*=([\'"]*)[\x00-\x20]*v[\x00-\x20]*b[\x00-\x20]*s[\x00-\x20]*c[\x00-\x20]*r[\x00-\x20]*i[\x00-\x20]*p[\x00-\x20]*t[\x00-\x20]*:#iu', '$1=$2novbscript...', $data);
        $data = preg_replace('#([a-z]*)[\x00-\x20]*=([\'"]*)[\x00-\x20]*-moz-binding[\x00-\x20]*:#u', '$1=$2nomozbinding...', $data);

        // Only works in IE: <span style="width: expression(alert('Ping!'));"></span>
        $data = preg_replace('#(<[^>]+?)style[\x00-\x20]*=[\x00-\x20]*[`\'"]*.*?expression[\x00-\x20]*\([^>]*+>#i', '$1>', $data);
        $data = preg_replace('#(<[^>]+?)style[\x00-\x20]*=[\x00-\x20]*[`\'"]*.*?behaviour[\x00-\x20]*\([^>]*+>#i', '$1>', $data);
        $data = preg_replace('#(<[^>]+?)style[\x00-\x20]*=[\x00-\x20]*[`\'"]*.*?s[\x00-\x20]*c[\x00-\x20]*r[\x00-\x20]*i[\x00-\x20]*p[\x00-\x20]*t[\x00-\x20]*:*[^>]*+>#iu', '$1>', $data);

        // Remove namespaced elements (we do not need them)
        $data = preg_replace('#</*\w+:\w[^>]*+>#i', '', $data);

        do {
            // Remove really unwanted tags
            $old_data = $data;
            //$data = preg_replace('#</*(?:applet|b(?:ase|gsound|link)|embed|frame(?:set)?|i(?:frame|layer)|l(?:ayer|ink)|meta|object|s(?:cript|tyle)|title|xml)[^>]*+>#i', '', $data);
            $data = preg_replace('#</*(?:applet|b(?:ase|gsound|link)|frame(?:set)?|i(?:frame|layer)|l(?:ayer|ink)|meta|object|s(?:cript|tyle)|title|xml)[^>]*+>#i', '', $data);
        } while ($old_data !== $data);

        // we are done...
        return $data;
    }

    public static function idtostr($input)
    {
        if (!is_numeric($input) || $input < 0) {
            return false;
        }

        $input = substr("00000000" . $input, -8);
        $sandNum = $input % 10;
        srand($input);
        $randstr = "" . rand(1, 9) . self::rand(7, 0);

        $retstr1 = "";
        $retstr2 = "";
        for ($i = 0; $i < 4; $i++) {
            $retstr1 .= $randstr[$i] . $input[$i];
            $retstr2 .= $input[7 - $i] . $randstr[7 - $i];
        }
        $retstr1 = substr(self::rand(6) . "g" . dechex($retstr1), -7);
        $retstr2 = substr(self::rand(6) . "g" . dechex($retstr2), -7);
        srand(time() + $input);
        $retstr = "1" . $sandNum;
        for ($i = 0; $i < 7; $i++) {
            $retstr .= $retstr1[$i] . $retstr2[$i];
        }
        return $retstr;
    }

    public static function strtoid($str)
    {
        if (strlen($str) != 16) {
            return $str;
        }
        //$type = $str1[0];

        $sandNum = $str[1];
        $retstr1 = $retstr2 = '';
        for ($i = 0; $i < 7; $i++) {
            if ($str[2+$i*2] == 'g') {
                $retstr1 = "";
            } else {
                $retstr1 .= $str[2+$i*2];
            }

            if ($str[3+$i*2] == 'g') {
                $retstr2 = "";
            } else {
                $retstr2 .= $str[3+$i*2];
            }
        }

        $retstr1 = "g".substr("00000000".hexdec($retstr1),-8);
        $retstr2 = "g".substr("00000000".hexdec($retstr2),-8);
        $ret1 = $ret2 = "";
        for ($i = 0; $i < 4; $i++) {
            $ret1 .= $retstr1[$i*2+2];
            $ret2 .= $retstr2[7-$i*2];
        }
        $ret = $ret1 * 10000 + $ret2;
        return $ret;
    }

    /**
     * cut
     * @param string $string
     * @param string $length
     * @param string $etc
     *
     * @return array
     */
    public static function cut($string, $length = 80, $etc = '...')
    {
        $result = '';
        $string = html_entity_decode(trim(strip_tags($string)), ENT_QUOTES, 'utf-8');
        for ($i = 0, $j = 0; $i < strlen($string); $i++) {
            if ($j >= $length) {
                for ($x = 0, $y = 0; $x < strlen($etc); $x++) {
                    if ($number = strpos(str_pad(decbin(ord(substr($string, $i, 1))), 8, '0', STR_PAD_LEFT), '0')) {
                        $x += $number - 1;
                        $y++;
                    } else {
                        $y += 0.5;
                    }
                }
                $length -= $y;
                break;
            }
            if ($number = strpos(str_pad(decbin(ord(substr($string, $i, 1))), 8, '0', STR_PAD_LEFT), '0')) {
                $i += $number - 1;
                $j++;
            } else {
                $j += 0.5;
            }
        }
        for ($i = 0; (($i < strlen($string)) && ($length > 0)); $i++) {
            if ($number = strpos(str_pad(decbin(ord(substr($string, $i, 1))), 8, '0', STR_PAD_LEFT), '0')) {
                if ($length < 1.0) {
                    break;
                }
                $result .= substr($string, $i, $number);
                $length -= 1.0;
                $i += $number - 1;
            } else {
                $result .= substr($string, $i, 1);
                $length -= 0.5;
            }
        }
        //$result = htmlentities($result, ENT_QUOTES, 'utf-8');
        if ($i < strlen($string)) {
            $result .= $etc;
        }
        return $result;
    }

    /**
     * strlen
     * @param string $string
     *
     * @return array
     */
    public static function strlen($string)
    {
        preg_match_all("/./us", $string, $match);

        return count($match[0]);
    }

    /**
     * str_explode
     * @param string $value
     * @param string $index
     *
     * @return array
     */
    public static function str_explode($value, $num = 2)
    {
        $len = mb_strlen($value);
        $result = array();
        for ($i = 0; $i < $len; $i = $i + $num) {
            $result[$i / $num] = mb_substr($value, $i, $num);
        }

        return $result;
    }

    /**
     * replace
     * @param string $replace
     * @param string $value
     * @param string $content
     *
     * @return string
     */
    public static function replace($replace, $value, $content)
    {
        if (!$replace) {
            return $value;
        }

        $content = str_replace($replace, $value, $content);

        return $content;
    }

    /**
     * addstr
     * @param string $str
     * @param string $index
     * @param string $sub
     *
     * @return string
     */
    public static function addstr($str, $i, $sub)
    {
        $str = self::str_explode($str, 1);
        $length = count($str);
        $num = floor($length / $i);
        for ($a = 1; $a <= $num; $a++) {
            $start = '';
            $b = $a * $i;
            foreach ($str as $k => $v) {
                if ($k == $b) {
                    $str[$k] = $sub . $v;
                }
            }
        }

        return implode('', $str);
    }

    /**
     * qqvideo
     * @param string $link
     *
     * @return string
     */
    public static function qqvideo($link)
    {
        if (!$link) {
            return '';
        }
        $temp = explode('/', $link);
        $temp = end($temp);
        $key = str_replace('.html', '', $temp);

        $api = 'http://vv.video.qq.com/getinfo?vids='.$key.'&platform=101001&charge=0&otype=json';

        $data = Curl::get($api);

        $data = Dever::json_decode(str_replace(array('QZOutputJson=', ';'), '', $data));
        if (isset($data['vl']) && isset($data['vl']['vi'][0])) {
            $data = $data['vl']['vi'][0];
            $urls = $data['ul']['ui'];
            foreach ($urls as $k => $v) {
                $url = $v['url'];
                break;
                if (strstr($v['url'], 'vhot2.qqvideo.tc.qq.com')) {
                    $url = $v['url'];
                }
            }
            if (empty($url)) {
                print_r($data);die;
                return $this->decode_api($link);
            }
            $link = $url . $data['fn'] . '?vkey=' . $data['fvkey'];
        } else {
            return '';
        }

        return $link;
    }

    public function ishtml($html)
    {
        if($html != strip_tags($html)) {
            return true;
        } else {
            return false;
        }
    }

    /**
     * filter 过滤一些有问题的代码
     * @param string $content 内容
     *
     * @return mixed
     */
    public static function filter($content)
    {
        if (strpos($content, 'font-family') !== false) {
            $content = preg_replace('/font-family:([\s\S]*?);/i', '', $content);
            $content = preg_replace('/"="([\s\S]*?)">/i', '">', $content);
        }

        return $content;
    }
}
