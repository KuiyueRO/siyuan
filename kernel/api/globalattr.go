// SiYuan - Refactor your thinking
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package api

import (
	"net/http"

	"github.com/88250/gulu"
	"github.com/gin-gonic/gin"
	"github.com/siyuan-note/siyuan/kernel/model"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func listGlobalAttrs(c *gin.Context) {
	ret := gulu.Ret.NewResult()
	defer c.JSON(http.StatusOK, ret)

	attrs, err := model.ListGlobalAttrs()
	if nil != err {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}
	ret.Data = map[string]interface{}{"attrs": attrs}
}

func createGlobalAttr(c *gin.Context) {
	ret := gulu.Ret.NewResult()
	defer c.JSON(http.StatusOK, ret)

	arg, ok := util.JsonArg(c, ret)
	if !ok {
		return
	}

	payload := &model.CreateGlobalAttrReq{}
	if err := mapToStruct(arg, payload); nil != err {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}

	attr, err := model.CreateGlobalAttr(payload)
	if nil != err {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}
	ret.Data = attr
}

func markGlobalAttrColumn(c *gin.Context) {
	ret := gulu.Ret.NewResult()
	defer c.JSON(http.StatusOK, ret)

	arg, ok := util.JsonArg(c, ret)
	if !ok {
		return
	}

	payload := &model.MarkGlobalAttrReq{}
	if err := mapToStruct(arg, payload); nil != err {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}
	if _, hasEnabled := arg["enabled"]; !hasEnabled {
		payload.Enabled = true
	}

	attr, err := model.MarkAttrViewColumnAsGlobal(payload)
	if nil != err {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}
	ret.Data = attr
}

func mapToStruct(arg map[string]interface{}, dst interface{}) error {
	data, err := gulu.JSON.MarshalJSON(arg)
	if nil != err {
		return err
	}
	return gulu.JSON.UnmarshalJSON(data, dst)
}

func bindBlockToGlobalAttr(c *gin.Context) {
	ret := gulu.Ret.NewResult()
	defer c.JSON(http.StatusOK, ret)

	arg, ok := util.JsonArg(c, ret)
	if !ok {
		return
	}

	blockID, _ := arg["blockID"].(string)
	gaID, _ := arg["gaId"].(string)

	if blockID == "" || gaID == "" {
		ret.Code = -1
		ret.Msg = "blockID and gaId are required"
		return
	}

	// 获取初始值（可选）
	var initialValue interface{}
	if val, exists := arg["value"]; exists {
		initialValue = val
	}

	err := model.BindBlockToGlobalAttr(blockID, gaID, initialValue)
	if err != nil {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}

	ret.Data = map[string]interface{}{
		"blockID": blockID,
		"gaId":    gaID,
	}
}

func unbindBlockFromGlobalAttr(c *gin.Context) {
	ret := gulu.Ret.NewResult()
	defer c.JSON(http.StatusOK, ret)

	arg, ok := util.JsonArg(c, ret)
	if !ok {
		return
	}

	blockID, _ := arg["blockID"].(string)
	gaID, _ := arg["gaId"].(string)

	if blockID == "" || gaID == "" {
		ret.Code = -1
		ret.Msg = "blockID and gaId are required"
		return
	}

	err := model.UnbindBlockFromGlobalAttr(blockID, gaID)
	if err != nil {
		ret.Code = -1
		ret.Msg = err.Error()
		return
	}
}
