//
// Copyright (c) 2021 通通停车武汉研发中心
// All rights reserved
// filename: uuid.go
// description: UUID生成工具
// version: 0.1.0
// created by hongliu(hongliu@egova.com.cn) at 2021-06-24
//

package uuid

import "github.com/twinj/uuid"

// UUID 获取uuid字符串
func UUID() string {
	id := uuid.NewV4()
	return id.String()
}
