package main

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
*/
import "C"

import "unsafe"

func preferredLocales() []string {
	languages := C.CFLocaleCopyPreferredLanguages()
	if languages == 0 {
		return nil
	}
	defer C.CFRelease(C.CFTypeRef(languages))
	var locales []string
	for i := C.CFIndex(0); i < C.CFArrayGetCount(languages); i++ {
		language := C.CFStringRef(uintptr(C.CFArrayGetValueAtIndex(languages, i)))
		var buffer [128]C.char
		if C.CFStringGetCString(language, (*C.char)(unsafe.Pointer(&buffer[0])), C.CFIndex(len(buffer)), C.kCFStringEncodingUTF8) != 0 {
			locales = append(locales, C.GoString(&buffer[0]))
		}
	}
	return locales
}
