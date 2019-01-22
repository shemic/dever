<?php namespace Dever\Support;

use Dever;

class Img
{
    /**
     * @desc 使用的图片转换类型,默认是gd库，也可以用imagemagick,值为im
     * @var string
     */
    private $_type = 'gd';//另一个值是im

    /**
     * @desc 进行缩略图 值为'600_100,100_200'
     * @var string
     */
    private $_thumb = '';

    /**
     * @desc 进行裁切图 值为'600_100,100_200'
     * @var string
     */
    private $_crop = '';

    /**
     * @desc 如果文件存在，值为true则强制再次操作，默认为false
     * @var string
     */
    private $_setup = false;

    /**
     * @desc 图片水平位置 0=>100 1=>100
     * @var string
     */
    private $_position = array();

    /**
     * @desc 源文件
     * @var string
     */
    private $_source;
    
    /**
     * @desc 裁切图片时候需要抛弃的大小
     * @var string
     */
    private $_dropSize = array();

    /**
     * @desc 目标文件
     * @var string
     */
    private $_dest = array();

    /**
     * @desc 添加水印图片
     * @var array
     */
    private $_mark = array();

    /**
     * @desc 添加文字
     * @var array
     */
    private $_txt = array();

    /**
     * @desc image的源信息
     * @var array
     */
    private $_image = null;

    /**
     * @desc image的类型
     * @var array
     */
    private $_imageType = null;
    
    /**
     * @desc 图片压缩的清晰度
     * @var int
     */
    private $_quality = 20;

    /**
     * @desc 生成的图片名
     * @var int
     */
    private $_name = null;

    /**
     * @desc 设置图片库类型
     * @param type(string) 类型
     */
    public function setType($type)
    {
        $this->_type = $type;
        return $this;
    }
    
    /**
     * @desc 设置清晰度
     * @param quality(int) 清晰度
     */
    public function setQuality($quality)
    {
        $this->_quality = $quality;
        return $this;
    }

    /**
     * @desc 设置强制功能
     * @param setup(bool) 是否强制再次生成重复的文件
     */
    public function setSetup($setup)
    {
        $this->_setup = $setup;
        return $this;
    }

    /**
     * @desc 设置缩略图
     * @param *
     */
    public function setThumb($thumb)
    {
        $this->_thumb = $thumb;
        
        return $this;
    }

    /**
     * @desc 设置裁切图
     * @param *
     */
    public function setCrop($crop)
    {
        $this->_crop = $crop;
        return $this;
    }

    /**
     * @desc 设置生成的文件名
     * @param *
     */
    public function setName($name)
    {
        if (is_string($name)) $name = explode(',', $name);
        $this->_name = $name;
        return $this;
    }

    /**
     * @desc 设置位置
     * @param *
     */
    public function setPosition($position)
    {
        $this->_position = $position;
        return $this;
    }

    /**
     * @desc 设置水印
     * @param *
     */
    public function setMark($mark)
    {
        $this->_mark = $mark;
        $this->_check('mark', 'water');
        $this->_check('mark', 'position');
        return $this;
    }

    /**
     * @desc 设置文字
     * @param *
     */
    public function setTxt($txt)
    {
        $this->_txt = $txt;
        
        return $this;
    }

    /**
     * @desc 设置源文件
     * @param *
     */
    public function setSource($source)
    {
        if (!$source) {
            return $this;
        }
        $this->_source = $source;
        $this->_check('source');
        $this->_image();
        return $this;
    }

    /**
     * @desc 获取生成的图片名字
     * @param *
     */
    public function getName($name = false, $key = false)
    {
        if ($this->_name) {
            if ($key) {
                return $this->_name[$key];
            }
            return $this->_name[0];
        } else {
            return $this->_source . $name;
        }
    }

    /**
     * @desc 获取目标文件
     * @param *
     */
    public function getDest($key = false)
    {
        $this->_destroy();
        return ($key && isset($this->_dest[$key])) ? $this->_dest[$key] : $this->_dest;
    }
    
    /**
     * @desc 获取宽高
     * @param *
     */
    public function getSize($source)
    {
        $this->setSource($source);
        $this->_image();
        return array($this->_image->getImageWidth(), $this->_image->getImageHeight());
    }

    /**
     * @desc 添加水印图
     * @param *
     */
    public function mark($source, $water, $setup = false, $name = false)
    {
        if ($setup == true) {
            $this->setSetup($setup);
        }
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($name) $this->setName($name);
        $this->setMark($water);
        //if($position) $this->setPosition($position);
        $this->loadMethod('mark');
        return $this->getDest('mark');
    }
    
    /**
     * @desc 对图片进行压缩处理
     * @param *
     */
    public function compress($source, $quality = 75, $name = false)
    {
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($name) $this->setName($name);
        $this->setQuality($quality);
        $this->loadMethod('compress');
        return $this->getDest('compress');
    }

    /**
     * @desc 对图片进行webp转换
     * @param *
     */
    public function webp($source, $quality = 75, $name = false)
    {
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($name) $this->setName($name);
        $this->setQuality($quality);
        $this->loadMethod('webp');
        return $this->getDest('webp');
    }

    /**
     * @desc 对图片进行jpg转换
     * @param *
     */
    public function jpg($source, $quality = 75, $name = false)
    {
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($name) $this->setName($name);
        $this->setQuality($quality);
        $this->loadMethod('jpg');
        return $this->getDest('jpg');
    }

    /**
     * @desc 构造函数 可批量建立
     * @param *
     */
    public function __construct($config = array())
    {
        if ($config) {
            $this->init($config);
        }
    }

    /**
     * @desc 批量建立
     * @param *
     */
    public function init($source, $config, $setup = false, $name = false)
    {
        if ($setup == true) {
            $this->setSetup($setup);
        }
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($name) $this->setName($name);
        foreach ($config as $k => $v) {
            $k = $v['method'];
            $m = 'set' . ucfirst($k);
            $this->$m($v);
            $this->loadMethod($k);
        }
        $this->_name = false;
        return $this->getDest();
    }

    /**
     * @desc 建立缩略图（原比例缩略）
     * @param *
     */
    public function thumb($source, $thumb, $setup = false, $name = false)
    {
        if ($setup == true) {
            $this->setSetup($setup);
        }
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($name) $this->setName($name);
        $this->setThumb($thumb);
        $this->loadMethod('thumb');
        $this->_name = false;
        return $this->getDest('thumb');
    }

    /**
     * @desc 建立剪切图（从图中剪切）
     * @param *
     */
    public function crop($source, $crop, $position = false, $setup = false, $name = false)
    {
        if ($setup == true) {
            $this->setSetup($setup);
        }
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($position) $this->setPosition($position);
        if ($name) $this->setName($name);
        $this->setCrop($crop);
        $this->loadMethod('crop');
        $this->_name = false;
        return $this->getDest('crop');
    }
    
    /**
     * @desc 建立剪切图（从图中剪切）
     * @param *
     */
    public function thumbAndCrop($source, $size, $dropSize = array(), $position = false, $setup = true, $name = false)
    {
        if ($setup == true) {
            $this->setSetup($setup);
        }
        
        $this->setSource($source);
        if (!$this->_image) {
            return $source;
        }
        if ($position) $this->setPosition($position);
        if ($name) $this->setName($name);
        $this->setCrop($size);
        
        if ($this->_imageType != 'gif') {
            $this->setThumb($size);
            $this->_next = true;
            $this->loadMethod('thumb');
            $this->_dropSize = $dropSize;
        }
        
        $this->loadMethod('crop');
        return $this->getDest('crop');
    }

    /**
     * @desc 加入文字
     * @param *
     */
    public function txt($source, $txt, $setup = false, $name = false)
    {
        if ($setup == true) {
            $this->setSetup($setup);
        }
        $this->setSource($source);
        if ($name) $this->setName($name);
        $this->setTxt($txt);
        $this->loadMethod('txt');
        return $this->getDest('txt');
    }

    /**
     * @desc 载入方法
     * @param *
     */
    public function loadMethod($method)
    {
        if ($this->_type == 'im' && !class_exists('\Imagick')) $this->_type = 'gd';
        $method = '_' . $this->_type . '_create_' . $method;
        $this->$method();
    }

    /**
     * @desc 对变量进行检测
     * @param name(string) 变量名称
     */
    private function _check($name, $key = false)
    {
        $name = '_' . $name;
        if ($name == '_mark') {
            return $this->_mark[$key];
        }
        if (isset($this->$name) && $this->$name) {
            if ($key == false) {
                return $this->$name;
            } else {
                return $this->{$name}[$key];
            }
        } else {
            $this->_error($name . ' error');
        }
    }

    /**
     * @desc 匹配错误
     * @param *
     */
    private function _error($string, $type = 1)
    {
        $errstr = '' ;
        $errstr .= "Img Tool Error:" . $string . "\n";
        Dever::log($errstr);
        return $errstr;
    }

    /**
     * @desc 获取文件源信息
     * @param *
     */
    private function _image()
    {
        $this->_check('source');
        if (!class_exists('\Imagick')) {
            $this->_type = 'gd';
        }
        if (!$this->_image) {
            switch ($this->_type) {
                case 'gd' :
                    $this->_image = $this->_gd_get($this->_source);
                    break;
                case 'im' :
                    $this->_image = $this->_im_get($this->_source);
                    if ($this->_image) {
                        $this->_imageType = strtolower($this->_image->getImageFormat());
                    }
                    
                    break;
            }
        }
        
        return $this->_image;
    }
    
    /*********************
     * gd库函数
     *********************/

    /**
     * @desc 水印
     * @param *
     */
    private function _gd_create_mark()
    {
        $this->_check('image');
        $this->_check('mark', 'water');
        $this->_check('mark', 'position');

        $this->_dest['mark'] = $this->getName('_mark.jpg');

        if ($this->_setup == true || !file_exists($this->_dest['mark'])) {
            if (isset($this->_mark['radius'])) {
                $water = $this->_gd_radius($this->_mark['water'], $this->_mark['radius']);
            } else {
                $water  = $this->_gd_get($this->_mark['water']);
            }

            $source_x = imagesx($this->_image);
            $source_y = imagesy($this->_image);
            $water_x = imagesx($water);
            $water_y = imagesy($water);
            $width = isset($this->_mark['width']) ? $this->_mark['width'] : $water_x;
            $height = isset($this->_mark['height']) ? $this->_mark['height'] : $water_y;

            if (isset($this->_mark['width']) || isset($this->_mark['height'])) {
                $water_w = $water_x/$water_y;
                $water_h = $water_y/$water_x;

                if ($water_x > $width) {
                    $dest_x = $width;
                    $dest_y = $width*$water_h;
                } elseif ($height > 0 && $water_y > $height) {
                    $dest_x = $height*$water_w;
                    $dest_y = $height;
                } else {
                    $dest_x = $water_x;
                    $dest_y = $water_y;
                }

                $water = $this->_gd_copy($water,$dest_x,$dest_y,$water_x,$water_y,0,0,false,2);

                $xy = $this->_get_mark($source_x, $source_y, $dest_x, $dest_y);
                $water_x = $dest_x;
                $water_y = $dest_y;
            } else {
                $xy = $this->_get_mark($source_x, $source_y, $width, $height);
            }

            if ($xy[2] == false) {
                $this->_gd_destroy($water);
                return;
            }
            $im = $this->_gd_copy($water, $water_x, $water_y, 0, 0, $xy[0], $xy[1], $this->_image);

            imagejpeg($im, $this->_dest['mark']);

            $this->_gd_destroy($water);
        }
    }

    /**
     * @desc 建立缩略图
     * @param *
     */
    private function _gd_create_thumb()
    {
        $this->_check('image');
        $this->_check('thumb');
        if (!is_array($this->_thumb)) {
            $array = explode(',',$this->_thumb);
        } else {
            $array = $this->_thumb;
        }
        $source_x = imagesx($this->_image);
        $source_y = imagesy($this->_image);
        $source_w = $source_x/$source_y;
        $source_h = $source_y/$source_x;
        foreach ($array as $k => $v) {
            $this->_dest['thumb'][$k] = $this->getName('_' . $v . '_thumb.jpg', $k);
            if ($this->_setup == true || !file_exists($this->_dest['thumb'][$k])) {
                $offset = explode('_',$v);
                if (isset($offset[2]) && $offset[2] == 1) {
                    //完全等比例
                    if ($source_x > $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($offset[1] > 0 && $source_y > $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } else {
                        $dest_x = $source_x;
                        $dest_y = $source_y;
                    }
                } elseif (isset($offset[2]) && $offset[2] == 2) {
                    //按照一定比例
                    if ($offset[0] == 0 && $offset[1] > 0) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($offset[1] > 0 && $source_x > $source_y && $source_y > $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y > $source_x && $source_x > $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($source_y == $source_x && $offset[0] == $offset[1]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[1];
                    } elseif ($source_x > $source_y && $source_y < $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif($source_y > $source_x && $source_x < $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif($source_x > $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } else {
                        $dest_x = $source_x;
                        $dest_y = $source_y;
                    }
                } elseif (isset($offset[2]) && $offset[2] == 3) {
                    //按照比例缩放，如有多余则留白（或黑...如果实在留不了白的话）
                    $b = $offset[0]/$offset[1];
                    $l = $source_x/$source_y;
                    
                    if ($b > $l) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } else {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    }
                } elseif (isset($offset[2]) && $offset[2] == 4) {
                    //按照一定比例
                    if ($offset[0] == 0 && $offset[1] > 0) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif($offset[1] > 0 && $source_x > $source_y && $source_y >= $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y > $source_x && $source_x >= $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($source_y == $source_x && $offset[0] < $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y == $source_x && $offset[0] > $offset[1]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($source_y == $source_x && $offset[0] == $offset[1]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[1];
                    } elseif ($source_x > $source_y && $source_y < $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y > $source_x && $source_x < $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } else {
                        $dest_x = $source_x;
                        $dest_y = $source_y;
                    }
                } else {
                    //直接放大和缩小
                    $dest_x = $offset[0];
                    $dest_y = $offset[1];
                }

                $im = $this->_gd_copy($this->_image,$dest_x,$dest_y,$source_x,$source_y,0,0,false,1);

                imagejpeg($im, $this->_dest['thumb'][$k]);
                $this->_gd_destroy($im);
            }
        }
    }

    /**
     * @desc 建立原图压缩图
     * @param *
     */
    private function _gd_create_compress()
    {
        $this->_dest['compress'] = $this->_source;
    }

    /**
     * @desc 建立剪切图
     * @param *
     */
    private function _gd_create_crop()
    {
        $this->_check('image');
        $this->_check('crop');
        //$this->_check('position');
        if (!is_array($this->_crop)) {
            $array = explode(',',$this->_crop);
        } else {
            $array = $this->_crop;
        }
        $source_x = imagesx($this->_image);
        $source_y = imagesy($this->_image);
        foreach ($array as $k => $v) {
            $this->_dest['crop'][$k] = $this->getName('_' . $v . '_crop.jpg', $k);
            if ($this->_setup == true || !file_exists($this->_dest['crop'][$k])) {
                $x = 0;
                $y = 0;

                $offset = explode('_',$v);
                if (isset($this->_dropSize[$k]) && $this->_dropSize[$k]) {
                    $offset[0] += $this->_dropSize[$k];
                    $offset[1] += $this->_dropSize[$k];
                }
                
                if ($this->_position) {
                    # 加入根据百分比计算裁图
                    if ($this->_position[0] <= 0) {
                        $this->_position[0] = $source_x/2 - $offset[0]/2;
                    } elseif (strstr($this->_position[0], '%')) {
                        $this->_position[0] = $source_x * intval(str_replace('%', '', $this->_position[0]))/100;
                    }
                    if ($this->_position[1] <= 0) {
                        $this->_position[1] = $source_y/2 - $offset[1]/2;
                    } elseif (strstr($this->_position[1], '%')) {
                        $this->_position[1] = $source_y * intval(str_replace('%', '', $this->_position[1]))/100;
                    }
                    $x = $this->_position[0];
                    $y = $this->_position[1];
                } else {
                    $x = $source_x/2 - $offset[0]/2;
                    $y = $source_y/2 - $offset[1]/2;
                }
                if ($x < 0) {
                    $x = 0;
                }
                if ($y < 0) {
                    $y = 0;
                }

                $im = $this->_gd_copy($this->_image,$offset[0],$offset[1],$offset[0],$offset[1],$x,$y);

                imagejpeg($im, $this->_dest['crop'][$k]);
                $this->_gd_destroy($im);
            }
        }
    }
    
    /**
     * @desc 添加水印文字
     * @param *
     */
    private function _gd_create_txt()
    {
        $this->_check('source');
        $this->_check('image');
        //$this->_check('txt','file');
        $this->_check('txt','color');
        $this->_check('txt','size');
        $this->_check('txt','angle');
        $this->_check('txt','name');
        $this->_check('txt', 'position');
        //$this->_check('txt','left');
        //$this->_check('txt','top');
        //$this->_check('txt','bgcolor');
        //$this->_check('txt','font');

        $this->_dest['txt'] = isset($this->_txt['file']) ? $this->_txt['file'] : $this->getName('_txt.jpg');

        if ($this->_setup == true || !file_exists($this->_dest['txt'])) {

            $color = $this->_txt['color'];

            $fontFile = isset($this->_txt['font']) ? $this->_txt['font'] : "SIMSUN.TTC";

            $this->_txt['autowrap'] = 0;
            if (isset($this->_txt['width']) && $this->_txt['width'] > 0) {
                $this->_txt['name'] = $this->_gd_autowrap($this->_txt['size'], $this->_txt['angle'], $fontFile, $this->_txt['name'], $this->_txt['width']);
            }
            
            $position = imagettfbbox($this->_txt['size'], $this->_txt['angle'], $fontFile, $this->_txt['name']);
            if ($position) {
                $source_x = imagesx($this->_image);
                $source_y = imagesy($this->_image);
                $water_x = $position[2] - $position[0];
                $water_y = $position[1] - $position[7];

                $xy = $this->_get_mark($source_x, $source_y, $water_x, $water_y, 'txt');
            }

            $this->_txt['left'] = isset($xy[0]) ? $xy[0] : 0;
            $this->_txt['top'] = isset($xy[1]) ? $xy[1] : 0;

            if (!empty($color) && (strlen($color)==7)) { 
                $R = hexdec(substr($color,1,2)); 
                $G = hexdec(substr($color,3,2)); 
                $B = hexdec(substr($color,5)); 
                putenv('GDFONTPATH=' . realpath('.'));
                
                imagettftext($this->_image, $this->_txt['size'],$this->_txt['angle'], $this->_txt['left'], $this->_txt['top'] + $this->_txt['autowrap'], imagecolorallocate($this->_image, $R, $G, $B),$fontFile,$this->_txt['name']);
            }

            imagejpeg($this->_image, $this->_dest['txt']);
        }
    }

    /**
     * @desc 文字自动换行
     * @param *
     */
    private function _gd_autowrap($fontsize, $angle, $fontface, $string, $width) {
        // 这几个变量分别是 字体大小, 角度, 字体名称, 字符串, 预设宽度
        $content = "";

        // 将字符串拆分成一个个单字 保存到数组 letter 中
        for ($i=0;$i<mb_strlen($string);$i++) {
            $letter[] = mb_substr($string, $i, 1);
        }

        foreach ($letter as $l) {
            $teststr = $content." ".$l;
            $testbox = imagettfbbox($fontsize, $angle, $fontface, $teststr);
            // 判断拼接后的字符串是否超过预设的宽度
            if (($testbox[2] > $width) && ($content !== "")) {
                $content .= "\n";
                $this->_txt['autowrap'] += $this->_txt['size'];
            }
            $content .= $l;
        }
        return $content;
    }

    /**
     * @desc 销毁资源
     * @param *
     */
    private function _gd_destroy($im)
    {
        imagedestroy($im);
        return;
    }

    /**
     * @desc copy
     * @param *
     */
    private function _gd_copy($im,$w,$h,$x,$y,$l,$t,$dim = false,$ti = 1)
    {
        if ($dim == false) {
            $dim = $this->_gd_create($w, $h,$ti); // 创建目标图gd2
            imagecopyresized($dim,$im,0,0,$l,$t,$w,$h,$x,$y);
        } else {

            imagecopy($dim, $im, $l,$t, 0, 0, $w,$h);
            //imagecopyresampled($dim, $im, $l,$t, 0, 0, $w,$h,$x,$y);
        }
        return $dim;
    }

    /**
     * @desc 获取数据源
     * @param *
     */
    private function _gd_get($image)
    {
        ini_set("memory_limit", "2048M");
        $imgstream = file_get_contents($image);
        $im = imagecreatefromstring($imgstream);
        return $im;
    }

    /**
     * @desc 创建背景图
     * @param *
     */
    private function _gd_create($w,$h,$t = 1)
    {
        $dim = imagecreatetruecolor($w,$h); // 创建目标图gd2

        //透明背景
        if ($t == 2) {
            
            imagealphablending($dim, false);
            imagesavealpha($dim,true);
            $transparent = imagecolorallocatealpha($dim, 255, 255, 255, 127);
            imagefilledrectangle($dim, 0, 0, $w, $h, $transparent);
            
        }
        
        
        //空白背景
        if ($t == 1) {
            $wite = ImageColorAllocate($dim,255,255,255);//白色
            imagefilledrectangle($dim, 0, 0, $w,$h, $wite);
            imagefilledrectangle($dim, $w, $h, 0,0, $wite);
            ImageColorTransparent($dim, $wite);
        }
        
        return $dim;
    }

    /*********************
     * im库函数
     *********************/

    /**
     * @desc 建立缩略图
     * @param *
     */
    private function _im_create_thumb()
    {
        $this->_check('source');
        $this->_check('image');
        $this->_check('thumb');
        $source_x   = $this->_image->getImageWidth();
        $source_y   = $this->_image->getImageHeight();
        $source_w = $source_x/$source_y;
        $source_h = $source_y/$source_x;

        if (!is_array($this->_thumb)) {
            $array = explode(',',$this->_thumb);
        } else {
            $array = $this->_thumb;
        }
        foreach ($array as $k => $v) {
            $this->_dest['thumb'][$k] = $this->getName('_' . $v . '_thumb.jpg', $k);

            if ($this->_setup == true || !file_exists($this->_dest['thumb'][$k])) {
                $offset = explode('_',$v);

                if (isset($offset[2]) && $offset[2] == 1) {
                    //完全等比例
                    if ($source_x > $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($offset[1] > 0 && $source_y > $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } else {
                        $dest_x = $source_x;
                        $dest_y = $source_y;
                    }
                } elseif (isset($offset[2]) && $offset[2] == 2) {
                    //按照一定比例
                    if ($offset[0] == 0 && $offset[1] > 0) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($offset[1] > 0 && $source_x > $source_y && $source_y > $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y > $source_x && $source_x > $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($source_y == $source_x && $offset[0] == $offset[1]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[1];
                    } elseif ($source_x > $source_y && $source_y < $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif($source_y > $source_x && $source_x < $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif($source_x > $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } else {
                        $dest_x = $source_x;
                        $dest_y = $source_y;
                    }
                } elseif (isset($offset[2]) && $offset[2] == 3) {
                    //按照比例缩放，如有多余则留白（或黑...如果实在留不了白的话）
                    $b = $offset[0]/$offset[1];
                    $l = $source_x/$source_y;
                    
                    if ($b > $l) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } else {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    }
                } elseif (isset($offset[2]) && $offset[2] == 4) {
                    //按照一定比例
                    if ($offset[0] == 0 && $offset[1] > 0) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif($offset[1] > 0 && $source_x > $source_y && $source_y >= $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y > $source_x && $source_x >= $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($source_y == $source_x && $offset[0] < $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y == $source_x && $offset[0] > $offset[1]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } elseif ($source_y == $source_x && $offset[0] == $offset[1]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[1];
                    } elseif ($source_x > $source_y && $source_y < $offset[1]) {
                        $dest_x = $offset[1]*$source_w;
                        $dest_y = $offset[1];
                    } elseif ($source_y > $source_x && $source_x < $offset[0]) {
                        $dest_x = $offset[0];
                        $dest_y = $offset[0]*$source_h;
                    } else {
                        $dest_x = $source_x;
                        $dest_y = $source_y;
                    }
                } else {
                    //直接放大和缩小
                    $dest_x = $offset[0];
                    $dest_y = $offset[1];
                }
                //echo $dest_y;die;
                $this->_image = $this->_im_get($this->_source);  
                $this->_image->thumbnailImage($dest_x, $dest_y);
                
                if (isset($offset[2]) && $offset[2] == 3) {
                    /* 按照缩略图大小创建一个有颜色的图片 */  
                    $canvas = $this->_im_get();  
                    $color = new \ImagickPixel("white");
                    $canvas->newImage($offset[0], $offset[1], $color, 'png');
                    //$canvas->paintfloodfillimage('transparent',2000,NULL,0,0);
                    /* 计算高度 */  
                    $x = ($offset[0] - $dest_x)/2;  
                    $y = ($offset[1] - $dest_y)/2;  
                    /* 合并图片  */  
                    $canvas->compositeImage($this->_image, \Imagick::COMPOSITE_OVER, $x, $y);
                    
                    $canvas->setCompression(\Imagick::COMPRESSION_JPEG); 
                    $canvas->setCompressionQuality(100);

                    $canvas->writeImage($this->_dest['thumb'][$k]);

                    
                    if (isset($offset[3]) && $offset[3] && $this->_dest['thumb'][$k]) {
                        $offset[3] = $offset[3] * 1024;
                        $size = abs(filesize($this->_dest['thumb'][$k]));
                        if ($size > $offset[3]) {
                            $this->_compress($canvas, $offset[3], 80, $this->_dest['thumb'][$k]);
                        }
                    }
                    
                    $canvas = false;
                } else {
                    //$this->_image->setCompression(\Imagick::COMPRESSION_JPEG); 
                    $this->_image->setCompressionQuality(90);
                    if ($this->_imageType == 'gif') {
                        $this->_image->writeImages($this->_dest['thumb'][$k], true);
                    } else {
                        $this->_image->writeImage($this->_dest['thumb'][$k]);
                    }
                }
            }
        }
        
        $this->_name = false;
    }
    /**
     * @desc 建立原图压缩图
     * @param *
     */
    private function _im_create_compress()
    {
        $this->_check('source');
        $this->_check('quality');
        $this->_dest['compress'] = $this->getName('_compress.jpg');
        
        $this->_image->stripImage();
        if (strstr($this->_dest['compress'], '.png')) {
            $this->_image->setImageType(\Imagick::IMGTYPE_PALETTE);
            $this->_image->writeImage($this->_dest['compress']);      
        } else {
            $this->_image->setCompression(\Imagick::COMPRESSION_JPEG); 
            $this->_image->setImageCompressionQuality($this->_quality);
            $this->_image->writeImage($this->_dest['compress']);
        }
    }
    
    /**
     * @desc 对图片进行压缩
     * @param *
     */
    private function _compress($canvas, $max, $num, $file)
    {
        $num = $num - 10;
        $temp = $file . '_temp_'.$num.'.jpg';
        $canvas->setCompressionQuality($num);
        $canvas->stripImage();
        
        $canvas->writeImage($temp);
            
        $size = abs(filesize($temp));
        if ($size > $max && $num > 0) {
            @unlink($temp);
            return $this->_compress($canvas, $max, $num, $file);
        } else {
            $canvas->destroy();
            @copy($temp, $file);
            @unlink($temp);
        }
    }

    /**
     * @desc 设置webp格式
     * @param *
     */
    private function _im_create_webp()
    {
        $this->_check('source');
        $this->_dest['webp'] = $this->getName('.webp');

        //apk add libwebp-tools
        //Dever::run('cwebp -q 75 '.$this->_source.' -o ' . $this->_dest['webp']);

        $this->_image->setImageFormat('webp');
        $this->_image->writeImage($this->_dest['webp']);
    }

    /**
     * @desc 设置jpg格式
     * @param *
     */
    private function _im_create_jpg()
    {
        $this->_check('source');
        $this->_dest['jpg'] = $this->getName('.jpg');

        //apk add libwebp-tools
        //Dever::run('cwebp -q 75 '.$this->_source.' -o ' . $this->_dest['webp']);

        $this->_image->setImageFormat('jpg');
        $this->_image->writeImage($this->_dest['jpg']);
    }

    /**
     * @desc 建立裁切图
     * @param *
     */
    private function _im_create_crop()
    {
        $this->_check('source');
        $this->_check('image');
        $this->_check('crop');
        
        if (!is_array($this->_crop)) {
            $array = explode(',',$this->_crop);
        } else {
            $array = $this->_crop;
        }
        foreach ($array as $k => $v) {
            if ($this->_name) {
                $this->_dest['crop'][$k] = $this->getName('_' . $v . '_crop.jpg', $k);
            } else {
                $this->_dest['crop'][$k] = (isset($this->_dest['thumb'][$k]) && $this->_dest['thumb'][$k]) ? $this->_dest['thumb'][$k] : $this->getName('_' . $v . '_crop.jpg', $k);
            }
            
            if ($this->_dropSize) {
                $this->_image = $this->_im_get($this->_dest['crop'][$k]);
                $source_x   = $this->_image->getImageWidth();
                $source_y   = $this->_image->getImageHeight();
            } else {
                $imageInfo = getimagesize($this->_source);
                $source_x   = $imageInfo[0];
                $source_y   = $imageInfo[1];
            }
            
            if ($this->_setup == true || !file_exists($this->_dest['crop'][$k])) {
                $offset = explode('_',$v);
                if (isset($this->_dropSize[$k]) && $this->_dropSize[$k]) {
                    $offset[0] += $this->_dropSize[$k];
                    $offset[1] += $this->_dropSize[$k];
                }
                
                if ($this->_position) {
                    # 加入根据百分比计算裁图
                    if ($this->_position[0] <= 0) {
                        $this->_position[0] = $source_x/2 - $offset[0]/2;
                    } elseif (strstr($this->_position[0], '%')) {
                        $this->_position[0] = $source_x * intval(str_replace('%', '', $this->_position[0]))/100;
                    }
                    if ($this->_position[1] <= 0) {
                        $this->_position[1] = $source_y/2 - $offset[1]/2;
                    } elseif (strstr($this->_position[1], '%')) {
                        $this->_position[1] = $source_y * intval(str_replace('%', '', $this->_position[1]))/100;
                    }
                    $x = $this->_position[0];
                    $y = $this->_position[1];
                } else {
                    $x = $source_x/2 - $offset[0]/2;
                    $y = $source_y/2 - $offset[1]/2;
                }
                if ($x < 0) {
                    $x = 0;
                }
                if ($y < 0) {
                    $y = 0;
                }

                if ($this->_imageType == 'gif') {
                    $this->_im_gif($offset[0], $offset[1], $x, $y);
                } else {  
                    $this->_image->cropImage($offset[0], $offset[1], $x, $y);
                }
                
                if (isset($offset[2]) && $offset[2] == 3 && isset($offset[3]) && $offset[3] > 0) {
                    $this->_image->writeImage($this->_dest['crop'][$k]);
                    $offset[3] = $offset[3] * 1024;
                    $size = abs(filesize($this->_dest['crop'][$k]));
                    if ($size > $offset[3]) {
                        $this->_compress($this->_image, $offset[3], 80, $this->_dest['crop'][$k]);
                    }
                } else {
                    //$this->_image->setCompression(\Imagick::COMPRESSION_JPEG); 
                    $this->_image->setCompressionQuality(90);
                    if ($this->_imageType == 'gif') {
                        $this->_image->writeImages($this->_dest['crop'][$k], true);
                    } else {
                        $this->_image->writeImage($this->_dest['crop'][$k]);
                    }
                }
            }
        }
    }

    private function _im_gif($w, $h, $x, $y, $d = false, $num = false)
    {
        $canvas = $this->_im_get();

        $canvas->setFormat("gif");

        $this->_image->coalesceImages();
        
        $num = $num ? $num : $this->_image->getNumberImages();

        for ($i = 0; $i < $num; $i++) {
            $this->_image->setImageIndex($i);
                    
            $img = $this->_im_get();
            $img->readImageBlob($this->_image);
            
            if ($d != false) {
                $img->drawImage($d);
            } else {
                $img->cropImage($w, $h, $x, $y);
            }

            $canvas->addImage($img);
            $canvas->setImageDelay($img->getImageDelay());
            if($d == false) $canvas->setImagePage($w, $h, 0, 0);
            
            $img->destroy();
            
            unset($img);
        }
        
        $this->_image->destroy();
        $this->_image = $canvas;
    }

    /**
     * @desc 建立水印
     * @param *
     */
    private function _im_create_mark()
    {
        $this->_check('source');
        $this->_check('image');
        $this->_check('mark', 'water');
        $this->_check('mark', 'position');

        $this->_dest['mark'] = $this->getName('_mark.jpg');

        if ($this->_setup == true || !file_exists($this->_dest['mark'])) {
            if (isset($this->_mark['radius'])) {
                $water = $this->_im_radius($this->_mark['water'], $this->_mark['radius']);
            } else {
                $water = $this->_im_get($this->_mark['water']);
            }
            $draw = new \ImagickDraw();

            $source_x   = $this->_image->getImageWidth();
            $source_y   = $this->_image->getImageHeight();
            $water_x = $water->getImageWidth();
            $water_y = $water->getImageHeight();

            $width = isset($this->_mark['width']) ? $this->_mark['width'] : $water_x;
            $height = isset($this->_mark['height']) ? $this->_mark['height'] : $water_y;

            if (isset($this->_mark['width']) || isset($this->_mark['height'])) {
                $water_w = $water_x/$water_y;
                $water_h = $water_y/$water_x;

                if ($water_x > $width) {
                    $dest_x = $width;
                    $dest_y = $width*$water_h;
                } elseif ($height > 0 && $water_y > $height) {
                    $dest_x = $height*$water_w;
                    $dest_y = $height;
                } else {
                    $dest_x = $water_x;
                    $dest_y = $water_y;
                }

                $water->thumbnailImage($dest_x, $dest_y);

                $xy = $this->_get_mark($source_x, $source_y, $dest_x, $dest_y);
                $water_x = $dest_x;
                $water_y = $dest_y;
            } else {
                $xy = $this->_get_mark($source_x, $source_y, $width, $height);
            }

            $draw->composite($water->getImageCompose(), $xy[0], $xy[1], $water_x, $water_y, $water);
      
            if ($this->_imageType == 'gif') {
                $this->_im_gif(0, 0, 0, 0, $draw);
            } else {
                $this->_image->drawImage($draw);
            }
    
            $this->_image->writeImage($this->_dest['mark']);
        }
    }

    /**
     * @desc 建立文字
     * @param *
     */
    private function _im_create_txt()
    {
        $this->_check('source');
        $this->_check('image');
        //$this->_check('txt','file');
        $this->_check('txt','color');
        $this->_check('txt','size');
        $this->_check('txt','angle');
        $this->_check('txt','name');
        $this->_check('txt', 'position');
        //$this->_check('txt','left');
        //$this->_check('txt','top');
        //$this->_check('txt','bgcolor');
        //$this->_check('txt','font');

        $this->_dest['txt'] = isset($this->_txt['file']) ? $this->_txt['file'] : $this->getName('_txt.jpg');

        if ($this->_setup == true || !file_exists($this->_dest['txt'])) {

            $fontFile = isset($this->_txt['font']) ? $this->_txt['font'] : "SIMSUN.TTC";
            
            $this->_txt['autowrap'] = 0;
            if (isset($this->_txt['width']) && $this->_txt['width'] > 0) {
                $this->_txt['name'] = $this->_gd_autowrap($this->_txt['size'], $this->_txt['angle'], $fontFile, $this->_txt['name'], $this->_txt['width']);
            }

            $draw = new \ImagickDraw();
            if ($fontFile) {
                $draw->setFont($fontFile);

                $position = imagettfbbox($this->_txt['size'], $this->_txt['angle'], $fontFile, $this->_txt['name']);
                if ($position) {
                    $source_x   = $this->_image->getImageWidth();
                    $source_y   = $this->_image->getImageHeight();
                    $water_x = $position[2] - $position[0];
                    $water_y = $position[1] - $position[7];

                    $xy = $this->_get_mark($source_x, $source_y, $water_x, $water_y, 'txt');
                }
            }
            if (isset($this->_txt['size'])) {
                $draw->setFontSize($this->_txt['size']);
            }
            if (isset($this->_txt['color'])) {
                $draw->setFillColor($this->_txt['color']);
            }
            if (isset($this->_txt['bgcolor'])) {
                $draw->setTextUnderColor($this->_txt['bgcolor']);
            }

            $this->_txt['left'] = isset($xy[0]) ? $xy[0] : 0;
            $this->_txt['top'] = isset($xy[1]) ? $xy[1] : 0;
              
            if ($this->_imageType == 'gif') {  
                foreach ($this->_image as $frame) {
                    $frame->annotateImage($draw, $this->_txt['left'], $this->_txt['top'], $this->_txt['angle'], $this->_txt['name']);
                }
            } else {
                $this->_image->annotateImage($draw, $this->_txt['left'], $this->_txt['top'] + $this->_txt['autowrap'], $this->_txt['angle'], $this->_txt['name']);
            }

            $this->_image->writeImage($this->_dest['txt']);
        }
    }

    /**
     * @desc 获取数据源
     * @param *
     */
    private function _im_get($image = false)
    {
        /*
        if (!Dever::is_file($image)) {
            return false;
        }
        */
        if ($image && strstr($image, 'http')) {
            $content = file_get_contents($image);
            $im = new \Imagick();
            $im->readImageBlob($content);
        } else {
            $im = new \Imagick($image);
        }
        
        return $im;
    }

    /**
     * @desc 圆角图片
     * @param *
     */
    private function _im_radius($img = '', $radius = -1)
    {
        $image = $this->_im_get($img);
        $image->setImageFormat('png');
        if ($radius == -1) {
            $x = $image->getImageWidth() / 2;
            $y = $image->getImageHeight() / 2;
        } else {
            $x = $image->getImageWidth() - $radius;
            $y = $image->getImageHeight() - $radius;
        }
        $image->roundCorners($x, $y);
        return $image;
    }

    /**
     * @desc 圆角图片
     * @param *
     */
    private function _gd_radius($imgpath = '', $radius = 0)
    {
        $ext     = pathinfo($imgpath);
        $src_img = null;
        switch ($ext['extension']) {
        case 'jpg':
            $src_img = imagecreatefromjpeg($imgpath);
            break;
        case 'png':
            $src_img = imagecreatefrompng($imgpath);
            break;
        }
        $wh = getimagesize($imgpath);
        $w  = $wh[0];
        $h  = $wh[1];
        $radius = $radius <= 0 ? (min($w, $h) / 2) : $radius;
        $img = imagecreatetruecolor($w, $h);
        //这一句一定要有
        imagesavealpha($img, true);
        //拾取一个完全透明的颜色,最后一个参数127为全透明
        $bg = imagecolorallocatealpha($img, 255, 255, 255, 127);
        imagefill($img, 0, 0, $bg);
        $r = $radius; //圆 角半径
        for ($x = 0; $x < $w; $x++) {
            for ($y = 0; $y < $h; $y++) {
                $rgbColor = imagecolorat($src_img, $x, $y);
                if (($x >= $radius && $x <= ($w - $radius)) || ($y >= $radius && $y <= ($h - $radius))) {
                    //不在四角的范围内,直接画
                    imagesetpixel($img, $x, $y, $rgbColor);
                } else {
                    //在四角的范围内选择画
                    //上左
                    $y_x = $r; //圆心X坐标
                    $y_y = $r; //圆心Y坐标
                    if (((($x - $y_x) * ($x - $y_x) + ($y - $y_y) * ($y - $y_y)) <= ($r * $r))) {
                        imagesetpixel($img, $x, $y, $rgbColor);
                    }
                    //上右
                    $y_x = $w - $r; //圆心X坐标
                    $y_y = $r; //圆心Y坐标
                    if (((($x - $y_x) * ($x - $y_x) + ($y - $y_y) * ($y - $y_y)) <= ($r * $r))) {
                        imagesetpixel($img, $x, $y, $rgbColor);
                    }
                    //下左
                    $y_x = $r; //圆心X坐标
                    $y_y = $h - $r; //圆心Y坐标
                    if (((($x - $y_x) * ($x - $y_x) + ($y - $y_y) * ($y - $y_y)) <= ($r * $r))) {
                        imagesetpixel($img, $x, $y, $rgbColor);
                    }
                    //下右
                    $y_x = $w - $r; //圆心X坐标
                    $y_y = $h - $r; //圆心Y坐标
                    if (((($x - $y_x) * ($x - $y_x) + ($y - $y_y) * ($y - $y_y)) <= ($r * $r))) {
                        imagesetpixel($img, $x, $y, $rgbColor);
                    }
                }
            }
        }
        return $img;
    }

    private function _get_mark($source_x, $source_y, $water_x, $water_y, $type = 'mark')
    {
        $this->_check($type, 'position');
        $this->_check($type, 'offset');
        $l = 0;
        $t = 0;
        $state = true;

        $method = '_' . $type;
        $position = $this->{$method}['position'];
        $offset = $this->{$method}['offset'];
        if ($position && is_array($position)) {
            $l = $position[0];
            $t = $position[1];
        } elseif ($position) {
            switch ($position) {
                case 1:
                    //左上
                    break;
                case 2:
                    //左下
                    $t = $source_y - $water_y;
                    break;
                case 3:
                    //右上
                    $l = $source_x - $water_x;
                    break;
                case 4:
                    //右下
                    $l = $source_x - $water_x;
                    $t = $source_y - $water_y;
                    break;
                case 5:
                    //中间
                    $l = $source_x/2 - $water_x/2;
                    $t = $source_y/2 - $water_y/2;
                    break;
                case 6:
                    //上中
                    $l = $source_x/2 - $water_x/2;
                    break;
                case 7:
                    //下中
                    $l = $source_x/2 - $water_x/2;
                    $t = $source_y - $water_y;
                    break;
                default :
                    $state = false;
                    break;
            }
        }

        if ($offset && is_array($offset)) {
            $l = $l + $offset[0];
            $t = $t + $offset[1];
        } else {
            $l = $l + $offset;
            $t = $t + $offset;
        }
        return array($l, $t, $state);
    }
    
    /**
     * @desc 判断是否网络文件，如果是，则生成一个本地路径
     * @param $file 网络文件地址
     * @param $path 生成的路径，默认为项目根目录下的'data/upload'
     */
    private function _setFileName($file, $path = '/upload/')
    {
        if (strstr($file, 'http://')) {
            $array = explode('/', $file);
            $filename = $array[count($array)-1];
            $file = Dever::path(Dever::path(Dever::path(Dever::path(Dever::data() . $path) . date("Y") . '/') . date("m") . '/') . date("d") . '/') . $filename;
        }
        return $file;
    }
    
    /**
     * @desc 对网络文件进行拷贝复制到本地
     * @param $file 网络文件地址
     * @param $path 生成的路径，默认为项目根目录下的'data/upload'
     */
    public function copyFile($file, $path = '/upload/')
    {
        if (strstr($file, 'http://')) {
            $new = $this->_setFileName($file, $path);
            if (!is_file($new)) {
                $content = file_get_contents($file);
                file_put_contents($new, $content);
            }
            $file = $new;
        }
        return $file;
    }

    /**
     * @desc 销毁资源
     * @param *
     */
    private function _destroy()
    {
        if ($this->_type == 'gd') {
            imagedestroy($this->_image);
        }
        
        $this->_image = false;
    }
}
