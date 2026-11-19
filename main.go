package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
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
	fmt.Println("Успешное подключение к SQLite (файл lab.db)")

	if err := runMigrations(ctx, db); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	if err := initFirstProducts(ctx, db); err != nil {
		log.Fatalf("Ошибка заполнения БД: %v", err)
	}

	fmt.Println("Инициализация завершена. База данных готова к работе!")
}
