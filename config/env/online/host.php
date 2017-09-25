<?php

# 本配置文件会根据部署位置的不同而修改，此处请自行修改

$local = isset($_SERVER['HTTP_HOST']) ? $_SERVER['HTTP_HOST'] : 'localhost';

# 域名后缀，可以随意修改
$domain = $_SERVER['SERVER_NAME'];

# 当前项目的主域名
$host = 'http://'.$local . '/';

# 定义当前项目assets的域名
$assets = $host . 'assets/';

# 跨域设置
header('Access-Control-Allow-Origin:http://'.$local);

return array
(
	# 项目跟域名
	'base' 	=> DEVER_APP_HOST,
	# 跟域名
	'workspace'	=> $host,

	# cookie 域名
	'cookie' => $domain,
	
	# assets 核心库访问地址 一般用不到，如果想把所有资源都放到这里，就要启用
	'core' 	=> $host . 'assets/lib/',
	
	# assets网络路径，会自动将assets替换为assets/模板
	'assets' => $assets,
	'css' => $assets . 'css/',
	'js' => $assets . 'js/',
	'lib' => $assets . 'lib/',
	'img' => $assets . 'img/',
	'images' => $assets . 'images/',
	'font' => $assets . 'fonts/',
	# 公共模块 不会替换
	'public' => $assets . 'public/',

	# 后台管理系统的assets路径
	'manage' => 'http://manage.'.$domain.'/assets/default/',
	
	# 合并之后的网络路径，填写之后自动合并资源，不填写则不合并，适合把资源放到云端
	'merge' => 'http://assets.'.$domain.'/' . DEVER_PROJECT . '/',
	
	# 上传系统的上传路径的域名(不带action)
	'upload' => 'http://upload.'.$domain.'/save',
	# 上传系统的访问域名
	'image' => 'http://file.'.$domain.'/',
	
	# 是否启用代理功能,代理接口
	'proxy' => $host . 'data.proxy?',
);