package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrInsufficientStock возвращается, когда товара недостаточно на складе
var ErrInsufficientStock = errors.New("недостаточное количество товара на складе")

func ProcessSale(ctx context.Context, db *sql.DB, productID int64, quantity int) error {
	if quantity <= 0 {
		return errors.New("количество товара должно быть больше нуля")
	}
	// Начало транзакций
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer tx.Rollback()
	var currentStock int
	err = tx.QueryRowContext(ctx, "SELECT available_count FROM products WHERE id = ?", productID).Scan(&currentStock)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("товар не найден")
		}
		return fmt.Errorf("ошибка проверки остатка товара: %w", err)
	}

	if currentStock < quantity {
		return ErrInsufficientStock
	}

	updateRes, err := tx.ExecContext(ctx,
		"UPDATE products SET available_count = available_count - ? WHERE id = ?",
		quantity, productID)
	if err != nil {
		return fmt.Errorf("ошибка обновления остатка товара: %w", err)
	}

	affected, err := updateRes.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения RowsAffected: %w", err)
	}
	if affected == 0 {
		return errors.New("не удалось обновить товар: возможно, он был удален")
	}

	var price float64
	err = tx.QueryRowContext(ctx, "SELECT price FROM products WHERE id = ?", productID).Scan(&price)
	if err != nil {
		return fmt.Errorf("ошибка получения цены товара: %w", err)
	}

	totalPrice := price * float64(quantity)

	// Создаем запись о продаже
	_, err = tx.ExecContext(ctx,
		"INSERT INTO sales (product_id, quantity, total_price) VALUES (?, ?, ?)",
		productID, quantity, totalPrice)
	if err != nil {
		return fmt.Errorf("ошибка создания записи о продаже: %w", err)
	}

	//Если все шаги прошли успешно, фиксируем транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	fmt.Printf("Продажа успешно оформлена! Товар ID: %d, Кол-во: %d, Сумма: %.2f\n", productID, quantity, totalPrice)
	return nil
}
