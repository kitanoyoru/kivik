// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package kivik

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"gitlab.com/flimzy/testy"
)

type TestFeed struct {
	max      int64
	i        int64
	closeErr error
}

var _ iterator = &TestFeed{}

func (f *TestFeed) Close() error { return f.closeErr }
func (f *TestFeed) Next(ifce interface{}) error {
	i, ok := ifce.(*int64)
	if ok {
		*i = f.i
		f.i++
		if f.i > f.max {
			return io.EOF
		}
		time.Sleep(5 * time.Millisecond)
		return nil
	}
	panic(fmt.Sprintf("unknown type: %T", ifce))
}

func TestIterator(t *testing.T) {
	iter := newIterator(context.Background(), nil, &TestFeed{max: 10}, func() interface{} { var i int64; return &i }())
	expected := []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	result := []int64{}
	for iter.Next() {
		val, ok := iter.curVal.(*int64)
		if !ok {
			panic("Unexpected type")
		}
		result = append(result, *val)
	}
	if err := iter.Err(); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if d := testy.DiffAsJSON(expected, result); d != nil {
		t.Errorf("Unexpected result:\n%s\n", d)
	}
}

func TestCancelledIterator(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	iter := newIterator(ctx, nil, &TestFeed{max: 10000}, func() interface{} { var i int64; return &i }())
	for iter.Next() { //nolint:revive // empty block necessary for loop
	}
	if err := iter.Err(); err.Error() != "context deadline exceeded" {
		t.Errorf("Unexpected error: %s", err)
	}
}
