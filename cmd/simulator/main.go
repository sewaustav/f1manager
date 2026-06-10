package main

import (
	"database/sql"
	"fmt"
	"log"

	"f1/internal/cli"
	"f1/internal/engine"
	"f1/internal/storage"

	_ "github.com/mattn/go-sqlite3" 
)

func main() {
	db, err := sql.Open("sqlite3", "./f1_simulation.db")
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	store := storage.NewStorage(db)
	err = store.InitSchema()
	if err != nil {
		log.Fatalf("Ошибка применения миграций: %v", err)
	}

	err = store.SeedData()
	if err != nil {
		log.Fatalf("Ошибка наполнения БД базовыми данными: %v", err)
	}

	simEngine := engine.NewEngine(db)
	ui := cli.NewCLI(store, simEngine)

	// Запуск игры
	ui.Start()
	fmt.Println("\nСезон завершен успешно. Сервер жив!")
}