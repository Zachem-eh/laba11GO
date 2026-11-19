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
		clear_screen()

		fmt.Println("Меню команд:")
		fmt.Println("0 - Завершить работу")
		a, err := readInt(reader, "Выберите пункт из меню управления:")
		if err != nil {
			fmt.Println(err)
			continue
		}
		switch a {
		case 0:
			return
		default:
			fmt.Println("Нет такого пункта меню!")
		}
	}
}
