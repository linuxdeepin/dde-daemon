// SPDX-FileCopyrightText: 2025 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package dconfig

import (
	"errors"
	"reflect"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/linuxdeepin/go-lib/dbusutil"
)

type base struct {
	mu                sync.Mutex
	dc                *DConfig
	key               string
	notifyChangedList []func(val interface{})
}

func (b *base) bind(dc *DConfig, keyName string,
	getFn func() (interface{}, *dbus.Error)) {

	b.dc = dc
	b.key = keyName

	dc.ConnectConfigChanged(keyName, func(val interface{}) {
		b.mu.Lock()
		notifyChangedList := b.notifyChangedList
		b.mu.Unlock()

		if notifyChangedList != nil {
			currentVal, _ := getFn()
			for _, notifyChanged := range notifyChangedList {
				if notifyChanged != nil {
					notifyChanged(currentVal)
				}
			}
		}
	})
}

func (b *base) SetNotifyChangedFunc(fn func(val interface{})) {
	b.mu.Lock()
	b.notifyChangedList = append(b.notifyChangedList, fn)
	b.mu.Unlock()
}

func checkSet(setOk bool) *dbus.Error {
	if setOk {
		return nil
	}
	return dbusutil.ToError(errors.New("write failed"))
}

type Bool struct {
	base
}

func (b *Bool) Bind(dc *DConfig, key string) {
	b.bind(dc, key, b.GetValue)
}

func (b *Bool) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valBool, ok := val.(bool)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(b.Set(valBool))
	return
}

func (b *Bool) GetValue() (val interface{}, err *dbus.Error) {
	val = b.Get()
	return
}

func (b *Bool) Get() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	v, err := b.dc.GetValueBool(b.key)
	if err != nil {
		return false
	}
	return v
}

func (b *Bool) Set(val bool) bool {
	if b.Get() != val {
		b.mu.Lock()
		defer b.mu.Unlock()

		err := b.dc.SetValue(b.key, val)
		return err == nil
	}
	return true
}

func (*Bool) GetType() reflect.Type {
	return reflect.TypeOf(false)
}

type Int64 struct {
	base
}

func (i *Int64) Bind(dc *DConfig, key string) {
	i.bind(dc, key, i.GetValue)
}

func (i *Int64) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valInt64, ok := val.(int64)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(i.Set(valInt64))
	return
}

func (i *Int64) GetValue() (val interface{}, err *dbus.Error) {
	val = i.Get()
	return
}

func (*Int64) GetType() reflect.Type {
	return reflect.TypeOf(int64(0))
}

func (i *Int64) Set(val int64) bool {
	if i.Get() != val {
		i.mu.Lock()
		defer i.mu.Unlock()

		err := i.dc.SetValue(i.key, val)
		return err == nil
	}
	return true
}

func (i *Int64) Get() int64 {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, err := i.dc.GetValueInt64(i.key)
	if err != nil {
		return 0
	}
	return v
}

type Int32 struct {
	base
}

func (i *Int32) Bind(dc *DConfig, key string) {
	i.bind(dc, key, i.GetValue)
}

func (i *Int32) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valInt32, ok := val.(int32)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(i.Set(valInt32))
	return
}

func (i *Int32) GetValue() (val interface{}, err *dbus.Error) {
	val = i.Get()
	return
}

func (*Int32) GetType() reflect.Type {
	return reflect.TypeOf(int32(0))
}

func (i *Int32) Set(val int32) bool {
	current := i.Get()
	if current != val {
		i.mu.Lock()
		defer i.mu.Unlock()

		err := i.dc.SetValue(i.key, int64(val))
		return err == nil
	}
	return true
}

func (i *Int32) Get() int32 {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, err := i.dc.GetValueInt64(i.key)
	if err != nil {
		return 0
	}
	return int32(v)
}

type Uint32 struct {
	base
}

func (u *Uint32) Bind(dc *DConfig, key string) {
	u.bind(dc, key, u.GetValue)
}

func (u *Uint32) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valUint32, ok := val.(uint32)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(u.Set(valUint32))
	return
}

func (u *Uint32) GetValue() (val interface{}, err *dbus.Error) {
	val = u.Get()
	return
}

func (*Uint32) GetType() reflect.Type {
	return reflect.TypeOf(uint32(0))
}

func (u *Uint32) Set(val uint32) bool {
	current := u.Get()
	if current != val {
		u.mu.Lock()
		defer u.mu.Unlock()

		err := u.dc.SetValue(u.key, int64(val))
		return err == nil
	}
	return true
}

func (u *Uint32) Get() uint32 {
	u.mu.Lock()
	defer u.mu.Unlock()

	v, err := u.dc.GetValueInt64(u.key)
	if err != nil {
		return 0
	}
	return uint32(v)
}

type Int struct {
	base
}

func (i *Int) Bind(dc *DConfig, key string) {
	i.bind(dc, key, i.GetValue)
}

func (i *Int) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valInt, ok := val.(int)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(i.Set(valInt))
	return
}

func (i *Int) GetValue() (val interface{}, err *dbus.Error) {
	val = i.Get()
	return
}

func (*Int) GetType() reflect.Type {
	return reflect.TypeOf(int(0))
}

func (i *Int) Set(val int) bool {
	current := i.Get()
	if current != val {
		i.mu.Lock()
		defer i.mu.Unlock()

		err := i.dc.SetValue(i.key, int64(val))
		return err == nil
	}
	return true
}

func (i *Int) Get() int {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, err := i.dc.GetValueInt64(i.key)
	if err != nil {
		return 0
	}
	return int(v)
}

type Float64 struct {
	base
}

func (f *Float64) Bind(dc *DConfig, key string) {
	f.bind(dc, key, f.GetValue)
}

func (f *Float64) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valFloat64, ok := val.(float64)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(f.Set(valFloat64))
	return
}

func (f *Float64) GetValue() (val interface{}, err *dbus.Error) {
	val = f.Get()
	return
}

func (*Float64) GetType() reflect.Type {
	return reflect.TypeOf(float64(0))
}

func (f *Float64) Get() float64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	v, err := f.dc.GetValueFloat64(f.key)
	if err != nil {
		return 0
	}
	return v
}

func (f *Float64) Set(val float64) bool {
	if f.Get() != val {
		f.mu.Lock()
		defer f.mu.Unlock()

		err := f.dc.SetValue(f.key, val)
		return err == nil
	}
	return true
}

type String struct {
	base
}

func (s *String) Bind(dc *DConfig, key string) {
	s.bind(dc, key, s.GetValue)
}

func (s *String) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valString, ok := val.(string)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(s.Set(valString))
	return
}

func (s *String) GetValue() (val interface{}, err *dbus.Error) {
	val = s.Get()
	return
}

func (*String) GetType() reflect.Type {
	return reflect.TypeOf("")
}

func (s *String) Get() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.dc.GetValueString(s.key)
	if err != nil {
		return ""
	}
	return v
}

func (s *String) Set(val string) bool {
	if s.Get() != val {
		s.mu.Lock()
		defer s.mu.Unlock()

		err := s.dc.SetValue(s.key, val)
		return err == nil
	}
	return true
}

type StringList struct {
	base
}

func (s *StringList) Bind(dc *DConfig, key string) {
	s.bind(dc, key, s.GetValue)
}

func (s *StringList) SetValue(val interface{}) (changed bool, err *dbus.Error) {
	valStringList, ok := val.([]string)
	if !ok {
		return false, dbusutil.ToError(errors.New("invalid value type"))
	}
	err = checkSet(s.Set(valStringList))
	return
}

func (s *StringList) GetValue() (val interface{}, err *dbus.Error) {
	val = s.Get()
	return
}

func (*StringList) GetType() reflect.Type {
	return reflect.TypeOf([]string{})
}

func (s *StringList) Get() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.dc.GetValueStringList(s.key)
	if err != nil {
		return nil
	}
	return v
}

func (s *StringList) Set(val []string) bool {
	if !stringListEqual(s.Get(), val) {
		s.mu.Lock()
		defer s.mu.Unlock()

		err := s.dc.SetValue(s.key, val)
		return err == nil
	}
	return true
}

func stringListEqual(a []string, b []string) bool {
	an := len(a)
	bn := len(b)
	if an != bn {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}
