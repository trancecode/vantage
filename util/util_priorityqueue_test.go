package util

import "testing"

type testElementWithPriority struct {
	priority int64
}

func (e testElementWithPriority) Priority() int64 {
	return e.priority
}

func NewTestElementWithPriority(priority int64) testElementWithPriority {
	return testElementWithPriority{priority: priority}
}

func NewTestPriorityQueue() *PriorityQueue[testElementWithPriority] {
	return NewPriorityQueue[testElementWithPriority]()
}

func TestPriorityQueue(t *testing.T) {
	tests := []struct {
		elements []int
		expected []int
	}{
		{
			elements: []int{},
			expected: []int{},
		},
		{
			elements: []int{2, 1, 3},
			expected: []int{1, 2, 3},
		},
		{
			elements: []int{3, 2, 1},
			expected: []int{1, 2, 3},
		},
		{
			elements: []int{2, 2, 2, 1, 1, 3},
			expected: []int{1, 1, 2, 2, 2, 3},
		},
	}

	for i, test := range tests {
		pq := NewTestPriorityQueue()

		for _, e := range test.elements {
			pq.Add(NewTestElementWithPriority(int64(e)))
		}

		if got, want := pq.Len(), len(test.expected); got != want {
			t.Errorf("%d: pq.Len() = %d, want %d", i, got, want)
		}

		for _, want := range test.expected {
			got, ok := pq.Peek()
			if !ok {
				t.Errorf("%d: pq.Peek() = (_, false), want (%d, true)", i, want)
				return
			}

			if got.Priority() != int64(want) {
				t.Errorf("%d: pq.Peek() = (%d, true), want (%d, true)", i, got, want)
			}

			got, ok = pq.Next()
			if !ok {
				t.Errorf("%d: pq.Next() = (_, false), want (%d, true)", i, want)
				return
			}

			if got.Priority() != int64(want) {
				t.Errorf("%d: pq.Next() = (%d, true), want (%d, true)", i, got, want)
			}
		}

		if got, want := pq.Len(), 0; got != want {
			t.Errorf("%d: pq.Len() = %d, want %d", i, got, want)
		}
	}
}
