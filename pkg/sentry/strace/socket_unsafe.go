// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strace

import (
	"reflect"
	"unsafe"

	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/marshal/primitive"
)

func makeControlMessageRights(numRights int) (prims []primitive.Int32, cmr linux.ControlMessageRights) {
	r := make([]int32, numRights)
	*(*reflect.SliceHeader)(unsafe.Pointer(&prims)) = *(*reflect.SliceHeader)(unsafe.Pointer(&r))
	*(*reflect.SliceHeader)(unsafe.Pointer(&cmr)) = *(*reflect.SliceHeader)(unsafe.Pointer(&r))
	return
}
