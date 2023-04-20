// SPDX-FileCopyrightText: 2021 - 2023 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package keyring

/*
#cgo CXXFLAGS:-O2 -std=c++11
#cgo CFLAGS: -W -Wall -fstack-protector-all -fPIC
#cgo LDFLAGS:-ldl
#include "common.h"
#include <stdlib.h>
*/
import "C"
import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"log"
	"sync"
	"unsafe"

	dutils "github.com/linuxdeepin/go-lib/utils"
)

var fileLocker sync.Mutex

const keyringSoPath = "/usr/lib/libkeyringcrypto.so"

func isFileExist(path string) bool {
	return dutils.IsFileExist(path)
}

func ucharToArrayByte(value *C.uchar) string {
	data := C.GoString((*C.char)(unsafe.Pointer(value)))
	return hex.EncodeToString([]byte(data))
}

func ucharToString(value *C.uchar) string {
	return C.GoString((*C.char)(unsafe.Pointer(value)))
}

func writeFile(filename, data string) (err error) {
	fileLocker.Lock()
	defer fileLocker.Unlock()

	fp, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("failed to open %q: %v", filename, err)
		return
	}
	defer func() {
		closeErr := fp.Close()
		if err == nil {
			err = closeErr
		} else {
			log.Printf("writeFile Close %v %v", fp.Name(), closeErr)
		}
	}()
	_, err = fp.WriteString(data + "\n")
	if err != nil {
		fmt.Println("failed to WriteString err :", err)
		return
	}

	err = fp.Sync()
	if err != nil {
		fmt.Println("fp Sync err :", err)
		return
	}

	return
}

func loadFile(filename string) (lines []string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		} else {
			log.Printf("LoadFile Close %v %v", f.Name(), closeErr)
		}
	}()
	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}
	if scanner.Err() != nil {
		err = scanner.Err()
		return
	}

	return
}

func createWhiteBoxUFile(dir, filePath string) error {
	fmt.Println("filePath: ", filePath)
	if !dutils.IsFileExist(dir) {
		err := os.MkdirAll(dir, 0600)
		if err != nil {
			fmt.Println("Mkdir err : ", err)
			return err
		}
	}
	if !dutils.IsFileExist(filePath) {
		err := dutils.CreateFile(filePath)
		if err != nil {
			fmt.Println("CreateFile err : ", err)
			return err
		}
	}
	return nil
}

func CreateWhiteBoxUFile(name string) error {
	if !isFileExist(keyringSoPath) {
		return errors.New("Not Exist Keyring So ")
	}
	C.set_debug_flag(0)

	UKEK := C.GoString(C.generate_random_len(C.MASTER_KEY_LEN))
	UKEKIV := C.GoString(C.generate_random_len(C.MASTER_KEY_LEN))
	//TODO: 待测试通过后删除
	fmt.Println("[CreateWhiteBoxUFile] UKEK generate_random_len   : ", UKEK)
	fmt.Println("[CreateWhiteBoxUFile] UKEKIV generate_random_len : ", UKEKIV)

	UKEK_ := unsafe.Pointer(C.CString(UKEK))
	defer C.free(UKEK_)

	UKEKIV_ := unsafe.Pointer(C.CString(UKEKIV))
	defer C.free(UKEKIV_)

	key := ""
	//key：16个0 白盒加密 UKEK --> WB_UKEK
	WB_UKEK := C.deepin_wb_encrypt((*C.uchar)(UKEK_), (*C.uchar)(unsafe.Pointer(C.CString(key))), false)
	//string -> []byte,数据中间有00就会直接返回，导致len < 16 : 这种情况重新创建WB_UKEK
	if len([]byte(ucharToString(WB_UKEK))) != C.MASTER_KEY_LEN {
		CreateWhiteBoxUFile(name)
		return errors.New("string to byte failed(len < 16)")
	}

	//key：UKEK 白盒加密 UKEKIV --> CIPHER_UKEKIV
	CIPHER_UKEKIV := C.deepin_wb_encrypt((*C.uchar)(UKEKIV_), (*C.uchar)(UKEK_), true)
	defer C.free(unsafe.Pointer(CIPHER_UKEKIV))
	if len([]byte(ucharToString(CIPHER_UKEKIV))) != C.MASTER_KEY_LEN {
		CreateWhiteBoxUFile(name)
		return errors.New("string to byte failed(len < 16)")
	}

	//创建新增账户WB_UFile文件
	dir := fmt.Sprintf("/var/lib/keyring/%s", name)
	filePath := path.Join(dir, "WB_UFile")
	createWhiteBoxUFile(dir, filePath)

	writeFile(filePath, hex.EncodeToString([]byte(ucharToString(WB_UKEK))))
	writeFile(filePath, hex.EncodeToString([]byte(ucharToString(CIPHER_UKEKIV))))

	// 读取WB_UFILE文件
	lines, err := loadFile(filePath)
	if err != nil {
		fmt.Println("loadFile err : ", err)
		return err
	}
	fmt.Println("loadFile lines : ", lines)

	return nil
}

func DeleteWhiteBoxUFile(name string) error {
	filePath := fmt.Sprintf("/var/lib/keyring/%s", name)
	if dutils.IsFileExist(filePath) {
		dirs, err := ioutil.ReadDir(filePath)
		if err != nil {
			fmt.Println("ReadDir err : ", err)
			return err
		}
		for _, dir := range dirs {
			err = os.RemoveAll(path.Join(filePath, dir.Name()))
			if err != nil {
				fmt.Println("RemoveAll dir err : ", err)
				return err
			}
		}
		err = os.Remove(filePath)
		if err != nil {
			fmt.Println("Remove filePath err : ", err)
			return err
		}
	}
	return nil
}
