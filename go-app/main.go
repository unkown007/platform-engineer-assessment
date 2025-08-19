package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"unicode"
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

func main() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		var sentence string
		switch r.Method {
		case http.MethodGet:
			sentence = r.URL.Query().Get("sentence")
		case http.MethodPost:
			dec := json.NewDecoder(r.Body)
			var req AnalyzeRequest
			if err := dec.Decode(&req); err != nil {
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

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
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
