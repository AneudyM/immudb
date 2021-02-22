/*
Copyright 2021 CodeNotary, Inc. All rights reserved.

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

package stream

import (
	"bytes"
	"encoding/binary"
	"io"
)

func NewMsgReceiver(stream ImmuServiceReceiver_Stream) *msgReceiver {
	return &msgReceiver{stream: stream, b: bytes.NewBuffer([]byte{})}
}

type MsgReceiver interface {
	Read(message []byte) (n int, err error)
}

type msgReceiver struct {
	stream ImmuServiceReceiver_Stream
	b      *bytes.Buffer
	l      uint64
	r      int
}

func (r *msgReceiver) Read(message []byte) (n int, err error) {
	for {
		chunk, err := r.stream.Recv()
		if err != nil {
			if err != io.EOF {
				return 0, err
			}
		}
		if chunk != nil {
			r.b.Write(chunk.Content)
		}
		if r.r == 0 && r.l == 0 {
			trailer := make([]byte, 8)
			_, err = r.b.Read(trailer)
			if err != nil {
				return 0, err
			}
			r.l = binary.BigEndian.Uint64(trailer)
		}
		if r.b.Len() >= len(message) {
			read, err := r.b.Read(message)
			if err != nil {
				return 0, err
			}
			r.r += read
			return read, err
		}
		// last message
		if r.b.Len() >= int(r.l)-r.r {
			lmsg := make([]byte, int(r.l)-r.r)
			read, err := r.b.Read(lmsg)
			if err != nil {
				return 0, err
			}
			r.r = 0
			r.l = 0
			copy(message, lmsg)
			return read, io.EOF
		}

	}
}