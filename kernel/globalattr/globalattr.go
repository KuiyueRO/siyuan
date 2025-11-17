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

package globalattr

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/88250/gulu"
	"github.com/siyuan-note/filelock"
	"github.com/siyuan-note/logging"
	"github.com/siyuan-note/siyuan/kernel/av"
	"github.com/siyuan-note/siyuan/kernel/util"
)

// ErrAttrNotFound 在访问不存在的全局属性文件时返回。
var ErrAttrNotFound = errors.New("global attribute not found")

// Attribute 表示存储在 data/storage/ga/ 下的单个全局属性文件。
type Attribute struct {
	Key    *av.Key     `json:"key"`
	Values []*av.Value `json:"values,omitempty"`
}

// ID 返回全局属性标识，用于定位文件名。
func (attr *Attribute) ID() string {
	if nil == attr || nil == attr.Key {
		return ""
	}

	if "" != attr.Key.GaID {
		return attr.Key.GaID
	}
	return attr.Key.ID
}

// GetDataDir 返回全局属性存储目录，必要时会进行创建。
func GetDataDir() string {
	dir := filepath.Join(util.DataDir, "storage", "ga")
	if gulu.File.IsDir(dir) {
		return dir
	}

	if err := os.MkdirAll(dir, 0o755); nil != err {
		logging.LogErrorf("create global attribute dir failed: %s", err)
	}
	return dir
}

// GetDataPath 返回指定 gaID 对应的 JSON 文件路径。
func GetDataPath(gaID string) string {
	if "" == gaID {
		return ""
	}
	return filepath.Join(GetDataDir(), gaID+".json")
}

// Parse 读取并解析指定 gaID 的全局属性文件。
func Parse(gaID string) (*Attribute, error) {
	path := GetDataPath(gaID)
	if "" == path || !filelock.IsExist(path) {
		return nil, ErrAttrNotFound
	}

	data, err := filelock.ReadFile(path)
	if nil != err {
		logging.LogErrorf("read global attribute [%s] failed: %s", gaID, err)
		return nil, err
	}

	attr := &Attribute{}
	if err = gulu.JSON.UnmarshalJSON(data, attr); nil != err {
		logging.LogErrorf("unmarshal global attribute [%s] failed: %s", gaID, err)
		return nil, err
	}
	return attr, nil
}

// Save 将全局属性写入磁盘。
func Save(attr *Attribute) error {
	if nil == attr || nil == attr.Key {
		return errors.New("global attribute key is empty")
	}

	if "" == attr.Key.GaID {
		if "" == attr.Key.ID {
			return errors.New("global attribute id is empty")
		}
		attr.Key.GaID = attr.Key.ID
	}

	var (
		data []byte
		err  error
	)
	if util.UseSingleLineSave {
		data, err = gulu.JSON.MarshalJSON(attr)
	} else {
		data, err = gulu.JSON.MarshalIndentJSON(attr, "", "\t")
	}
	if nil != err {
		logging.LogErrorf("marshal global attribute [%s] failed: %s", attr.ID(), err)
		return err
	}

	path := GetDataPath(attr.ID())
	if "" == path {
		return errors.New("global attribute path is empty")
	}

	if err = filelock.WriteFile(path, data); nil != err {
		logging.LogErrorf("save global attribute [%s] failed: %s", attr.ID(), err)
		return err
	}
	return nil
}

// Remove 删除全局属性文件。
func Remove(gaID string) error {
	path := GetDataPath(gaID)
	if "" == path {
		return nil
	}
	if !filelock.IsExist(path) {
		return nil
	}
	if err := os.Remove(path); nil != err {
		logging.LogErrorf("remove global attribute [%s] failed: %s", gaID, err)
		return err
	}
	return nil
}

// ListAttributes 扫描目录并返回全部全局属性。
func ListAttributes() ([]*Attribute, error) {
	dir := GetDataDir()
	entries, err := os.ReadDir(dir)
	if nil != err {
		logging.LogErrorf("list global attribute dir failed: %s", err)
		return nil, err
	}

	var ret []*Attribute
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		gaID := strings.TrimSuffix(entry.Name(), ".json")
		attr, parseErr := Parse(gaID)
		if nil != parseErr {
			// 已记录日志，忽略损坏的文件避免阻断其他条目
			continue
		}
		ret = append(ret, attr)
	}
	return ret, nil
}
