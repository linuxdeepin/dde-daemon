package accounts

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/godbus/dbus"
	"github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.authenticate"
	"github.com/linuxdeepin/go-dbus-factory/org.freedesktop.login1"
	"pkg.deepin.io/dde/daemon/accounts/users"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/procfs"
)

const pwdChangerUserName = "pwd_changer"

// copy from golang 1.17, comment out some code because of unexported member
// String returns a human-readable description of c.
// It is intended only for debugging.
// In particular, it is not suitable for use as input to a shell.
// The output of String may vary across Go releases.
func cmdToString(c *exec.Cmd) string {

	// if c.lookPathErr != nil {
	// // failed to resolve path; report the original requested path (plus args)
	// return strings.Join(c.Args, " ")
	// }

	// report the exact executable path (plus args)
	b := new(strings.Builder)
	b.WriteString(c.Path)
	for _, a := range c.Args[1:] {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	return b.String()
}

// 表示启动短信验证码更改密码过程的调用者
type caller struct {
	display string
	xauth   string
	app     string
	proc    procfs.Process
	user    *user.User
}

// 验证调用者二进制文件路径, 并获得其 X 凭证位置, 以及 DISPLAY, 创建 caller 对象
func newCaller(service *dbusutil.Service, sender dbus.Sender) (ret *caller, err error) {
	pid, err := service.GetConnPID(string(sender))
	if err != nil {
		err = fmt.Errorf("fail to get sender PID: %v", err)
		return
	}

	proc := procfs.Process(pid)
	login1Manager := login1.NewManager(service.Conn())
	sessionPath, err := login1Manager.GetSessionByPID(0, pid)
	if err != nil {
		err = fmt.Errorf("fail to get sender session path: %v", err)
		return
	}

	session, err := login1.NewSession(service.Conn(), sessionPath)
	if err != nil {
		err = fmt.Errorf("fail to create session object proxy: %v", err)
		return
	}

	display, err := session.Display().Get(0)
	if err != nil {
		err = fmt.Errorf("fail to get sender display: %v", err)
	}

	exe, err := proc.Exe()
	if err != nil {
		err = fmt.Errorf("get sender exe error: %v", err)
		return
	}

	// 只允许来自控制中心, 锁屏和 greetter 的调用
	var app string
	switch exe {
	case "/usr/bin/dde-control-center":
		app = "control-center"
	case "/usr/bin/dde-lock":
		app = "lock"
	case "/usr/bin/lightdm-deepin-greeter":
		app = "greeter"
	default:
		err = fmt.Errorf("set password with Union ID called by %s, which is not allow", exe)
		return
	}

	status, err := proc.Status()
	if err != nil {
		err = fmt.Errorf("failed to get sender status: %v", err)
		return
	}

	uids, err := status.Uids()
	if err != nil {
		err = fmt.Errorf("failed to get sender euid: %v", err)
		return
	}

	uid := uids[1] // EUID

	user, err := user.LookupId(fmt.Sprint(uid))
	if err != nil {
		err = fmt.Errorf("failed to lookup sender username: %v", err)
		return
	}

	envs, err := proc.Environ()
	if err != nil {
		err = fmt.Errorf("failed to get sender environment variables: %v", err)
		return
	}

	xauth, found := envs.Lookup("XAUTHORITY")
	if !found {
		// $HOME/.Xauthority is default authority file if XAUTHORITY isn't defined.
		xauth = filepath.Join(user.HomeDir, ".Xauthority")
	}

	ret = &caller{
		display: display,
		xauth:   xauth,
		proc:    proc,
		user:    user,
		app:     app,
	}
	return
}

type pwdChanger struct {
	user    *user.User
	uid     int
	lang    string
	xauth   string
	display string
}

// 此函数负责初始化用于更改密码的用户, 主要是要将来自 caller 的 xauth 凭证发送给 pwd_changer
func setupPwdChanger(caller *caller, lang string) (ret *pwdChanger, err error) {

	user, err := user.Lookup(pwdChangerUserName)
	if err != nil {
		err = fmt.Errorf("fail to get user info of %s: %v", pwdChangerUserName, err)
		return
	}

	path := filepath.Join("/run", "user", user.Uid)
	err = os.Mkdir(path, os.FileMode(0700))
	if err != nil {
		if os.IsExist(err) {
			logger.Warningf("path %s existed, remove it now", path)
			err = os.RemoveAll(path)
			if err != nil {
				err = fmt.Errorf("fail to remove existed dir that hold .Xauthority: %v", err)
				return
			}
			err = os.Mkdir(path, os.FileMode(0700))
		}
		if err != nil {
			err = fmt.Errorf("fail to create dir that hold .Xauthority: %v", err)
			return
		}
	}

	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		err = fmt.Errorf("fail to convert uid to int: %v", err)
		return
	}

	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		err = fmt.Errorf("fail to convert gid to int: %v", err)
		return
	}

	err = os.Chown(path, uid, gid)
	if err != nil {
		err = fmt.Errorf("fail to chown dir that hold .Xauthority: %v", err)
		return
	}

	xauthPath := filepath.Join(path, ".Xauthority")
	file, err := os.Create(xauthPath)
	if err != nil {
		err = fmt.Errorf("fail to create .Xauthority: %v", err)
		return
	}

	err = file.Chown(uid, gid)
	if err != nil {
		err = fmt.Errorf("fail to chown .Xauthority: %v", err)
		return
	}

	err = file.Close()
	if err != nil {
		err = fmt.Errorf("fail to close .Xauthority: %v", err)
		return
	}

	cmd := exec.Command("runuser", "-u", caller.user.Username, "--", "xauth", "list") //#nosec G204
	cmd.Env = append(cmd.Env, "XAUTHORITY="+caller.xauth)
	logger.Debugf("set password with union id: run \"%s\", envs: %v", cmdToString(cmd), cmd.Env)

	xauths, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("get xauth cookie failed: %v", err)
		return
	}

	lines := strings.Split(string(xauths), "\n")
	if len(lines) < 2 {
		err = fmt.Errorf("fail to get .Xauthority content of sender, result too short")
		return
	}

	args := []string{"-u", pwdChangerUserName, "--", "xauth", "add"}
	found := false
	for _, line := range lines {
		fields := strings.Split(line, "  ")
		if strings.HasSuffix(fields[0], caller.display) {
			found = true
			args = append(args, fields...)
			break
		}
	}
	if !found {
		err = fmt.Errorf("fail to get xauth cookie of caller, no display match")
		return
	}

	cmd = exec.Command("runuser", args...) //#nosec G204
	cmd.Env = append(cmd.Env, "XAUTHORITY="+xauthPath)
	logger.Debugf("set password with union id: run \"%s\", envs: %v", cmdToString(cmd), cmd.Env)

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("add xauth fail: %v", err)
		return
	}

	ret = &pwdChanger{
		user:    user,
		uid:     uid,
		lang:    lang,
		xauth:   xauthPath,
		display: caller.display,
	}
	return
}

func (u *User) setPwdWithUnionID(sender dbus.Sender) (err error) {
	// TODO lock?

	caller, err := newCaller(u.service, sender)
	if err != nil {
		err = fmt.Errorf("newCaller failed: %v", err)
		return
	}

	pwdChanger, err := setupPwdChanger(caller, u.Locale)
	defer func() {
		if pwdChanger != nil {
			e := os.RemoveAll(filepath.Dir(pwdChanger.xauth))
			if e != nil {
				logger.Warningf("fail to remove tmp xauth dir: %v", e)
			}
		}
	}()
	if err != nil {
		err = fmt.Errorf("setup pwdChanger fail: %v", err)
		return
	}

	// 检验可执行文件的属性

	finfo, err := os.Stat("/usr/lib/dde-control-center/reset-password-dialog")
	if err != nil {
		err = fmt.Errorf("get reset-password-dislog stat fail: %v", err)
		return
	}

	fileSys := finfo.Sys()
	stat, ok := fileSys.(*syscall.Stat_t)
	if !ok {
		err = errors.New("fail to get stat of reset-password-dialog")
		return
	}

	// TODO does this convert to uint32 safe?
	if !(finfo.Mode() == 0500 && stat.Uid == uint32(pwdChanger.uid)) {
		err = errors.New("reset-password-dialog permission check failed")
		return err
	}

	// -u 用户的 UUID
	// -a 应用类型

	cmd := exec.Command("runuser", "-u", pwdChanger.user.Username, "--", "/usr/lib/dde-control-center/reset-password-dialog", "-u", u.UUID, "-a", caller.app) //#nosec G204

	stdin, err := cmd.StdinPipe()
	if err != nil {
		err = fmt.Errorf("get stdinpipe failed: %v", err)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("get stdoutpipe failed: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		err = fmt.Errorf("get stderrpipe failed: %v", err)
		return
	}

	cmd.Env = append(
		cmd.Env,
		"XAUTHORITY="+pwdChanger.xauth,
		"DISPLAY="+pwdChanger.display,
		"LANG="+pwdChanger.lang,
		"LANGUAGE="+pwdChanger.lang,
	)

	logger.Debugf("set password with union id: run \"%s\", envs: %v", cmdToString(cmd), cmd.Env)

	err = cmd.Start()
	if err != nil {
		err = fmt.Errorf("fail to start reset-password-dialog: %v", err)
		return
	}

	r := bufio.NewReader(stdout)
	w := bufio.NewWriter(stdin)
	e := bufio.NewReader(stderr)

	buf := bytes.NewBuffer([]byte{})
	end := make(chan struct{}, 1)
	go func() {
		_, err := io.Copy(buf, e)
		if err != nil {
			logger.Warningf("fail to get stderr of reset-password-dialog: %v", err)
		}
		end <- struct{}{}
	}()

	// 如果以上的过程失败了, defer 会将刚创建的文件夹删掉.
	// 如果成功了, 那么我们将 pwdChanger 置为空, 延迟到对话框结束运行再删除资源.
	_pwdChanger := pwdChanger
	pwdChanger = nil

	go func() {
		defer func() {
			if _pwdChanger != nil {
				e := os.RemoveAll(filepath.Dir(_pwdChanger.xauth))
				if e != nil {
					logger.Warningf("fail to remove tmp auth dir: %v", e)
				}
			}
			err = cmd.Wait()
			if err != nil {
				<-end
				logger.Warningf("reset-password-dialog exited: %v\nstderr:\n%v", err, buf)
			}
		}()
		line, err := r.ReadString('\n')
		if err != nil {
			logger.Warningf("set password with union ID: read line from reset-password-dialog failed: %v", err)
			_, _ = w.Write([]byte(fmt.Sprintf("fail to read from reset-password-dialog: %v\n", err)))
			_ = w.Flush()
			return
		}

		line = line[:len(line)-1]
		err = users.ModifyPasswd(line, u.UserName)
		if err != nil {
			logger.Warningf("set password with union ID: fail to modify password: %v", err)
			_, _ = w.Write([]byte(fmt.Sprintf("fail to modify password: %v\n", err)))
			_ = w.Flush()
			return
		}

		auth := authenticate.NewAuthenticate(u.service.Conn())
		err = auth.ResetLimits(0, u.UserName)
		if err != nil {
			logger.Warningf("set password with union ID: fail to reset limits: %v", err)
			_, _ = w.Write([]byte(fmt.Sprintf("fail to reset limits: %v\n", err)))
			_ = w.Flush()
			return
		}
		_, err = w.Write([]byte("success\n"))
		_ = w.Flush()
		if err != nil {
			logger.Warningf("set password with union ID: fail to write success message: %v", err)
			return
		}
	}()
	return
}
