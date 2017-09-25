<?php

return array
(
	# 开启错误提示 生产环境建议禁止
	'error' => false,
	
	# 开始日志记录 暂时无效
	'log' => true,

	# 开始访问报告
	# 生产环境建议禁止或添加ip限制，多个ip用逗号隔开
	# 如禁止，值为false，下述shell也将失效
	# 值为2，则开启强制模式，任何输出都将打印debug
	'request' => false,

	# 设定打印访问报告的指令，值为request
	'shell' => 'dever_debug',
	# 以上指令，请通过&dever_debug=request来执行，如果你想设置断点或者打印当前业务逻辑下的sql，请直接用Dever::debug();打印数据

);
