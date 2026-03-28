// Copyright (c) 2024-2025 Zhenyi. All rights reserved.
// Use of this source code is governed by the AGPL license.

package zpoolobs

// Pool-name constants for zhenyi-layer object-pool observability and docs aggregation.
// 池名常量，用于 zhenyi 层对象池的观测命名与文档聚合。
// Note: zhenyi-base pools are outside this repo scope; this list catalogs zhenyi-side pools only.
// 注意：zhenyi-base 层池不在本仓库修改，此处仅 catalog zhenyi 侧池。
const (
	PoolNameZActorAsyncTask = "zactor.asyncTask"
	PoolNameZMsgMessage     = "zmsg.Message"
	PoolNameZJsArgs         = "zjs.args"
	PoolNameZJsConsoleArgs  = "zjs.consoleArgs"
	PoolNameZJsVMWrapper    = "zjs.vmWrapper"
	PoolNameZStarlarkArgs   = "zstarlark.args"
	PoolNameZLuaArgs        = "zlua.args"
	PoolNameZLuaVMWrapper   = "zlua.vmWrapper"
	PoolNameZAoiEntityNode  = "zaoi.EntityNode"
	PoolNameZScriptContext  = "zscript.ScriptContext"
)
