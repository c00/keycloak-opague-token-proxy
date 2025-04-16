package util

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func PrintRequest(r *http.Request, debugMode bool) {
	// Print the request line
	fmt.Printf("%s %s %s\n", r.Method, r.RequestURI, r.Proto)
	fmt.Printf("Host: %s\n", r.Host)

	// Print headers in raw HTTP format
	for name, values := range r.Header {
		for _, value := range values {
			valLen := len(value)
			cutoff := 40
			if strings.ToLower(name) == "authorization" && !debugMode {
				cutoff = 10
			}
			if valLen > cutoff+5 {
				value = fmt.Sprintf("%v... (%v more)", value[:cutoff], valLen-cutoff)
			}
			fmt.Printf("%s: %s\n", name, value)
		}
	}

	// Print the body
	if debugMode && r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Println("Error reading body:", err)
		} else if len(body) > 0 {
			fmt.Printf("Body: %s\n", string(body))
		}
		r.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	fmt.Println()
}
