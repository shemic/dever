<?php
# 集成项目的大部分配置，此为默认设置。环境不同，以下的配置也有可能不同，可以根据项目名建立配置文件

# 基本配置
$config['base'] = array
(
	#　项目部署的相对路径（部署在服务器的根目录，如果不定义DEVER_PROJECT_NAME，则本项必须启用并有效）
	'path' => DIRECTORY_SEPARATOR . 'workspace' . DIRECTORY_SEPARATOR,

	# 访问assets目录的物理路径
	'assets' => DEVER_APP_PATH . 'assets' . DIRECTORY_SEPARATOR,

	# 访问data目录的物理路径
	'data' 	=> DEVER_APP_PATH . 'data' . DIRECTORY_SEPARATOR,

	# 访问当前项目目录的物理路径，如果项目和dever类库在一个目录中，则为DEVER_PATH，如果不在，则为DEVER_APP_PATH，当然也可随意更改，这里目前只影响合并操作
	'workspace' => DEVER_APP_PATH,
	
	# 定义api的token明文，如果和其他业务有合作，建议使用系统自带的接口api，自带加密解密程序。
	'token' => 'dever_api_2016',
	
	# 是否启用nocache，如果是互动类的项目且主域增加了cdn，建议开启
	'clearHeaderCache' => false,

	# api文档生成是否开启，开启后，将会根据访问来生成文档。生产环境建议禁止
	'apiDoc' => true,
	# api日志是否开启，开启后，将会记录所有带有_api后缀方法的请求参数和响应参数
	'apiLog' => false,

	# 定义自动转为api的目录，可以将该目录下的所有类的公共方法，都转为可以访问的api，开启该功能可能有安全性问题。
	'apiOpenPath' => 'src',

	# 启用后，将会根据api目录下的配置文件自动定位api
	//'apiConfig' => true,
	
	# 开启用户触发cron，主要用于无法加到系统计划任务的虚拟主机，必须安装manage组件，谨慎开启，会稍微影响程序执行效率
	//'cron' => true,
);

# 模板配置
$config['template'] = array
(
	# 是否启用静态资源域名动态化，启用之后，静态资源的域名将动态加载，适合使用多个域名或publish启用
	'domain' => true,

	# 是否开启强制刷新页面缓存
	'shell' => 'temp',
	
	# 是否开启手动更改模板名称，允许通过$_GET的方式来更改当前模板，值为$_GET的key值，默认关闭
	//'name' => 'template',

	# publish 是否发布，此项开启后，系统不会检测service（意味着不用将service打包上线），适合生产环境，并能对代码起到一定的加密保护。
	//'publish' => true,
);

# 数据库配置
$config['database'] = array
(
	# database 中的reuqest的兼容定义，如果启用了该选项，需要自行开发database/compatible目录下相对应的数据表文件中的request方法。
	//'compatible' => 'model',

	# 是否开启sql自动优化，将sql中的select * 转换为 select a,b形式，将sql中的where条件按照索引从左到右自动排序，必须打开上述的opt选项，数据量大时建议打开。
	//'sql' => true,

	# 关闭自助建表，生产环境建议开启，开启之后无法对数据表结构进行更新操作
	//'create' => true,

	# 是否开启mysql自助优化功能，开启后，会记录所有where条件和order的字段，可以方便的在后台进行分析、增加索引，必须安装manage组件
	'opt' => true,

	# 默认数据库配置
	'default' => array
	(
		'type' => 'pdo',
		'host' => array
		(
			'read' => 'web-mysql:3306',
			'update' => 'web-mysql:3306',
			//'read' => '192.168.1.203:3307',
			//'update' => '192.168.1.203:3307',
		),
		'database' => 'dever',
		'username' => 'root',
		'password' => '123456',
		'charset' => 'utf8',
	),
	
	# 持久化服务器，只负责保存数据，跟读写分离差不多，但可以更换不同的数据库类型
	/*
	'save' => array
	(
		'type' => 'pdo',
		'host' => 'localhost:3306',
		'database' => 'dever_test',
		'username' => 'root',
		'password' => '123456',
		'charset' => 'utf8',
	),
	*/
	# 迁移旧的数据库服务器，使用方法：Dever::db('atom/article:old')
	'old' => array
	(
		'type' => 'pdo',
		'host' => '192.168.1.205:3307',
		'database' => 'voguecms',
		'username' => 'root',
		'password' => '123456',
		'charset' => 'utf8',
	),

	'elastic' => array
	(
		'type' => 'elastic',
		'host' => '192.168.1.203:9200',
		'database' => 'purify1',
		'username' => 'elastic',
		'password' => 'changeme',
		# 分词插件 只针对text类型的字段有效
		'analyzer' => 'ik_max_word',
		# 基本配置
		'setting' => array
		(
			'index' => array
			(
				'number_of_shards' => 2,
				'number_of_replicas' => 1,
			),
		),
	),

	'mongo' => array
	(
		'type' => 'mongo',
		'host' => '192.168.1.203:27017',
		'database' => 'condenast',
		'timeout' => 1000,
	)
	/*
	# 换成mongodb
	array
	(
		'type' => 'mongo',
		'host' => '127.0.0.1:27017',
		'database' => 'farm',
		'timeout' => 1000,
	),
	*/
);

# 缓存配置
$config['cache'] = array
(
	# 启用mysql数据库缓存，这个缓存是根据表名自动生成，dever::load形式和service的all、one形式均自动支持，无需手动添加
	'mysql' => 0,
	# 启用页面缓存 会根据当前的url来生成缓存，相当于页面静态化。
	'html' => 0,
	# 启用数据级别缓存 这个缓存是程序员自定义的：Dever::cache('name', 'value', 3600);
	'data' => 3600,
	# 启用load加载器缓存，一般不加载
	'load' => 0,
	# 启用load加载器的远程加载缓存
	'curl' => 3600,

	# 缓存精细控制，可以根据缓存的key（mysql为表名、service为小写类名，规则是模糊匹配），来控制每一条缓存
	'loadKey' => array
	(
		# 定义缓存名为auth.data的缓存时间
		'auth.data' => 200,
	),
	
	# 缓存清理的参数名,请通过shell=clearcache执行
	'shell' => 'clearcache',

	# 是否启用key失效时间记录，启用之后，将会记录每个key的失效时间
	'expire' => true,

	# 缓存类型
	'type' => 'memcache',//memcache、redis

	# 缓存保存方式，支持多个数据源、多台缓存服务器
	'store' => array
	(
		/*
		array
		(
			'host' => 'server-memcached',
			'port' => '11211',
			'weight' => 100,
		),

		array
		(
			'host' => 'server-memcached',
			'port' => '11212',
			'weight' => 100,
		),
		*/
	),
);

# debug配置
$config['debug'] = array
(
	# 开启错误提示 生产环境建议禁止
	'error' => true,
	
	# 错误日志记录，为空则不开启，为数组则为socket等服务
	'log' => true,//array('type' => 'udp', 'host' => 'host', 'port' => 'port')
	# 是否开启记录超时时间，单位为秒
	'overtime' => 3,

	# 开始访问报告
	# 生产环境建议禁止或添加ip限制，多个ip用逗号隔开
	# 如禁止，值为false，下述shell也将失效
	# 值为2，则开启强制模式，任何输出都将打印debug
	'request' => Dever::ip(),

	# 设定打印访问报告的指令
	'shell' => 'debug',
	# 以上指令，请通过&shell=debug来执行，如果你想设置断点或者打印当前业务逻辑下的sql，请直接用Dever::debug();打印数据

);
$local = isset($_SERVER['HTTP_HOST']) ? $_SERVER['HTTP_HOST'] : 'localhost';
# 本地可用这个
$host = 'http://'.$local . '/';

# 定义assets的域名
$assets = DEVER_APP_HOST . 'assets/';

# host 设置
$config['host'] = array
(
	# 跟域名
	'base' 	=> DEVER_APP_HOST,

	# cookie 域名
	'cookie' => '',
	
	# assets网络路径，会自动将assets替换为assets/模板
	'assets' => $assets,
	# public
	'public' => $assets . 'public/',
	'css' => $assets . 'css/',
	'js' => $assets . 'js/',
	'img' => $assets . 'img/',
	'images' => $assets . 'images/',
	'lib' => $assets . 'lib/',

	# 后台管理系统的assets路径
	'manage' => $host . 'dever/package/manage/assets/default/',
	
	# 合并之后的网络路径，填写之后自动合并资源，不填写则不合并，适合把资源放到云端
	'merge' => $host . 'dever/data/assets/' . DEVER_PROJECT . '/',
	
	# 上传系统的上传路径的域名(不带action)
	'upload'=> $host . 'dever/package/upload/?save',
	# 上传系统的资源访问地址
	'uploadRes'	=> $host . 'dever/data/upload/',

	# 域名替换，支持*通配符
	/*
    'domain' => array
    (
        'rule' => function()
        {
            $source = $desc = 'http://';
            if(function_exists('isHttps') && isHttps())
            {
                $desc = 'https://';
            }

            return array($source, $desc);
        },
        'replace' => array('*.selfimg.com.cn')
    ),
    */
	
	# 是否启用代理功能
	//'proxy' => $host . 'dever/application/plant/main/?data.proxy?',

	# 项目定义,Dever::load将自动转为这个配置，替换掉data/project/default.php里的数据
	'project' => array
	(
		'test' => array('url' => '', 'path' => ''),
	),

	'apiServer' => array
	(
		'type' => 'tcp',
	
		# 以下为tcp模式特有的配置
		# 是否使用后台运行
		//'backend' => 1,
		# 以下为swoole的配置
		'worker_num' => 1,

	),
);

return $config;
