<?php
# 动态配置，可以用于seo配置，请在项目下建立该配置文件：使用dever::dyna('home', $data);

$name = 'dever开发框架';
return array
(
	# 首页 可与route相同
	'home' => array
	(
		'title' 		=> 'dever是由rabin独自开发的一款面向服务的php框架_' . $name,
		'keyword' 		=> 'dever,php框架,$data[name]',
		'desc' 			=> 'dever是由rabin独自开发的一款面向服务的php框架',
	)
);