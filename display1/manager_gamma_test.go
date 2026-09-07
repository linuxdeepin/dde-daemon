// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package display1

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linuxdeepin/go-lib/dbusutil"
	"github.com/linuxdeepin/go-x11-client/ext/randr"
)

func TestDetectGammaSupport(t *testing.T) {
	const (
		intelVGA    = "00:02.0 VGA compatible controller: Intel Corporation UHD Graphics 620"
		loongsonVGA = "00:06.0 VGA compatible controller: Loongson Technology 7A1000"
		tesla3D     = "01:00.0 3D controller: NVIDIA Corporation Tesla T4"
		amdDisplay  = "03:00.0 Display controller: Advanced Micro Devices, Inc. [AMD/ATI] Device 744c"
		loongson3D  = "01:00.0 3D controller: Loongson Technology Test Device"
	)
	blacklist := []string{"Loongson"}
	tests := []struct {
		name           string
		lspciOut       string
		probeResult    bool
		wantSupport    bool
		wantProbeCalls int
	}{
		{"intel VGA preserves existing support", intelVGA, false, true, 0},
		{"blacklisted VGA", loongsonVGA, true, false, 0},
		// A positive global gamma query does not establish which PCI GPU owns the output.
		{"blacklisted VGA plus headless Tesla", loongsonVGA + "\n" + tesla3D, true, false, 0},
		{"headless Tesla before blacklisted VGA", tesla3D + "\n" + loongsonVGA, true, false, 0},
		{"blacklisted VGA plus display controller", loongsonVGA + "\n" + amdDisplay, true, false, 0},
		{"usable VGA after blacklisted VGA", loongsonVGA + "\n" + intelVGA, false, true, 0},
		{"usable VGA before blacklisted VGA", intelVGA + "\n" + loongsonVGA, false, true, 0},
		{"Tesla without gamma support", tesla3D, false, false, 1},
		{"3D controller with output gamma support", tesla3D, true, true, 1},
		{"display controller with output gamma support", amdDisplay, true, true, 1},
		{"empty PCI list with platform GPU gamma", "", true, true, 1},
		{"empty PCI list without gamma", "", false, false, 1},
		{"no PCI displays with platform GPU gamma",
			"00:00.0 PCI bridge: Phytium Technology Co., Ltd. Device dc01\n" +
				"02:00.0 Network controller: Realtek Semiconductor Co., Ltd. RTL8852BE", true, true, 1},
		{"blacklisted 3D device", loongson3D, true, false, 0},
		{"blacklisted 3D device plus Tesla", loongson3D + "\n" + tesla3D, true, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probeCalls := 0
			got := detectGammaSupport(func() (string, error) {
				return tt.lspciOut, nil
			}, blacklist, func() bool {
				probeCalls++
				return tt.probeResult
			})
			if got != tt.wantSupport {
				t.Errorf("detectGammaSupport() = %v, want %v", got, tt.wantSupport)
			}
			if probeCalls != tt.wantProbeCalls {
				t.Errorf("gamma probe called %d times, want %d", probeCalls, tt.wantProbeCalls)
			}
		})
	}
}

func TestDetectGammaSupportPCIEnumeration(t *testing.T) {
	tests := []struct {
		name           string
		script         string
		wantSupport    bool
		wantProbeCalls int
	}{
		{name: "lspci missing"},
		{name: "lspci exits unsuccessfully", script: "#!/bin/sh\nexit 1\n"},
		{
			name: "partial VGA output from a failed scan",
			script: "#!/bin/sh\nprintf '%s\\n' " +
				"'00:02.0 VGA compatible controller: Intel Corporation UHD Graphics 620'\nexit 1\n",
		},
		{
			name:           "successful empty scan permits platform GPU probing",
			script:         "#!/bin/sh\nexit 0\n",
			wantSupport:    true,
			wantProbeCalls: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandDir := t.TempDir()
			if tt.script != "" {
				if err := os.WriteFile(filepath.Join(commandDir, "lspci"), []byte(tt.script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", commandDir)
			probeCalls := 0
			got := detectGammaSupport(getLspci, []string{"Loongson"}, func() bool {
				probeCalls++
				return true
			})
			if got != tt.wantSupport || probeCalls != tt.wantProbeCalls {
				t.Fatalf("support = %v, gamma probe calls = %d; want %v, %d",
					got, probeCalls, tt.wantSupport, tt.wantProbeCalls)
			}
		})
	}
}

func TestDetectGammaSupportPCIScanFailureAfterBlacklist(t *testing.T) {
	commandDir := t.TempDir()
	t.Setenv("PATH", commandDir)
	for _, script := range []string{
		"#!/bin/sh\nprintf '%s\\n' '00:06.0 VGA compatible controller: Loongson Technology 7A1000'\n",
		"#!/bin/sh\nexit 1\n",
	} {
		if err := os.WriteFile(filepath.Join(commandDir, "lspci"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		if detectGammaSupport(getLspci, []string{"Loongson"}, func() bool {
			t.Error("gamma probing must not bypass the blacklist after a failed PCI scan")
			return true
		}) {
			t.Fatal("an unsupported display GPU became supported")
		}
	}
}

func TestProbeRandrGammaSupport(t *testing.T) {
	tests := []struct {
		name        string
		resources   randr.GetScreenResourcesReply
		outputs     map[randr.Output]randr.GetOutputInfoReply
		gammaSizes  map[randr.Crtc]uint16
		gammaError  randr.Crtc
		wantSupport bool
	}{
		{
			name: "connected output with gamma",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionConnected, Crtc: 10},
			},
			gammaSizes:  map[randr.Crtc]uint16{10: 256},
			wantSupport: true,
		},
		{
			name: "headless CRTC cannot enable gamma",
			resources: randr.GetScreenResourcesReply{
				Crtcs: []randr.Crtc{10},
			},
			gammaSizes: map[randr.Crtc]uint16{10: 256},
		},
		{
			name: "unused CRTC cannot override an active output without gamma",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10, 11},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionConnected, Crtc: 10, Crtcs: []randr.Crtc{10, 11}},
			},
			gammaSizes: map[randr.Crtc]uint16{11: 256},
		},
		{
			name: "unused CRTC cannot override an active gamma query failure",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10, 11},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionConnected, Crtc: 10, Crtcs: []randr.Crtc{10, 11}},
			},
			gammaSizes: map[randr.Crtc]uint16{10: 256, 11: 256},
			gammaError: 10,
		},
		{
			name: "disconnected output with a compatible CRTC",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionDisconnected, Crtcs: []randr.Crtc{10}},
			},
			gammaSizes:  map[randr.Crtc]uint16{10: 256},
			wantSupport: true,
		},
		{
			name: "connected output awaiting CRTC assignment",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionConnected, Crtcs: []randr.Crtc{10}},
			},
			gammaSizes:  map[randr.Crtc]uint16{10: 256},
			wantSupport: true,
		},
		{
			name: "unrelated CRTC cannot cover an inactive output without gamma",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10, 11},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionDisconnected, Crtcs: []randr.Crtc{10}},
			},
			gammaSizes: map[randr.Crtc]uint16{11: 256},
		},
		{
			name: "output query failure cannot establish scanout capability",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
				Crtcs:   []randr.Crtc{10},
			},
			gammaSizes: map[randr.Crtc]uint16{10: 256},
		},
		{
			name: "failed output status cannot enable the inactive CRTC fallback",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1, 2},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionDisconnected, Crtcs: []randr.Crtc{10}},
				2: {Status: randr.StatusInvalidConfigTime},
			},
			gammaSizes: map[randr.Crtc]uint16{10: 256},
		},
		{
			name: "failed output status cannot establish active gamma support",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Status: randr.StatusInvalidConfigTime, Connection: randr.ConnectionConnected, Crtc: 10},
			},
			gammaSizes: map[randr.Crtc]uint16{10: 256},
		},
		{
			name: "valid active output can establish support after another output query fails",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1, 2},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Status: randr.StatusInvalidConfigTime},
				2: {Connection: randr.ConnectionConnected, Crtc: 11},
			},
			gammaSizes:  map[randr.Crtc]uint16{11: 256},
			wantSupport: true,
		},
		{
			name: "second connected output supports gamma",
			resources: randr.GetScreenResourcesReply{
				Outputs: []randr.Output{1, 2},
				Crtcs:   []randr.Crtc{10, 11},
			},
			outputs: map[randr.Output]randr.GetOutputInfoReply{
				1: {Connection: randr.ConnectionConnected, Crtc: 10},
				2: {Connection: randr.ConnectionConnected, Crtc: 11},
			},
			gammaSizes:  map[randr.Crtc]uint16{11: 256},
			wantSupport: true,
		},
		{name: "no resources"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeRandrGammaSupport(&tt.resources,
				func(output randr.Output) (*randr.GetOutputInfoReply, error) {
					info, ok := tt.outputs[output]
					if !ok {
						return nil, errors.New("output query failed")
					}
					return &info, nil
				},
				func(crtc randr.Crtc) (uint16, error) {
					if crtc == tt.gammaError {
						return 0, errors.New("gamma query failed")
					}
					return tt.gammaSizes[crtc], nil
				})
			if got != tt.wantSupport {
				t.Errorf("probeRandrGammaSupport() = %v, want %v", got, tt.wantSupport)
			}
		})
	}
}

func startGammaSupportTestWatcher(t *testing.T, detect func() bool) *Manager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		// No object is exported, so property updates do not contact a D-Bus service.
		service:                   dbusutil.NewService(nil),
		gammaSupportUpdates:       make(chan struct{}, 1),
		gammaSupportCtx:           ctx,
		gammaSupportCancel:        cancel,
		ColorTemperatureEnabled:   true,
		ColorTemperatureMode:      ColorTemperatureModeCustom,
		ColorTemperatureManual:    4200,
		CustomColorTempTimePeriod: "20:00-07:00",
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.watchGammaSupport(detect)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("gamma support watcher did not stop")
		}
	})
	return m
}

func waitForGammaSupport(t *testing.T, m *Manager, want bool) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		methodValue, err := m.SupportSetColorTemperature()
		if err != nil {
			t.Fatal(err)
		}
		m.PropsMu.RLock()
		propertyValue := m.SupportColorTemperature
		m.PropsMu.RUnlock()
		if methodValue == want && propertyValue == want {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatalf("gamma support method = %v, property = %v, want %v", methodValue, propertyValue, want)
		}
	}
}

func TestGammaSupportRefreshAfterOutputChanges(t *testing.T) {
	var gammaOutputEnabled atomic.Bool
	probe := func() bool {
		outputs := map[randr.Output]randr.GetOutputInfoReply{
			1: {Connection: randr.ConnectionConnected, Crtc: 10, Crtcs: []randr.Crtc{10}},
			2: {Connection: randr.ConnectionConnected, Crtcs: []randr.Crtc{11}},
		}
		if gammaOutputEnabled.Load() {
			info := outputs[2]
			info.Crtc = 11
			outputs[2] = info
		}
		return detectGammaSupport(func() (string, error) {
			return "", nil
		}, nil, func() bool {
			return probeRandrGammaSupport(&randr.GetScreenResourcesReply{Outputs: []randr.Output{1, 2}},
				func(output randr.Output) (*randr.GetOutputInfoReply, error) {
					info := outputs[output]
					return &info, nil
				},
				func(crtc randr.Crtc) (uint16, error) {
					if crtc == 11 {
						return 256, nil
					}
					return 0, nil
				})
		})
	}
	m := startGammaSupportTestWatcher(t, probe)
	m.updateGammaSupport(probe())
	waitForGammaSupport(t, m, false)
	for _, enabled := range []bool{true, false, true} {
		gammaOutputEnabled.Store(enabled)
		m.queueGammaSupportUpdate()
		waitForGammaSupport(t, m, enabled)
	}
	if !m.ColorTemperatureEnabled || m.ColorTemperatureMode != ColorTemperatureModeCustom ||
		m.ColorTemperatureManual != 4200 || m.CustomColorTempTimePeriod != "20:00-07:00" {
		t.Fatal("capability refresh changed the user's color temperature settings")
	}
}

func TestGammaSupportRefreshKeepsChangesDuringProbe(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	var calls atomic.Int32
	m := startGammaSupportTestWatcher(t, func() bool {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return false
		}
		return true
	})
	// Release the blocked probe before the watcher's cleanup if an assertion fails.
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	m.queueGammaSupportUpdate()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("gamma probe did not start")
	}
	for i := 0; i < 100; i++ {
		m.queueGammaSupportUpdate()
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("gamma probes ran concurrently: %d probes started", got)
	}
	releaseOnce.Do(func() { close(release) })
	waitForGammaSupport(t, m, true)
	if got := calls.Load(); got != 2 {
		t.Fatalf("queued changes triggered %d probes, want 2", got)
	}
}

func TestGammaSupportMethodMatchesProperty(t *testing.T) {
	for _, inVM := range []bool{false, true} {
		m := &Manager{service: dbusutil.NewService(nil), isVM: inVM}
		for _, supported := range []bool{false, true, false, true} {
			m.updateGammaSupport(supported)
			got, err := m.SupportSetColorTemperature()
			want := !inVM && supported
			if err != nil || got != want || m.SupportColorTemperature != want {
				t.Fatalf("inVM=%v gamma=%v: method=%v property=%v err=%v, want %v",
					inVM, supported, got, m.SupportColorTemperature, err, want)
			}
		}
	}
}
