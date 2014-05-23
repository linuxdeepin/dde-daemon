// This file is automatically generated, please don't edit manully.
package main

import (
	"fmt"
)

// Get key type
func getSettingGsmKeyType(key string) (t ktype) {
	switch key {
	default:
		t = ktypeUnknown
	case NM_SETTING_GSM_NUMBER:
		t = ktypeString
	case NM_SETTING_GSM_USERNAME:
		t = ktypeString
	case NM_SETTING_GSM_PASSWORD_FLAGS:
		t = ktypeUint32
	case NM_SETTING_GSM_PASSWORD:
		t = ktypeString
	case NM_SETTING_GSM_APN:
		t = ktypeString
	case NM_SETTING_GSM_NETWORK_ID:
		t = ktypeString
	case NM_SETTING_GSM_NETWORK_TYPE:
		t = ktypeInt32
	case NM_SETTING_GSM_ALLOWED_BANDS:
		t = ktypeBoolean
	case NM_SETTING_GSM_HOME_ONLY:
		t = ktypeBoolean
	case NM_SETTING_GSM_PIN:
		t = ktypeString
	case NM_SETTING_GSM_PIN_FLAGS:
		t = ktypeUint32
	}
	return
}

// Check is key in current setting field
func isKeyInSettingGsm(key string) bool {
	switch key {
	case NM_SETTING_GSM_NUMBER:
		return true
	case NM_SETTING_GSM_USERNAME:
		return true
	case NM_SETTING_GSM_PASSWORD_FLAGS:
		return true
	case NM_SETTING_GSM_PASSWORD:
		return true
	case NM_SETTING_GSM_APN:
		return true
	case NM_SETTING_GSM_NETWORK_ID:
		return true
	case NM_SETTING_GSM_NETWORK_TYPE:
		return true
	case NM_SETTING_GSM_ALLOWED_BANDS:
		return true
	case NM_SETTING_GSM_HOME_ONLY:
		return true
	case NM_SETTING_GSM_PIN:
		return true
	case NM_SETTING_GSM_PIN_FLAGS:
		return true
	}
	return false
}

// Get key's default value
func getSettingGsmDefaultValue(key string) (value interface{}) {
	switch key {
	default:
		logger.Error("invalid key:", key)
	case NM_SETTING_GSM_NUMBER:
		value = ""
	case NM_SETTING_GSM_USERNAME:
		value = ""
	case NM_SETTING_GSM_PASSWORD_FLAGS:
		value = 0
	case NM_SETTING_GSM_PASSWORD:
		value = ""
	case NM_SETTING_GSM_APN:
		value = ""
	case NM_SETTING_GSM_NETWORK_ID:
		value = ""
	case NM_SETTING_GSM_NETWORK_TYPE:
		value = -1
	case NM_SETTING_GSM_ALLOWED_BANDS:
		value = 1
	case NM_SETTING_GSM_HOME_ONLY:
		value = false
	case NM_SETTING_GSM_PIN:
		value = ""
	case NM_SETTING_GSM_PIN_FLAGS:
		value = 0
	}
	return
}

// Get JSON value generally
func generalGetSettingGsmKeyJSON(data connectionData, key string) (value string) {
	switch key {
	default:
		logger.Error("generalGetSettingGsmKeyJSON: invalide key", key)
	case NM_SETTING_GSM_NUMBER:
		value = getSettingGsmNumberJSON(data)
	case NM_SETTING_GSM_USERNAME:
		value = getSettingGsmUsernameJSON(data)
	case NM_SETTING_GSM_PASSWORD_FLAGS:
		value = getSettingGsmPasswordFlagsJSON(data)
	case NM_SETTING_GSM_PASSWORD:
		value = getSettingGsmPasswordJSON(data)
	case NM_SETTING_GSM_APN:
		value = getSettingGsmApnJSON(data)
	case NM_SETTING_GSM_NETWORK_ID:
		value = getSettingGsmNetworkIdJSON(data)
	case NM_SETTING_GSM_NETWORK_TYPE:
		value = getSettingGsmNetworkTypeJSON(data)
	case NM_SETTING_GSM_ALLOWED_BANDS:
		value = getSettingGsmAllowedBandsJSON(data)
	case NM_SETTING_GSM_HOME_ONLY:
		value = getSettingGsmHomeOnlyJSON(data)
	case NM_SETTING_GSM_PIN:
		value = getSettingGsmPinJSON(data)
	case NM_SETTING_GSM_PIN_FLAGS:
		value = getSettingGsmPinFlagsJSON(data)
	}
	return
}

// Set JSON value generally
func generalSetSettingGsmKeyJSON(data connectionData, key, valueJSON string) (err error) {
	switch key {
	default:
		logger.Error("generalSetSettingGsmKeyJSON: invalide key", key)
	case NM_SETTING_GSM_NUMBER:
		err = setSettingGsmNumberJSON(data, valueJSON)
	case NM_SETTING_GSM_USERNAME:
		err = setSettingGsmUsernameJSON(data, valueJSON)
	case NM_SETTING_GSM_PASSWORD_FLAGS:
		err = setSettingGsmPasswordFlagsJSON(data, valueJSON)
	case NM_SETTING_GSM_PASSWORD:
		err = setSettingGsmPasswordJSON(data, valueJSON)
	case NM_SETTING_GSM_APN:
		err = setSettingGsmApnJSON(data, valueJSON)
	case NM_SETTING_GSM_NETWORK_ID:
		err = setSettingGsmNetworkIdJSON(data, valueJSON)
	case NM_SETTING_GSM_NETWORK_TYPE:
		err = setSettingGsmNetworkTypeJSON(data, valueJSON)
	case NM_SETTING_GSM_ALLOWED_BANDS:
		err = setSettingGsmAllowedBandsJSON(data, valueJSON)
	case NM_SETTING_GSM_HOME_ONLY:
		err = setSettingGsmHomeOnlyJSON(data, valueJSON)
	case NM_SETTING_GSM_PIN:
		err = setSettingGsmPinJSON(data, valueJSON)
	case NM_SETTING_GSM_PIN_FLAGS:
		err = setSettingGsmPinFlagsJSON(data, valueJSON)
	}
	return
}

// Check if key exists
func isSettingGsmNumberExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER)
}
func isSettingGsmUsernameExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME)
}
func isSettingGsmPasswordFlagsExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS)
}
func isSettingGsmPasswordExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD)
}
func isSettingGsmApnExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN)
}
func isSettingGsmNetworkIdExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID)
}
func isSettingGsmNetworkTypeExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE)
}
func isSettingGsmAllowedBandsExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS)
}
func isSettingGsmHomeOnlyExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY)
}
func isSettingGsmPinExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN)
}
func isSettingGsmPinFlagsExists(data connectionData) bool {
	return isSettingKeyExists(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS)
}

// Ensure field and key exists and not empty
func ensureFieldSettingGsmExists(data connectionData, errs fieldErrors, relatedKey string) {
	if !isSettingFieldExists(data, NM_SETTING_GSM_SETTING_NAME) {
		rememberError(errs, relatedKey, NM_SETTING_GSM_SETTING_NAME, fmt.Sprintf(NM_KEY_ERROR_MISSING_SECTION, NM_SETTING_GSM_SETTING_NAME))
	}
	fieldData, _ := data[NM_SETTING_GSM_SETTING_NAME]
	if len(fieldData) == 0 {
		rememberError(errs, relatedKey, NM_SETTING_GSM_SETTING_NAME, fmt.Sprintf(NM_KEY_ERROR_EMPTY_SECTION, NM_SETTING_GSM_SETTING_NAME))
	}
}
func ensureSettingGsmNumberNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmNumberExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingGsmNumber(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingGsmUsernameNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmUsernameExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingGsmUsername(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingGsmPasswordFlagsNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmPasswordFlagsExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS, NM_KEY_ERROR_MISSING_VALUE)
	}
}
func ensureSettingGsmPasswordNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmPasswordExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingGsmPassword(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingGsmApnNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmApnExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingGsmApn(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingGsmNetworkIdNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmNetworkIdExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingGsmNetworkId(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingGsmNetworkTypeNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmNetworkTypeExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE, NM_KEY_ERROR_MISSING_VALUE)
	}
}
func ensureSettingGsmAllowedBandsNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmAllowedBandsExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS, NM_KEY_ERROR_MISSING_VALUE)
	}
}
func ensureSettingGsmHomeOnlyNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmHomeOnlyExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY, NM_KEY_ERROR_MISSING_VALUE)
	}
}
func ensureSettingGsmPinNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmPinExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN, NM_KEY_ERROR_MISSING_VALUE)
	}
	value := getSettingGsmPin(data)
	if len(value) == 0 {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN, NM_KEY_ERROR_EMPTY_VALUE)
	}
}
func ensureSettingGsmPinFlagsNoEmpty(data connectionData, errs fieldErrors) {
	if !isSettingGsmPinFlagsExists(data) {
		rememberError(errs, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS, NM_KEY_ERROR_MISSING_VALUE)
	}
}

// Getter
func getSettingGsmNumber(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingGsmNumber: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmUsername(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingGsmUsername: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmPasswordFlags(data connectionData) (value uint32) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(uint32)
		if !ok {
			logger.Warningf("getSettingGsmPasswordFlags: value type is invalid, should be uint32 instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmPassword(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingGsmPassword: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmApn(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingGsmApn: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmNetworkId(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingGsmNetworkId: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmNetworkType(data connectionData) (value int32) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(int32)
		if !ok {
			logger.Warningf("getSettingGsmNetworkType: value type is invalid, should be int32 instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmAllowedBands(data connectionData) (value bool) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(bool)
		if !ok {
			logger.Warningf("getSettingGsmAllowedBands: value type is invalid, should be bool instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmHomeOnly(data connectionData) (value bool) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(bool)
		if !ok {
			logger.Warningf("getSettingGsmHomeOnly: value type is invalid, should be bool instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmPin(data connectionData) (value string) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(string)
		if !ok {
			logger.Warningf("getSettingGsmPin: value type is invalid, should be string instead of %#v", ivalue)
		}
	}
	return
}
func getSettingGsmPinFlags(data connectionData) (value uint32) {
	ivalue := getSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS)
	if !isInterfaceEmpty(ivalue) {
		var ok bool
		value, ok = ivalue.(uint32)
		if !ok {
			logger.Warningf("getSettingGsmPinFlags: value type is invalid, should be uint32 instead of %#v", ivalue)
		}
	}
	return
}

// Setter
func setSettingGsmNumber(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER, value)
}
func setSettingGsmUsername(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME, value)
}
func setSettingGsmPasswordFlags(data connectionData, value uint32) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS, value)
}
func setSettingGsmPassword(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD, value)
}
func setSettingGsmApn(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN, value)
}
func setSettingGsmNetworkId(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID, value)
}
func setSettingGsmNetworkType(data connectionData, value int32) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE, value)
}
func setSettingGsmAllowedBands(data connectionData, value bool) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS, value)
}
func setSettingGsmHomeOnly(data connectionData, value bool) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY, value)
}
func setSettingGsmPin(data connectionData, value string) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN, value)
}
func setSettingGsmPinFlags(data connectionData, value uint32) {
	setSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS, value)
}

// JSON Getter
func getSettingGsmNumberJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER, getSettingGsmKeyType(NM_SETTING_GSM_NUMBER))
	return
}
func getSettingGsmUsernameJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME, getSettingGsmKeyType(NM_SETTING_GSM_USERNAME))
	return
}
func getSettingGsmPasswordFlagsJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS, getSettingGsmKeyType(NM_SETTING_GSM_PASSWORD_FLAGS))
	return
}
func getSettingGsmPasswordJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD, getSettingGsmKeyType(NM_SETTING_GSM_PASSWORD))
	return
}
func getSettingGsmApnJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN, getSettingGsmKeyType(NM_SETTING_GSM_APN))
	return
}
func getSettingGsmNetworkIdJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID, getSettingGsmKeyType(NM_SETTING_GSM_NETWORK_ID))
	return
}
func getSettingGsmNetworkTypeJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE, getSettingGsmKeyType(NM_SETTING_GSM_NETWORK_TYPE))
	return
}
func getSettingGsmAllowedBandsJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS, getSettingGsmKeyType(NM_SETTING_GSM_ALLOWED_BANDS))
	return
}
func getSettingGsmHomeOnlyJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY, getSettingGsmKeyType(NM_SETTING_GSM_HOME_ONLY))
	return
}
func getSettingGsmPinJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN, getSettingGsmKeyType(NM_SETTING_GSM_PIN))
	return
}
func getSettingGsmPinFlagsJSON(data connectionData) (valueJSON string) {
	valueJSON = getSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS, getSettingGsmKeyType(NM_SETTING_GSM_PIN_FLAGS))
	return
}

// JSON Setter
func setSettingGsmNumberJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_NUMBER))
}
func setSettingGsmUsernameJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_USERNAME))
}
func setSettingGsmPasswordFlagsJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_PASSWORD_FLAGS))
}
func setSettingGsmPasswordJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_PASSWORD))
}
func setSettingGsmApnJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_APN))
}
func setSettingGsmNetworkIdJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_NETWORK_ID))
}
func setSettingGsmNetworkTypeJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_NETWORK_TYPE))
}
func setSettingGsmAllowedBandsJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_ALLOWED_BANDS))
}
func setSettingGsmHomeOnlyJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_HOME_ONLY))
}
func setSettingGsmPinJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_PIN))
}
func setSettingGsmPinFlagsJSON(data connectionData, valueJSON string) (err error) {
	return setSettingKeyJSON(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS, valueJSON, getSettingGsmKeyType(NM_SETTING_GSM_PIN_FLAGS))
}

// Logic JSON Setter

// Remover
func removeSettingGsmNumber(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NUMBER)
}
func removeSettingGsmUsername(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_USERNAME)
}
func removeSettingGsmPasswordFlags(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD_FLAGS)
}
func removeSettingGsmPassword(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PASSWORD)
}
func removeSettingGsmApn(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_APN)
}
func removeSettingGsmNetworkId(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_ID)
}
func removeSettingGsmNetworkType(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_NETWORK_TYPE)
}
func removeSettingGsmAllowedBands(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_ALLOWED_BANDS)
}
func removeSettingGsmHomeOnly(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_HOME_ONLY)
}
func removeSettingGsmPin(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN)
}
func removeSettingGsmPinFlags(data connectionData) {
	removeSettingKey(data, NM_SETTING_GSM_SETTING_NAME, NM_SETTING_GSM_PIN_FLAGS)
}
