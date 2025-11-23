// e2e/suite_test.go
//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	dbContainer *postgres.PostgresContainer
	dbURL       string // postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Запускаем PostgreSQL контейнер
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("Не удалось запустить контейнер PostgreSQL: %v", err)
	}
	dbContainer = container

	// 2. Получаем connection string
	dbURL, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("Не удалось получить connection string: %v", err)
	}

	// 3. Применяем твои SQL-миграции из папки database/migrations
	if err := applyMigrations(dbURL); err != nil {
		log.Fatalf("Ошибка применения миграций: %v", err)
	}

	// 4. Запускаем тесты
	code := m.Run()

	// 5. Останавливаем контейнер
	_ = dbContainer.Terminate(ctx)

	os.Exit(code)
}

// Применяем миграции используя библиотеку migrate напрямую
func applyMigrations(databaseURL string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Ищем go.mod вверх по дереву
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return fmt.Errorf("go.mod not found")
		}
		projectRoot = parent
	}

	migrationsPath := filepath.Join(projectRoot, "database", "migrations")

	// Try to use migrate CLI first (if available)
	migrateCmd := "migrate"
	if _, err := exec.LookPath("migrate"); err == nil {
		cmd := exec.Command(
			migrateCmd,
			"-path", migrationsPath,
			"-database", databaseURL,
			"up",
		)
		_, err := cmd.CombinedOutput()
		if err == nil {
			fmt.Printf("✓ Миграции успешно применены из %s (CLI)\n", migrationsPath)
			return nil
		}
		// If CLI fails, fall back to library
	}

	// Fall back to using migrate library directly
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	driver, err := pgmigrate.WithInstance(db, &pgmigrate.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	fmt.Printf("✓ Миграции успешно применены из %s (library)\n", migrationsPath)
	return nil
}

// Используй эту функцию в каждом e2e-тесте — она возвращает чистую БД
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err, "Не удалось подключиться к тестовой БД")

	// Полная очистка всех таблиц в схеме public
	_, err = db.Exec(`
		DO $$ DECLARE
		    r RECORD;
		BEGIN
		    -- Отключаем проверку внешних ключей на время очистки
		    SET session_replication_role = replica;

		    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
		        EXECUTE 'TRUNCATE TABLE ' || quote_ident(r.tablename) || ' RESTART IDENTITY CASCADE';
		    END LOOP;

		    SET session_replication_role = origin;
		END $$;
	`)
	require.NoError(t, err, "Не удалось очистить таблицы")

	t.Cleanup(func() { db.Close() })
	return db
}
