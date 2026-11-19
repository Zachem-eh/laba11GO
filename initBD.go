package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations запускает все миграции
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

// initFirstProducts заполняет бд начальными данными, если она пуста
func initFirstProducts(ctx context.Context, db *sql.DB) error {
	// 1. Проверяем, есть ли уже данные в таблице категорий
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM categories").Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка проверки данных: %w", err)
	}

	// Если данные уже есть, ничего не делаем (защита от дубликатов при перезапуске)
	if count > 0 {
		fmt.Println("База данных уже содержит данные, пропускаем инициализацию начальных данных.")
		return nil
	}

	// 2. Начинаем транзакцию (пункт 6 лабораторной)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	// Гарантированный откат, если что-то пойдет не так или мы просто выйдем из функции
	defer tx.Rollback()

	// 3. Вставляем категории и получаем их ID
	var DrinksID, SnacksID int64

	res, err := tx.ExecContext(ctx, "INSERT INTO categories (name, description) VALUES (?, ?)", "Газированные напитки", "Освежающие напитки")
	if err != nil {
		return fmt.Errorf("ошибка вставки категории: %w", err)
	}
	DrinksID, _ = res.LastInsertId()

	res, err = tx.ExecContext(ctx, "INSERT INTO categories (name, description) VALUES (?, ?)", "Снеки", "Шоколадные батончики и закуски")
	if err != nil {
		return fmt.Errorf("ошибка вставки категории: %w", err)
	}
	SnacksID, _ = res.LastInsertId()

	// 4. Вставляем товары, привязывая их к полученным ID категорий
	// Кока-кола (категория: Напитки, цена 100.00, остаток 50)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO products (category_id, name, description, price, available_count) VALUES (?, ?, ?, ?, ?)",
		DrinksID, "Кока-кола", "Классическая", 100.00, 52,
	)
	if err != nil {
		return fmt.Errorf("ошибка вставки товара: %w", err)
	}

	// Спрайт (категория: Напитки, цена 95.00, остаток 40)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO products (category_id, name, description, price, available_count) VALUES (?, ?, ?, ?, ?)",
		DrinksID, "Спрайт", "Со вкусом лайма", 95.00, 42,
	)
	if err != nil {
		return fmt.Errorf("ошибка вставки товара: %w", err)
	}

	// Сникерс (категория: Снеки, цена 60.00, остаток 100)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO products (category_id, name, description, price, available_count) VALUES (?, ?, ?, ?, ?)",
		SnacksID, "Сникерс", "Шоколадный батончик с нуугой", 60.00, 100,
	)
	if err != nil {
		return fmt.Errorf("ошибка вставки товара: %w", err)
	}

	// 5. Фиксируем изменения (Commit)
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	fmt.Println("База данных успешно заполнена тестовыми данными!")
	return nil
}
