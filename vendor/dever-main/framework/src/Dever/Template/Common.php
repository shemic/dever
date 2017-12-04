<?php namespace Dever\Template;

use Dever\Http\Url;
use Dever\Loader\Config;

class Common
{
    /**
     * 读取dom
     * @param string $value
     * @param string $index
     *
     * @return array
     */
    public static function dom($value)
    {
        return \Sunra\PhpSimple\HtmlDomParser::str_get_html($value,
            $lowercase          = true, 
            $forceTagsClosed    = true, 
            $target_charset     = DEFAULT_TARGET_CHARSET, 
            $stripRN            = false, 
            $defaultBRText      = DEFAULT_BR_TEXT, 
            $defaultSpanText    = DEFAULT_SPAN_TEXT);
    }

    /**
     * 设置循环里每行不同的值
     * @param string $value
     * @param string $index
     *
     * @return array
     */
    public static function lace($value, $index, $key = 2, $num = 0)
    {
        $value = explode(',', $value);
        if (!isset($value[1])) {
            $value[1] = '';
        }

        if ($index > 0 && $index % $key == $num) {
            return $value[0];
        } else {
            return $value[1];
        }
    }

    /**
     * 设置循环里最新一条数据的值
     * @param string $value
     * @param string $index
     *
     * @return array
     */
    public static function first($value, $index)
    {
        if ($index == 0) {
            return $value;
        }
    }

    /**
     * 设置循环里最后一条数据的值
     * @param string $value
     * @param string $index
     * @param string $total
     *
     * @return array
     */
    public static function last($value, $index, $total)
    {
        if ($index == $total) {
            return $value;
        }
    }

    /**
     * 除了最后一条
     * @param string $value
     * @param string $index
     * @param string $total
     *
     * @return array
     */
    public static function other($value, $index, $total)
    {
        if ($index != $total) {
            return $value;
        }
    }

    /**
     * 对多维数组进行排序
     * @param string $array
     * @param string $field
     * @param string $desc
     *
     * @return array
     */
    private function sort($array, $field, $desc = false)
    {
        $fieldArr = array();
        foreach ($array as $k => $v) {
            $fieldArr[$k] = $v[$field];
        }
        $sort = $desc == false ? SORT_ASC : SORT_DESC;
        array_multisort($fieldArr, $sort, $array);
    }

    /**
     * 生成html a标记 以后统一修改
     * @param string $name
     * @param string $link
     * @param string $target
     *
     * @return array
     */
    private function a($name, $link, $target = '_self')
    {
        return '<a href="' . Url::get($link) . '" target="' . $target . '">' . $name . '</a>';
    }

    /**
     * 生成html img标记 以后统一修改
     * @param string $link
     * @param string $target
     *
     * @return array
     */
    private function img($link, $target = '_self')
    {
        return self::a('<img src="' . $link . '" />', $link, $target);
    }

    /**
     * 获取assets的文件网络路径
     * @param string $value
     * @param string $type
     *
     * @return array
     */
    public static function assets($value, $type = 'css')
    {
        return Config::get('host')->$type . $value;
    }

    /**
     * 生成table
     * @param string $data
     * @param string $class
     *
     * @return array
     */
    public static function table($data, $class = '', $num = 1)
    {
        if ($class) {
            $style = 'class=' . $class . '';
        } else {
            $style = 'border=1 width=100% height=100%';
        }

        $html = '<table ' . $style . '>';

        foreach ($data as $k => $v) {
            if (is_array($v) && $num == 2) {
                $tbody = array($k);
                foreach ($v as $j) {
                    array_push($tbody, $j);
                }
                $html .= self::tbody($tbody);
            } else {
                if (is_array($v)) {
                    $v = self::table($v, $class);
                    //$v = var_export($v, true);
                }
                if (is_numeric($k)) {
                    $html .= self::tbody(array($v));
                } else {
                    $html .= self::tbody(array($k, $v));
                }
            }
        }

        $html .= '</table>';

        return $html;
    }

    /**
     * 生成tbody
     * @param string $data
     * @param string $class
     *
     * @return array
     */
    public static function tbody($data)
    {
        $html = '<tr>';
        foreach ($data as $k => $v) {
            if ($k == 0) {
                $html .= '<td style=width:30%;line-height:1.75;font-weight:bold>' . $v . '</td>';
            } elseif ($k == 1) {
                $html .= '<td style=width:30%;word-break:break-all;word-wrap:break-word;>' . $v . '</td>';
            } else {
                $html .= '<td style=word-break:break-all;word-wrap:break-word;>' . $v . '</td>';
            }
        }

        $html .= '</tr>';

        return $html;
    }

    /**
     * dever tag parsing
     * @param string $content
     *
     * @return string
     */
    public static function tag($content)
    {
        $parsing = array();
        if (strpos($content, '<dever') !== false && strpos($content, '</dever>') !== false) {
            preg_match_all('/<dever(.*?)>([\s\S]*?)<\/dever>/i', $content, $matches);
            if (isset($matches[2][0]) && $matches[2][0]) {
                $parsing = self::parsing($matches[2][0]);
            }
        }

        return $parsing;
    }

    /**
     * dever parsing
     * @param string $content
     *
     * @return string
     */
    public static function parsing($content, $api = false)
    {
        $parsing = array();
        #提取每行开头为$的字符串
        preg_match_all('/(?:^|\n)\$\("(.*?)"\).(.*?)\("(.*?)"([\s\S]*?)\);/i', $content, $matches);
        if (isset($matches[2][0]) && $matches[2][0]) {
            foreach ($matches[2] as $k => $v) {
                $parsing[$k]['method'] = $v;
                $parsing[$k]['param'][0] = $matches[1][$k];
                $parsing[$k]['param'][1] = $matches[3][$k];
                if (isset($matches[4][$k]) && $matches[4][$k]) {
                    $matches[4][$k] = ltrim($matches[4][$k], ',');
                    $parsing[$k]['param'][2] = json_decode($matches[4][$k], true);
                }
                if ($api) {
                    self::parsingApi($parsing[$k]['param'][1], isset($matches[4][$k]) ? $matches[4][$k] : false);
                }
            }
        }
        return $parsing;
    }

    /**
     * parsingApi
     * @param string $content
     * @param string $url
     *
     * @return string
     */
    private static function parsingApi($url, $content)
    {
        $api = array();
        if (strpos($url, '/') && (strpos($url, '.') || strpos($url, '-') || strpos($url, '!'))) {
            $api[$url] = array();
            if ($content) {
                preg_match_all('/(\|)\$v\.(.*?)"([\s\S]*?)[\}]/i', $content, $parent);
                preg_match_all('/\$v\.(.*?)["|.]/i', $content, $current);
                foreach ($current[1] as $k => $v) {
                    $api[$url][$v] = $k;
                }
                foreach ($parent[3] as $k => $v) {
                    preg_match_all('/\$v([0-9]+)\.(.*?)["|.]/i', $v, $child);
                    $api[$url][$parent[2][$k]] = array_flip($child[2]);
                }
            }

            if (!Config::get('base')->ai) {
                Config::get('base')->ai = array();
            }

            Config::get('base')->ai = array_merge(Config::get('base')->ai, $api);
        }
    }

    /**
     * fetch
     * @param string $content
     * @param array $fetch
     *
     * @return string
     */
    public static function fetch($content, $parsing)
    {
        $dom = new Dom($content, new Parsing());
        foreach ($parsing as $k => $v) {
            $k = $v['method'];
            $dom->$k($v['param']);
        }
        return $dom->get();
    }
}