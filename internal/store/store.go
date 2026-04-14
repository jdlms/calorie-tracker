package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joshua-lamorey/calorie-counter/internal/model"
	bbolt "go.etcd.io/bbolt"
)

const (
	entriesBucket  = "entries"
	metadataBucket = "metadata"
)

// Store persists entries in BoltDB.
type Store struct {
	db *bbolt.DB
}

// Summary aggregates nutrition totals for a period.
type Summary struct {
	Entries int `json:"entries"`
	Kcal    int `json:"kcal"`
	Protein int `json:"protein"`
	Fat     int `json:"fat"`
	Carbs   int `json:"carbs"`
}

// Open opens the BoltDB database and ensures required buckets exist.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	db, err := bbolt.Open(path, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening boltdb: %w", err)
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(entriesBucket))
		if err != nil {
			return fmt.Errorf("creating entries bucket: %w", err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(metadataBucket))
		if err != nil {
			return fmt.Errorf("creating metadata bucket: %w", err)
		}

		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing boltdb: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}

	return s.db.Close()
}

// SaveEntry stores an entry.
func (s *Store) SaveEntry(ctx context.Context, entry model.Entry) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("checking context: %w", err)
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}

	key := []byte(entryKey(entry))

	if err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return fmt.Errorf("entries bucket missing")
		}

		if err := bucket.Put(key, payload); err != nil {
			return fmt.Errorf("putting entry: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("saving entry: %w", err)
	}

	return nil
}

// EntriesByDate returns all entries for a given UTC day in YYYY-MM-DD format.
func (s *Store) EntriesByDate(ctx context.Context, date string) ([]model.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("checking context: %w", err)
	}

	prefix := date + ":"
	entries := make([]model.Entry, 0)

	if err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return fmt.Errorf("entries bucket missing")
		}

		cursor := bucket.Cursor()
		for key, value := cursor.Seek([]byte(prefix)); key != nil && strings.HasPrefix(string(key), prefix); key, value = cursor.Next() {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("checking context during scan: %w", err)
			}

			var entry model.Entry
			if err := json.Unmarshal(value, &entry); err != nil {
				return fmt.Errorf("unmarshaling entry %q: %w", string(key), err)
			}

			entries = append(entries, entry)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("listing entries by date: %w", err)
	}

	return entries, nil
}

// Summary returns totals for entries between start and end, inclusive.
func (s *Store) Summary(ctx context.Context, start, end time.Time) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, fmt.Errorf("checking context: %w", err)
	}

	var summary Summary

	if err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return fmt.Errorf("entries bucket missing")
		}

		cursor := bucket.Cursor()
		startPrefix := []byte(start.UTC().Format(time.DateOnly))
		for key, value := cursor.Seek(startPrefix); key != nil; key, value = cursor.Next() {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("checking context during summary scan: %w", err)
			}

			var entry model.Entry
			if err := json.Unmarshal(value, &entry); err != nil {
				return fmt.Errorf("unmarshaling entry %q: %w", string(key), err)
			}

			ts := time.Unix(entry.Timestamp, 0).UTC()
			if ts.After(end) {
				break
			}
			if ts.Before(start) {
				continue
			}

			summary.Entries++
			summary.Kcal += entry.Kcal
			summary.Protein += entry.Protein
			summary.Fat += entry.Fat
			summary.Carbs += entry.Carbs
		}

		return nil
	}); err != nil {
		return Summary{}, fmt.Errorf("building summary: %w", err)
	}

	return summary, nil
}

// TelegramOffset returns the next Telegram update offset to request.
func (s *Store) TelegramOffset(ctx context.Context) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("checking context: %w", err)
	}

	var offset int64
	if err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket missing")
		}

		value := bucket.Get([]byte("telegram_offset"))
		if value == nil {
			return nil
		}

		if err := json.Unmarshal(value, &offset); err != nil {
			return fmt.Errorf("unmarshaling telegram offset: %w", err)
		}

		return nil
	}); err != nil {
		return 0, fmt.Errorf("loading telegram offset: %w", err)
	}

	return offset, nil
}

// SaveTelegramOffset persists the next Telegram update offset to request.
func (s *Store) SaveTelegramOffset(ctx context.Context, offset int64) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("checking context: %w", err)
	}

	payload, err := json.Marshal(offset)
	if err != nil {
		return fmt.Errorf("marshaling telegram offset: %w", err)
	}

	if err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket missing")
		}

		if err := bucket.Put([]byte("telegram_offset"), payload); err != nil {
			return fmt.Errorf("putting telegram offset: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("saving telegram offset: %w", err)
	}

	return nil
}

// TelegramFailureCount returns the number of recorded failures for an update.
func (s *Store) TelegramFailureCount(ctx context.Context, updateID int64) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("checking context: %w", err)
	}

	var count int
	if err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket missing")
		}

		value := bucket.Get([]byte(telegramFailureKey(updateID)))
		if value == nil {
			return nil
		}

		parsed, err := strconv.Atoi(string(value))
		if err != nil {
			return fmt.Errorf("parsing telegram failure count: %w", err)
		}
		count = parsed
		return nil
	}); err != nil {
		return 0, fmt.Errorf("loading telegram failure count: %w", err)
	}

	return count, nil
}

// IncrementTelegramFailureCount increments and returns the failure count for an update.
func (s *Store) IncrementTelegramFailureCount(ctx context.Context, updateID int64) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("checking context: %w", err)
	}

	var count int
	if err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket missing")
		}

		key := []byte(telegramFailureKey(updateID))
		value := bucket.Get(key)
		if value != nil {
			parsed, err := strconv.Atoi(string(value))
			if err != nil {
				return fmt.Errorf("parsing telegram failure count: %w", err)
			}
			count = parsed
		}

		count++
		if err := bucket.Put(key, []byte(strconv.Itoa(count))); err != nil {
			return fmt.Errorf("saving telegram failure count: %w", err)
		}

		return nil
	}); err != nil {
		return 0, fmt.Errorf("incrementing telegram failure count: %w", err)
	}

	return count, nil
}

// ClearTelegramFailureCount removes the failure count for an update.
func (s *Store) ClearTelegramFailureCount(ctx context.Context, updateID int64) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("checking context: %w", err)
	}

	if err := s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(metadataBucket))
		if bucket == nil {
			return fmt.Errorf("metadata bucket missing")
		}

		if err := bucket.Delete([]byte(telegramFailureKey(updateID))); err != nil {
			return fmt.Errorf("deleting telegram failure count: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("clearing telegram failure count: %w", err)
	}

	return nil
}

func entryKey(entry model.Entry) string {
	day := time.Unix(entry.Timestamp, 0).UTC().Format(time.DateOnly)
	return fmt.Sprintf("%s:%d:%s", day, entry.Timestamp, entry.ID)
}

func telegramFailureKey(updateID int64) string {
	return fmt.Sprintf("telegram_failure:%d", updateID)
}
