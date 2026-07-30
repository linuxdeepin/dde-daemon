// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package brightness

import (
	"math"
	"sync"
	"time"
)

const (
	// transitionStepMs 是状态机 tick 间隔，兼顾人眼连续感与 D-Bus 往返延迟。
	transitionStepMs = 50 * time.Millisecond
	// hwBrightnessStep 是硬件最小可感知变化量，约等于 8-bit 背光的一级 (1/256)。
	hwBrightnessStep = 0.004
	// defaultRampMs 是每次渐变的总时长。
	defaultRampMs = 800 * time.Millisecond
)

type transitionRun struct {
	cancel     chan struct{}
	done       chan struct{}
	cancelOnce sync.Once
	stopping   bool
}

// BrightnessTransition 串行执行亮度渐变。同一时刻最多存在一个 worker；新的自动
// 目标通过 Update 重定向当前事务，Stop 会等待旧事务彻底退出。
type BrightnessTransition struct {
	mu         sync.Mutex
	from       float64
	target     float64
	current    float64
	startedAt  time.Time
	revision   uint64
	run        *transitionRun
	setter     func(float64) error
	onComplete func(value float64)
}

func NewBrightnessTransition(setter func(float64) error) *BrightnessTransition {
	return &BrightnessTransition{setter: setter}
}

// SetOnComplete 设置渐变完成后的回调。渐变正常完成时以最终亮度值调用；
// Stop 取消或 Update 重定向时不调用。
func (t *BrightnessTransition) SetOnComplete(fn func(value float64)) {
	t.mu.Lock()
	t.onComplete = fn
	t.mu.Unlock()
}

// IsRunning 返回当前是否仍有渐变事务（包括正在同步停止的事务）。
func (t *BrightnessTransition) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.run != nil
}

// Run 启动渐变。如果已有自动渐变，则等价于 Update，保持单 worker 串行执行。
func (t *BrightnessTransition) Run(from, to float64) {
	for {
		t.mu.Lock()
		if t.run != nil {
			if t.run.stopping {
				done := t.run.done
				t.mu.Unlock()
				<-done
				continue
			}
			current := t.current
			t.from = current
			t.target = to
			t.startedAt = time.Now()
			t.revision++
			t.mu.Unlock()
			return
		}

		if math.Abs(from-to) < hwBrightnessStep {
			setter := t.setter
			t.mu.Unlock()
			if setter != nil {
				if err := setter(to); err != nil {
					logger.Warningf("[AutoBrightness] failed to set brightness directly: %v", err)
				}
			}
			return
		}

		run := &transitionRun{
			cancel: make(chan struct{}),
			done:   make(chan struct{}),
		}
		t.from = from
		t.target = to
		t.current = from
		t.startedAt = time.Now()
		t.revision++
		t.run = run
		setter := t.setter
		t.mu.Unlock()

		go t.loop(run, setter)
		return
	}
}

// Update 重定向正在运行的自动渐变，并从事务当前值重新计时。
func (t *BrightnessTransition) Update(to float64) bool {
	t.mu.Lock()
	if t.run == nil || t.run.stopping {
		t.mu.Unlock()
		return false
	}
	current := t.current
	t.from = current
	t.target = to
	t.startedAt = time.Now()
	t.revision++
	t.mu.Unlock()

	return true
}

// Stop 同步取消当前事务。返回后旧事务不会再写入亮度。
func (t *BrightnessTransition) Stop() {
	t.mu.Lock()
	run := t.run
	if run == nil {
		t.mu.Unlock()
		return
	}
	run.stopping = true
	run.cancelOnce.Do(func() {
		close(run.cancel)
	})
	done := run.done
	t.mu.Unlock()

	<-done
}

// loop 是唯一的渐变 worker。
func (t *BrightnessTransition) loop(run *transitionRun, setter func(float64) error) {
	defer func() {
		t.mu.Lock()
		if t.run == run {
			t.run = nil
		}
		t.mu.Unlock()
		close(run.done)
	}()

	ticker := time.NewTicker(transitionStepMs)
	defer ticker.Stop()
	step := 0

	for {
		select {
		case <-run.cancel:
			return
		case <-ticker.C:
		}

		t.mu.Lock()
		if t.run != run || run.stopping {
			t.mu.Unlock()
			return
		}
		from := t.from
		target := t.target
		previous := t.current
		startedAt := t.startedAt
		revision := t.revision
		t.mu.Unlock()

		progress := float64(time.Since(startedAt)) / float64(defaultRampMs)
		if progress >= 1 {
			progress = 1
		}
		eased := progress * progress * (3 - 2*progress)
		value := from + (target-from)*eased
		if progress < 1 && math.Abs(value-previous) < hwBrightnessStep {
			continue
		}

		step++
		if setter == nil {
			logger.Warning("[AutoBrightness] brightness transition setter is nil")
			return
		}
		if err := setter(value); err != nil {
			logger.Warningf("[AutoBrightness] brightness transition failed at step %d: %v", step, err)
			return
		}

		t.mu.Lock()
		if t.run != run || run.stopping {
			t.mu.Unlock()
			return
		}
		t.current = value
		if revision != t.revision {
			// Update 发生在硬件写入期间；以真正写入的值作为新事务起点。
			t.from = value
			t.startedAt = time.Now()
			t.mu.Unlock()
			continue
		}
		completed := progress >= 1
		t.mu.Unlock()

		if completed {
			t.mu.Lock()
			fn := t.onComplete
			t.mu.Unlock()
			if fn != nil {
				fn(target)
			}
			return
		}
	}
}
