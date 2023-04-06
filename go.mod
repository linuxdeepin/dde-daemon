module github.com/linuxdeepin/dde-daemon

go 1.20

require (
	github.com/axgle/mahonia v0.0.0-20180208002826-3358181d7394
	github.com/davecgh/go-spew v1.1.1
	github.com/fsnotify/fsnotify v1.6.0
	github.com/godbus/dbus/v5 v5.1.0
	github.com/gosexy/gettext v0.0.0-20160830220431-74466a0a0c4a
	github.com/jouyouyun/hardware v0.1.8
	github.com/linuxdeepin/dde-api v0.0.0-20230131030236-862dfbfc7b4e
	github.com/linuxdeepin/go-dbus-factory v0.0.0-20230208033821-bda82fd6525e
	github.com/linuxdeepin/go-gir v0.0.0-20230331033513-a8d7a9e89f9b
	github.com/linuxdeepin/go-lib v0.0.0-00010101000000-000000000000
	github.com/linuxdeepin/go-x11-client v0.0.0-20230131052004-7503e2337ee1
	github.com/mdlayher/netlink v1.7.1
	github.com/msteinert/pam v1.1.0
	github.com/rickb777/date v1.20.1
	github.com/stretchr/testify v1.8.1
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2
	google.golang.org/protobuf v1.28.1
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/josharian/native v1.0.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mdlayher/socket v0.4.0 // indirect
	github.com/mozillazg/go-pinyin v0.19.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rickb777/plural v1.4.1 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/youpy/go-riff v0.1.0 // indirect
	github.com/youpy/go-wav v0.3.2 // indirect
	github.com/zaf/g711 v0.0.0-20220109202201-cf0017bf0359 // indirect
	golang.org/x/image v0.5.0 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/text v0.7.0 // indirect
)

replace github.com/linuxdeepin/go-lib => github.com/Decodetalkers/go-lib v0.0.0-20230404025406-a17a10117d09

replace github.com/linuxdeepin/go-dbus-factory => github.com/Decodetalkers/go-dbus-factory v0.0.0-20230404030011-0eb743393708

replace github.com/linuxdeepin/dde-api => github.com/Decodetalkers/dde-api v0.0.0-20230404030416-afbfa3d5be94
