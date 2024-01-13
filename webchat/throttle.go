package webchat

import (
	"encoding/csv"
	"net/http"
	"strings"
)

func getClientFingerprint(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		values, _ := csv.NewReader(strings.NewReader(xff)).Read()
		return strings.Join(values, "|")
	}
	return r.RemoteAddr
}
