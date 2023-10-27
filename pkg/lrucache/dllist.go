package lrucache

import "sync"

type DLList interface {
	Len() int
	Front() *DLListItem
	Back() *DLListItem
	PushFront(v interface{}) *DLListItem
	PushBack(v interface{}) *DLListItem
	Remove(i *DLListItem)
	MoveToFront(i *DLListItem)
}

type DLListItem struct {
	Value interface{}
	Next  *DLListItem
	Prev  *DLListItem
}

type dllist struct {
	len   int
	mu    sync.Mutex
	front *DLListItem
	back  *DLListItem
}

// Len returns the length of the dllist.
func (l *dllist) Len() int {
	// critical section start, it ends when return is called
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.len
}

// Front returns address of the front DLListItem of the dllist.
func (l *dllist) Front() *DLListItem {
	// critical section start, it ends when return is called
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.front
}

// Back returns address of the back DLListItem of the dllist.
func (l *dllist) Back() *DLListItem {
	// critical section start, it ends when return is called
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.back
}

// pushFront private method adds an already created DLListItem to the front of the dllist
// BEWARE! it assumes that the mutex lock should be set by the caller.
func (l *dllist) pushFront(i *DLListItem) *DLListItem {
	// exit if i is nil
	if i == nil {
		return nil
	}

	// initialize the new front of the dllist
	i.Prev = nil
	i.Next = l.front

	// if our front is initialized with pointer set old front to point to i (new front)
	if l.front != nil {
		l.front.Prev = i
	} else {
		// if our front is nil then it's the first element and both front and back should point to it
		l.back = i
	}

	// set front to the i
	l.front = i

	// increase array size
	l.len++
	return i
}

// PushFront adds a new value to the back of the dllist.
func (l *dllist) PushFront(v interface{}) *DLListItem {
	// critical section start, it ends when return is called
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.pushFront(&DLListItem{Value: v, Next: nil, Prev: nil})
}

// PushBack adds a new value to the back of the dllist.
func (l *dllist) PushBack(v interface{}) *DLListItem {
	// critical section start
	l.mu.Lock()
	defer l.mu.Unlock()

	// create a new DLListItem with value from v
	newback := &DLListItem{Value: v, Next: nil, Prev: l.back}

	// if out back is initilized then make sure that previous element points to new back element
	if l.back != nil {
		l.back.Next = newback
	} else {
		// if our back is nil then newback is the first element and both front and back should point to it
		l.front = newback
	}
	// set back to the newback element
	l.back = newback
	// increase array size
	l.len++
	return newback
}

// remove removes a DLListItem from the dllist
// no checking if the item is in the dllist performed
// BEWARE! it assumes that the mutex lock should be set by the caller.
func (l *dllist) remove(i *DLListItem) {
	// exit if i is nil
	if i == nil {
		return
	}

	// if previous element of element i is defined, then make sure that it points to the element that is after alement i
	if i.Prev != nil {
		i.Prev.Next = i.Next
	} else {
		// there were no previous element, so we are removing the first element. Next element to current becomes new first
		l.front = i.Next
	}
	// if next element of element i is defined, then make sure that it points back to the element that is before i
	if i.Next != nil {
		i.Next.Prev = i.Prev
	} else {
		// there was no next element, so we are removing the last element. Previous element to current becomes new last
		l.back = i.Prev
	}
	// we removed an element from the dllist reduce array size
	l.len--
}

// Remove will set mutex lock and call the remove to remove a DLListItem from the dllist.
func (l *dllist) Remove(i *DLListItem) {
	// critical section start, it ends when return is called
	l.mu.Lock()
	defer l.mu.Unlock()

	l.remove(i)
}

// MoveToFront moves a DLListItem to the front of the dllist
// no checking if the item is from the the dllist performed.
func (l *dllist) MoveToFront(i *DLListItem) {
	// critical section start, it ends when return is called
	l.mu.Lock()
	defer l.mu.Unlock()

	l.remove(i)
	l.pushFront(i)
}

// NewDLList creates a new dllist.
func NewDLList() DLList {
	return new(dllist)
}
