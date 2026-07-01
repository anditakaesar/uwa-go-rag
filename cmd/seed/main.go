package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/anditakaesar/uwa-go-rag/internal/domain"
	"github.com/anditakaesar/uwa-go-rag/internal/env"
	"github.com/anditakaesar/uwa-go-rag/internal/infra"
	"github.com/anditakaesar/uwa-go-rag/internal/xlog"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	pool, err := pgxpool.New(context.Background(), env.Values.DBUrl)
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("Unable to connect database: %v", err))
		os.Exit(1)
	}
	defer pool.Close()

	err = pool.Ping(context.Background())
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("Unable to connect database: %v", err))
		os.Exit(1)
	}

	services := infra.NewInfra(pool)
	seedUsers(ctx, services)
	seedSQL(ctx, pool)
}

type UserSeed struct {
	Username string
	Password string
	Role     string
}

func readUserFromCSV(path string) ([]UserSeed, error) {
	userSeedFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer userSeedFile.Close()

	r := csv.NewReader(userSeedFile)
	r.TrimLeadingSpace = true

	_, err = r.Read() // skip header
	if err != nil {
		return nil, err
	}

	var users []UserSeed

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		users = append(users, UserSeed{
			Username: record[0],
			Password: record[1],
			Role:     record[2],
		})
	}

	return users, nil
}

func seedUsers(ctx context.Context, services *infra.Services) {
	// users, err := readUserFromCSV("users.csv") // for debug use this
	users, err := readUserFromCSV(filepath.Join("db", "seed", "users.csv"))
	if err != nil {
		xlog.Logger.Error(fmt.Sprintf("error reading user seed file: %v", err))
		return
	}

	var seedErrs []string
	for i, u := range users {
		newUser := domain.User{
			Username: u.Username,
			Password: u.Password,
		}

		if i == 0 { // the first one is user admin
			_, seedErr := services.UserService.CreateUserWithRole(ctx, newUser, u.Role)
			if seedErr != nil {
				seedErrs = append(seedErrs, fmt.Sprintf("error processing admin roled user (%s): %v", u.Username, seedErr))
			}
			continue
		}

		_, seedErr := services.UserService.CreateUser(ctx, newUser)
		if seedErr != nil {
			seedErrs = append(seedErrs, fmt.Sprintf("error processing (%s): %v", u.Username, seedErr))
		}
	}

	if len(seedErrs) > 0 {
		xlog.Logger.Error(strings.Join(seedErrs, "\n"))
	}
}

func seedSQL(ctx context.Context, pool *pgxpool.Pool) {
	// list file on db/seed/*.sql
	var seedErrs []string
	pattern := filepath.Join("db", "seed", "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		xlog.Logger.Error("error get seed sql files")
		return
	}
	// run each sql file
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			seedErrs = append(seedErrs, fmt.Sprintf("error reading file: %s with error: %v", file, err))
			continue
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			seedErrs = append(seedErrs, fmt.Sprintf("error executing sql file: %s with error: %v", file, err))
		}
	}

	if len(seedErrs) > 0 {
		xlog.Logger.Error(strings.Join(seedErrs, "\n"))
	}
}
