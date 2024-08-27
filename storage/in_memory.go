package storage

import (
	"fmt"
)

type Storage interface {
	Save(shortURL, originalURL string) error
	Get(shortURL string) (string, error)
}

type InMemoryStorage struct {
	data map[string]string
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{data: make(map[string]string)}
}

func (s *InMemoryStorage) Save(shortURL, originalURL string) error {
	s.data[shortURL] = originalURL
	return nil
}

func (s *InMemoryStorage) Get(shortURL string) (string, error) {
	if url, ok := s.data[shortURL]; ok {
		return url, nil
	}
	return "", fmt.Errorf("URL not found")
}
