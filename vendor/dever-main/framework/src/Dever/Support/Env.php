<?php namespace Dever\Support;

class Env
{
    /**
     * ua
     *
     * @return string
     */
    public static function ua()
    {
        return $_SERVER['HTTP_USER_AGENT'];
    }

    /**
     * weixin
     *
     * @return string
     */
    public static function weixin()
    {
        if (strpos($_SERVER['HTTP_USER_AGENT'], 'MicroMessenger') !== false) {
            return true;
        }
        return false;
    }

    /**
     * browser
     *
     * @return string
     */
    public static function browser()
    {
        $agent  = $_SERVER['HTTP_USER_AGENT'];
        $browser  = '';
        $version  = '';
  
        if (preg_match('/OmniWeb\/(v*)([^\s|;]+)/i', $agent, $regs)) {
            $browser  = 'OmniWeb';
            $version  = $regs[2];
        } elseif (preg_match('/Netscape([\d]*)\/([^\s]+)/i', $agent, $regs)) {
            $browser  = 'Netscape';
            $version  = $regs[2];
        } elseif (preg_match('/safari\/([^\s]+)/i', $agent, $regs)) {
            $browser  = 'Safari';
            $version  = $regs[1];
        } elseif (preg_match('/MSIE\s([^\s|;]+)/i', $agent, $regs)) {
            $browser  = 'Internet Explorer';
            $version  = $regs[1];
        } elseif (preg_match('/Opera[\s|\/]([^\s]+)/i', $agent, $regs)) {
            $browser  = 'Opera';
            $version  = $regs[1];
        } elseif (preg_match('/NetCaptor\s([^\s|;]+)/i', $agent, $regs)) { 
            $browser  = '(Internet Explorer ' .$version. ') NetCaptor';
            $version  = $regs[1];
        } elseif (preg_match('/Maxthon/i', $agent, $regs)) {
            $browser  = '(Internet Explorer ' .$version. ') Maxthon';
            $version  = '';
        } elseif (preg_match('/360SE/i', $agent, $regs)) {
            $browser   = '(Internet Explorer ' .$version. ') 360SE';
            $version   = '';
        } elseif (preg_match('/SE 2.x/i', $agent, $regs)) {
            $browser   = '(Internet Explorer ' .$version. ') 搜狗';
            $version   = '';
        } elseif (preg_match('/FireFox\/([^\s]+)/i', $agent, $regs)) {
            $browser  = 'FireFox';
            $version  = $regs[1];
        } elseif (preg_match('/Lynx\/([^\s]+)/i', $agent, $regs)) {
            $browser  = 'Lynx';
            $version  = $regs[1];
        } elseif (preg_match('/Chrome\/([^\s]+)/i', $agent, $regs)){
            $browser  = 'Chrome';
            $version  = $regs[1];
        } elseif (preg_match('/MicroMessenger/i', $agent, $regs)){
            $browser  = 'MicroMessenger';
            $version  = '';
        }
  
        if ($browser != '') {
            return $browser . ':' . $version;
        } else {
            return 'unknow browser';
        }
    }

    /**
     * os
     *
     * @return string
     */
    public static function os()
    {
        $agent = $_SERVER['HTTP_USER_AGENT'];
        $os = false;
       
        if (preg_match('/win/i', $agent) && strpos($agent, '95')) {
            $os = 'Windows 95';
        } elseif (preg_match('/win 9x/i', $agent) && strpos($agent, '4.90')) {
            $os = 'Windows ME';
        } elseif (preg_match('/win/i', $agent) && preg_match('/98/i', $agent)) {
            $os = 'Windows 98';
        } elseif (preg_match('/win/i', $agent) && preg_match('/nt 6.0/i', $agent)) {
            $os = 'Windows Vista';
        } elseif (preg_match('/win/i', $agent) && preg_match('/nt 6.1/i', $agent)) {
            $os = 'Windows 7';
        } elseif (preg_match('/win/i', $agent) && preg_match('/nt 6.2/i', $agent)) {
            $os = 'Windows 8';
        } elseif(preg_match('/win/i', $agent) && preg_match('/nt 10.0/i', $agent)) {
            $os = 'Windows 10'; 
        } elseif (preg_match('/win/i', $agent) && preg_match('/nt 5.1/i', $agent)) {
            $os = 'Windows XP';
        } elseif (preg_match('/win/i', $agent) && preg_match('/nt 5/i', $agent)) {
            $os = 'Windows 2000';
        } elseif (preg_match('/win/i', $agent) && preg_match('/nt/i', $agent)) {
            $os = 'Windows NT';
        } elseif (preg_match('/win/i', $agent) && preg_match('/32/i', $agent)) {
            $os = 'Windows 32';
        } elseif (preg_match('/linux/i', $agent)) {
            $os = 'Linux';
        } elseif (preg_match('/unix/i', $agent)) {
            $os = 'Unix';
        } elseif (preg_match('/sun/i', $agent) && preg_match('/os/i', $agent)) {
            $os = 'SunOS';
        } elseif (preg_match('/ibm/i', $agent) && preg_match('/os/i', $agent)) {
            $os = 'IBM OS/2';
        } elseif (preg_match('/Mac/i', $agent) && preg_match('/PC/i', $agent)) {
            $os = 'Macintosh';
        } elseif (preg_match('/PowerPC/i', $agent)) {
            $os = 'PowerPC';
        } elseif (preg_match('/AIX/i', $agent)) {
            $os = 'AIX';
        } elseif (preg_match('/HPUX/i', $agent)) {
            $os = 'HPUX';
        } elseif (preg_match('/NetBSD/i', $agent)) {
            $os = 'NetBSD';
        } elseif (preg_match('/BSD/i', $agent)) {
            $os = 'BSD';
        } elseif (preg_match('/OSF1/i', $agent)) {
            $os = 'OSF1';
        } elseif (preg_match('/IRIX/i', $agent)) {
            $os = 'IRIX';
        } elseif (preg_match('/FreeBSD/i', $agent)) {
            $os = 'FreeBSD';
        } elseif (preg_match('/teleport/i', $agent)) {
            $os = 'teleport';
        } elseif (preg_match('/flashget/i', $agent)) {
            $os = 'flashget';
        } elseif (preg_match('/webzip/i', $agent)) {
            $os = 'webzip';
        } elseif (preg_match('/offline/i', $agent)) {
            $os = 'offline';
        } else {
            $os = '未知操作系统';
        }  
        return $os;
    }

    /**
     * mobile
     *
     * @return string
     */
    public static function mobile()
    {
        $_SERVER['ALL_HTTP'] = isset($_SERVER['ALL_HTTP']) ? $_SERVER['ALL_HTTP'] : '';
        $mobile_browser = '0';
        if (preg_match('/(up.browser|up.link|mmp|symbian|smartphone|midp|wap|phone|iphone|ipad|ipod|android|xoom)/i', strtolower($_SERVER['HTTP_USER_AGENT']))) {
            $mobile_browser++;
        }
        if ((isset($_SERVER['HTTP_ACCEPT'])) && (strpos(strtolower($_SERVER['HTTP_ACCEPT']), 'application/vnd.wap.xhtml+xml') !== false)) {
            $mobile_browser++;
        }
        if (isset($_SERVER['HTTP_X_WAP_PROFILE'])) {
            $mobile_browser++;
        }
        if (isset($_SERVER['HTTP_PROFILE'])) {
            $mobile_browser++;
        }
        $mobile_ua = strtolower(substr($_SERVER['HTTP_USER_AGENT'], 0, 4));
        $mobile_agents = array(
            'w3c ', 'acs-', 'alav', 'alca', 'amoi', 'audi', 'avan', 'benq', 'bird', 'blac',
            'blaz', 'brew', 'cell', 'cldc', 'cmd-', 'dang', 'doco', 'eric', 'hipt', 'inno',
            'ipaq', 'java', 'jigs', 'kddi', 'keji', 'leno', 'lg-c', 'lg-d', 'lg-g', 'lge-',
            'maui', 'maxo', 'midp', 'mits', 'mmef', 'mobi', 'mot-', 'moto', 'mwbp', 'nec-',
            'newt', 'noki', 'oper', 'palm', 'pana', 'pant', 'phil', 'play', 'port', 'prox',
            'qwap', 'sage', 'sams', 'sany', 'sch-', 'sec-', 'send', 'seri', 'sgh-', 'shar',
            'sie-', 'siem', 'smal', 'smar', 'sony', 'sph-', 'symb', 't-mo', 'teli', 'tim-',
            'tosh', 'tsm-', 'upg1', 'upsi', 'vk-v', 'voda', 'wap-', 'wapa', 'wapi', 'wapp',
            'wapr', 'webc', 'winw', 'winw', 'xda', 'xda-',
        );
        if (in_array($mobile_ua, $mobile_agents)) {
            $mobile_browser++;
        }
        if (strpos(strtolower($_SERVER['ALL_HTTP']), 'operamini') !== false) {
            $mobile_browser++;
        }
        // Pre-final check to reset everything if the user is on Windows
        if (strpos(strtolower($_SERVER['HTTP_USER_AGENT']), 'windows') !== false) {
            $mobile_browser = 0;
        }
        // But WP7 is also Windows, with a slightly different characteristic
        if (strpos(strtolower($_SERVER['HTTP_USER_AGENT']), 'windows phone') !== false) {
            $mobile_browser++;
        }
        if ($mobile_browser > 0) {
            return true;
        } else {
            return false;
        }
    }

    /**
     * ip
     *
     * @return mixed
     */
    public static function ip()
    {
        $ip = '';
        if (getenv('HTTP_CLIENT_IP') && strcasecmp(getenv('HTTP_CLIENT_IP'), 'unknown')) {
            $ip = getenv('HTTP_CLIENT_IP');
        } elseif (getenv('HTTP_X_FORWARDED_FOR') && strcasecmp(getenv('HTTP_X_FORWARDED_FOR'), 'unknown')) {
            $ip = getenv('HTTP_X_FORWARDED_FOR');
        } elseif (getenv('REMOTE_ADDR') && strcasecmp(getenv('REMOTE_ADDR'), 'unknown')) {
            $ip = getenv('REMOTE_ADDR');
        } elseif (isset($_SERVER['REMOTE_ADDR']) && $_SERVER['REMOTE_ADDR'] && strcasecmp($_SERVER['REMOTE_ADDR'], 'unknown')) {
            $ip = $_SERVER['REMOTE_ADDR'];
        }
        return preg_match('/[\d\.]{7,15}/', $ip, $matches) ? $matches[0] : '';
    }

    /**
     * zero 检测是否是0
     * @param int $value
     *
     * @return bool
     */
    public static function zero($value)
    {
        return is_numeric($value) === true && $value == 0;
    }
}
