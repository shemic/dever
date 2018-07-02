<?php
# 基础配置，此处的配置所有的环境都一样，自动加入到env中
# 区别是，这里的配置优先级大于env中的，而且也无需根据env的变化而更改
$config['base'] = array
(
	# 名称
	'name' => 'Dever开发框架',
	# 基本描述
	'desc' => '专注编程与开发的架构',
	# copyright
	'copyright' => '© 2015-2020 dever.cc,Inc. Licensed under MIT license.',
	# github
	'github' => 'https://github.com/dever-main',
	# web
	'web' => 'http://www.dever.cc/',
	# 语言包设置
	'lang' => 'zh-cn',
	# api是否开启，默认关闭，如果开启，则需要在项目下建立api目录，手动指定api，类的方法后缀无需加上_api和_secure_api
	'api' => true,
	# 版本配置
	'version' => '1.0.0 Beta',

	# url默认参数，所有Dever::url生成的链接都会加上这个参数
	//'url' => 'loading=content',
	
	# 开启url中某个字段加密
    //'urlEncode' => array('id'),
    # url的原始路径里包含什么字符，则不加密
    'urlEncodeFilter' => array('tag'),
    # 使用加密解密的方法
    'urlEncodeMethod' => array('Dever::idtostr', 'Dever::strtoid'),
	
	# 是否启用自动过滤功能，必须加载manage包，目前可选的值为：manage（自带的过滤功能，非常简单，小型站点可以开启），bao10jie（必须申请账号）
	//'filter' => array('manage' => 1,'bao10jie' => '账号'),
	//'filter' => array('manage' => 1),

	# 基本数据类型
	'state' => array
	(
		1 => '恢复',
		2 => '删除',
	),
);

$config['template'] = array
(
	# 替换设置 一般用于替换资源，将模板中的（html中的）js等相对url换成绝对url，如果不定义，则默认为../js这样的
	'replace' => array
	(
		'css' => '../css/',
		'js' => '../js/',
		'img' => array('../image/', '../img/'),
		'images' => '../images/',
		'lib' => '../lib/',
		'font' => '../fonts/',
		'script' => '../script/',
	),

	# 模板html文件的所在目录，默认为html
	'path' => 'html',
	
	# assets里使用的模板 注意：定义这个之后，将会强制将本项目模板变成这个 定义成数组的话则为pc和手机版 默认为default
	//'assets' => array('pc', 'm'),
	
	# 定义这个之后，将强制将template的目录改为这个，不定义或不填写，则强制使用为assets定义
	//'template' => 'pc',

	# 模板编译时是否过滤\r\n
	'strip' => false,

	# 是否启用layout 如启用，填写要替换的class或者id即可，具体效果可参考youtube，只加载部分内容，前端请加载pjax.js
	//'layout' => '.content',

	# 编译器与模板对应关系目录，可以为空，为空则一一对应，参考manage
	/*
	'relation' => array
	(
		'task/list' => 'tasks',
	),
	*/
);

return $config;