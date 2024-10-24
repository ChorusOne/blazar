package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		env := os.Environ()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, strings.Join(env, "\n"))
	})

	http.ListenAndServe(":8080", nil)
}
