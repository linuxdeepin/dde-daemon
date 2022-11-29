// Code generated by "dbusutil-gen -type Manager manager.go"; DO NOT EDIT.

package systeminfo

func (v *Manager) setPropMemorySize(value uint64) (changed bool) {
	if v.MemorySize != value {
		v.MemorySize = value
		v.emitPropChangedMemorySize(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedMemorySize(value uint64) error {
	return v.service.EmitPropertyChanged(v, "MemorySize", value)
}

func (v *Manager) setPropMemorySizeHuman(value string) (changed bool) {
	if v.MemorySizeHuman != value {
		v.MemorySizeHuman = value
		v.emitPropChangedMemorySizeHuman(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedMemorySizeHuman(value string) error {
	return v.service.EmitPropertyChanged(v, "MemorySizeHuman", value)
}

func (v *Manager) setPropCurrentSpeed(value uint64) (changed bool) {
	if v.CurrentSpeed != value {
		v.CurrentSpeed = value
		v.emitPropChangedCurrentSpeed(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedCurrentSpeed(value uint64) error {
	return v.service.EmitPropertyChanged(v, "CurrentSpeed", value)
}
