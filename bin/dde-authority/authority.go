package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/linuxdeepin/go-dbus-factory/com.deepin.daemon.accounts"
	"github.com/linuxdeepin/go-dbus-factory/net.reactivated.fprint"
	ofdbus "github.com/linuxdeepin/go-dbus-factory/org.freedesktop.dbus"
	"pkg.deepin.io/lib/dbus1"
	"pkg.deepin.io/lib/dbusutil"
	"pkg.deepin.io/lib/pam"
)

const (
	pamConfigDir = "/etc/pam.d"
)

func isPamServiceExist(name string) bool {
	_, err := os.Stat(filepath.Join(pamConfigDir, name))
	return err == nil
}

type Authority struct {
	service       *dbusutil.Service
	sigLoop       *dbusutil.SignalLoop
	count         uint64
	mu            sync.Mutex
	txs           map[uint64]Transaction
	fprintManager *fprint.Manager
	dbusDaemon    *ofdbus.DBus
	accounts      *accounts.Accounts

	methods *struct {
		Start       func() `in:"authType,user,agentObj" out:"transactionObj"`
		CheckCookie func() `in:"user,cookie" out:"result"`
	}
}

func newAuthority(service *dbusutil.Service) *Authority {
	sysBus := service.Conn()
	auth := &Authority{
		service:       service,
		txs:           make(map[uint64]Transaction),
		fprintManager: fprint.NewManager(sysBus),
		dbusDaemon:    ofdbus.NewDBus(sysBus),
		accounts:      accounts.NewAccounts(sysBus),
		sigLoop:       dbusutil.NewSignalLoop(sysBus, 10),
	}

	auth.sigLoop.Start()
	auth.listenDBusSignals()
	return auth
}

func (*Authority) GetInterfaceName() string {
	return dbusInterface
}

var authTypeMap = map[string]string{
	"keyboard": "deepin-auth-keyboard",
}

func (a *Authority) listenDBusSignals() {
	a.dbusDaemon.InitSignalExt(a.sigLoop, true)
	_, err := a.dbusDaemon.ConnectNameOwnerChanged(func(name string, oldOwner string, newOwner string) {
		if strings.HasPrefix(name, ":") && newOwner == "" {
			var lostTxs []Transaction
			a.mu.Lock()
			for _, tx := range a.txs {
				if tx.matchSender(name) {
					log.Println("lost tx", name, tx.getId())
					lostTxs = append(lostTxs, tx)
				}
			}
			a.mu.Unlock()

			go func() {
				for _, tx := range lostTxs {
					_ = tx.End(dbus.Sender(name))
				}
			}()
		}
	})
	if err != nil {
		log.Println("WARN:", err)
	}
}

func (a *Authority) Start(sender dbus.Sender, authType, user string, agent dbus.ObjectPath) (dbus.ObjectPath, *dbus.Error) {
	a.service.DelayAutoQuit()
	if !agent.IsValid() {
		return "/", dbusutil.ToError(errors.New("agent path is invalid"))
	}

	var path dbus.ObjectPath
	var err error
	if authType == "fprint" {
		path, err = a.StartFPrint(sender, user, agent)
	} else {
		path, err = a.StartPAM(sender, authType, user, agent)
	}
	if err != nil {
		return "/", dbusutil.ToError(err)
	}
	return path, nil
}

func (a *Authority) StartFPrint(sender dbus.Sender, user string, agent dbus.ObjectPath) (dbus.ObjectPath, error) {
	a.mu.Lock()
	id := a.count
	a.count++
	a.mu.Unlock()

	tx := &FPrintTransaction{
		Sender: string(sender),
		baseTransaction: baseTransaction{
			id:     id,
			parent: a,
			user:   user,
		},
	}

	tx.agent = a.service.Conn().Object(string(sender), agent)
	path := getTxObjPath(id)
	err := a.service.Export(path, tx)
	if err != nil {
		return "/", err
	}

	a.mu.Lock()
	a.txs[id] = tx
	a.mu.Unlock()

	return path, nil
}

func (a *Authority) StartPAM(sender dbus.Sender, authType, user string, agent dbus.ObjectPath) (dbus.ObjectPath, error) {

	var tx *PAMTransaction
	pamService, ok := authTypeMap[authType]
	if !ok {
		return "/", errors.New("invalid auth type")
	}
	if !isPamServiceExist(pamService) {
		return "/", fmt.Errorf("pam service %q not exist", pamService)
	}

	tx, err := a.startPAMTx(pamService, user, string(sender))
	if err != nil {
		return "/", err
	}

	tx.agent = a.service.Conn().Object(string(sender), agent)
	path := getTxObjPath(tx.id)
	err = a.service.Export(path, tx)
	if err != nil {
		return "/", err
	}
	return path, nil
}

func (a *Authority) startPAMTx(service, user, sender string) (*PAMTransaction, error) {
	a.mu.Lock()
	id := a.count
	a.count++
	a.mu.Unlock()

	tx := &PAMTransaction{
		Sender: sender,
		baseTransaction: baseTransaction{
			id:     id,
			parent: a,
			user:   user,
		},
	}

	pamTx, err := pam.Start(service, user, tx)
	if err != nil {
		return nil, err
	}
	tx.core = pamTx

	a.mu.Lock()
	a.txs[id] = tx
	a.mu.Unlock()

	return tx, nil
}

func (a *Authority) CheckCookie(user, cookie string) (bool, *dbus.Error) {
	a.service.DelayAutoQuit()
	if user == "" || cookie == "" {
		return false, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, tx := range a.txs {
		user0, cookie0 := tx.getUserCookie()
		if cookie == cookie0 && user == user0 {
			tx.clearCookie()
			return true, nil
		}
	}
	return false, nil
}

func (a *Authority) deleteTx(id uint64) {
	log.Println("deleteTx", id)
	a.mu.Lock()
	defer a.mu.Unlock()

	tx := a.txs[id]
	if tx == nil {
		return
	}

	impl := tx.(dbusutil.Implementer)
	err := a.service.StopExport(impl)
	if err != nil {
		log.Println("warning:", err)
	}
	delete(a.txs, id)
}

func (a *Authority) getUserLocale(username string) (string, error) {
	user, err := a.accounts.FindUserByName(0, username)
	if err != nil {
		return "", err
	}
	userPath := dbus.ObjectPath(user)
	sysBus := a.service.Conn()
	userObj, err := accounts.NewUser(sysBus, userPath)
	if err != nil {
		return "", err
	}

	locale, err := userObj.Locale().Get(0)
	return locale, err
}
