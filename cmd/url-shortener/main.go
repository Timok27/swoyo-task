package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"swoyo-task/internal/config"
	"swoyo-task/storage"

	_ "github.com/lib/pq"
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

type URLShortener struct {
	store   storage.Storage
	urlBase string
}

func NewURLShortener(store storage.Storage, urlBase string) *URLShortener {
	return &URLShortener{
		store:   store,
		urlBase: urlBase,
	}
}

func encodeBase62(num *big.Int) string {
	if num.Cmp(big.NewInt(0)) == 0 {
		return string(base62Chars[0])
	}

	var encoded strings.Builder
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := &big.Int{}

	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		encoded.WriteByte(base62Chars[mod.Int64()])
	}

	return reverse(encoded.String())
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func generateShortKey(longURL string) string {
	hash := sha256.Sum256([]byte(longURL))
	hashNum := new(big.Int).SetBytes(hash[:])
	encodedKey := encodeBase62(hashNum)

	if len(encodedKey) > 7 {
		return encodedKey[:7]
	}
	return encodedKey
}

func isValidURL(u string) bool {
	parsedURL, err := url.ParseRequestURI(u)
	return err == nil && parsedURL.Scheme != "" && parsedURL.Host != ""
}

func (us *URLShortener) Shorten(longURL string) (string, error) {
	if !isValidURL(longURL) {
		return "", fmt.Errorf("неправильный URL")
	}

	shortKey := generateShortKey(longURL)
	shortURL := us.urlBase + "/" + shortKey

	if err := us.store.Save(shortKey, longURL); err != nil {
		return "", err
	}

	return shortURL, nil
}

func (us *URLShortener) Expand(shortKey string) (string, error) {
	longURL, err := us.store.Get(shortKey)
	if err != nil {
		return "", err
	}
	return longURL, nil
}

func shortenHandler(us *URLShortener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			URL string `json:"url"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Неверный запрос", http.StatusBadRequest)
			return
		}

		if req.URL == "" {
			http.Error(w, "URL обязателен", http.StatusBadRequest)
			return
		}

		shortURL, err := us.Shorten(req.URL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка сокращения URL: %v", err), http.StatusInternalServerError)
			return
		}

		resp := map[string]string{"short_url": shortURL}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func expandHandler(us *URLShortener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		pathSegments := strings.Split(r.URL.Path, "/")
		shortKey := pathSegments[len(pathSegments)-1]

		if shortKey == "" {
			http.Error(w, "Короткий URL не найден", http.StatusBadRequest)
			return
		}

		longURL, err := us.Expand(shortKey)
		if err != nil {
			http.Error(w, "URL не найден", http.StatusNotFound)
			return
		}

		resp := map[string]string{"long_url": longURL}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func main() {
	usePostgres := flag.Bool("d", false, "Используйте PostgreSQL в качестве хранилища")
	flag.Parse()

	cfg, err := config.LoadConfig("config/local.yaml")
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию: %v", err)
	}

	var store storage.Storage
	if *usePostgres {
		fmt.Println("Используется Postgres")
		store, err = storage.NewPostgresStorage(cfg.Postgres.ConnectionString())
		if err != nil {
			log.Fatalf("Не удалось инициализировать хранилище Postgres: %v", err)
		}

		if err = storage.CheckAndCreateTable(store.(*storage.PostgresStorage)); err != nil {
			log.Fatalf("Не удалось создать таблицы: %v", err)
		}
	} else {
		fmt.Println("Используется хранилище в памяти")
		store = storage.NewInMemoryStorage()
	}

	us := NewURLShortener(store, "http://localhost:8080")

	http.HandleFunc("/shorten", shortenHandler(us))
	http.HandleFunc("/", expandHandler(us))

	fmt.Println("Сервер запущен на порту: 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
