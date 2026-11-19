package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// handleTopProducts — пункт «Топ товаров по выручке»: спрашивает фильтр и выводит результат GetTopProducts
func handleTopProducts(reader *bufio.Reader, db *sql.DB) error {
	minPrice, err := readFloatDefault(reader, "Минимальная цена товара (Enter — любая): ", 0)
	if err != nil {
		return err
	}
	if minPrice < 0 {
		return errors.New("цена не может быть отрицательной")
	}
	limit, err := readIntDefault(reader, "Сколько товаров показать (по умолчанию 5): ", 5)
	if err != nil {
		return err
	}
	if limit <= 0 {
		return errors.New("количество должно быть больше нуля")
	}

	ctx, cancel := newDBContext()
	defer cancel()

	top, err := GetTopProducts(ctx, db, minPrice, limit)
	if err != nil {
		return err
	}
	if len(top) == 0 {
		fmt.Println("Под условие не попало ни одной продажи.")
		return nil
	}

	separator := strings.Repeat("=", 80)
	fmt.Println(separator)
	fmt.Printf("%-5s | %-20s | %-20s | %-8s | %-10s\n", "Место", "Товар", "Категория", "Продано", "Выручка")
	fmt.Println(separator)
	for i, p := range top {
		fmt.Printf("%-5d | %-20s | %-20s | %-8d | %-10.2f\n",
			i+1, p.ProductName, p.CategoryName, p.TotalQuantity, p.TotalRevenue)
	}
	fmt.Println(separator)
	return nil
}
