package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Анонимный импорт драйвера
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// функция для очистки экрана (работает и на Windows, и на macOS/Linux)
func clear_screen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

type Category struct {
	ID          int64
	Name        string
	Description sql.NullString
}

type Product struct {
	ID             int64
	CategoryID     int64
	Name           string
	Description    sql.NullString
	Price          float64 // или string/int64, если работаем с деньгами строго
	AvailableCount int
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	// Создаем таблицу для учета примененных миграций, если её нет
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("не удалось создать таблицу schema_migrations: %w", err)
	}

	// Читаем встроенные файлы
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("не удалось прочитать папку migrations: %w", err)
	}

	// Сортируем файлы по имени, чтобы 0001 применилась раньше 0002
	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	// Применяем каждую миграцию
	for _, filename := range upFiles {
		// Проверяем, применена ли уже эта миграция
		var exists int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", filename).Scan(&exists)
		if err != nil {
			return fmt.Errorf("ошибка проверки миграции %s: %w", filename, err)
		}

		if exists > 0 {
			fmt.Printf("Миграция %s уже применена, пропускаем\n", filename)
			continue
		}

		// Читаем SQL-код из файла
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("не удалось прочитать файл %s: %w", filename, err)
		}

		// Выполняем SQL-скрипт
		_, err = db.ExecContext(ctx, string(content))
		if err != nil {
			return fmt.Errorf("ошибка выполнения миграции %s: %w", filename, err)
		}

		_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", filename)
		if err != nil {
			return fmt.Errorf("не удалось сохранить статус миграции %s: %w", filename, err)
		}

		fmt.Printf("Успешно применена миграция: %s\n", filename)
	}

	return nil
}

func main() {
	clear_screen()
	db, err := sql.Open("sqlite", "file:ProductsDB.db")
	if err != nil {
		log.Fatalf("Ошибка при открытии БД: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Нет связи с БД: %v", err)
	}
	fmt.Println("Успешное подключение к SQLite (файл lab.db)")

	if err := runMigrations(ctx, db); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	fmt.Println("Инициализация завершена. База данных готова к работе!")
}
