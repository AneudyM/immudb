/*
Copyright 2019-2020 vChain, Inc.

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

package database

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/codenotary/immudb/embedded/store"
	"github.com/codenotary/immudb/embedded/tbtree"
	"github.com/codenotary/immudb/pkg/api/schema"
)

const setLenLen = 8
const scoreLen = 8
const txIDLen = 8

// ZAdd adds a score for an existing key in a sorted set
// As a parameter of ZAddOptions is possible to provide the associated index of the provided key. In this way, when resolving reference, the specified version of the key will be returned.
// If the index is not provided the resolution will use only the key and last version of the item will be returned
// If ZAddOptions.index is provided key is optional
func (d *db) ZAdd(req *schema.ZAddRequest) (*schema.TxMetadata, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if req == nil {
		return nil, store.ErrIllegalArguments
	}

	// check referenced key exists
	if _, err := d.getAt(req.Key, req.AtTx, 0, d.st, d.tx1); err != nil {
		return nil, err
	}

	zKey := wrapZAddReferenceAt(req.Set, req.Key, req.AtTx, req.Score)

	meta, err := d.st.Commit([]*store.KV{{Key: zKey, Value: nil}})

	return schema.TxMetatadaTo(meta), err
}

// math.Float64frombits(bits)

// ZScan ...
func (d *db) ZScan(req *schema.ZScanRequest) (*schema.ZItemList, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if req == nil || len(req.Set) == 0 {
		return nil, store.ErrIllegalArguments
	}

	if req.Limit > MaxKeyScanLimit {
		return nil, ErrMaxKeyScanLimitExceeded
	}

	limit := req.Limit

	if req.Limit == 0 {
		limit = MaxKeyScanLimit
	}

	prefix := make([]byte, 1+setLenLen+len(req.Set))
	prefix[0] = sortedSetKeyPrefix
	binary.BigEndian.PutUint64(prefix[1:], uint64(len(req.Set)))
	copy(prefix[1+setLenLen:], req.Set)

	var seekKey []byte

	if len(req.SeekKey) == 0 {
		seekKey = make([]byte, len(prefix)+scoreLen)
		// here we compose the offset if Min score filter is provided only if is not reversed order
		if req.MinScore > 0 && !req.Desc {
			binary.BigEndian.PutUint64(prefix[1+setLenLen+len(req.Set):], math.Float64bits(req.MinScore))
		}
		// here we compose the offset if Max score filter is provided only if is reversed order
		if req.MaxScore > 0 && req.Desc {
			binary.BigEndian.PutUint64(prefix[1+setLenLen+len(req.Set):], math.Float64bits(req.MaxScore))
		}
	} else {
		seekKey = make([]byte, len(prefix)+scoreLen+len(req.SeekKey))
		copy(seekKey, prefix)
		binary.BigEndian.PutUint64(seekKey[len(prefix):], math.Float64bits(req.SeekScore))
		binary.BigEndian.PutUint64(seekKey[len(prefix)+scoreLen:], req.SeekAtTx)
		copy(seekKey[len(prefix)+scoreLen+txIDLen:], req.SeekKey)
	}

	snap, err := d.st.SnapshotSince(req.SinceTx)
	if err != nil {
		return nil, err
	}
	defer snap.Close()

	r, err := d.st.NewReader(
		snap,
		&tbtree.ReaderSpec{
			SeekKey:   seekKey,
			Prefix:    prefix,
			DescOrder: req.Desc,
		})
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var items []*schema.ZItem
	i := uint64(0)

	for {
		zKey, _, _, err := r.Read()
		if err == tbtree.ErrNoMoreEntries {
			break
		}
		if err != nil {
			return nil, err
		}

		// zKey = [1+setLenLen+len(req.Set)+scoreLen+txIDLen+len(req.Key)]
		scoreOff := 1 + setLenLen + len(req.Set)
		scoreB := binary.BigEndian.Uint64(zKey[scoreOff:])
		score := math.Float64frombits(scoreB)

		// Guard to ensure that score match the filter range if filter is provided
		if req.MinScore > 0 && score < req.MinScore {
			continue
		}
		if req.MaxScore > 0 && score > req.MaxScore {
			continue
		}

		atTx := binary.BigEndian.Uint64(zKey[scoreOff+scoreLen:])

		keyOff := scoreOff + scoreLen + txIDLen
		key := make([]byte, len(zKey)-keyOff)
		copy(key, zKey[keyOff:])

		item, err := d.getAt(key, atTx, 0, snap, d.tx1)

		zitem := &schema.ZItem{
			Set:   req.Set,
			Key:   key,
			Item:  item,
			Score: score,
			AtTx:  atTx,
		}

		items = append(items, zitem)
		if i++; i == limit {
			break
		}
	}

	list := &schema.ZItemList{
		Items: items,
	}

	return list, nil
}

//VerifiableZAdd ...
func (d *db) VerifiableZAdd(opts *schema.VerifiableZAddRequest) (*schema.VerifiableTx, error) {
	return nil, fmt.Errorf("Functionality not yet supported: %s", "VerifiableZAdd")
}

func wrapZAddReferenceAt(set, key []byte, atTx uint64, score float64) []byte {
	zKey := make([]byte, 1+setLenLen+len(set)+scoreLen+txIDLen+len(key))
	zi := 0

	zKey[0] = sortedSetKeyPrefix
	zi++
	binary.BigEndian.PutUint64(zKey[zi:], uint64(len(set)))
	zi += setLenLen
	copy(zKey[zi:], set)
	zi += len(set)
	binary.BigEndian.PutUint64(zKey[zi:], math.Float64bits(score))
	zi += scoreLen
	binary.BigEndian.PutUint64(zKey[zi:], atTx)
	zi += txIDLen
	copy(zKey[zi:], key)

	return zKey
}