package pacing

type roundRobin struct {
	items []uint32
	index int
}

func (rr *roundRobin) Add(item uint32) {
	for _, existing := range rr.items {
		if existing == item {
			return
		}
	}
	rr.items = append(rr.items, item)
}

func (rr *roundRobin) Remove(item uint32) {
	for i, existing := range rr.items {
		if existing == item {
			rr.items = append(rr.items[:i], rr.items[i+1:]...)
			if rr.index == len(rr.items) {
				rr.index = 0
			}
			return
		}
	}
}

func (rr *roundRobin) Next() (uint32, bool) {
	if len(rr.items) == 0 {
		return 0, false
	}

	item := rr.items[rr.index]
	rr.index = (rr.index + 1) % len(rr.items)

	return item, true
}

func (rr *roundRobin) Size() int {
	return len(rr.items)
}
