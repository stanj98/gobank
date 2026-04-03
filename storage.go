package main

import (
	"database/sql"
	"fmt"

	"github.com/alexedwards/argon2id"
	_ "github.com/lib/pq"
)

type Storage interface {
	CreateAccount(*Account) error
	DeleteAccount(int) error
	UpdateAccount(*Account) error
	GetAccounts() ([]*Account, error)
	GetAccountByID(int) (*Account, error)
	ValidateAccount(int64, string) (*int64, error)
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore() (*PostgresStore, error) {
	//https://hub.docker.com/_/postgres
	//$ docker run --name <username> -e POSTGRES_PASSWORD=<pass> -p 5432:5432 -d <dbname>
	//check if instance is up: docker ps
	//test if working: telnet localhost 5432

	usr := GoDotEnvVariable("POSTGRES_USERNAME")
	pwd := GoDotEnvVariable("POSTGRES_PASSWORD")
	db_name := GoDotEnvVariable("POSTGRES_DB")
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", usr, pwd, db_name)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{
		db: db,
	}, nil
}

func (s *PostgresStore) Init() error {
	return s.CreateAccountTable()
}

func (s *PostgresStore) CreateAccountTable() error {
	query := `create table if not exists account (
		id serial primary key,
		first_name varchar(50),
		last_name varchar(50),
		password_hash varchar(255),
		number bigint,
		balance bigint,
		created_at timestamp
	)`
	_, err := s.db.Exec(query)
	return err
}

func (s *PostgresStore) CreateAccount(acc *Account) error {
	query := `
	insert into account (first_name, last_name, password_hash, number, balance, created_at) values
	($1, $2, $3, $4, $5, $6)
	`
	resp, err := s.db.Query(
		query,
		acc.FirstName,
		acc.LastName,
		acc.PasswordHash,
		acc.Number,
		acc.Balance,
		acc.CreatedAt,
	)
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", resp)
	return nil
}

func (s *PostgresStore) DeleteAccount(id int) error {
	_, err := s.db.Query("delete from account where id = $1", id)
	return err
}

func (s *PostgresStore) UpdateAccount(*Account) error {
	return nil
}

func (s *PostgresStore) GetAccountByID(id int) (*Account, error) {
	rows, err := s.db.Query("select * from account where id = $1", id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		return scanIntoAccount(rows)
	}
	return nil, fmt.Errorf("Account %d not found", id)
}

func (s *PostgresStore) GetAccounts() ([]*Account, error) {
	rows, err := s.db.Query("select * from account")
	if err != nil {
		return nil, err
	}
	accounts := []*Account{}
	for rows.Next() {
		account, err := scanIntoAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func scanIntoAccount(rows *sql.Rows) (*Account, error) {
	account := new(Account)
	err := rows.Scan(
		&account.ID,
		&account.FirstName,
		&account.LastName,
		&account.PasswordHash,
		&account.Number,
		&account.Balance,
		&account.CreatedAt,
	)
	return account, err
}

func (s *PostgresStore) ValidateAccount(account_number int64, password string) (*int64, error) {
	row := s.db.QueryRow("select password_hash, number from account where number = $1", account_number)
	var number int64
	var hash string
	if err := row.Scan(&hash, &number); err != nil {
		return nil, err
	}
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return nil, fmt.Errorf("Something went wrong! Please try again.")
	}
	if match == true {
		return &number, nil
	}
	return nil, fmt.Errorf("Account %d not found", account_number)
}
