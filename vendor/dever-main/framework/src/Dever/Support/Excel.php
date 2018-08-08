<?php namespace Dever\Support;

use Dever\Output\Export;
# 以下代码来源自网络

class Excel
{
    /**
     * instance
     *
     * @var string
     */
    protected static $instance;

    /**
     * cell
     *
     * @var array
     */
    private $cell = array('A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', 'AA', 'AB', 'AC', 'AD', 'AE', 'AF', 'AG', 'AH', 'AI', 'AJ', 'AK', 'AL', 'AM', 'AN', 'AO', 'AP', 'AQ', 'AR', 'AS', 'AT', 'AU', 'AV', 'AW', 'AX', 'AY', 'AZ');

    /**
     * getInstance
     *
     * @return Dever\Support\Excel
     */
    public static function getInstance()
    {
        if (empty(self::$instance)) {
            self::$instance = new self();
        }

        return self::$instance;
    }

    /**
     * export
     *
     * @return mixed
     */
    public static function export($data = array(), $header = array(), $fileName = '', $sheet = 0, $sheetName = '', $method = false)
    {
        if (!$method) {
            $state = class_exists('\PHPExcel');
            if ($state) {
                $method = 'excel';
            } else {
                $method = 'csv';
            }
        }

        $method .= '_export';

        return self::getInstance()->$method($data, $header, $fileName, $sheet, $sheetName);
    }

    /**
     * import
     *
     * @return array
     */
    public static function import($file, $sheet = 0, $offset = 0)
    {
        if (!$method) {
            $state = class_exists('\PHPExcel');
            if ($state) {
                $method = 'excel';
            } else {
                $method = 'csv';
            }
        }

        $method .= '_import';

        return self::getInstance()->$method($file, $sheet, $offset);
    }

    private function excel_import($file = '', $sheet = 0, $offset = 0)
    {
        $file = iconv("utf-8", "gb2312", $file);
        if(empty($file) OR !file_exists($file)) {
            Export::alert('file not exists!');
        }
        $objRead = new \PHPExcel_Reader_Excel2007();
        if (!$objRead->canRead($file)) {
            $objRead = new \PHPExcel_Reader_Excel5();
            if (!$objRead->canRead($file)) {
                Export::alert('file not exists!');
            }
        }
      
        $obj = $objRead->load($file);
        $currSheet = $obj->getSheet($sheet);
        $columnH = $currSheet->getHighestColumn();
        $columnCnt = array_search($columnH, $this->cell);
        $rowCnt = $currSheet->getHighestRow();
      
        $data = array();
        for ($_row = 1; $_row <= $rowCnt; $_row++) {
            for ($_column = 0; $_column <= $columnCnt; $_column++) {
                $cellId = $this->cell[$_column].$_row;
                //$cellValue = $currSheet->getCell($cellId)->getValue();
                $cellValue = $currSheet->getCell($cellId)->getCalculatedValue();
                if ($cellValue instanceof \PHPExcel_RichText) {
                    $cellValue = $cellValue->__toString();
                }
                $data[$_row][$this->cell[$_column]] = $cellValue;
            }
        }
      
        return $data;  
    }

    private function excel_export($data = array(), $header = array(), $fileName = '', $sheet = 0, $sheetName = '')
    {
        $xls = new \PHPExcel();

        if ($sheet > 0) {
            $xls->createSheet();
        }
        $act = $xls->setActiveSheetIndex($sheet);

        if ($sheetName) {
            $act->setTitle($sheetName);
        }
        
        $row = 1;
        if($header) {
            $i = 0;
            foreach($header AS $v) {
                $act->setCellValue($this->cell[$i] . $row, $v);
                $act->getColumnDimension($this->cell[$i])->setWidth(20);
                $i++;
            }
            $row++;
        }

        if($data) {
            $i = 0;
            foreach($data AS $v) {
                $j = 0;
                foreach($v AS $cell) {
                    $act->setCellValue($this->cell[$j] . ($i+$row), $cell);
                    $act->getColumnDimension($this->cell[$j])->setWidth(20);
                    $j++;
                }
                $i++;
            }
        }

        if (!$fileName) {
            $fileName = uniqid(time(),true);
        }

        $write = \PHPExcel_IOFactory::createWriter($xls, 'Excel2007');
        header('pragma:public');
        header("Content-Disposition:attachment;filename=$fileName.xlsx");
        $write->save('php://output');
    }

    private function csv_import($file, $lines = 0, $offset = 0)
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

    private function csv_export($data = array(), $header = array(), $fileName = '', $sheet = 0, $sheetName = '')
    {
        header('Content-Type: application/vnd.ms-excel;charset=gb2312');
        header('Content-Disposition: attachment;filename='.$fileName.'.csv');
        header('Cache-Control: max-age=0');
        header('Content-Transfer-Encoding: binary'); 
        $fp = fopen('php://output', 'a');
        fwrite($fp, chr(0xEF).chr(0xBB).chr(0xBF));
        if (!empty($header)) {
            fputcsv($fp, $header);
        }
        $num = 0;
        //每隔$limit行，刷新一下输出buffer，不要太大，也不要太小
        $limit = 100000;
        //逐行取出数据，不浪费内存
        $count = count($data);
        $data = array_reverse($data);
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
                fputcsv($fp, $row);
            }
        }
        fclose($fp);
    }
}
