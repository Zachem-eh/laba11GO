package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Структура для хранения результата JOIN-запроса
type ProductWithCategory struct {
	ID             int64
	Name           string
	Price          float64
	AvailableCount int
	Description    sql.NullString
	CategoryName   string
}

// GetAllProducts выводит список всех товаров с названиями их категорий
func GetAllProducts(ctx context.Context, db *sql.DB) error {
	// SQL-запрос с использованием INNER JOIN
	query := `
		SELECT 
			p.id, 
			p.name, 
			p.price, 
			p.available_count, 
			p.description, 
			c.name AS category_name
		FROM products p
		JOIN categories c ON p.category_id = c.id
		ORDER BY c.name, p.name;
	`

	// Выполняем запрос. ВАЖНО: используем QueryContext, так как ожидаем много строк
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	// закрываем rows, чтобы не было утечки соединений
	defer rows.Close()

	separator := strings.Repeat("=", 91)

	// Красивый вывод заголовка таблицы
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("%-4s | %-20s | %-20s | %-10s | %-10s\n", "ID", "Категория", "Товар", "Цена", "Количество в ассортименте")
	fmt.Printf("%s\n", separator)

	count := 0
	// Итерируемся по полученным строкам
	for rows.Next() {
		var p ProductWithCategory

		// Сканируем колонки в поля нашей структуры.
		// Порядок переменных должен строго совпадать с порядком колонок в SELECT!
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Price,
			&p.AvailableCount,
			&p.Description,
			&p.CategoryName,
		)
		if err != nil {
			return fmt.Errorf("ошибка сканирования строки: %w", err)
		}

		// Красивый вывод строки
		fmt.Printf("%-4d | %-20s | %-20s | %-10.2f | %-10d\n",
			p.ID, p.CategoryName, p.Name, p.Price, p.AvailableCount)
		count++
	}

	// Проверяем rows.Err() после цикла
	// Это отловит ошибки, которые могли возникнуть в процессе чтения данных из БД.
	if err = rows.Err(); err != nil {
		return fmt.Errorf("ошибка во время итерации: %w", err)
	}

	if count == 0 {
		fmt.Println("В базе данных пока нет товаров.")
	} else {
		fmt.Printf("\nВсего найдено товаров: %d\n", count)
	}
	fmt.Printf("%s\n", separator)

	return nil
}
