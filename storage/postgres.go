package storage

import (
	"database/sql"
	"fmt"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(connStr string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return &PostgresStorage{db: db}, nil
}

func CheckAndCreateTable(s *PostgresStorage) error {
	checkQuery := `
    SELECT EXISTS (
        SELECT 1
        FROM information_schema.tables 
        WHERE table_name = 'urls'
    );
    `

	var exists bool
	err := s.db.QueryRow(checkQuery).Scan(&exists)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования таблицы: %w", err)
	}

	if !exists {
		createQuery := `
        CREATE TABLE urls (
            id SERIAL PRIMARY KEY,
            short_url VARCHAR(255) UNIQUE NOT NULL,
            original_url TEXT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        `
		_, err = s.db.Exec(createQuery)
		if err != nil {
			return fmt.Errorf("ошибка создании таблицы 'urls': %w", err)
		}
	}
	return nil
}

func (s *PostgresStorage) Save(shortURL, originalURL string) error {
	query := `INSERT INTO urls (short_url, original_url) VALUES ($1, $2)`
	_, err := s.db.Exec(query, shortURL, originalURL)
	if err != nil {
		return fmt.Errorf("не удалось сохранить URL: %v", err)
	}
	return nil
}

func (s *PostgresStorage) Get(shortURL string) (string, error) {
	var originalURL string
	query := `SELECT original_url FROM urls WHERE short_url = $1`
	err := s.db.QueryRow(query, shortURL).Scan(&originalURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("URL не найден")
		}
		return "", fmt.Errorf("не удалось получить URL: %v", err)
	}
	return originalURL, nil
}
