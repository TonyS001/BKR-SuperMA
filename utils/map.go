// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package utils

type Map struct {
	data    map[interface{}]interface{}
	isValue bool
}

func NewMap() Map {
	return Map{data: nil, isValue: false}
}

func (m *Map) Insert(k interface{}, v interface{}) {
	if m.isValue {
		if m.data[k] == nil {
			m.data[k] = make(map[interface{}]interface{})
		}
		m.data[k] = v
	} else {

	}
}
