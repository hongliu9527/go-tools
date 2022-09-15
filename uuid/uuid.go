/*
 * @Author: hongliu
 * @Date: 2022-04-26 09:34:48
 * @LastEditors: hongliu
 * @LastEditTime: 2022-04-26 10:34:25
 * @FilePath: \go-tools\logger\logger.go
 * @Description: UUID字符串获取工具
 *
 * Copyright (c) 2022 by 洪流, All Rights Reserved.
 */

package uuid

import "github.com/twinj/uuid"

// UUID 获取uuid字符串
func UUID() string {
	id := uuid.NewV4()
	return id.String()
}
