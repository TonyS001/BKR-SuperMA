// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"BKR-SuperMA/common"
)

type NetWork interface {
	Start()
	Stop()
	BroadcastMessage(msg *common.Message)
	SendMessage(id uint32, msg *common.Message)
}
