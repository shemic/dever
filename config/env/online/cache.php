<?php

return array
(
	# 启用mysql数据库缓存
	'mysql' => 0,
	# 启用页面缓存 会根据当前的url来生成缓存，相当于页面静态化。
	'html' => 0,
	# 启用路由缓存 暂时不支持
	//'route' => 3600,

	# 缓存类型
	'type' => 'memcache',//file、memcache、redis 目前仅有memcache实现，其余后续加上

	# 缓存保存方式，支持多个数据源、多台缓存服务器
	'store' => array
	(
		array
		(
			'host' => '127.0.0.1',
			'port' => '11211',
			'weight' => 100,
		),

		/*
		array
		(
			'host' => '127.0.0.1',
			'port' => '11212',
			'weight' => 100,
		),
		*/
	),
);
