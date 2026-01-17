package tikistore

import "github.com/boolean-maybe/tiki/store"

// AddListener registers a callback for change notifications.
// returns a listener ID that can be used to remove the listener.
func (s *TikiStore) AddListener(listener store.ChangeListener) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextListenerID
	s.nextListenerID++
	s.listeners[id] = listener
	return id
}

// RemoveListener removes a previously registered listener by ID
func (s *TikiStore) RemoveListener(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.listeners, id)
}

func (s *TikiStore) notifyListeners() {
	s.mu.RLock()
	listeners := make([]store.ChangeListener, 0, len(s.listeners))
	for _, l := range s.listeners {
		listeners = append(listeners, l)
	}
	s.mu.RUnlock()

	for _, l := range listeners {
		l()
	}
}
