package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
)

type AnalyzeRequest struct {
	Sentence string `json:"sentence"`
}

type AnalyzeResponse struct {
	Words      int    `json:"words"`
	Vowels     int    `json:"vowels"`
	Consonants int    `json:"consonants"`
	Sentence   string `json:"sentence,omitempty"`
}

// Custom JWT claims with a simple role string.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func main() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal("JWT_SECRET is not set (refuse to start without auth secret)")
	}

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, routes([]byte(secret))); err != nil {
		log.Fatal(err)
	}
}

func routes(secret []byte) http.Handler {
	mux := http.NewServeMux()

	// No auth for health
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Require auth (role: user or admin)
	analyze := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sentence string
		switch r.Method {
		case http.MethodGet:
			sentence = r.URL.Query().Get("sentence")
			if sentence == "" {
				http.Error(w, "missing 'sentence' query", http.StatusBadRequest)
				return
			}
		case http.MethodPost:
			var req AnalyzeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			sentence = req.Sentence
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		words, vowels, consonants := Analyze(sentence)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AnalyzeResponse{
			Words:      words,
			Vowels:     vowels,
			Consonants: consonants,
			Sentence:   sentence,
		})
	})
	mux.Handle("/analyze", authMiddleware(secret, "user", "admin")(analyze))

	return mux
}

// authMiddleware verifies a Bearer JWT and enforces that the "role" claim is in allowedRoles.
func authMiddleware(secret []byte, allowedRoles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(authz, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authz, "Bearer ")

			var claims Claims
			token, err := jwt.ParseWithClaims(tokenStr, &claims, func(token *jwt.Token) (interface{}, error) {
				if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
					return nil, errors.New("unexpected signing method")
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
				http.Error(w, "token expired", http.StatusUnauthorized)
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				http.Error(w, "forbidden (insufficient role)", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Analyze returns number of words, vowels and consonants in a sentence.
func Analyze(s string) (words int, vowels int, consonants int) {
	words = len(strings.Fields(s))
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		switch unicode.ToLower(r) {
		case 'a', 'e', 'i', 'o', 'u':
			vowels++
		default:
			consonants++
		}
	}
	return
}
