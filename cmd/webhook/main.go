package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type Event struct {
	Type string `json:"type"`
	Body struct {
		Text string `json:"text"`
		User string `json:"user"`
	} `json:"body"`
}

func main() {
	http.HandleFunc("/", health)
	http.HandleFunc("/events", events)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("だせい起動")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "dasei running")
}

func events(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}

	var e Event

	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	reply := createReply(e.Body.Text)

	res := map[string]string{
		"reply": reply,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func createReply(text string) string {

	t := strings.ToLower(text)

	switch {

	case strings.Contains(t, "こんにちは"):
		return "こんにちはだせい。"

	case strings.Contains(t, "おはよう"):
		return "おはようだせい。"

	case strings.Contains(t, "おやすみ"):
		return "またあとでだせい。"

	case strings.Contains(t, "カレー"):
		return "カレーは飲み物だせい。"

	case strings.Contains(t, "疲れた"):
		return "無理しなくていいだせい。"

	case strings.Contains(t, "眠い"):
		return "だせいも眠いだせい。"

	case strings.Contains(t, "かわいい"):
		return "照れるだせい。"

	case strings.Contains(t, "だせい"):
		return "呼んだだせい？"

	default:
		return "なるほどだせい。"
	}
}
