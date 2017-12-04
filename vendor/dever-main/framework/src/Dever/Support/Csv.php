<?php namespace Dever\Support;
#csv类，收集自：http://www.thinkphp.cn/code/2198.html
class Csv
{
    /**
     * read csv
     * @param string $file      csv file
     * @param int $lines        read line
     * @param int $offset       start line
     * @return array
     */
    public static function read($file, $lines = 0, $offset = 0)
    {
        if (!$fp = fopen($file, 'r')) {
            return false;
        }
        $i = $j = 0;
        while (false !== ($line = fgets($fp))) {
            if ($i++ < $offset) {
                continue;
            }
            break;
        }
        $data = array();
        while (($j++ < $lines) && !feof($fp)) {
            $data[] = fgetcsv($fp);
        }
        fclose($fp);
        return $data;
    }

    /**
     * export csv
     * @param array $data
     * @param array $header_data
     * @param string $file_name
     * @return string
     */
    public static function export($data = [], $header_data = [], $file_name = '')
    {
        header('Content-Type: application/vnd.ms-excel');
        header('Content-Disposition: attachment;filename='.$file_name);
        header('Cache-Control: max-age=0');
        $fp = fopen('php://output', 'a');
        if (!empty($header_data)) {
            foreach ($header_data as $key => $value) {
                $header_data[$key] = iconv('utf-8', 'gbk', $value);
            }
            fputcsv($fp, $header_data);
        }
        $num = 0;
        //每隔$limit行，刷新一下输出buffer，不要太大，也不要太小
        $limit = 100000;
        //逐行取出数据，不浪费内存
        $count = count($data);
        if ($count > 0) {
            for ($i = 0; $i < $count; $i++) {
                $num++;
                //刷新一下输出buffer，防止由于数据过多造成问题
                if ($limit == $num) {
                    ob_flush();
                    flush();
                    $num = 0;
                }
                $row = $data[$i];
                foreach ($row as $key => $value) {
                    $row[$key] = iconv('utf-8', 'gbk', $value);
                }
                fputcsv($fp, $row);
            }
        }
        fclose($fp);
    }

    /**
     * export csv 2
     * @param array $data
     * @param array $header_data
     * @param string $file_name
     * @return string
     */
    public static function export_2($data = [], $header_data = [], $file_name = '')
    {
        header('Content-Type: application/octet-stream');
        header('Content-Disposition: attachment; filename=' . $file_name);
        if (!empty($header_data)) {
            echo iconv('utf-8','gbk//TRANSLIT','"'.implode('","',$header_data).'"'."\n");
        }
        foreach ($data as $key => $value) {
            $output = array();
            $output[] = $value['id'];
            $output[] = $value['name'];
            echo iconv('utf-8','gbk//TRANSLIT','"'.implode('","', $output)."\"\n");
        }
    }
}
