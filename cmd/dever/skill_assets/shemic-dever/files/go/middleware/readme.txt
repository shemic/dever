项目自定义 middleware 是可选目录。

如需项目级中间件，在本目录增加 Go 文件并提供 Register() 函数。
Dever 生成 data/router.go 时会自动导入并调用。

普通项目不要复制 package/front、package/bot 等组件自己的 middleware。
