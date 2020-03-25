package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	maxMemAlloc     = 200 // 200Mb
	maxMemLeakCount = 3
)

func (s *SessionDaemon) memLeakLoop() {
	var count = 0
	var duration = time.Minute * 10
	var timer = time.NewTimer(duration)
	for {
		select {
		case <-timer.C:
			if !isMemLeak() {
				timer.Reset(duration)
				break
			}

			if count > maxMemLeakCount {
				s.CallTrace(3, 120)
				time.Sleep(time.Second * 150)
				logger.Errorf("[IMPORTANT] Memory leak > %d, self terminate\n", count)
				count = 0
				os.Exit(-1)
			}

			count++
			logger.Error("Found memory leak, the count is: ", count)
			runtime.GC()
			debug.FreeOSMemory()
			timer.Reset(duration)
		}
	}
}

// check memory alloc
func isMemLeak() bool {
	pid := os.Getpid()
	fr, err := os.Open(fmt.Sprintf("/proc/%v/status", pid))
	if err != nil {
		logger.Warning("Failed to open pid status file:", err)
		return false
	}
	defer fr.Close()

	var scanner = bufio.NewScanner(fr)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || !strings.Contains(line, "VmRSS") {
			continue
		}
		items := strings.Split(line, " ")
		if len(items) < 3 {
			continue
		}
		s, _ := strconv.ParseInt(items[len(items)-2], 10, 64)
		logger.Debug("VmRSS:", s)
		return (s / 1024) > maxMemAlloc
	}

	return false
}
