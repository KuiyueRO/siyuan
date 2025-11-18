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

package av

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/88250/gulu"
	"github.com/siyuan-note/filelock"
	"github.com/siyuan-note/logging"
	"github.com/siyuan-note/siyuan/kernel/util"
)

// ErrGlobalAttrNotFound is returned when the requested global attribute file is missing.
var ErrGlobalAttrNotFound = errors.New("global attribute not found")

// GlobalAttribute represents a single global attribute stored under data/storage/ga/.
type GlobalAttribute struct {
	Key    *Key     `json:"key"`
	Values []*Value `json:"values,omitempty"`
}

// ID returns the identifier of the global attribute.
func (attr *GlobalAttribute) ID() string {
	if attr == nil || attr.Key == nil {
		return ""
	}

	if attr.Key.GaID != "" {
		return attr.Key.GaID
	}
	return attr.Key.ID
}

// UpsertValue stores/updates the value bound to the specified block ID.
func (attr *GlobalAttribute) UpsertValue(blockID string, value *Value) {
	if attr == nil || attr.Key == nil || blockID == "" || value == nil {
		return
	}

	cloned := value.Clone()
	if cloned == nil {
		return
	}

	cloned.BlockRefID = blockID
	cloned.BlockID = ""
	cloned.KeyID = ""

	for idx, v := range attr.Values {
		if v.BlockRefID == blockID {
			attr.Values[idx] = cloned
			return
		}
	}
	attr.Values = append(attr.Values, cloned)
}

// RemoveValue removes the value bound to the specified block ID.
func (attr *GlobalAttribute) RemoveValue(blockID string) bool {
	if attr == nil || blockID == "" {
		return false
	}

	for idx, v := range attr.Values {
		if v.BlockRefID == blockID {
			attr.Values = append(attr.Values[:idx], attr.Values[idx+1:]...)
			return true
		}
	}
	return false
}

// ensureGaID fills GaID using the local ID when necessary.
func (attr *GlobalAttribute) ensureGaID() {
	if attr == nil || attr.Key == nil {
		return
	}
	if attr.Key.GaID == "" {
		if attr.Key.ID == "" {
			attr.Key.ID = attr.ID()
		}
		attr.Key.GaID = attr.Key.ID
	}
}

func getGlobalAttrDataDir() string {
	dir := filepath.Join(util.DataDir, "storage", "ga")
	if gulu.File.IsDir(dir) {
		return dir
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		logging.LogErrorf("create global attribute dir failed: %s", err)
	}
	return dir
}

func getGlobalAttrDataPath(gaID string) string {
	if gaID == "" {
		return ""
	}
	return filepath.Join(getGlobalAttrDataDir(), gaID+".json")
}

// ParseGlobalAttribute reads and parses the specified global attribute file.
func ParseGlobalAttribute(gaID string) (*GlobalAttribute, error) {
	path := getGlobalAttrDataPath(gaID)
	if path == "" || !filelock.IsExist(path) {
		return nil, ErrGlobalAttrNotFound
	}

	data, err := filelock.ReadFile(path)
	if err != nil {
		logging.LogErrorf("read global attribute [%s] failed: %s", gaID, err)
		return nil, err
	}

	attr := &GlobalAttribute{}
	if err = gulu.JSON.UnmarshalJSON(data, attr); err != nil {
		logging.LogErrorf("unmarshal global attribute [%s] failed: %s", gaID, err)
		return nil, err
	}
	return attr, nil
}

// SaveGlobalAttribute writes the given global attribute to disk.
func SaveGlobalAttribute(attr *GlobalAttribute) error {
	if attr == nil || attr.Key == nil {
		return errors.New("global attribute key is empty")
	}

	attr.ensureGaID()

	var (
		data []byte
		err  error
	)
	if util.UseSingleLineSave {
		data, err = gulu.JSON.MarshalJSON(attr)
	} else {
		data, err = gulu.JSON.MarshalIndentJSON(attr, "", "\t")
	}
	if err != nil {
		logging.LogErrorf("marshal global attribute [%s] failed: %s", attr.ID(), err)
		return err
	}

	path := getGlobalAttrDataPath(attr.ID())
	if path == "" {
		return errors.New("global attribute path is empty")
	}

	if err = filelock.WriteFile(path, data); err != nil {
		logging.LogErrorf("save global attribute [%s] failed: %s", attr.ID(), err)
		return err
	}
	return nil
}

// RemoveGlobalAttribute deletes the JSON file for the given global attribute.
func RemoveGlobalAttribute(gaID string) error {
	path := getGlobalAttrDataPath(gaID)
	if path == "" {
		return nil
	}
	if !filelock.IsExist(path) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		logging.LogErrorf("remove global attribute [%s] failed: %s", gaID, err)
		return err
	}
	return nil
}

// ListGlobalAttributes scans storage and returns all global attributes.
func ListGlobalAttributes() ([]*GlobalAttribute, error) {
	dir := getGlobalAttrDataDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		logging.LogErrorf("list global attribute dir failed: %s", err)
		return nil, err
	}

	var ret []*GlobalAttribute
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		gaID := strings.TrimSuffix(entry.Name(), ".json")
		attr, parseErr := ParseGlobalAttribute(gaID)
		if parseErr != nil {
			// Already logged, skip broken files.
			continue
		}
		ret = append(ret, attr)
	}
	return ret, nil
}
