// Code generated by "dbusutil-gen -type Manager manager.go"; DO NOT EDIT.

package power_manager

func (v *Manager) setPropVirtualMachineName(value string) (changed bool) {
	if v.VirtualMachineName != value {
		v.VirtualMachineName = value
		v.emitPropChangedVirtualMachineName(value)
		return true
	}
	return false
}

func (v *Manager) emitPropChangedVirtualMachineName(value string) error {
	return v.service.EmitPropertyChanged(v, "VirtualMachineName", value)
}