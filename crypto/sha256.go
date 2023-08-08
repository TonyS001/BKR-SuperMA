// (C) 2016-2023 Ant Group Co.,Ltd.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"crypto/sha256"
	"encoding/base64"
)

func Hash(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	result := hash.Sum(nil)
	return base64.StdEncoding.EncodeToString(result)
	// return utils.Bytes2Str(result)
}
