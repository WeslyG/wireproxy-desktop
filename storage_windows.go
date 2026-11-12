package main

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const configRegistryPath = `Software\Wireproxy Desktop\Configuration`

func readProtectedConfig(scope string) ([]byte, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, configRegistryPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, errConfigMissing
	}
	if err != nil {
		return nil, err
	}
	defer key.Close()
	ciphertext, _, err := key.GetBinaryValue(scope)
	if errors.Is(err, registry.ErrNotExist) {
		return nil, errConfigMissing
	}
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 {
		return nil, errors.New("protected configuration is empty")
	}
	input := windows.DataBlob{Size: uint32(len(ciphertext)), Data: &ciphertext[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	plaintext := unsafe.Slice(output.Data, output.Size)
	defer clear(plaintext)
	return append([]byte(nil), plaintext...), nil
}

func writeProtectedConfig(scope string, data []byte) error {
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(output.Data))))
	key, _, err := registry.CreateKey(registry.CURRENT_USER, configRegistryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetBinaryValue(scope, unsafe.Slice(output.Data, output.Size))
}

func protectedStoreName() string { return "Windows DPAPI" }
