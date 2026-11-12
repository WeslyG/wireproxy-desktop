package main

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

static CFMutableDictionaryRef configQuery(const char *scope) {
	CFMutableDictionaryRef query = CFDictionaryCreateMutable(kCFAllocatorDefault, 0,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFStringRef account = CFStringCreateWithCString(kCFAllocatorDefault, scope, kCFStringEncodingUTF8);
	CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
	CFDictionarySetValue(query, kSecAttrService, CFSTR("com.weslyg.wireproxy-desktop"));
	CFDictionarySetValue(query, kSecAttrAccount, account);
	CFRelease(account);
	return query;
}

static OSStatus configRead(const char *scope, CFDataRef *data) {
	CFMutableDictionaryRef query = configQuery(scope);
	CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	if (status == errSecSuccess) {
		*data = (CFDataRef)result;
	}
	return status;
}

static OSStatus configWrite(const char *scope, const UInt8 *bytes, CFIndex size) {
	CFMutableDictionaryRef query = configQuery(scope);
	CFDataRef data = CFDataCreate(kCFAllocatorDefault, bytes, size);
	const void *keys[] = {kSecValueData};
	const void *values[] = {data};
	CFDictionaryRef update = CFDictionaryCreate(kCFAllocatorDefault, keys, values, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	OSStatus status = SecItemUpdate(query, update);
	if (status == errSecItemNotFound) {
		CFMutableDictionaryRef item = CFDictionaryCreateMutableCopy(kCFAllocatorDefault, 0, query);
		CFDictionarySetValue(item, kSecValueData, data);
		CFDictionarySetValue(item, kSecAttrLabel, CFSTR("Wireproxy Desktop"));
		status = SecItemAdd(item, NULL);
		CFRelease(item);
		if (status == errSecDuplicateItem) {
			status = SecItemUpdate(query, update);
		}
	}
	CFRelease(update);
	CFRelease(data);
	CFRelease(query);
	return status;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func readProtectedConfig(scope string) ([]byte, error) {
	account := C.CString(scope)
	defer C.free(unsafe.Pointer(account))
	var data C.CFDataRef
	switch status := C.configRead(account, &data); status {
	case C.errSecSuccess:
		defer C.CFRelease(C.CFTypeRef(data))
		return C.GoBytes(unsafe.Pointer(C.CFDataGetBytePtr(data)), C.int(C.CFDataGetLength(data))), nil
	case C.errSecItemNotFound:
		return nil, errConfigMissing
	default:
		return nil, fmt.Errorf("macOS Keychain read failed (OSStatus %d)", int(status))
	}
}

func writeProtectedConfig(scope string, data []byte) error {
	account := C.CString(scope)
	defer C.free(unsafe.Pointer(account))
	status := C.configWrite(account, (*C.UInt8)(unsafe.Pointer(&data[0])), C.CFIndex(len(data)))
	if status != C.errSecSuccess {
		return fmt.Errorf("macOS Keychain write failed (OSStatus %d)", int(status))
	}
	return nil
}

func protectedStoreName() string { return "macOS Keychain" }
