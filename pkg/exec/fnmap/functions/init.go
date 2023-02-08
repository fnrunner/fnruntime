/*
Copyright 2023 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functions

import (
	"github.com/fnrunner/fnruntime/pkg/exec/fnmap"
	ctrlcfgv1alpha1 "github.com/fnrunner/fnsyntax/apis/controllerconfig/v1alpha1"
)

func Init(c *fnmap.Config) fnmap.FuncMap {
	fnMap := fnmap.New(c)
	fnMap.Register(ctrlcfgv1alpha1.RootType, func() fnmap.Function {
		return NewRootFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.BlockType, func() fnmap.Function {
		return NewBlockFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.SliceType, func() fnmap.Function {
		return NewSliceFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.MapType, func() fnmap.Function {
		return NewMapFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.QueryType, func() fnmap.Function {
		return NewQueryFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.GoTemplateType, func() fnmap.Function {
		return NewGTFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.JQType, func() fnmap.Function {
		return NewJQFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.WasmType, func() fnmap.Function {
		return NewImageFn()
	})
	fnMap.Register(ctrlcfgv1alpha1.ContainerType, func() fnmap.Function {
		return NewImageFn()
	})
	return fnMap
}
