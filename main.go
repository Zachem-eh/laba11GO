package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Анонимный импорт драйвера
)

// функция для очистки экрана (работает и на Windows, и на macOS/Linux)
func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// readInt читает целое число (пункт меню, ID)
func readInt(reader *bufio.Reader, prompt string) (int, error) {
	fmt.Println(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("не удалось прочитать ввод: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, errors.New("нужно ввести целое число")
	}
	return n, nil
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

func main() {
	clearScreen()
	// _pragma=foreign_keys(1) включает проверку внешних ключей (в SQLite она по умолчанию выключена)
	db, err := sql.Open("sqlite", "file:ProductsDB.db?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatalf("Ошибка при открытии БД: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Нет связи с БД: %v", err)
	}
	fmt.Println("Успешное подключение к SQLite (файл ProductsDB.db)")

	if err := runMigrations(ctx, db); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	if err := initFirstProducts(ctx, db); err != nil {
		log.Fatalf("Ошибка заполнения БД: %v", err)
	}

	fmt.Println("Инициализация завершена. База данных готова к работе!")

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\nДля продолжения работы с БД нажмите Enter...")
		if _, err := reader.ReadString('\n'); err != nil {
			return // ввод закончился (Ctrl+D) — выходим, а не крутимся вечно
		}
		clearScreen()

		fmt.Println("Меню команд:")
		fmt.Println("0 - Завершить работу")
		fmt.Println("1 - Вывести все товары с названием их категорий")
		fmt.Println("2 - Вывести выручку по категориям товаров")
		fmt.Println("3 - Оформление продажи(реализация транзакции)")
		fmt.Println("4 - Найти товар по ID")
		fmt.Println("5 - Список товаров с фильтрами и по страницам")
		fmt.Println("6 - Добавить товар")
		fmt.Println("7 - Изменить товар")
		fmt.Println("8 - Удалить товар")
		fmt.Println("9 - Список категорий")
		fmt.Println("10 - Добавить категорию")
		fmt.Println("11 - Удалить категорию")
		a, err := readInt(reader, "Выберите пункт из меню управления:")
		if err != nil {
			fmt.Println(err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		switch a {
		case 0:
			return
		case 1:
			err = GetAllProducts(ctx, db)
		case 2:
			fmt.Println("Выручка по категориям:")
			revenues, err := GetRevenueByCategory(ctx, db)
			if err != nil {
				fmt.Println("Ошибка:", err)
			} else if len(revenues) == 0 {
				fmt.Println("Продаж пока не было.")
			} else {
				for _, r := range revenues {
					fmt.Printf("Категория: %-15s | Продаж: %-3d | Выручка: %-8.2f | Средний чек: %.2f\n",
						r.CategoryName, r.TotalSalesCount, r.TotalRevenue, r.AvgRevenue)
				}
			}

		case 3:
			printAvailableProducts(ctx, db)
			prodID, err := readInt(reader, "Введите ID товара для покупки:")
			if err != nil {
				fmt.Println("Ошибка ввода ID товара", err)
				continue
			}
			qty, err := readInt(reader, "Введите количество:")
			if err != nil {
				fmt.Println("Ошибка ввода количества товара", err)
				continue
			}
			// Вызываем нашу функцию транзакции из файла sale.go
			err = ProcessSale(ctx, db, int64(prodID), qty)
			if err != nil {
				fmt.Println("Ошибка оформления продажи:", err)
			} else {
				fmt.Println("Товар успешно добавлен в продажи, остаток на складе обновлен!")
			}
		case 4:
			err = handleGetProduct(reader, db)
		case 5:
			err = handleListProducts(reader, db)
		case 6:
			err = handleCreateProduct(reader, db)
		case 7:
			err = handleUpdateProduct(reader, db)
		case 8:
			err = handleDeleteProduct(reader, db)
		case 9:
			err = handleListCategories(db)
		case 10:
			err = handleCreateCategory(reader, db)
		case 11:
			err = handleDeleteCategory(reader, db)
		default:
			fmt.Println("Нет такого пункта меню!")
		}
		cancel()
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
		}
	}
}
func printAvailableProducts(ctx context.Context, db *sql.DB) {
	fmt.Println("Доступные товары")
	rows, err := db.QueryContext(ctx, "SELECT id, name, price, available_count FROM products ORDER BY id")
	if err != nil {
		fmt.Println("Ошибка получения списка товаров:", err)
		return
	}
	defer rows.Close() // Обязательно закрываем rows

	fmt.Printf("%-5s | %-25s | %-10s | %-10s\n", "ID", "Название", "Цена", "Остаток")
	fmt.Println(strings.Repeat("-", 55))
	for rows.Next() {
		var id int64
		var name string
		var price float64
		var count int
		if err := rows.Scan(&id, &name, &price, &count); err != nil {
			continue
		}
		fmt.Printf("%-5d | %-25s | %-10.2f | %-10d\n", id, name, price, count)
	}
}

// printAvailableCategories выводит список категорий
func printAvailableCategories(ctx context.Context, db *sql.DB) {
	fmt.Println("Доступные категории")
	rows, err := db.QueryContext(ctx, "SELECT id, name FROM categories ORDER BY id")
	if err != nil {
		fmt.Println("Ошибка получения списка категорий:", err)
		return
	}
	defer rows.Close()

	fmt.Printf("%-5s | %-25s\n", "ID", "Название")
	fmt.Println(strings.Repeat("-", 35))
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		fmt.Printf("%-5d | %-25s\n", id, name)
	}
}
