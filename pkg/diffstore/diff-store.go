/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package diffstore

import (
	"bytes"

	"github.com/google/btree"
)

const DefaultSeparator = '|'

// DiffStore tracks changes by prefix and sub keys
type DiffStore struct {
	tree *btree.BTree
}

func New() *DiffStore {
	return &DiffStore{tree: btree.New(2)}
}

// Reset the store to clear, marking all entries as deleted (and removing previously deleted ones)
func (s *DiffStore) Reset(state ItemState) {
	toDelete := make([]*storeKV, 0)

	s.tree.Ascend(func(i btree.Item) bool {
		v := i.(*storeKV)
		if v.state == ItemDeleted {
			// previous deleted items are removed
			toDelete = append(toDelete, v)
		} else {
			v.state = state
			v.value = nil
		}
		return true
	})

	for _, i := range toDelete {
		s.tree.Delete(i)
	}
}

func (s *DiffStore) Set(key []byte, hash uint64, value interface{}) {
	item := s.tree.Get(&storeKV{key: key})

	if item == nil {
		s.tree.ReplaceOrInsert(&storeKV{
			key:   key,
			hash:  hash,
			value: value,
			state: ItemChanged,
		})
		return
	}

	v := item.(*storeKV)

	if v.hash == hash {
		if v.state == ItemDeleted {
			v.value = value
			v.state = ItemUnchanged
		}
		return
	}

	v.hash = hash
	v.value = value
	v.state = ItemChanged
}

func (s *DiffStore) DeleteByPrefix(prefix []byte) {
	s.tree.AscendGreaterOrEqual(&storeKV{key: prefix}, func(i btree.Item) bool {
		v := i.(*storeKV)

		if !bytes.HasPrefix(v.key, prefix) {
			return false
		}

		v.state = ItemDeleted

		return true
	})
}

func (s *DiffStore) Updated() (updated []KV) {
	s.tree.Ascend(func(i btree.Item) bool {
		v := i.(*storeKV)

		if v.state == ItemChanged {
			updated = append(updated, KV{v.key, v.value})
		}

		return true
	})
	return
}

func (s *DiffStore) Deleted() (deleted []KV) {
	s.tree.Descend(func(i btree.Item) bool {
		v := i.(*storeKV)

		if v.state == ItemDeleted {
			deleted = append(deleted, KV{v.key, v.value})
		}

		return true
	})
	return
}
