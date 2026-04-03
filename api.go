package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

type APIServer struct {
	listenAddr string
	store      Storage
}

type contextKey struct{}

var ClaimsKey = contextKey{}

func NewAPIServer(listenAddr string, store Storage) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *APIServer) Run() {
	router := mux.NewRouter()
	router.HandleFunc("/login", makeHTTPHandleFunc(s.handleLogin))
	router.HandleFunc("/account", makeHTTPHandleFunc(s.handleAccount))
	router.HandleFunc("/account/{id}", withJWTAuth(makeHTTPHandleFunc(s.handleGetAccountByID)))
	router.HandleFunc("/transfer", withJWTAuth(makeHTTPHandleFunc(s.handleTransfer)))
	log.Println("JSON API server running on port: ", s.listenAddr)
	err := http.ListenAndServe(s.listenAddr, router)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "POST" {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}
		number, err := s.store.ValidateAccount(req.Number, req.Password)
		if err != nil {
			return WriteJSON(w, http.StatusUnauthorized, "error: Invalid Login!")
		}
		tokenStr, err := generateJWT(*number)
		if err != nil {
			return err
		}
		return WriteJSON(w, http.StatusOK, tokenStr)
	}
	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (s *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "GET" {
		return s.handleGetAccount(w, r)
	}
	if r.Method == "POST" {
		return s.handleCreateAccount(w, r)
	}

	return fmt.Errorf("Method not allowed %s", r.Method)
}

func (s *APIServer) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	accounts, err := s.store.GetAccounts()
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, accounts)
}

func (s *APIServer) handleGetAccountByID(w http.ResponseWriter, r *http.Request) error {
	if r.Method == "DELETE" {
		return s.handleDeleteAccount(w, r)
	}
	if r.Method == "GET" {
		id, err := getID(r)
		if err != nil {
			return err
		}
		account, err := s.store.GetAccountByID(id)
		if err != nil {
			return err
		}
		return WriteJSON(w, http.StatusOK, account)
	}
	return fmt.Errorf("method not allowed %s", r.Method)
}

func (s *APIServer) handleCreateAccount(w http.ResponseWriter, r *http.Request) error {
	createAccRequest := new(CreateAccountRequest)
	if err := json.NewDecoder(r.Body).Decode(&createAccRequest); err != nil {
		return err
	}
	account, err := NewAccount(createAccRequest.FirstName, createAccRequest.LastName, createAccRequest.Password)
	if err := s.store.CreateAccount(account); err != nil {
		return err
	}
	number := account.Number
	tokenStr, err := generateJWT(number)
	if err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, tokenStr)
}

func (s *APIServer) handleDeleteAccount(w http.ResponseWriter, r *http.Request) error {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		return err
	}

	userIDFromToken := int(claims["accountNumber"].(float64)) // JWT numbers come as float64

	id, err := getID(r)
	if err != nil {
		return err
	}

	// Authorization check
	if userIDFromToken != id {
		return fmt.Errorf("forbidden: cannot delete another user's account")
	}

	if err := s.store.DeleteAccount(id); err != nil {
		return err
	}

	return WriteJSON(w, http.StatusOK, map[string]int{"deleted": id})
}

func (s *APIServer) handleTransfer(w http.ResponseWriter, r *http.Request) error {
	claims, err := getClaimsFromContext(r)
	if err != nil {
		return err
	}
	userIDFromToken := int(claims["accountNumber"].(float64))
	transferReq := new(TransferRequest)
	if err := json.NewDecoder(r.Body).Decode(transferReq); err != nil {
		return err
	}
	id, err := getID(r)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	if userIDFromToken != id {
		return fmt.Errorf("forbidden: cannot transfer from another user's account")
	}

	return WriteJSON(w, http.StatusOK, transferReq)
}

func WriteJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func getClaimsFromContext(r *http.Request) (jwt.MapClaims, error) {
	claims, ok := r.Context().Value(ClaimsKey).(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("unauthorized: no claims found")
	}
	return claims, nil
}

func generateJWT(number int64) (string, error) {
	secret := GoDotEnvVariable("JWT_SECRET")
	claims := &jwt.MapClaims{
		"expiresAt":     15000,
		"accountNumber": number,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func withJWTAuth(handlerFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("x-jwt-token")
		token, err := validateJWT(tokenStr)
		if err != nil || token == nil {
			WriteJSON(w, http.StatusForbidden, APIError{Error: "permission denied"})
			return
		}
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// Store claims in request context
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			// Create new request with updated context
			r = r.WithContext(ctx)
		}
		handlerFunc(w, r)
	}
}
func validateJWT(tokenString string) (*jwt.Token, error) {
	secret := GoDotEnvVariable("JWT_SECRET")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	return token, err
}

type apiFunc func(http.ResponseWriter, *http.Request) error

type APIError struct {
	Error string `json:"error"`
}

func makeHTTPHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJSON(w, http.StatusBadRequest, APIError{Error: err.Error()})
		}
	}
}

func getID(r *http.Request) (int, error) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return id, fmt.Errorf("invalid id given %s", idStr)
	}
	return id, nil
}
