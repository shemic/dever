<?php namespace Dever\Support;

use Dever;
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
    public static function export($data = array(), $header = array(), $fileName = '', $sheet = 0, $sheetName = '', $return = false, $method = false)
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

        return self::getInstance()->$method($data, $header, $fileName, $sheet, $sheetName, $return);
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

    private function excel_export($data = array(), $header = array(), $fileName = '', $sheet = 0, $sheetName = '', $return = false)
    {
        if (!is_object($return)) {
            $xls = new \PHPExcel();
        } else {
            $xls = $return;
        }

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
            foreach($header as $v) {
                $act->setCellValue($this->cell[$i] . $row, $v);
                $act->getColumnDimension($this->cell[$i])->setWidth(30);
                $i++;
            }
            $row++;
        }

        if($data) {
            $i = 0;
            $height = $max = 80;
            foreach($data as $v) {
                $j = 0;
                foreach($v as $cell) {
                    $html = Dever::ishtml($cell);
                    if ($html) {
                        $wizard = new \PHPExcel_Helper_HTML;
                        $cell = $wizard->toRichTextObject($cell);
                    }
                    if (!$html && (strstr($cell, '.jpg') || strstr($cell, '.gif') || strstr($cell, '.png'))) {
                        $key = ($i+$row);
                        $value = false;

                        if (strpos($cell, '||')) {
                            $t = explode('||', $cell);
                            $cell = $t[1];
                            $value = $t[0];
                        }
                        $temp = explode(',', $cell);

                        foreach ($temp as $ck => $cv) {
                            $objDrawing[$ck] = new \PHPExcel_Worksheet_Drawing();
                            if (Dever::project('upload')) {
                                $cv = str_replace('.jpg', '_t1.jpg', $cv);
                                $cv = str_replace('.png', '_t1.png', $cv);
                                $cv = Dever::load('upload/view')->get($cv);
                            }
                            $cv = Dever::local($cv);
                            $objDrawing[$ck]->setPath($cv);
                            $objDrawing[$ck]->setHeight($height);
                            //$objDrawing[$ck]->setWidth(150);
                            $objDrawing[$ck]->setCoordinates($this->cell[$j] . ($i+$row));
                            $objDrawing[$ck]->setOffsetX(12);
                            if ($ck == 0) {
                                $offsetY = 5;
                            } else {
                                $offsetY = $offsetY + $height + 5;
                            }
                            $objDrawing[$ck]->setOffsetY($offsetY);
                            $objDrawing[$ck]->setWorksheet($act);
                        }
                        if ($value) {
                            $act->setCellValue($this->cell[$j] . ($i+$row), $value);
                        }
                        
                        $th = $height * count($temp);
                        if ($th > $max) {
                            $max = $th;
                        }
                        $act->getRowDimension($i+$row)->setRowHeight($max);
                        
                    } else {
                        if (!$cell) {
                            $cell = "";
                        }
                        $act->setCellValue($this->cell[$j] . ($i+$row), $cell);
                        $act->getStyle($this->cell[$j] . ($i+$row))->getAlignment()->setVertical(\PHPExcel_Style_Alignment::HORIZONTAL_CENTER);
                    }
                    
                    $act->getColumnDimension($this->cell[$j])->setAutoSize();
                    $act->getColumnDimension($this->cell[$j])->setWidth(30);
                    $j++;
                }
                $i++;
            }
        }

        if ($header && $return) {
            return $xls;
        }

        if (!$fileName) {
            $fileName = uniqid(time(),true);
        }
        
        $write = \PHPExcel_IOFactory::createWriter($xls, 'Excel2007');
        ob_end_clean();
        header('Content-Type: application/vnd.ms-excel');
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

    private function csv_export($data = array(), $header = array(), $fileName = '', $sheet = 0, $sheetName = '', $return = false)
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
        if ($count > 0) {
            for ($i = 0; $i < $count; $i++) {
                if (isset($data[$i])) {
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
        }
        fclose($fp);
    }
}
