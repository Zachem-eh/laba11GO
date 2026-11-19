package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound возвращается, когда записи с таким id нет в базе
var ErrNotFound = errors.New("запись не найдена")

// ErrHasProducts возвращается при попытке удалить категорию, в которой есть товары
var ErrHasProducts = errors.New("в категории есть товары, сначала удалите или перенесите их")

// ErrHasLinks возвращается, когда удалить запись не даёт внешний ключ
var ErrHasLinks = errors.New("на запись ссылаются другие таблицы, удаление запрещено")

// ProductFilter хранит условия для списка товаров.
// Нулевое значение поля означает «не фильтровать по нему».
type ProductFilter struct {
	CategoryID int64   // 0 — любая категория
	NamePart   string  // "" — любое название
	MinPrice   float64 // 0 — без нижней границы
	MaxPrice   float64 // 0 — без верхней границы
	Page       int     // номер страницы, начиная с 1
	PageSize   int     // сколько товаров на странице
}

// isForeignKeyError проверяет, что ошибка пришла от проверки внешнего ключа в SQLite
func isForeignKeyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

// ---------- Товары ----------

// CreateProduct добавляет товар и возвращает его новый id
func CreateProduct(ctx context.Context, db *sql.DB, p Product) (int64, error) {
	res, err := db.ExecContext(ctx,
		"INSERT INTO products (category_id, name, description, price, available_count) VALUES (?, ?, ?, ?, ?)",
		p.CategoryID, p.Name, p.Description, p.Price, p.AvailableCount,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return 0, fmt.Errorf("категории с id %d нет: %w", p.CategoryID, ErrNotFound)
		}
		return 0, fmt.Errorf("ошибка добавления товара: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("не удалось получить id нового товара: %w", err)
	}
	return id, nil
}

// GetProductByID возвращает один товар по id
func GetProductByID(ctx context.Context, db *sql.DB, id int64) (Product, error) {
	var p Product
	err := db.QueryRowContext(ctx,
		"SELECT id, category_id, name, description, price, available_count FROM products WHERE id = ?",
		id,
	).Scan(&p.ID, &p.CategoryID, &p.Name, &p.Description, &p.Price, &p.AvailableCount)

	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, fmt.Errorf("товар с id %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Product{}, fmt.Errorf("ошибка чтения товара: %w", err)
	}
	return p, nil
}

// ListProducts возвращает одну страницу товаров по фильтру и общее число подходящих товаров
func ListProducts(ctx context.Context, db *sql.DB, f ProductFilter) ([]ProductWithCategory, int, error) {
	// Собираем условия WHERE по кусочкам: только те, что пользователь задал
	var conditions []string
	var args []any

	if f.CategoryID > 0 {
		conditions = append(conditions, "p.category_id = ?")
		args = append(args, f.CategoryID)
	}
	if f.NamePart != "" {
		conditions = append(conditions, "p.name LIKE ?")
		args = append(args, "%"+f.NamePart+"%")
	}
	if f.MinPrice > 0 {
		conditions = append(conditions, "p.price >= ?")
		args = append(args, f.MinPrice)
	}
	if f.MaxPrice > 0 {
		conditions = append(conditions, "p.price <= ?")
		args = append(args, f.MaxPrice)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 1. Сколько всего товаров подходит под фильтр (нужно, чтобы посчитать страницы)
	var total int
	countQuery := "SELECT COUNT(*) FROM products p " + where
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчёта товаров: %w", err)
	}

	// 2. Сами товары нужной страницы
	if f.PageSize <= 0 {
		f.PageSize = 5
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.PageSize

	query := `
		SELECT p.id, p.name, p.price, p.available_count, p.description, c.name
		FROM products p
		JOIN categories c ON p.category_id = c.id
		` + where + `
		ORDER BY p.id
		LIMIT ? OFFSET ?`
	args = append(args, f.PageSize, offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer rows.Close()

	var products []ProductWithCategory
	for rows.Next() {
		var p ProductWithCategory
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.AvailableCount, &p.Description, &p.CategoryName); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования строки: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ошибка во время итерации: %w", err)
	}

	return products, total, nil
}

// UpdateProduct меняет все поля товара с id = p.ID
func UpdateProduct(ctx context.Context, db *sql.DB, p Product) error {
	res, err := db.ExecContext(ctx,
		"UPDATE products SET category_id = ?, name = ?, description = ?, price = ?, available_count = ? WHERE id = ?",
		p.CategoryID, p.Name, p.Description, p.Price, p.AvailableCount, p.ID,
	)
	if err != nil {
		if isForeignKeyError(err) {
			return fmt.Errorf("категории с id %d нет: %w", p.CategoryID, ErrNotFound)
		}
		return fmt.Errorf("ошибка обновления товара: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("не удалось узнать число изменённых строк: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("товар с id %d: %w", p.ID, ErrNotFound)
	}
	return nil
}

// DeleteProduct удаляет товар по id
func DeleteProduct(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, "DELETE FROM products WHERE id = ?", id)
	if err != nil {
		// Пригодится, когда появится таблица sales: товар с продажами удалить нельзя
		if isForeignKeyError(err) {
			return fmt.Errorf("товар с id %d: %w", id, ErrHasLinks)
		}
		return fmt.Errorf("ошибка удаления товара: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("не удалось узнать число удалённых строк: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("товар с id %d: %w", id, ErrNotFound)
	}
	return nil
}

// ---------- Категории ----------

// ListCategories возвращает все категории
func ListCategories(ctx context.Context, db *sql.DB) ([]Category, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, name, description FROM categories ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения категорий: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description); err != nil {
			return nil, fmt.Errorf("ошибка сканирования категории: %w", err)
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка во время итерации: %w", err)
	}
	return categories, nil
}

// CreateCategory добавляет категорию и возвращает её id
func CreateCategory(ctx context.Context, db *sql.DB, c Category) (int64, error) {
	res, err := db.ExecContext(ctx,
		"INSERT INTO categories (name, description) VALUES (?, ?)",
		c.Name, c.Description,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return 0, fmt.Errorf("категория %q уже есть", c.Name)
		}
		return 0, fmt.Errorf("ошибка добавления категории: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("не удалось получить id новой категории: %w", err)
	}
	return id, nil
}

// DeleteCategory удаляет категорию, только если в ней нет товаров (ON DELETE RESTRICT)
func DeleteCategory(ctx context.Context, db *sql.DB, id int64) error {
	// Сначала сами проверяем товары, чтобы сказать пользователю понятную причину
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM products WHERE category_id = ?", id).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка проверки товаров категории: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("категория с id %d (товаров: %d): %w", id, count, ErrHasProducts)
	}

	res, err := db.ExecContext(ctx, "DELETE FROM categories WHERE id = ?", id)
	if err != nil {
		// Подстраховка: если товар добавили между проверкой и удалением, сработает внешний ключ
		if isForeignKeyError(err) {
			return fmt.Errorf("категория с id %d: %w", id, ErrHasProducts)
		}
		return fmt.Errorf("ошибка удаления категории: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("не удалось узнать число удалённых строк: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("категория с id %d: %w", id, ErrNotFound)
	}
	return nil
}
