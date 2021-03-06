<?php
/*
|--------------------------------------------------------------------------
| utf-8
|--------------------------------------------------------------------------
*/
header('Content-Type: text/html; charset=utf-8');

if (!defined('DEVER_PROJECT')) {
	define('DEVER_PROJECT', 'default');
}
/*
|--------------------------------------------------------------------------
| date rpc
|--------------------------------------------------------------------------
*/
date_default_timezone_set("PRC");
/*
|--------------------------------------------------------------------------
| start time
|--------------------------------------------------------------------------
*/
define('DEVER_START', microtime());
/*
|--------------------------------------------------------------------------
| DEVER time
|--------------------------------------------------------------------------
*/
define('DEVER_TIME', $_SERVER['REQUEST_TIME']);
/*
|--------------------------------------------------------------------------
| DEVER path
|--------------------------------------------------------------------------
*/
define('DEVER_PATH', dirname(__FILE__) . DIRECTORY_SEPARATOR);
/*
|--------------------------------------------------------------------------
| DEVER env config path
|--------------------------------------------------------------------------
*/
//define('DEVER_ENV_PATH', DEVER_PATH);
/*
|--------------------------------------------------------------------------
| DEVER project host
|--------------------------------------------------------------------------
*/
if (!defined('DEVER_ENTRY')) {
	define('DEVER_ENTRY', 'index.php');
}
if (isset($_SERVER['HTTP_HOST'])) {
	define('DEVER_HOST_TYPE', ((isset($_SERVER['HTTPS']) && $_SERVER['HTTPS'] == 'on') || (isset($_SERVER['HTTP_X_FORWARDED_PROTO']) && $_SERVER['HTTP_X_FORWARDED_PROTO'] == 'https')) ? 'https://' : 'http://');
	define('DEVER_APP_HOST', DEVER_HOST_TYPE . $_SERVER['HTTP_HOST'] . ($_SERVER['SCRIPT_NAME'] ? substr($_SERVER['SCRIPT_NAME'], 0, strpos($_SERVER['SCRIPT_NAME'], DEVER_ENTRY)) : DIRECTORY_SEPARATOR));
} else {
	define('DEVER_APP_HOST', '');
}
/*
|--------------------------------------------------------------------------
| autoload
|--------------------------------------------------------------------------
*/
if (is_file(DEVER_PATH . 'build/dever.phar1')) {
	require DEVER_PATH . 'build/dever.phar';

	if (is_file(DEVER_PATH . 'composer.json')) {
		require DEVER_PATH . 'vendor/autoload.php';
	}
} else {
	require DEVER_PATH . 'vendor/autoload.php';
}

if (is_file(DEVER_PROJECT_PATH . 'vendor/autoload.php')) {
	require DEVER_PROJECT_PATH . 'vendor/autoload.php';
}
/*
|--------------------------------------------------------------------------
| init config
|--------------------------------------------------------------------------
*/
Dever\Loader\Config::init();
/*
|--------------------------------------------------------------------------
| load debug
|--------------------------------------------------------------------------
*/
if (Dever\Loader\Config::get('debug')->error) {
	Dever\Output\Debug::report();
}
/*
|--------------------------------------------------------------------------
| project register
|--------------------------------------------------------------------------
*/
Dever\Loader\Project::register();
/*
|--------------------------------------------------------------------------
| route
|--------------------------------------------------------------------------
*/
$route = new Dever\Routing\Route;
/*
|--------------------------------------------------------------------------
| route run and out
|--------------------------------------------------------------------------
*/
if (!defined('DEVER_DAEMON')) {
	$route->runing()->output();
}
$route->close();
$route = null;