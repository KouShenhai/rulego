/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package base

import (
	"errors"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/json"
	"strings"
	"sync/atomic"
)

var (
	ErrNetPoolNil    = errors.New("net pool is nil")
	ErrClientNotInit = errors.New("client not init")
)

var NodeUtils = &nodeUtils{}

type nodeUtils struct {
}

func (n *nodeUtils) GetVars(configuration types.Configuration) map[string]interface{} {
	if v, ok := configuration[types.Vars]; ok {
		fromVars := make(map[string]interface{})
		fromVars[types.Vars] = v
		return fromVars
	} else {
		return nil
	}
}

func (n *nodeUtils) GetEvn(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return n.getEvnAndMetadata(ctx, msg, false)
}

// GetEvnAndMetadata 和Metadata key合并
func (n *nodeUtils) GetEvnAndMetadata(ctx types.RuleContext, msg types.RuleMsg) map[string]interface{} {
	return n.getEvnAndMetadata(ctx, msg, true)
}

func (n *nodeUtils) IsNetPool(config types.Config, server string) bool {
	return strings.HasPrefix(server, types.NodeConfigurationPrefixNetResource)
}

func (n *nodeUtils) GetNetResourceId(config types.Config, server string) string {
	if n.IsNetPool(config, server) {
		//截取资源ID
		return server[len(types.NodeConfigurationPrefixNetResource):]
	}
	return ""
}

func (n *nodeUtils) IsInitNetResource(_ types.Config, configuration types.Configuration) bool {
	_, ok := configuration[types.NodeConfigurationKeyIsInitNetResource]
	return ok
}

func (n *nodeUtils) getEvnAndMetadata(_ types.RuleContext, msg types.RuleMsg, useMetadata bool) map[string]interface{} {
	var data interface{}
	if msg.DataType == types.JSON {
		// 解析 JSON 字符串到 map
		if err := json.Unmarshal([]byte(msg.Data), &data); err != nil {
			// 解析失败，使用原始数据
			data = msg.Data
		}
	} else {
		// 如果不是 JSON 类型，直接使用原始数据
		data = msg.Data
	}
	var evn = make(map[string]interface{})
	evn[types.IdKey] = msg.Id
	evn[types.TsKey] = msg.Ts
	evn[types.DataKey] = msg.Data
	evn[types.MsgKey] = data
	evn[types.MetadataKey] = map[string]string(msg.Metadata)
	evn[types.MsgTypeKey] = msg.Type
	evn[types.TypeKey] = msg.Type
	evn[types.DataTypeKey] = msg.DataType
	if useMetadata {
		for k, v := range msg.Metadata {
			evn[k] = v
		}
	}
	return evn
}

type NetResourceNode[T any] struct {
	RuleConfig types.Config
	//节点类型
	NodeType string
	//资源ID
	NetResourceId string
	//初始化资源函数
	InitNetResourceFunc func() (T, error)
	//是否正在连接资源
	Connecting int32
}

func (x *NetResourceNode[T]) Init(ruleConfig types.Config, nodeType, server string, initNetResourceFunc func() (T, error)) error {
	x.RuleConfig = ruleConfig
	x.NodeType = nodeType

	if netResourceId := NodeUtils.GetNetResourceId(ruleConfig, server); netResourceId == "" {
		x.InitNetResourceFunc = initNetResourceFunc
		//非资源池方式，初始化mqtt客户端
		_, err := x.InitNetResourceFunc()
		return err
	} else {
		x.NetResourceId = netResourceId
		return nil
	}
}

func (x *NetResourceNode[T]) GetClient() (T, error) {
	if x.NetResourceId != "" {
		//从网络资源池获取
		if x.RuleConfig.NetPool == nil {
			return zeroValue[T](), ErrNetPoolNil
		}
		if p, err := x.RuleConfig.NetPool.GetNetResource(x.NodeType, x.NetResourceId); err == nil {
			return p.(T), nil
		} else {
			return zeroValue[T](), err
		}
	} else if x.InitNetResourceFunc != nil {
		//根据当前组件配置初始化一个客户端
		return x.InitNetResourceFunc()
	} else {
		return zeroValue[T](), ErrClientNotInit
	}
}

// Connect 尝试连接中
func (x *NetResourceNode[T]) Connect() bool {
	return atomic.CompareAndSwapInt32(&x.Connecting, 0, 1)
}

// IsConnecting 正在连接中
func (x *NetResourceNode[T]) IsConnecting() bool {
	return atomic.LoadInt32(&x.Connecting) == 1
}

// Connected 连接完成
func (x *NetResourceNode[T]) Connected() {
	atomic.StoreInt32(&x.Connecting, 0)
}

// zeroValue 函数用于返回 T 类型的零值
func zeroValue[T any]() T {
	var zero T
	return zero
}
