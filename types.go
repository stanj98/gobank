package main

import (
	"math/rand"
	"time"

	"github.com/alexedwards/argon2id"
)

type LoginRequest struct {
	Number   int64  `json:"number"`
	Password string `json:"password"`
}

type TransferRequest struct {
	ToAccount int `json:"to_account"`
	Amount    int `json:"amount"`
}

type CreateAccountRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

type Account struct {
	ID           int       `json:"id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	PasswordHash string    `json:"-"`
	Number       int64     `json:"number"`
	Balance      int64     `json:"acc_balance"`
	CreatedAt    time.Time `json:"created_at"`
}

func NewAccount(firstName, lastName string, password string) (*Account, error) {
	//as per: https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html#argon2id
	params := &argon2id.Params{
		Memory:      19 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
	hash, err := argon2id.CreateHash(password, params)
	if err != nil {
		return nil, err
	}
	return &Account{
		FirstName:    firstName,
		LastName:     lastName,
		Number:       int64(rand.Intn(1000000)),
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}, nil
}
