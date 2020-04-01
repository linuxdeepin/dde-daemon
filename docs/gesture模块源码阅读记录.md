# dde-daemon 项目 gesture 模块源码阅读记录

项目当时 commit 01f25ae50a6e174d3d31881a8282aa19a6def346

位置 gesture 文件夹

## 文件 daemon.go

常量 dbusServiceName 服务名

常量 dbusServicePath 对象路径

常量 dbusServiceIFC 对象主要接口

定义了全局日志 logger

NewDaemon 函数创建新模块

init 函数注册模块

Daemon.GetDependencies 方法，为了实现 loader.Module 接口，返回空的依赖列表。

Daemon.Start 方法，也是为了实现 loader.Module 接口，作为一个模块的入口函数。
调用 newManager 创建 Manager 对象，再导出它，然后申请服务名， 再调用 Manager.init 方法进一步初始化。

Daemon.Stop 方法，也是为了实现 loader.Module 接口，停止模块时执行，负责清理。
关闭与 X 的连接，调用 Manager.destory 方法清理 Manager 内的东西。

## 文件 config.go

常量 ActionTypeShortcut 快捷键类动作

常量 ActionTypeCommandline 执行命令行类动作

常量 ActionTypeBuiltin 执行内建函数类动作

全局变量 configSystemPath 系统级别配置文件路径，适宜改成常量。

全局变量 configUserPath 用户级别配置文件路径

全局变量 gestureSchemaId 是 gsettings 配置方案的 id，适宜改成常量。

全局变量 gsKeyEnabled 启用设置键，适宜改成常量。

结构体 ActionInfo 描述动作定义信息，Type 类型，Action 具体动作。

结构体 gestureInfo 描述手势定义的信息，Name 名称，Direction 方向，Fingers 手指个数，Action 手势触发的动作。

类型 gestureInfos 是 gestureInfo 指针的 slice 类型

gestureInfos.Get 方法 根据名称、方向、手指个数找到某个 gestureInfo。

gestureInfos.Set 方法 先用 Get 方法找到某个 gestureInfo 然后设置它的 Action 字段值。

newGestureInfosFromFile 方法 读取文件内容，解析 JSON。

## 文件 utils.go

全局变量 xconn 与 X 的连接

全局变量 `_dconn` 是 dbus 连接，系统 bus。

全局变量 `_self` 是 org.freedesktop.login1 的 Session 对象

----

isKbdAlreadyGrabbed 函数返回键盘是否被 grab 了。
调用 getX11Conn 函数初始化全局变量 xconn，建立与 X 的连接。
求 grabWin，用 ewmh.GetActiveWindow 函数获取活跃窗口，如果没有活跃窗口，则 grabWin 为 root 窗口。如果有活跃窗口，还接着检查此窗口的可见性， 调用 x.GetWindowAttributes 函数获取窗口属性，然后根据 attrs.MapState 的值是否是 x.MapStateViewable 判断窗口是否可见，如果窗口可见，则 grabWin 为活跃窗口，否则 grabWin 为 root 窗口。 小结一下， grabWin 就是尽可能要可见的活跃窗口，否则就用 root 窗口替代。

为什么要这么麻烦地获取 grabWin 不清楚。

grabWin 是用于测试键盘是否被 grab 的窗口。

然后调用 keybind.GrabKeyboard 函数，如果成功则调用 keybind.UngrabKeyboard 函数立即取消 grab 键盘，然后返回 false，表示键盘没有被 grab 。

接着打印一条警告信息。

然后判定 err 是否是 keybind.GrabKeyboardError 类型，如果是，并且错误的 Status 字段值为 x.GrabStatusAlreadyGrabbed 则返回 true，表示键盘被 grab 了。

----

getCurrentActionWindowCmd 方法，返回当前活跃窗口的命令行，可能是用于识别窗口。
调用 ewmh.GetActiveWindow 获取活跃窗口，调用 ewmh.GetWMPid 获取窗口上的 pid 属性，得到创建窗口的进程的 pid。

然后读取文件 `/proc/$pid/cmdline` 来获取进程的命令行。

----

isSessionActive 方法 判定当期 session 是否为活跃状态。

如果 `_dconn` 为 nil，则调用 dbus.SystemBus 获取系统 bus 连接，否则用上次的。

如果 `_self` 为 nil，则调用 login1.NewSession 方法创建 logind 服务的 /org/freedesktop/login1/session/self 对象，否则用上次的。

获取 `_self` 的 Active 属性值，然后作为结果返回。

----
getX11Conn 方法 获取与 X 的连接。
如果 xconn 为空，则调用 x.NewConn 方法创建与 X 的连接，否则返回上次的。

isInWindowBlacklist 根据窗口的命令行粗略地判断是窗口是否在黑名单中。