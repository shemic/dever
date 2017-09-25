dever是一个php框架，与众不同。

这是一个面向开发者的框架，在这里，你可以抛弃一切束缚，不需要写让人头疼的后台系统，不需要学习模板引擎，不需要额外的写接口，没有MVC（即使dever最终的编译结果是MVC模式）。

你只需要开发你的业务，写好你的业务逻辑即可，你可以专心研究你的算法、你的架构。你甚至可以把你的业务打包，上传给dever，共享给其他dever开发者。


文档手册在此：https://www.kancloud.cn/rabin/dever-c/254006

这个手册是为condenast写的，但完全可以独立使用。后续会整理一份出来的。

新版本将使用dm工具进行安装：
<pre>
dm use dever //使用dever工具包
dm init //初始化并更新dever代码到php的share目录，开发项目时直接include即可。这快仿照python和go了。
dm install manage //安装由dever开发的后台管理系统
dm install passport //安装由dever开发的passport

//如果想使用扩展，可以用如下方式安装
dm use php //使用php工具包
dm install redis //安装redis扩展
dm use composer //使用composer工具包
dm install laravel/workerman //使用composer安装第三方开源的程序，默认是从国内源下载的。可以使用dm set 源名 更改源地址

详细的dm工具使用命令参考dm工具包
</pre>

以下描述已经过时，保留是因为有些地方要结合到dm中。

1、安装：
<pre>
git clone http://git.shemic.com/dever/workspace
cd workspace
chmod +x dever
</pre>

2、初始化：
<pre>
./dever init
OR
./dever install init
</pre>
注意：初始化也支持composer安装，但是dever类库并未上传至github。

3、配置：
<pre>
3.1、请把web/data/目录及其子目录设置为可写
3.2、请修改config/localhost下的几个配置文件，当然你的服务器也可以配置server，随意设置DEVER_SERVER的值，默认值是localhost
</pre>

4、dever包管理：
dever自带很多业务包，都是基于dever开发的：

通用的后台系统(manage)

微信管理(weixin)

oauth2.0客户端(oauth)

博客系统(blog)

小型cms系统(cms)


目前的包管理器中，只有manage开放给大家下载，后续会将所有包开放：

dever自带的后台管理组件，所有项目都可以通过这个后台来进行管理业务数据，而你的项目是不需要做额外的事情的。

安装manage：
<pre>
./dever install manage
</pre>

测试并访问manage：
<pre>
http://localhost/workspace/web/package/manage
</pre>

后台的默认管理员：
<pre>
账号：DMC@dever.cc
密码：admin_123
</pre>

安装demo：
<pre>
./dever install demo
</pre>

测试并访问demo：
<pre>
http://localhost/workspace/web/application/demo
</pre>

装好demo之后，请到你的manage后台中查看一下是否安装了这个demo项目

5、composer包管理：
dever还支持composer，导入你所需要的包。

初始化：
<pre>
./dever install composer
</pre>

更新：
<pre>
./dever up composer
</pre>

后续将使用dever开通官网：dever.cc以及code.dever.cc代码交流社区。

也会尽快开放wiki和完整版教程。
