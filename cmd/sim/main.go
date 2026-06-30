package main

import (
	"context"
	"database/sql"
	"f1/internal/storage/sqlite_repo"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	
	"f1/internal/cli"
	"f1/internal/engine"
	
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./f1_simulation.db")
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()
	
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := sqlite_repo.NewSqliteF1Repo(db)

	simEngine := engine.NewEngine(db)
	ui := cli.NewCLI(store, simEngine)

	// Запуск игры
	go ui.Start(ctx)
	
	<-ctx.Done()
	
	fmt.Println("\nСезон завершен с прерыванием. Сервер жив!")
	
	shutdown, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	
	if err := store.ResetSession(shutdown); err != nil {
		fmt.Errorf("Ошибка при сбросе сессии: %v", err)
	}
	
	<-shutdown.Done()
}