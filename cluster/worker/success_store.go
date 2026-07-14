package worker

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"bilibili-ticket-golang/cluster/domain"
)

type SuccessStore struct {
	mu      sync.Mutex
	path    string
	results map[string]domain.ExecutionResult
	acked   map[string]bool
	order   []string
}

type successStoreRecord struct {
	Type      string                  `json:"type"`
	AttemptID string                  `json:"attemptId,omitempty"`
	Result    *domain.ExecutionResult `json:"result,omitempty"`
}

const maxCachedSuccessResults = 10000

func OpenSuccessStore(path string) (*SuccessStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	s := &SuccessStore{path: path, results: make(map[string]domain.ExecutionResult), acked: make(map[string]bool)}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_ = os.Chmod(path, 0600)
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			handled := false
			var record successStoreRecord
			if json.Unmarshal(line, &record) == nil && record.Type != "" {
				handled = true
				switch record.Type {
				case "result":
					if record.Result != nil && record.Result.AttemptID != "" {
						s.putResultLocked(*record.Result)
						s.acked[record.Result.AttemptID] = false
					}
				case "ack":
					if record.AttemptID != "" {
						s.acked[record.AttemptID] = true
					}
				}
			}
			// Backwards compatibility: historical files stored raw results.
			if !handled {
				var result domain.ExecutionResult
				if json.Unmarshal(line, &result) == nil && result.AttemptID != "" {
					s.putResultLocked(result)
					s.acked[result.AttemptID] = false
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
	}
	s.trimLocked()
	return s, nil
}

func (s *SuccessStore) Append(result domain.ExecutionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	persisted := result
	persisted.Credentials = domain.Credentials{}
	err := s.appendRecordLocked(successStoreRecord{Type: "result", Result: &persisted})
	if err == nil {
		s.putResultLocked(persisted)
		s.acked[result.AttemptID] = false
		s.trimLocked()
	}
	return err
}

// Ack durably records that the employer has persisted the result. Until this
// record is fsynced, Unacked continues to return the result for redelivery.
func (s *SuccessStore) Ack(attemptID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.results[attemptID]; !ok || s.acked[attemptID] {
		return nil
	}
	if err := s.appendRecordLocked(successStoreRecord{Type: "ack", AttemptID: attemptID}); err != nil {
		return err
	}
	s.acked[attemptID] = true
	s.trimLocked()
	return nil
}

func (s *SuccessStore) appendRecordLocked(record successStoreRecord) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return err
}

func (s *SuccessStore) putResultLocked(result domain.ExecutionResult) {
	if _, exists := s.results[result.AttemptID]; !exists {
		s.order = append(s.order, result.AttemptID)
	}
	s.results[result.AttemptID] = result
}

func (s *SuccessStore) trimLocked() {
	for len(s.order) > maxCachedSuccessResults {
		remove := -1
		for i, id := range s.order {
			if s.acked[id] {
				remove = i
				break
			}
		}
		if remove < 0 {
			return // unacknowledged results are never evicted
		}
		id := s.order[remove]
		s.order = append(s.order[:remove], s.order[remove+1:]...)
		delete(s.results, id)
		delete(s.acked, id)
	}
}

func (s *SuccessStore) Unacked() []domain.ExecutionResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.ExecutionResult, 0)
	for _, id := range s.order {
		if !s.acked[id] {
			out = append(out, s.results[id])
		}
	}
	return out
}

func (s *SuccessStore) IsUnacked(attemptID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.results[attemptID]
	return ok && !s.acked[attemptID]
}

func (s *SuccessStore) All() map[string]domain.ExecutionResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]domain.ExecutionResult, len(s.results))
	for k, v := range s.results {
		out[k] = v
	}
	return out
}

func (s *SuccessStore) Get(attemptID string) (domain.ExecutionResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[attemptID]
	return result, ok
}
