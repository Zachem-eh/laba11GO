package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// dbTimeout — сколько ждём ответа базы на один запрос
const dbTimeout = 5 * time.Second

// newDBContext создаёт контекст с таймаутом для одного обращения к базе.
// Создаём его ПОСЛЕ ввода с клавиатуры, чтобы время, пока человек печатает, не съедало таймаут.
func newDBContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbTimeout)
}

// ---------- Ввод с клавиатуры ----------

// readLine печатает подсказку и читает строку без пробелов по краям
func readLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("не удалось прочитать ввод: %w", err)
	}
	return strings.TrimSpace(input), nil
}

// readIntDefault читает целое число; пустой ввод (Enter) возвращает def
func readIntDefault(reader *bufio.Reader, prompt string, def int) (int, error) {
	s, err := readLine(reader, prompt)
	if err != nil {
		return 0, err
	}
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q — не целое число", s)
	}
	return n, nil
}

// readFloatDefault читает дробное число (можно через запятую); пустой ввод возвращает def
func readFloatDefault(reader *bufio.Reader, prompt string, def float64) (float64, error) {
	s, err := readLine(reader, prompt)
	if err != nil {
		return 0, err
	}
	if s == "" {
		return def, nil
	}
	x, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil {
		return 0, fmt.Errorf("%q — не число", s)
	}
	return x, nil
}

// readID читает обязательный положительный id
func readID(reader *bufio.Reader, prompt string) (int64, error) {
	n, err := readIntDefault(reader, prompt, 0)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, errors.New("id должен быть положительным числом")
	}
	return int64(n), nil
}

// toNullString превращает пустую строку в NULL, а непустую — в обычное значение
func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// validateProduct проверяет поля товара до отправки в базу
func validateProduct(p Product) error {
	if p.CategoryID <= 0 {
		return errors.New("id категории должен быть положительным")
	}
	if p.Name == "" {
		return errors.New("название не может быть пустым")
	}
	if p.Price < 0 {
		return errors.New("укажите цену не меньше 0")
	}
	if p.AvailableCount < 0 {
		return errors.New("количество не может быть отрицательным")
	}
	return nil
}

// ---------- Вывод ----------

// printProduct выводит один товар подробно
func printProduct(p Product) {
	desc := "(нет)"
	if p.Description.Valid {
		desc = p.Description.String
	}
	fmt.Printf("ID:          %d\n", p.ID)
	fmt.Printf("Категория:   %d\n", p.CategoryID)
	fmt.Printf("Название:    %s\n", p.Name)
	fmt.Printf("Описание:    %s\n", desc)
	fmt.Printf("Цена:        %.2f\n", p.Price)
	fmt.Printf("Количество:  %d\n", p.AvailableCount)
}

// printProductsTable выводит товары таблицей
func printProductsTable(products []ProductWithCategory) {
	separator := strings.Repeat("=", 80)
	fmt.Println(separator)
	fmt.Printf("%-4s | %-20s | %-20s | %-10s | %-10s\n", "ID", "Категория", "Товар", "Цена", "Количество")
	fmt.Println(separator)
	for _, p := range products {
		fmt.Printf("%-4d | %-20s | %-20s | %-10.2f | %-10d\n",
			p.ID, p.CategoryName, p.Name, p.Price, p.AvailableCount)
	}
	fmt.Println(separator)
}

// printCategories выводит список категорий
func printCategories(db *sql.DB) error {
	ctx, cancel := newDBContext()
	defer cancel()

	categories, err := ListCategories(ctx, db)
	if err != nil {
		return err
	}
	if len(categories) == 0 {
		fmt.Println("Категорий пока нет.")
		return nil
	}
	fmt.Println("Категории:")
	for _, c := range categories {
		if c.Description.Valid {
			fmt.Printf("  %d — %s (%s)\n", c.ID, c.Name, c.Description.String)
		} else {
			fmt.Printf("  %d — %s\n", c.ID, c.Name)
		}
	}
	return nil
}

// ---------- Пункты меню ----------

// handleGetProduct — пункт «Найти товар по ID»
func handleGetProduct(reader *bufio.Reader, db *sql.DB) error {
	id, err := readID(reader, "ID товара: ")
	if err != nil {
		return err
	}

	ctx, cancel := newDBContext()
	defer cancel()

	p, err := GetProductByID(ctx, db, id)
	if err != nil {
		return err
	}
	printProduct(p)
	return nil
}

// handleListProducts — пункт «Список товаров с фильтрами и пагинацией»
func handleListProducts(reader *bufio.Reader, db *sql.DB) error {
	fmt.Println("Фильтры (Enter — не фильтровать):")
	if err := printCategories(db); err != nil {
		return err
	}

	var f ProductFilter
	categoryID, err := readIntDefault(reader, "ID категории: ", 0)
	if err != nil {
		return err
	}
	f.CategoryID = int64(categoryID)

	if f.NamePart, err = readLine(reader, "Часть названия: "); err != nil {
		return err
	}
	if f.MinPrice, err = readFloatDefault(reader, "Цена от: ", 0); err != nil {
		return err
	}
	if f.MaxPrice, err = readFloatDefault(reader, "Цена до: ", 0); err != nil {
		return err
	}
	if f.PageSize, err = readIntDefault(reader, "Товаров на странице (по умолчанию 5): ", 5); err != nil {
		return err
	}
	if f.PageSize <= 0 {
		return errors.New("размер страницы должен быть больше нуля")
	}
	f.Page = 1

	// Листаем страницы, пока пользователь не нажмёт Enter
	for {
		ctx, cancel := newDBContext()
		products, total, err := ListProducts(ctx, db, f)
		cancel() // запрос выполнен, контекст больше не нужен
		if err != nil {
			return err
		}

		if total == 0 {
			fmt.Println("Под фильтр не подошёл ни один товар.")
			return nil
		}

		pages := (total + f.PageSize - 1) / f.PageSize // деление с округлением вверх
		printProductsTable(products)
		fmt.Printf("Страница %d из %d, всего товаров: %d\n", f.Page, pages, total)

		cmd, err := readLine(reader, "n — следующая, p — предыдущая, Enter — в меню: ")
		if err != nil {
			return err
		}
		switch cmd {
		case "n":
			if f.Page < pages {
				f.Page++
			} else {
				fmt.Println("Это последняя страница.")
			}
		case "p":
			if f.Page > 1 {
				f.Page--
			} else {
				fmt.Println("Это первая страница.")
			}
		default:
			return nil
		}
	}
}

// handleCreateProduct — пункт «Добавить товар»
func handleCreateProduct(reader *bufio.Reader, db *sql.DB) error {
	if err := printCategories(db); err != nil {
		return err
	}

	var p Product
	var err error
	if p.CategoryID, err = readID(reader, "ID категории: "); err != nil {
		return err
	}
	if p.Name, err = readLine(reader, "Название: "); err != nil {
		return err
	}
	desc, err := readLine(reader, "Описание (Enter — без описания): ")
	if err != nil {
		return err
	}
	p.Description = toNullString(desc)
	if p.Price, err = readFloatDefault(reader, "Цена: ", -1); err != nil {
		return err
	}
	if p.AvailableCount, err = readIntDefault(reader, "Количество (по умолчанию 0): ", 0); err != nil {
		return err
	}

	if err := validateProduct(p); err != nil {
		return err
	}

	ctx, cancel := newDBContext()
	defer cancel()

	id, err := CreateProduct(ctx, db, p)
	if err != nil {
		return err
	}
	fmt.Printf("Товар добавлен, его id = %d\n", id)
	return nil
}

// handleUpdateProduct — пункт «Изменить товар»: Enter оставляет старое значение
func handleUpdateProduct(reader *bufio.Reader, db *sql.DB) error {
	id, err := readID(reader, "ID товара: ")
	if err != nil {
		return err
	}

	// Сначала читаем текущий товар, чтобы показать старые значения
	ctx, cancel := newDBContext()
	p, err := GetProductByID(ctx, db, id)
	cancel()
	if err != nil {
		return err
	}
	fmt.Println("Сейчас в базе:")
	printProduct(p)
	fmt.Println("Введите новые значения (Enter — оставить как есть, \"-\" в описании — убрать его):")

	categoryID, err := readIntDefault(reader, fmt.Sprintf("ID категории [%d]: ", p.CategoryID), int(p.CategoryID))
	if err != nil {
		return err
	}
	p.CategoryID = int64(categoryID)

	name, err := readLine(reader, fmt.Sprintf("Название [%s]: ", p.Name))
	if err != nil {
		return err
	}
	if name != "" {
		p.Name = name
	}

	desc, err := readLine(reader, "Описание: ")
	if err != nil {
		return err
	}
	if desc == "-" {
		p.Description = sql.NullString{} // Valid = false → в базу запишется NULL
	} else if desc != "" {
		p.Description = toNullString(desc)
	}

	if p.Price, err = readFloatDefault(reader, fmt.Sprintf("Цена [%.2f]: ", p.Price), p.Price); err != nil {
		return err
	}
	if p.AvailableCount, err = readIntDefault(reader, fmt.Sprintf("Количество [%d]: ", p.AvailableCount), p.AvailableCount); err != nil {
		return err
	}

	if err := validateProduct(p); err != nil {
		return err
	}

	ctx, cancel = newDBContext()
	defer cancel()

	if err := UpdateProduct(ctx, db, p); err != nil {
		return err
	}
	fmt.Println("Товар обновлён.")
	return nil
}

// handleDeleteProduct — пункт «Удалить товар»
func handleDeleteProduct(reader *bufio.Reader, db *sql.DB) error {
	id, err := readID(reader, "ID товара: ")
	if err != nil {
		return err
	}
	answer, err := readLine(reader, "Точно удалить? (y/n): ")
	if err != nil {
		return err
	}
	if answer != "y" {
		fmt.Println("Удаление отменено.")
		return nil
	}

	ctx, cancel := newDBContext()
	defer cancel()

	if err := DeleteProduct(ctx, db, id); err != nil {
		return err
	}
	fmt.Println("Товар удалён.")
	return nil
}

// handleListCategories — пункт «Список категорий»
func handleListCategories(db *sql.DB) error {
	return printCategories(db)
}

// handleCreateCategory — пункт «Добавить категорию»
func handleCreateCategory(reader *bufio.Reader, db *sql.DB) error {
	var c Category
	var err error
	if c.Name, err = readLine(reader, "Название категории: "); err != nil {
		return err
	}
	if c.Name == "" {
		return errors.New("название не может быть пустым")
	}
	desc, err := readLine(reader, "Описание (Enter — без описания): ")
	if err != nil {
		return err
	}
	c.Description = toNullString(desc)

	ctx, cancel := newDBContext()
	defer cancel()

	id, err := CreateCategory(ctx, db, c)
	if err != nil {
		return err
	}
	fmt.Printf("Категория добавлена, её id = %d\n", id)
	return nil
}

// handleDeleteCategory — пункт «Удалить категорию»
func handleDeleteCategory(reader *bufio.Reader, db *sql.DB) error {
	if err := printCategories(db); err != nil {
		return err
	}
	id, err := readID(reader, "ID категории: ")
	if err != nil {
		return err
	}

	ctx, cancel := newDBContext()
	defer cancel()

	if err := DeleteCategory(ctx, db, id); err != nil {
		return err
	}
	fmt.Println("Категория удалена.")
	return nil
}
