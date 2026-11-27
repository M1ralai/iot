package health

import (
	"fmt"
	"io"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	_, err := io.WriteString(w, "server is up")
	if err != nil {
		fmt.Printf("Error Occured")
	}
}
