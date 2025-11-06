// SPDX-FileCopyrightText: 2018 - 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package audio

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/linuxdeepin/go-lib/strv"

	dbus "github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/dde-daemon/common/dconfig"
	"github.com/linuxdeepin/dde-daemon/common/dsync"
	notifications "github.com/linuxdeepin/go-dbus-factory/session/org.freedesktop.notifications"
	systemd1 "github.com/linuxdeepin/go-dbus-factory/system/org.freedesktop.systemd1"
	"github.com/linuxdeepin/go-lib/dbusutil"
	. "github.com/linuxdeepin/go-lib/gettext"
	"github.com/linuxdeepin/go-lib/pulse"
	"golang.org/x/xerrors"
)

const (
	gsKeyEnabled         = "enabled"
	dconfigSoundEffectId = "org.deepin.dde.daemon.soundeffect"
	dconfigDaemonAppId   = "org.deepin.dde.daemon"
	dconfigAudioId       = "org.deepin.dde.daemon.audio"

	dconfigDccAppid = "org.deepin.dde.control-center"
	dconfigSoundId  = "org.deepin.dde.control-center.sound"

	dbusServiceName = "org.deepin.dde.Audio1"
	dbusPath        = "/org/deepin/dde/Audio1"
	dbusInterface   = dbusServiceName

	pulseaudioService    = "pulseaudio.service"
	pipewireService      = "pipewire.service"
	pipewirePulseService = "pipewire-pulse.service"

	pulseaudioSocket    = "pulseaudio.socket"
	pipewireSocket      = "pipewire.socket"
	pipewirePulseSocket = "pipewire-pulse.socket"

	increaseMaxVolume = 1.5
	normalMaxVolume   = 1.0

	dsgkeySoundShowDeviceManager = "showDeviceManager"

	dsgkeyPausePlayer             = "pausePlayer"
	dsgKeyAutoSwitchPort          = "autoSwitchPort"
	dsgKeyBluezModeFilterList     = "bluezModeFilterList"
	dsgKeyPortFilterList          = "portFilterList"
	dsgKeyInputDefaultPriorities  = "inputDefaultPrioritiesByType"
	dsgKeyOutputDefaultPriorities = "outputDefaultPrioritiesByType"
	dsgKeyBluezModeDefault        = "bluezModeDefault"
	dsgKeyMonoEnabled             = "monoEnabled"
	dsgKeyReduceNoiseEnabled      = "reduceNoiseEnabled" // 降噪配置，保存用户数据

	dsgKeyFirstRun                 = "firstRun"
	dsgKeyInputVolume              = "inputVolume"
	dsgKeyOutputVolume             = "outputVolume"
	dsgKeyHeadphoneOutputVolume    = "headphoneOutputVolume"
	dsgKeyHeadphoneUnplugAutoPause = "headphoneUnplugAutoPause"
	dsgKeyVolumeIncrease           = "volumeIncrease"
	dsgKeyDisableAutoMute          = "disableAutoMute"

	changeIconStart    = "notification-change-start"
	changeIconFailed   = "notification-change-failed"
	changeIconFinished = "notification-change-finished"
)

var (
	defaultInputVolume           = 0.1
	defaultOutputVolume          = 0.5
	defaultHeadphoneOutputVolume = 0.17
	gMaxUIVolume                 float64

	// 保存 pulaudio ,pipewire 相关的服务
	pulseaudioServices = []string{pulseaudioService, pulseaudioSocket}
	pipewireServices   = []string{pipewireService, pipewirePulseService, pipewireSocket, pipewirePulseSocket}
)

const (
	// 音频服务更改状态：已经完成
	AudioStateChanged = true
	// 音频服务更改状态：正在修改中
	AudioStateChanging = false
)

//go:generate dbusutil-gen -type Audio,Sink,SinkInput,Source,Meter -import github.com/godbus/dbus audio.go sink.go sinkinput.go source.go meter.go
//go:generate dbusutil-gen em -type Audio,Sink,SinkInput,Source,Meter

func objectPathSliceEqual(v1, v2 []dbus.ObjectPath) bool {
	if len(v1) != len(v2) {
		return false
	}
	for i, e1 := range v1 {
		if e1 != v2[i] {
			return false
		}
	}
	return true
}

func isStrvEqual(l1, l2 []string) bool {
	if len(l1) != len(l2) {
		return false
	}

	sort.Strings(l1)
	sort.Strings(l2)
	for i, v := range l1 {
		if v != l2[i] {
			return false
		}
	}
	return true
}

type Audio struct {
	service *dbusutil.Service
	PropsMu sync.RWMutex
	// dbusutil-gen: equal=objectPathSliceEqual
	SinkInputs []dbus.ObjectPath
	// dbusutil-gen: equal=objectPathSliceEqual
	Sinks []dbus.ObjectPath
	// dbusutil-gen: equal=objectPathSliceEqual
	Sources                 []dbus.ObjectPath
	DefaultSink             dbus.ObjectPath
	DefaultSource           dbus.ObjectPath
	Cards                   string
	CardsWithoutUnavailable string
	BluetoothAudioMode      string // 蓝牙模式
	// dbusutil-gen: equal=isStrvEqual
	BluetoothAudioModeOpts []string // 可用的蓝牙模式
	CurrentAudioServer     string   // 当前使用的音频服务
	AudioServerState       bool     // 音频服务状态

	// dbusutil-gen: ignore
	IncreaseVolume dconfig.Bool `prop:"access:rw"`

	PausePlayer bool `prop:"access:rw"`

	ReduceNoise bool `prop:"access:rw"`

	defaultPaCfg defaultPaConfig

	// 最大音量
	MaxUIVolume float64 // readonly

	// 单声道设置
	Mono bool

	headphoneUnplugAutoPause bool

	audioDConfig    *dconfig.DConfig
	dccSoundDconfig *dconfig.DConfig

	ctx       *pulse.Context
	eventChan chan *pulse.Event
	stateChan chan int

	// 正常输出声音的程序列表
	sinkInputs    map[uint32]*SinkInput
	defaultSink   *Sink
	defaultSource *Source
	sinks         map[uint32]*Sink
	sources       map[uint32]*Source
	meters        map[string]*Meter
	mu            sync.Mutex
	quit          chan struct{}

	cards CardList

	isSaving    bool
	saverLocker sync.Mutex

	syncConfig     *dsync.Config
	sessionSigLoop *dbusutil.SignalLoop

	noRestartPulseAudio bool
	// 自动端口切换
	enableAutoSwitchPort bool

	// 控制中心-声音-设备管理 是否显示
	controlCenterDeviceManager dconfig.Bool

	// nolint
	signals *struct {
		PortEnabledChanged struct {
			cardId   uint32
			portName string
			enabled  bool
		}
	}
}

func newAudio(service *dbusutil.Service) *Audio {
	a := &Audio{
		service:          service,
		meters:           make(map[string]*Meter),
		MaxUIVolume:      pulse.VolumeUIMax,
		AudioServerState: AudioStateChanged,
	}

	var err error
	a.audioDConfig, err = dconfig.NewDConfig(dconfigDaemonAppId, dconfigAudioId, "")
	if err != nil {
		logger.Warningf("NewAudioDConfig failed: %v", err)
		return a
	}

	a.dccSoundDconfig, err = dconfig.NewDConfig(dconfigDccAppid, dconfigSoundId, "")
	if err != nil {
		logger.Warningf("NewDccDConfig failed: %v", err)
	}

	a.audioDConfig.Reset(dsgKeyInputVolume)
	a.audioDConfig.Reset(dsgKeyOutputVolume)
	a.audioDConfig.Reset(dsgKeyHeadphoneOutputVolume)
	a.IncreaseVolume.Bind(a.audioDConfig, dsgKeyVolumeIncrease)
	a.PausePlayer = false
	a.ReduceNoise = false
	a.emitPropChangedReduceNoise(a.ReduceNoise)
	a.CurrentAudioServer = a.getCurrentAudioServer()
	a.headphoneUnplugAutoPause, err = a.audioDConfig.GetValueBool(dsgKeyHeadphoneUnplugAutoPause)
	if err != nil {
		logger.Warningf("Get headphoneUnplugAutoPause ValueBool failed: %v", err)
	}

	if a.IncreaseVolume.Get() {
		a.MaxUIVolume = increaseMaxVolume
	} else {
		a.MaxUIVolume = normalMaxVolume
	}
	gMaxUIVolume = a.MaxUIVolume

	if a.dccSoundDconfig != nil {
		a.controlCenterDeviceManager.Bind(a.dccSoundDconfig, dsgkeySoundShowDeviceManager)
	}

	a.sessionSigLoop = dbusutil.NewSignalLoop(service.Conn(), 10)
	a.syncConfig = dsync.NewConfig("audio", &syncConfig{a: a},
		a.sessionSigLoop, dbusPath, logger)
	a.sessionSigLoop.Start()

	return a
}

func (a *Audio) setAudioServerFailed(oldAudioServer string) {
	sendNotify(changeIconFailed, "", Tr("Failed to change Audio Server, please try later"))
	// 还原音频服务
	a.PropsMu.Lock()
	a.setPropCurrentAudioServer(oldAudioServer)
	a.setPropAudioServerState(AudioStateChanged)
	a.PropsMu.Unlock()
}

func (a *Audio) getCurrentAudioServer() (serverName string) {
	audioServers := []string{pulseaudioService, pipewireService}
	systemd := systemd1.NewManager(a.service.Conn())

	for _, server := range audioServers {
		path, err := systemd.GetUnit(0, server)
		if err == nil {
			serverSystemdUnit, err := systemd1.NewUnit(a.service.Conn(), path)
			if err == nil {
				state, err := serverSystemdUnit.Unit().LoadState().Get(0)
				if err != nil {
					logger.Warning("Failed to get LoadState of unit", path)
				} else if state == "loaded" {
					return strings.Split(server, ".")[0]
				}
			}
		}
	}

	return ""
}

func (a *Audio) SetCurrentAudioServer(serverName string) *dbus.Error {
	a.PropsMu.Lock()
	a.setPropAudioServerState(AudioStateChanging)
	a.setPropCurrentAudioServer(serverName)
	a.PropsMu.Unlock()

	sendNotify(changeIconStart, "", Tr("Changing Audio Server, please wait..."))

	var activeServices, deactiveServices []string
	if serverName == "pulseaudio" {
		activeServices = pulseaudioServices
		deactiveServices = pipewireServices
	} else {
		activeServices = pipewireServices
		deactiveServices = pulseaudioServices
	}

	oldAudioServer := a.CurrentAudioServer
	systemd := systemd1.NewManager(a.service.Conn())
	_, err := systemd.UnmaskUnitFiles(0, activeServices, false)
	if err != nil {
		logger.Warning("Failed to unmask unit files", activeServices, "\nError:", err)
		a.setAudioServerFailed(oldAudioServer)
		return dbusutil.ToError(err)
	}

	_, err = systemd.MaskUnitFiles(0, deactiveServices, false, true)
	if err != nil {
		logger.Warning("Failed to mask unit files", deactiveServices, "\nError:", err)
		a.setAudioServerFailed(oldAudioServer)
		return dbusutil.ToError(err)
	}

	err = systemd.Reload(0)
	if err != nil {
		logger.Warning("Failed to reload unit files. Error:", err)
		return dbusutil.ToError(err)
	}

	sendNotify(changeIconFinished, "", Tr("Audio Server changed, please log out and then log in"))

	a.PropsMu.Lock()
	a.setPropAudioServerState(AudioStateChanged)
	a.PropsMu.Unlock()
	return nil
}

func sendNotify(icon, summary, body string) {
	sessionBus, err := dbus.SessionBus()
	if err != nil {
		logger.Warning(err)
		return
	}
	n := notifications.NewNotifications(sessionBus)
	_, err = n.Notify(0, Tr("dde-control-center"), 0,
		icon, summary, body,
		nil, nil, -1)
	logger.Debugf("send notification icon: %q, summary: %q, body: %q",
		icon, summary, body)

	if err != nil {
		logger.Warning(err)
	}
}

func startAudioServer(service *dbusutil.Service) error {
	//var pulseaudioState string
	var activeServices, deactiveServices, needMaskedServices []string
	audioServers := map[string][]string{
		pulseaudioService: pulseaudioServices,
		pipewireService:   pipewireServices,
	}
	// 默认音频服务
	var defaultAudioService = pipewireService

	var hasTreeland = false
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		hasTreeland = true
	}

	// 查询已安装的音频服务状态
	systemd := systemd1.NewManager(service.Conn())
	for server, _ := range audioServers {
		path, err := systemd.GetUnit(0, server)
		if err == nil {
			serverSystemdUnit, err := systemd1.NewUnit(service.Conn(), path)
			if err != nil {
				logger.Warning("failed to create service systemd unit", err)
				return err
			}

			state, err := serverSystemdUnit.Unit().LoadState().Get(0)
			if err != nil {
				logger.Warning("failed to get service active state", err)
				return err
			}

			if state == "loaded" {
				// 可加载的服务列表
				activeServices = append(activeServices, server)
			} else if state == "masked" {
				// 需要disable的服务列表
				deactiveServices = append(deactiveServices, server)
			}
		}
	}
	logger.Infof("get audio service, actived: %v， deactive: %v", activeServices, deactiveServices)
	var activeService string
	var found bool
	activeService = defaultAudioService
	needMaskedServices, found = strv.Strv(activeServices).Delete(defaultAudioService)
	if !found {
		if hasTreeland {
			// 如果是treeland环境，只支持pipewire，需要强制切换
			// 如果不存在，则在masked 服务中查找
			// 场景：X11环境切换到treeland环境，需要强制切换音频服务为pipewire
			found = strv.Strv(deactiveServices).Contains(defaultAudioService)
			if !found {
				err := fmt.Errorf("not found supported audio services")
				return err
			}
			logger.Warning("ready to unmask service:", audioServers[defaultAudioService])
		} else {
			if len(activeServices) > 0 {
				activeService = activeServices[0]
			} else {
				err := fmt.Errorf("no active services found")
				return err
			}
		}
	}

	// 将剩余可选的audio服务都mask
	logger.Info("need to active audio service:", activeService)
	logger.Info("ready to deactive audio service:", needMaskedServices)
	for _, server := range needMaskedServices {
		for _, deactiveService := range audioServers[server] {
			deactiveServicePath, err := systemd.GetUnit(0, deactiveService)
			if err == nil {
				if len(deactiveServicePath) != 0 {
					serverSystemdUnit, err := systemd1.NewUnit(service.Conn(), deactiveServicePath)

					if err != nil {
						logger.Warning("failed to create service systemd unit", err)
						return err
					}

					state, err := serverSystemdUnit.Unit().LoadState().Get(0)
					if err != nil {
						logger.Warning("failed to get service active state", err)
						return err
					}

					if state != "masked" {
						_, err := systemd.MaskUnitFiles(0, []string{deactiveService}, false, true)

						if err != nil {
							logger.Warning("Failed to mask unit files", err)
							return err
						}
					}

					// 服务在 mask 之前服务，可能被激活，调用 stop
					_, err = systemd.StopUnit(0, deactiveService, "replace")
					if err != nil {
						logger.Warning("Failed to stop service", err)
						return err
					}
				}
			}
		}
	}
	_, err := systemd.UnmaskUnitFiles(0, audioServers[activeService], false)
	if err != nil {
		logger.Warning("Failed to unmask unit files", activeServices, "\nError:", err)
		return err
	}
	err = systemd.Reload(0)
	if err != nil {
		logger.Warning("Failed to reload unit files. Error:", err)
		return err
	}

	for _, activeService := range audioServers[activeService] {
		activeServicePath, err := systemd.GetUnit(0, activeService)
		if err == nil {
			logger.Debug("ready to start audio server", activeServicePath)

			if len(activeServicePath) != 0 {
				serverSystemdUnit, err := systemd1.NewUnit(service.Conn(), activeServicePath)
				if err != nil {
					logger.Warning("failed to create audio server systemd unit", err)
					return err
				}

				state, err := serverSystemdUnit.Unit().ActiveState().Get(0)
				if err != nil {
					logger.Warning("failed to get audio server active state", err)
					return err
				}
				logger.Info("start audio service", activeService, state)
				if state != "active" {
					go func() {

						_, err := serverSystemdUnit.Unit().Start(0, "replace")
						if err != nil {
							logger.Warning("failed to start audio server unit:", err)
						}
					}()
				}
			}

		}
	}

	return nil
}

func getCtx() (ctx *pulse.Context, err error) {
	ctx = pulse.GetContextForced()
	if ctx == nil {
		err = errors.New("failed to get pulse context")
		return
	}
	return
}

const (
	dndVirtualSinkName          = "deepin_network_displays"
	fakeCardName                = "VirtualCard"
	dndVirtualSinkDescription   = "NetworkDisplayDevice "
	monoSinkName                = "remap-sink-mono"
	monoSinkModuleName          = "module-remap-sink"
	reduceNoiseSourceName       = "echo-cancel-source"
	reduceNoiseSourceModuleName = "module-echo-cancel"
	nullSinkName                = "null-sink"
	nullSinkModuleName          = "module-null-sink"
)

// genFakeCard 用于适配虚拟sink的场景,生成一个假的card
func (a *Audio) genFakeCard() *Card {
	sinkList := a.ctx.GetSinkList()
	for _, sink := range sinkList {
		if sink.Name == dndVirtualSinkName {
			port := pulse.CardPortInfo{
				PortInfo: pulse.PortInfo{
					Name:        dndVirtualSinkName,
					Description: dndVirtualSinkDescription,
					Priority:    0,
					Available:   2,
				},
				Direction: 1,
				Profiles:  nil,
			}
			card := &pulse.Card{
				Index:         sink.Index,
				Name:          fakeCardName,
				OwnerModule:   0,
				Driver:        "",
				PropList:      nil,
				Profiles:      nil,
				ActiveProfile: pulse.ProfileInfo2{},
				Ports: pulse.CardPortInfos{
					port,
				},
			}

			fakeCard := &Card{
				Id:            sink.Card,
				Name:          card.Name,
				ActiveProfile: nil,
				Profiles:      nil,
				Ports: pulse.CardPortInfos{
					port,
				},
				core: card,
			}
			return fakeCard
		}
	}
	return nil
}

func (a *Audio) refreshCards() {
	a.cards = newCardList(a.ctx.GetCardList())
	fakeCard := a.genFakeCard()
	if fakeCard != nil {
		a.cards = append(a.cards, fakeCard)
	}
	cards := a.cards.string()
	logger.Infof("cards : %s", cards)
	a.setPropCards(cards)
	a.setPropCardsWithoutUnavailable(a.cards.stringWithoutUnavailable())
}

// 添加一个新的sink,参数是pulse的Sink
func (a *Audio) addSink(sinkInfo *pulse.Sink) {
	sink := newSink(sinkInfo, a)
	a.sinks[sinkInfo.Index] = sink
	sinkPath := sink.getPath()
	err := a.service.Export(sinkPath, sink)
	if err != nil {
		logger.Warning(err)
	}
	a.updatePropSinks()
}

// 添加一个新的source,参数是pulse的Source
func (a *Audio) addSource(sourceInfo *pulse.Source) {
	source := newSource(sourceInfo, a)
	a.sources[sourceInfo.Index] = source
	sourcePath := source.getPath()
	err := a.service.Export(sourcePath, source)
	if err != nil {
		logger.Warning(err)
	}
	a.updatePropSources()
}

// 添加一个新的sink-input,参数是pulse的SinkInput
func (a *Audio) addSinkInput(sinkInputInfo *pulse.SinkInput) {
	sinkInput := newSinkInput(sinkInputInfo, a)
	a.sinkInputs[sinkInputInfo.Index] = sinkInput
	sinkInputPath := sinkInput.getPath()
	err := a.service.Export(sinkInputPath, sinkInput)
	if err != nil {
		logger.Warning(err)
	}
	a.updatePropSinkInputs()
}

func (a *Audio) refreshSinks() {
	if a.sinks == nil {
		a.sinks = make(map[uint32]*Sink)
	}

	// 获取当前的sinks
	sinkInfoMap := make(map[uint32]*pulse.Sink)
	sinkInfoList := a.ctx.GetSinkList()

	hasNullSink := false
	for _, sinkInfo := range sinkInfoList {
		if sinkInfo.Name == dndVirtualSinkName {
			port := pulse.PortInfo{
				Name:        sinkInfo.Name,
				Description: dndVirtualSinkDescription,
				Priority:    0,
				Available:   2,
			}
			sinkInfo.Ports = append(sinkInfo.Ports, port)
			sinkInfo.ActivePort = port
		}
		if sinkInfo.Name == nullSinkName {
			hasNullSink = true
		}
		sinkInfoMap[sinkInfo.Index] = sinkInfo
		sink, exist := a.sinks[sinkInfo.Index]
		if exist {
			// 存在则更新
			logger.Debugf("update sink #%d", sinkInfo.Index)
			sink.update(sinkInfo)
		} else {
			// 不存在则添加
			logger.Debugf("add sink #%d", sinkInfo.Index)
			a.addSink(sinkInfo)
		}
	}

	// 删除不存在的旧sink
	for key, sink := range a.sinks {
		_, exist := sinkInfoMap[key]
		if !exist {
			logger.Debugf("delete sink #%d", key)
			a.service.StopExport(sink)
			delete(a.sinks, key)
		}
	}

	// 加载module-null-sink，噪音抑制时，将sink-input端口Echo-Cancel Playback引入到null-sink
	if !hasNullSink {
		a.LoadNullSinkModule()
	}
}

func (a *Audio) refreshSources() {
	if a.sources == nil {
		a.sources = make(map[uint32]*Source)
	}

	// 获取当前的sources
	sourceInfoMap := make(map[uint32]*pulse.Source)
	sourceInfoList := a.ctx.GetSourceList()

	for _, sourceInfo := range sourceInfoList {
		sourceInfoMap[sourceInfo.Index] = sourceInfo
		source, exist := a.sources[sourceInfo.Index]
		if exist {
			// 存在则更新
			logger.Debugf("update source #%d", sourceInfo.Index)
			source.update(sourceInfo)
		} else {
			// 不存在则添加
			logger.Debugf("add source #%d", sourceInfo.Index)
			a.addSource(sourceInfo)
		}
	}

	// 删除不存在的旧source
	for key, source := range a.sources {
		_, exist := sourceInfoMap[key]
		if !exist {
			logger.Debugf("delete source #%d", key)
			a.service.StopExport(source)
			delete(a.sources, key)
		}
	}

	a.updatePropSources()
}

func (a *Audio) refershSinkInputs() {
	if a.sinkInputs == nil {
		a.sinkInputs = make(map[uint32]*SinkInput)
	}

	// 获取当前的sink-inputs
	sinkInputInfoMap := make(map[uint32]*pulse.SinkInput)
	sinkInputInfoList := a.ctx.GetSinkInputList()

	for _, sinkInputInfo := range sinkInputInfoList {
		sinkInputInfoMap[sinkInputInfo.Index] = sinkInputInfo
		sinkInput, exist := a.sinkInputs[sinkInputInfo.Index]
		if exist {
			logger.Debugf("update sink-input #%d", sinkInputInfo.Index)
			sinkInput.update(sinkInputInfo)
		} else {
			logger.Debugf("add sink-input #%d", sinkInputInfo.Index)
			a.addSinkInput(sinkInputInfo)
		}
	}

	// 删除不存在的旧sink-inputs
	for key, sinkInput := range a.sinkInputs {
		_, exist := sinkInputInfoMap[key]
		if !exist {
			logger.Debugf("delete sink-input #%d", key)
			a.service.StopExport(sinkInput)
			delete(a.sinkInputs, key)
		}
	}
}

func (a *Audio) autoPause() {
	if !a.PausePlayer {
		return
	}
	go pauseAllPlayers()
}

func (a *Audio) refreshDefaultSinkSource() {
	defaultSink := a.ctx.GetDefaultSink()
	defaultSource := a.ctx.GetDefaultSource()
	if defaultSink != "" {
		a.updateDefaultSink(defaultSink)
	}
	if defaultSource != "" {
		a.updateDefaultSource(defaultSource)
	}
}

func (a *Audio) refresh() {
	logger.Debug("refresh cards")
	a.refreshCards()
	logger.Debug("refresh sinks")
	a.refreshSinks()
	logger.Debug("refresh sources")
	a.refreshSources()
	logger.Debug("refresh sinkinputs")
	a.refershSinkInputs()
	logger.Debug("refresh default")
	a.refreshDefaultSinkSource()
	logger.Debug("refresh bluetooth mode opts")
	a.refreshBluetoothOpts()
	logger.Debug("refresh done")
}

func (a *Audio) init() error {
	disableAutoMuteValue, _ := a.audioDConfig.GetValueBool(dsgKeyDisableAutoMute)
	if disableAutoMuteValue {
		err := disableAutoMuteMode()
		if err != nil {
			logger.Warning(err)
		}
	}
	a.initDefaultVolumes()
	ctx, err := getCtx()
	if err != nil {
		return xerrors.Errorf("failed to get context: %w", err)
	}

	a.defaultPaCfg = loadDefaultPaConfig(defaultPaFile)
	logger.Debugf("defaultPaConfig: %+v", a.defaultPaCfg)

	a.ctx = ctx

	err = a.initDsgProp()
	if err != nil {
		return err
	}

	// 更新本地数据
	a.refresh()
	GetConfigKeeper().Load()

	logger.Debug("init cards")
	a.PropsMu.Lock()
	a.setPropCards(a.cards.string())
	a.setPropCardsWithoutUnavailable(a.cards.stringWithoutUnavailable())
	a.PropsMu.Unlock()

	a.eventChan = make(chan *pulse.Event, 100)
	a.stateChan = make(chan int, 10)
	a.quit = make(chan struct{})
	a.ctx.AddEventChan(a.eventChan)
	a.ctx.AddStateChan(a.stateChan)

	// priorities.Load(globalPrioritiesFilePath, a.cards) // TODO: 删除
	GetPriorityManager().Init(a.cards)
	GetPriorityManager().Print()

	go a.handleEvent()
	go a.handleStateChanged()
	logger.Debug("init done")

	if !a.autoSwitchOutputPort() {
		a.resumeSinkConfig(a.defaultSink)
	}

	if !a.autoSwitchInputPort() {
		a.resumeSourceConfig(a.defaultSource)
	}

	a.moveSinkInputsToSink(nil)

	// 蓝牙支持的模式
	a.refreshBluetoothOpts()

	return nil
}

func (a *Audio) destroyCtxRelated() {
	a.mu.Lock()
	a.ctx.RemoveEventChan(a.eventChan)
	a.ctx.RemoveStateChan(a.stateChan)
	close(a.quit)
	a.ctx = nil

	for _, sink := range a.sinks {
		err := a.service.StopExportByPath(sink.getPath())
		if err != nil {
			logger.Warningf("failed to stop export sink #%d: %v", sink.index, err)
		}
	}
	a.sinks = nil

	for _, source := range a.sources {
		err := a.service.StopExportByPath(source.getPath())
		if err != nil {
			logger.Warningf("failed to stop export source #%d: %v", source.index, err)
		}
	}
	a.sources = nil

	for _, sinkInput := range a.sinkInputs {
		err := a.service.StopExportByPath(sinkInput.getPath())
		if err != nil {
			logger.Warningf("failed to stop export sink input #%d: %v", sinkInput.index, err)
		}
	}
	a.sinkInputs = nil

	for _, meter := range a.meters {
		err := a.service.StopExport(meter)
		if err != nil {
			logger.Warning(err)
		}
	}
	a.mu.Unlock()
}

func (a *Audio) destroy() {
	a.sessionSigLoop.Stop()
	a.syncConfig.Destroy()
	a.destroyCtxRelated()
}

func (a *Audio) initDefaultVolumes() {
	inVolumePerValue, err := a.audioDConfig.GetValueFloat64(dsgKeyInputVolume)
	if err != nil {
		logger.Warning(err)
	}
	outVolumePerValue, err := a.audioDConfig.GetValueFloat64(dsgKeyOutputVolume)
	if err != nil {
		logger.Warning(err)
	}
	headphoneOutVolumePerValue, err := a.audioDConfig.GetValueFloat64(dsgKeyHeadphoneOutputVolume)
	if err != nil {
		logger.Warning(err)
	}

	defaultInputVolume = inVolumePerValue / 100.0
	defaultOutputVolume = outVolumePerValue / 100.0
	defaultHeadphoneOutputVolume = headphoneOutVolumePerValue / 100.0
}

// SetPort activate the port for the special card.
// The available sinks and sources will also change with the profile changing.
func (a *Audio) SetPort(cardId uint32, portName string, direction int32) *dbus.Error {
	logger.Infof("dbus call SetPort with cardId %d, portName %s and direction %d", cardId, portName, direction)

	if !a.isPortEnabled(cardId, portName, direction) {
		return dbusutil.ToError(fmt.Errorf("card idx: %d, port name: %q is disabled", cardId, portName))
	}

	card, err := a.cards.get(cardId)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	logger.Debugf("output port %s %s now is first priority", card.core.Name, portName)
	GetPriorityManager().SetTheFirstPort(card.core.Name, portName, int(direction))

	err = a.setPort(cardId, portName, int(direction), false)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	return dbusutil.ToError(err)
}

func (a *Audio) findSink(cardId uint32, activePortName string) *Sink {
	for _, sink := range a.sinks {

		if sink.Card == cardId {
			for _, port := range sink.Ports {
				if port.Name == activePortName {
					return sink
				}
			}
		}
	}

	return nil
}

func (a *Audio) findSource(cardId uint32, activePortName string) *Source {
	for _, source := range a.sources {
		if source.Card == cardId && source.ActivePort.Name == activePortName {
			return source
		}
	}

	return nil
}

func (a *Audio) SetPortEnabled(cardId uint32, portName string, enabled bool) *dbus.Error {
	logger.Infof("dbus call SetPortEnabled with cardId %d, portName %s and enabled %t", cardId, portName, enabled)

	if !a.enableAutoSwitchPort {
		err := errors.New("DConfig of org.deepin.dde.daemon.audio autoSwitchPort is false")
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	card, err := a.cards.get(cardId)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	GetConfigKeeper().SetEnabled(card, portName, enabled)

	err = a.service.Emit(a, "PortEnabledChanged", cardId, portName, enabled)
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	a.setPropCards(a.cards.string())
	a.setPropCardsWithoutUnavailable(a.cards.stringWithoutUnavailable())
	GetPriorityManager().refreshPorts(a.cards)
	a.autoSwitchPort()
	return nil
}

func (a *Audio) IsPortEnabled(cardId uint32, portName string) (enabled bool, busErr *dbus.Error) {
	// 不建议使用这个接口，可以从Cards和CardsWithoutUnavailable属性中获取此状态
	logger.Infof("dbus call IsPortEnabled with cardId %d and portName %s", cardId, portName)

	card, err := a.cards.get(cardId)
	if err != nil {
		logger.Warning(err)
		return false, dbusutil.ToError(err)
	}
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(card, portName)
	return portConfig.Enabled, nil
}

// setPort: 设置端口，auto为true表示自动切换，false表示手动切换
// 手动切换场景需要设置期望端口，用于恢复配置使用（保证禁用启用端口的正常）
// 设置声卡的期望配置文件，是用于自动切换是是否达到期望状态，防止频繁切换
func (a *Audio) setPort(cardId uint32, portName string, direction int, auto bool) error {
	// 获取声卡信息
	card := a.getCardById(cardId)
	if card == nil {
		err := fmt.Errorf("not found card:port<%v:%v>", card, portName)
		logger.Warning(err)
		return err
	}

	// 获取端口信息
	port, err := card.Ports.Get(portName, direction)
	if err != nil {
		logger.Warning(err)
		return err
	}
	// 获取声卡配置中的profile
	var profile string
	if direction == pulse.DirectionSink {
		profile = GetConfigKeeper().GetMode(card, portName)
		if profile == "" {
			profile = port.Profiles.SelectProfile()
			GetConfigKeeper().SetMode(card, portName, profile)
		}
	} else {
		profile = card.ActiveProfile.Name
	}

	if card.ActiveProfile.Name == profile {
		return a.setPortDirectly(cardId, portName, direction)
	} else {
		// 暂不设置端口，先切换配置文件
		// 等待事件完成后，再自动设置端口
		logger.Info("modify card profile:", card.core.Name, "from", card.ActiveProfile.Name, "to", profile)
		card.core.SetProfile(profile)
	}

	return nil
}

// setPortDirectly 直接设置端口，无需切换配置文件
func (a *Audio) setPortDirectly(cardId uint32, portName string, direction int) error {
	if direction == pulse.DirectionSink {
		return a.setSinkPortDirectly(cardId, portName)
	}
	return a.setSourcePortDirectly(cardId, portName)
}

// setSinkPortDirectly 直接设置输出端口
func (a *Audio) setSinkPortDirectly(cardId uint32, portName string) error {
	sink := a.findSink(cardId, portName)
	if sink == nil {
		return fmt.Errorf("sink not found for card #%d and port %q", cardId, portName)
	}

	// 设置端口
	if sink.ActivePort.Name != portName {
		logger.Infof("active port not match, set port: %s, %s", sink.Name, portName)
		a.ctx.SetSinkPortByIndex(sink.index, portName)
	}

	// 设置默认sink
	if a.getDefaultSinkName() != sink.Name {
		logger.Infof("set default sink: %s", sink.Name)
		a.ctx.SetDefaultSink(sink.Name)
	}

	return nil
}

// setSourcePortDirectly 直接设置输入端口
func (a *Audio) setSourcePortDirectly(cardId uint32, portName string) error {
	source := a.findSource(cardId, portName)
	if source == nil {
		return fmt.Errorf("source not found for card #%d and port %q", cardId, portName)
	}

	logger.Debugf("current source already exists, set source: %s, %s", source.Name, portName)

	// 设置端口
	if source.ActivePort.Name != portName {
		logger.Debugf("active port not match, set port: %s, %s", source.Name, portName)
		a.ctx.SetSourcePortByIndex(source.index, portName)
	}

	// 设置默认source
	if a.getDefaultSourceName() != source.Name {
		logger.Debugf("set default source: %s", source.Name)
		a.ctx.SetDefaultSource(source.Name)
	}

	return nil
}

func (a *Audio) resetSinksVolume() {
	logger.Debug("reset sink volume", defaultOutputVolume)
	for _, s := range a.ctx.GetSinkList() {
		a.ctx.SetSinkMuteByIndex(s.Index, false)
		curPort := s.ActivePort.Name
		portList := s.Ports
		sidx := s.Index
		for _, port := range portList {
			a.ctx.SetSinkPortByIndex(sidx, port.Name)
			// wait port active
			time.Sleep(time.Millisecond * 100)
			s, _ = a.ctx.GetSink(sidx)
			pname := strings.ToLower(port.Name)
			var cv pulse.CVolume
			if strings.Contains(pname, "headphone") || strings.Contains(pname, "headset") {
				cv = s.Volume.SetAvg(defaultHeadphoneOutputVolume).SetBalance(s.ChannelMap,
					0).SetFade(s.ChannelMap, 0)
			} else {
				cv = s.Volume.SetAvg(defaultOutputVolume).SetBalance(s.ChannelMap,
					0).SetFade(s.ChannelMap, 0)
			}
			a.ctx.SetSinkVolumeByIndex(sidx, cv)
			time.Sleep(time.Millisecond * 100)
		}
		a.ctx.SetSinkPortByIndex(sidx, curPort)
	}
}

func (a *Audio) resetSourceVolume() {
	logger.Debug("reset source volume", defaultInputVolume)
	for _, s := range a.ctx.GetSourceList() {
		if s.ActivePort.Name != "" {
			a.ctx.SetSourceMuteByIndex(s.Index, false)
			cv := s.Volume.SetAvg(defaultInputVolume).SetBalance(s.ChannelMap,
				0).SetFade(s.ChannelMap, 0)
			a.ctx.SetSourceVolumeByIndex(s.Index, cv)
		}
	}
}

func (a *Audio) Reset() *dbus.Error {
	logger.Infof("dbus call Reset")

	a.resetSinksVolume()
	a.resetSourceVolume()

	dconfigSoundEffect, err := dconfig.NewDConfig(dconfigDaemonAppId, dconfigSoundEffectId, "")
	if err != nil {
		logger.Warning(err)
		return dbusutil.ToError(err)
	}

	return dbusutil.ToError(dconfigSoundEffect.ResetAll())
}

func (a *Audio) moveSinkInputsToSink(inputs []uint32) {
	if len(a.sinkInputs) == 0 {
		return
	}

	sinkName := a.context().GetDefaultSink()
	if sinkName == "" {
		return
	}

	if inputs == nil {
		inputs = make([]uint32, 0)
		for _, sinkInput := range a.sinkInputs {
			inputs = append(inputs, sinkInput.index)
		}
	} else {
		if len(inputs) == 0 {
			return
		}
	}

	var list = make([]uint32, 0)
	var vrList = make([]uint32, 0)
	for _, index := range inputs {
		s, err := a.ctx.GetSinkInput(index)
		if err != nil {
			logger.Warning(err)
			continue
		}
		if strings.Contains(s.Name, "Echo-Cancel") || strings.Contains(s.Name, "echo-cancel") {
			a.context().MoveSinkInputsByName([]uint32{index}, nullSinkName)
			continue
		} else if strings.Contains(s.Name, "remap-sink-mono") {
			vrList = append(vrList, index)
		} else {
			list = append(list, index)
		}
	}

	// 虚拟通道例如mono，映射到物理通道上
	isPhy := isPhysicalDevice(sinkName)
	if !isPhy {
		virSink := a.getSinkInfoByName(sinkName)
		if virSink == nil {
			logger.Warning("failed to get virtual device sinkInfo for name:", sinkName)
			return
		}
		master := a.getMasterNameFromVirtualDevice(sinkName)
		if master == "" {
			logger.Warning("failed to get virtual device sinkInfo for name:", sinkName)
			return
		}
		if len(vrList) > 0 {
			logger.Infof("will move virtual sink-input %v to sink %s", vrList, master)
			a.context().MoveSinkInputsByName(vrList, master)
		}
	}
	if len(list) > 0 {
		logger.Infof("will move sink-input %v to sink %s", list, sinkName)
		a.context().MoveSinkInputsByName(list, sinkName)
	}
}

func isPortExists(name string, ports []pulse.PortInfo) bool {
	for _, port := range ports {
		if port.Name == name {
			return true
		}
	}
	return false
}

func (*Audio) GetInterfaceName() string {
	return dbusInterface
}

func (a *Audio) resumeSinkConfig(s *Sink) {
	// 如果是null-sink，则不做任何配置
	if s == nil || strings.Contains(s.Name, nullSinkName) {
		logger.Warning("nil sink")
		return
	}
	if s.ActivePort.Name == "" {
		logger.Debug("no active port")
		return
	}

	logger.Debugf("resume sink %s %s", a.getCardNameById(s.Card), s.ActivePort.Name)
	card, err := a.cards.get(s.Card)
	if err != nil {
		logger.Warning(err)
		return
	}
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(card, s.ActivePort.Name)

	a.IncreaseVolume.Set(portConfig.IncreaseVolume)
	if portConfig.IncreaseVolume {
		a.MaxUIVolume = increaseMaxVolume
	} else {
		a.MaxUIVolume = normalMaxVolume
	}

	dbusErr := s.setVBF(portConfig.Volume, portConfig.Balance, 0.0)
	if dbusErr != nil {
		logger.Warning(dbusErr)
	}

	logger.Warningf("set %v mute %v", s.Name, GetConfigKeeper().Mute.MuteOutput || !portConfig.Enabled)
	s.setMute(GetConfigKeeper().Mute.MuteOutput || !portConfig.Enabled)
	s.setMono(a.Mono)
}

func (a *Audio) resumeSourceConfig(s *Source) {
	// 如果是null-sink，则不做任何配置
	if s == nil || strings.Contains(s.Name, nullSinkName) {
		logger.Warning("nil source")
		return
	}
	if s.ActivePort.Name == "" {
		logger.Debug("no active port", s.Name, s.index)
		return
	}

	logger.Debugf("resume source %s %s", a.getCardNameById(s.Card), s.ActivePort.Name)
	card, err := a.cards.get(s.Card)
	if err != nil {
		logger.Warning(err)
		return
	}
	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(card, s.ActivePort.Name)

	dbusError := s.setVBF(portConfig.Volume, portConfig.Balance, 0.0)
	if dbusError != nil {
		logger.Warning(dbusError)
	}

	s.setMute(GetConfigKeeper().Mute.MuteInput || !portConfig.Enabled)
	s.setReduceNoise(a.ReduceNoise)
}

func (a *Audio) refreshBluetoothOpts() {
	var btModeOpts strv.Strv

	sink := a.getDefaultSink()
	if sink == nil {
		return
	}
	// 云平台无card
	if sink.Card == math.MaxUint32 {
		return
	}
	card, err := a.cards.get(sink.Card)
	if err != nil {
		logger.Warning("failed to get card for sink:", err)
		return
	}

	if !isBluezAudio(card.Name) {
		return
	}

	for _, port := range card.Ports {
		if port.Name == sink.ActivePort.Name && port.Direction == pulse.DirectionSink {
			for _, profile := range port.Profiles {
				if !btModeOpts.Contains(profile.Name) {
					btModeOpts = append(btModeOpts, profile.Name)
				}
			}
			break
		}
	}

	if len(btModeOpts) > 0 {
		logger.Debugf("set bluetoothAudio mode opts %+v", btModeOpts)
		a.setPropBluetoothAudioModeOpts(btModeOpts)
	}
	logger.Debugf("set bluetoothAudio mode %s", card.ActiveProfile.Name)
	a.setPropBluetoothAudioMode(card.ActiveProfile.Name)
}

func (a *Audio) updateDefaultSink(sinkName string) {
	sinkInfo := a.getPhySinkInfoByName(sinkName)
	if sinkInfo == nil {
		logger.Warning("failed to get sinkInfo for name:", sinkName)
		a.setPropDefaultSink("/")
		a.defaultSink = nil
		return
	}
	a.moveSinkInputsToSink(nil)
	if a.defaultSink != nil {
		if a.defaultSink.Name == sinkInfo.Name {
			logger.Infof("defaultSink %s is the same as sinkName %s", a.defaultSink.Name, sinkName)
			return
		}
		// 暂停音乐播放器
		if a.defaultSink.pluggable {
			logger.Debug("default sink is pluggable, need to check auto pause")
			go pauseAllPlayers()
		}
	}
	// 部分机型defaultSink事件必add事件先上报。
	// 如果是这种情况触发SinkAdd逻辑再设置默认Sink
	a.mu.Lock()
	sink, ok := a.sinks[sinkInfo.Index]
	a.mu.Unlock()
	if !ok {
		logger.Warningf("default sink changed, but the sink not add yet. handel sinkadd %s first", sinkName)
		a.handleSinkAdded(sinkInfo.Index)
		a.mu.Lock()
		sink, ok = a.sinks[sinkInfo.Index]
		a.mu.Unlock()
		if !ok {
			logger.Warningf("default sink %v not found", sink)
			return
		}
	}
	if sink == nil {
		logger.Warningf("default sink %v not found", sink)
		return
	}

	logger.Debugf("updateDefaultSink #%d %s", sinkInfo.Index, sinkName)

	a.defaultSink = sink
	defaultSinkPath := sink.getPath()
	logger.Debugf("set default sink %s", defaultSinkPath)

	// 虚拟通道不需要恢复配置
	auto, _, _ := a.checkAutoSwitchOutputPort()
	if isPhysicalDevice(sinkName) && !auto {
		a.resumeSinkConfig(sink)
	}

	a.PropsMu.Lock()
	a.setPropDefaultSink(defaultSinkPath)
	a.PropsMu.Unlock()

	// 刷新蓝牙模式选项
	a.refreshBluetoothOpts()

	logger.Debug("set prop default sink:", defaultSinkPath, a.defaultSink.ActivePort.Name)
}

func (a *Audio) updateDefaultSource(sourceName string) {
	sourceInfo := a.getPhySourceInfoByName(sourceName)
	if sourceInfo == nil {
		logger.Warning("failed to get sourceInfo for name:", sourceName)
		a.setPropDefaultSource("/")
		a.defaultSource = nil
		return
	}
	if a.defaultSource != nil && a.defaultSource.Name == sourceName {
		logger.Infof("defaultSource %s is the same as sourceName %s", a.defaultSource.Name, sourceName)
		return
	}

	a.mu.Lock()
	source, ok := a.sources[sourceInfo.Index]
	a.mu.Unlock()
	if !ok {
		logger.Warningf("default source changed, but the source not add yet. handel sourceadd %s first", sourceName)
		a.handleSourceAdded(sourceInfo.Index)
		a.mu.Lock()
		source, ok = a.sources[sourceInfo.Index]
		a.mu.Unlock()
		if !ok {
			logger.Warningf("default source %s not found", sourceName)
			return
		}
	}

	a.mu.Lock()
	a.defaultSource = source
	a.mu.Unlock()

	defaultSourcePath := source.getPath()

	// 虚拟通道不需要恢复配置
	auto, _, _ := a.checkAutoSwitchInputPort()
	if isPhysicalDevice(sourceName) && !auto {
		a.resumeSourceConfig(source)
	}

	a.PropsMu.Lock()
	a.setPropDefaultSource(defaultSourcePath)
	a.PropsMu.Unlock()

	logger.Debug("set prop default source:", defaultSourcePath)
}

func (a *Audio) context() *pulse.Context {
	a.mu.Lock()
	c := a.ctx
	a.mu.Unlock()
	return c
}

func (a *Audio) getDefaultSource() *Source {
	a.mu.Lock()
	v := a.defaultSource
	a.mu.Unlock()
	return v
}

func (a *Audio) getDefaultSourceName() string {
	source := a.getDefaultSource()
	if source == nil {
		return ""
	}

	source.PropsMu.RLock()
	v := source.Name
	source.PropsMu.RUnlock()
	return v
}

func (a *Audio) getDefaultSink() *Sink {
	a.mu.Lock()
	v := a.defaultSink
	a.mu.Unlock()
	return v
}

func (a *Audio) getDefaultSinkName() string {
	sink := a.getDefaultSink()
	if sink == nil {
		return ""
	}

	sink.PropsMu.RLock()
	v := sink.Name
	sink.PropsMu.RUnlock()
	return v
}

func (a *Audio) getMasterNameFromVirtualDevice(device string) string {
	modules := a.ctx.GetModuleList()
	for _, module := range modules {
		if strings.Contains(module.Name, "echo-cancel") && strings.Contains(device, "echo-cancel") {
			re := regexp.MustCompile(`source_master=([^\s]+)`)
			match := re.FindStringSubmatch(module.Argument)
			if len(match) > 1 {
				return match[1]
			}
		} else if strings.Contains(module.Name, "module-remap-sink") && strings.Contains(device, "remap-sink") {
			re := regexp.MustCompile(`sink_master=([^\s]+)`)
			match := re.FindStringSubmatch(module.Argument)
			if len(match) > 1 {
				return match[1]
			}
		}
	}
	return ""
}

func (a *Audio) getSinkInfoByName(sinkName string) *pulse.Sink {
	for _, sinkInfo := range a.ctx.GetSinkList() {
		if sinkInfo.Name == sinkName {
			return sinkInfo
		}
	}
	return nil
}

func (a *Audio) getPhySinkInfoByName(sinkName string) *pulse.Sink {
	for _, sinkInfo := range a.ctx.GetSinkList() {
		if sinkInfo.Name == sinkName {
			if isPhysicalDevice(sinkName) {
				return sinkInfo
			} else {
				master := a.getMasterNameFromVirtualDevice(sinkName)
				if master == "" {
					logger.Warning("failed to get virtual device sinkInfo for name:", sinkName)
					return nil
				}
				sinkInfo = a.getSinkInfoByName(master)
				if sinkInfo != nil {
					logger.Infof("get master sink %v from %v", master, sinkName)
				}
			}
			return sinkInfo
		}
	}
	return nil
}

func (a *Audio) getSourceInfoByName(sourceName string) *pulse.Source {
	for _, sourceInfo := range a.ctx.GetSourceList() {
		if sourceInfo.Name == sourceName {
			return sourceInfo
		}
	}
	return nil
}
func (a *Audio) getPhySourceInfoByName(sourceName string) *pulse.Source {
	for _, sourceInfo := range a.ctx.GetSourceList() {
		if sourceInfo.Name == sourceName {
			if isPhysicalDevice(sourceName) {
				return sourceInfo
			} else {
				master := a.getMasterNameFromVirtualDevice(sourceName)
				if master == "" {
					logger.Warning("failed to get virtual device sinkInfo for name:", sourceName)
					return nil
				}
				sourceInfo = a.getSourceInfoByName(master)
				if sourceInfo != nil {
					logger.Infof("get master source %v from %v", master, sourceName)
				}
			}
			return sourceInfo
		}
	}
	return nil
}

func (a *Audio) NoRestartPulseAudio() *dbus.Error {
	a.noRestartPulseAudio = true
	return nil
}

func (a *Audio) StopAudioService() *dbus.Error {
	service := a.service
	systemd := systemd1.NewManager(service.Conn())
	systemd.InitSignalExt(a.sessionSigLoop, true)
	var audioServices strv.Strv
	if a.CurrentAudioServer == "pulseaudio" {
		audioServices = append(audioServices, pulseaudioServices...)
	} else {
		audioServices = append(audioServices, pipewireServices...)
	}
	var runningServices strv.Strv
	runningServices = append(runningServices, audioServices...)

	serviceMap := make(map[dbus.ObjectPath]string, 0)
	var serviceLock sync.Mutex
	done := make(chan bool, 1)
	systemd.ConnectJobRemoved(func(jobId uint32, jobPath dbus.ObjectPath, unit string, result string) {
		serviceLock.Lock()
		if _, ok := serviceMap[jobPath]; !ok {
			serviceLock.Unlock()
			return
		} else {
			runningServices, _ = runningServices.Delete(serviceMap[jobPath])
			logger.Warningf("service %s stopped, %v is running", serviceMap[jobPath], runningServices)
			if len(runningServices) == 0 {
				done <- true
			}
		}
		serviceLock.Unlock()
	})
	for _, svc := range audioServices {
		serviceLock.Lock()
		path, err := systemd.StopUnit(0, svc, "replace")
		if err != nil {
			logger.Warning(err)
			serviceLock.Unlock()
			continue
		}
		serviceMap[path] = svc
		serviceLock.Unlock()
	}
	for {
		select {
		case <-done:
			return nil
		case <-time.After(3 * time.Second):
			logger.Warningf("%v is still running, but timeout", runningServices)
			return dbusutil.ToError(fmt.Errorf("stop audio services timeout"))
		}
	}
}

func (a *Audio) isPortEnabled(cardId uint32, portName string, direction int32) bool {
	// 判断cardId 以及 portName的有效性
	a.mu.Lock()
	card, _ := a.cards.get(cardId)
	a.mu.Unlock()
	if card == nil {
		logger.Warningf("not found card #%d", cardId)
		return false
	}

	var err error
	_, err = card.Ports.Get(portName, int(direction))
	if err != nil {
		logger.Warningf("get port %s info failed: %v", portName, err)
		return false
	}

	_, portConfig := GetConfigKeeper().GetCardAndPortConfig(card, portName)
	return portConfig.Enabled
}

// 设置蓝牙模式
func (a *Audio) SetBluetoothAudioMode(mode string) *dbus.Error {
	logger.Infof("dbus call SetBluetoothAudioMode with mode %s", mode)

	if a.defaultSink == nil {
		logger.Warning("default sink is nil")
		return nil
	}
	card, err := a.cards.get(a.defaultSink.Card)
	if err != nil {
		logger.Warning(err)
	}
	if card == nil {
		logger.Warningf("card %v is nil", a.defaultSink.Card)
		return nil
	}
	if len(a.defaultSink.Ports) == 0 || a.defaultSink.ActivePort.Name == "" {
		logger.Warningf("card %s ports is nil", card.core.Name)
		return nil
	}

	if !isBluezAudio(card.core.Name) {
		err = fmt.Errorf("current card %s is not bluetooth audio device", card.core.Name)
		logger.Warning(err)
		return dbusutil.ToError(err)
	}
	if GetConfigKeeper().GetMode(card, a.defaultSink.ActivePort.Name) == mode {
		return nil
	}
	GetConfigKeeper().SetMode(card, a.defaultSink.ActivePort.Name, mode)
	card.core.SetProfile(mode)

	return dbusutil.ToError(fmt.Errorf("%s cannot support %s mode", card.core.Name, mode))
}

func (a *Audio) setEnableAutoSwitchPort(value bool) {
	a.PropsMu.Lock()
	a.enableAutoSwitchPort = value
	a.PropsMu.Unlock()

	if a.dccSoundDconfig != nil && a.enableAutoSwitchPort != a.controlCenterDeviceManager.Get() {
		a.controlCenterDeviceManager.Set(a.enableAutoSwitchPort)
		logger.Info("setEnableAutoSwitchPort set a.enableAutoSwitchPort to a.controlCenterDeviceManager. value : ", a.enableAutoSwitchPort)
	}
}

// 初始化 dsg 配置的属性
func (a *Audio) initDsgProp() error {
	val, err := a.audioDConfig.GetValueBool(dsgKeyAutoSwitchPort)
	if err != nil {
		logger.Warning(err)
	}
	a.setEnableAutoSwitchPort(val)

	pausePlayerValue, err := a.audioDConfig.GetValueBool(dsgkeyPausePlayer)
	if err != nil {
		logger.Warning(err)
	} else {
		logger.Info("pause player:", pausePlayerValue)
		a.PropsMu.Lock()
		a.PausePlayer = pausePlayerValue
		a.PropsMu.Unlock()
	}

	bluezModeFilterListValue, err := a.audioDConfig.GetValueStringList(dsgKeyBluezModeFilterList)
	if err != nil {
		logger.Warning(err)
	} else {
		bluezModeFilterList = bluezModeFilterList[:0]
		bluezModeFilterList = append(bluezModeFilterList, bluezModeFilterListValue...)
		logger.Info("bluez filter audio mode opts", bluezModeFilterList)
	}

	portFilterListValue, err := a.audioDConfig.GetValueStringList(dsgKeyPortFilterList)
	if err != nil {
		logger.Warning(err)
	} else {
		portFilterList = portFilterList[:0]
		portFilterList = append(portFilterList, portFilterListValue...)
		logger.Info("port filter list", portFilterList)
	}

	inputDefaultPrioritiesValue, err := a.audioDConfig.GetValueInt64List(dsgKeyInputDefaultPriorities)
	if err != nil {
		logger.Warning(err)
	} else {
		for i := range inputDefaultPrioritiesValue {
			inputDefaultPriorities = append(inputDefaultPriorities, int(inputDefaultPrioritiesValue[i]))
		}
		logger.Info("input default priority list", inputDefaultPriorities)
	}

	outputDefaultPrioritiesValue, err := a.audioDConfig.GetValueInt64List(dsgKeyOutputDefaultPriorities)
	if err != nil {
		logger.Warning(err)
	} else {
		for i := range outputDefaultPrioritiesValue {
			outputDefaultPriorities = append(outputDefaultPriorities, int(outputDefaultPrioritiesValue[i]))
		}
		logger.Info("output default priority list", outputDefaultPriorities)
	}

	getBluezModeDefault := func() {
		val, err := a.audioDConfig.GetValueString(dsgKeyBluezModeDefault)
		if err != nil {
			logger.Warning(err)
		} else {
			if val == bluezModeA2dp || val == bluezModeHandsfree || val == bluezModeHeadset {
				bluezModeDefault = val
				logger.Info("bluez default mode:", bluezModeDefault)
			}
		}
	}

	getMonoEnabled := func() {
		monoEnabled, err := a.audioDConfig.GetValueBool(dsgKeyMonoEnabled)
		if err != nil {
			logger.Warning(err)
		} else {
			a.setMono(monoEnabled)
			logger.Info("mono enabled:", a.Mono)
		}
	}
	getMonoEnabled()

	getReduceNoiseEnabled := func() {
		reduceNoiseEnabled, err := a.audioDConfig.GetValueBool(dsgKeyReduceNoiseEnabled)
		if err != nil {
			logger.Warning(err)
		} else {
			logger.Info("reduce noise enabled:", reduceNoiseEnabled)
			a.setReduceNoise(reduceNoiseEnabled)
		}
	}
	getReduceNoiseEnabled()

	a.audioDConfig.ConnectValueChanged(func(key string) {
		switch key {
		case dsgKeyAutoSwitchPort:
			value, _ := a.audioDConfig.GetValueBool(dsgKeyAutoSwitchPort)
			logger.Info("auto switch port:", value)
			a.setEnableAutoSwitchPort(value)
		case dsgKeyBluezModeDefault:
			getBluezModeDefault()
		case dsgkeyPausePlayer:
			pausePlayer, _ := a.audioDConfig.GetValueBool(dsgkeyPausePlayer)
			logger.Info("pausePlayer config:", pausePlayer)
			a.emitPropChangedPausePlayer(pausePlayer)
		case dsgKeyMonoEnabled:
			getMonoEnabled()
		case dsgKeyVolumeIncrease:
			a.handleVolumeIncrease()
		}
	})

	return nil
}

// 是否支持自动端口切换策略
func (a *Audio) canAutoSwitchPort() bool {
	a.PropsMu.RLock()
	defer a.PropsMu.RUnlock()
	return a.enableAutoSwitchPort
}

func (a *Audio) SetMono(enable bool) *dbus.Error {
	err := a.setMono(enable)
	return dbusutil.ToError(err)
}

func (a *Audio) unsetMono() {
	for _, sink := range a.sinks {
		if sink.Name == monoSinkName {
			a.ctx.UnloadModuleByName(monoSinkModuleName)
			break
		}
	}
}

func (a *Audio) setMono(enable bool) error {
	if a.Mono == enable {
		return nil
	}
	sink := a.getDefaultSink()
	if sink != nil {
		logger.Infof("set sink %v mono: %v", sink.Name, enable)
		sink.setMono(enable)
	}

	a.setPropMono(enable)
	err := a.audioDConfig.SetValue(dsgKeyMonoEnabled, enable)
	if err != nil {
		return dbusutil.ToError(errors.New("dconfig Cannot set value " + dsgKeyMonoEnabled))
	}
	return nil
}

func (a *Audio) handleVolumeIncrease() {
	volInc, err := a.audioDConfig.GetValueBool(dsgKeyVolumeIncrease)
	if err != nil {
		logger.Warning("dconfig get volume increase error:", err)
		return
	}
	if volInc {
		a.MaxUIVolume = increaseMaxVolume
	} else {
		a.MaxUIVolume = normalMaxVolume
	}
	gMaxUIVolume = a.MaxUIVolume
	err = a.emitPropChangedMaxUIVolume(a.MaxUIVolume)
	if err != nil {
		logger.Warning("changed Max UI Volume failed: ", err)
	} else {
		sink := a.getDefaultSink()
		if sink == nil {
			logger.Warning("default sink is nil")
			return
		}
		card, err := a.cards.get(sink.Card)
		if err != nil {
			logger.Warning(err)
			return
		}
		GetConfigKeeper().SetIncreaseVolume(card, sink.ActivePort.Name, volInc)
	}
}

func (a *Audio) LoadNullSinkModule() {
	a.context().LoadModule(nullSinkModuleName, "")
}

func (a *Audio) unsetReduceNoise() {
	logger.Info("unload moudle", reduceNoiseSourceModuleName)
	for _, s := range a.sources {
		if s.Name == reduceNoiseSourceName {
			a.context().UnloadModuleByName(reduceNoiseSourceModuleName)
		}
	}

}

func (a *Audio) setReduceNoise(enable bool) error {
	if a.ReduceNoise == enable {
		return nil
	}
	source := a.getDefaultSource()
	if source != nil {
		logger.Infof("set source <%s> reduce noise <%t>", source.Name, enable)
		source.setReduceNoise(enable)
	}
	a.setPropReduceNoise(enable)
	err := a.audioDConfig.SetValue(dsgKeyReduceNoiseEnabled, enable)
	if err != nil {
		return dbusutil.ToError(errors.New("dconfig Cannot set value " + dsgKeyMonoEnabled))
	}
	return nil
}

func (a *Audio) isModuleExist(name string) bool {
	modules := a.ctx.GetModuleList()
	for _, module := range modules {
		if module.Name == name {
			return true
		}
	}
	return false
}
