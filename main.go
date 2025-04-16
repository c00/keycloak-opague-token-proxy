package main

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/c00/keycloak-opague-token-proxy/util"
	"github.com/spf13/viper"
)

type CachedToken struct {
	accessToken string
	expires     time.Time
}

// token cache
var tokens util.SimpleKeyValueStore[CachedToken]

var forwardTo string
var listenPort string
var debug bool

func main() {
	tokens = util.NewSimpleKeyValueStore[CachedToken]()

	// Set the file name of the configurations file
	viper.SetConfigFile(".env")
	viper.SetConfigType("dotenv")
	viper.ReadInConfig()
	viper.AutomaticEnv()

	viper.SetDefault("KC_UPSTREAM", "http://keycloak:8080")
	forwardTo = viper.GetString("KC_UPSTREAM")
	viper.SetDefault("PORT", ":8080")
	listenPort = viper.GetString("PORT")
	viper.SetDefault("DEBUG", false)
	debug = viper.GetBool("DEBUG")

	go cleanup()

	fmt.Printf("Listening on: http://localhost%v\n\n", listenPort)
	http.HandleFunc("/", handler)
	http.ListenAndServe(listenPort, nil)
}

func handler(w http.ResponseWriter, r *http.Request) {

	util.PrintRequest(r, debug)
	expectToken := false

	//Detect if there is an auth token
	header := r.Header.Get("Authorization")
	if header != "" {
		parts := strings.Split(header, " ")
		if len(parts) != 2 {
			fmt.Printf("malformed auth header: %v", header)
			http.Error(w, "Malformed Auth Header", http.StatusBadRequest)
			return
		}

		authType := strings.ToLower(parts[0])
		if authType == "basic" {
			// if token is Basic, then store the result when it comes back
			expectToken = true
		} else if authType == "bearer" {
			// if token is bearer, then switch it out
			if tokens.Has(parts[1]) {
				token, _ := tokens.Get(parts[1])
				r.Header.Set("Authorization", fmt.Sprintf("%v %v", parts[0], token.accessToken))
				token.expires = time.Now().Add(time.Hour)
				tokens.Set(parts[1], token)
			} else {
				fmt.Println("got auth header, but no matching tokens")
			}
		}
	}

	// Forward the request
	client := &http.Client{}
	req, err := http.NewRequest(r.Method, forwardTo+r.RequestURI, r.Body)
	if err != nil {
		fmt.Printf("cannot create request: %v", err)
		http.Error(w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	// Copy headers
	req.Header = r.Header

	// Forward the request and response
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("cannot forward request: %v", err)
		http.Error(w, "Request forwarding failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy the response headers
	maps.Copy(w.Header(), resp.Header)

	if expectToken {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading body:", err)
			http.Error(w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}

		var tokenResponse map[string]any
		err = json.Unmarshal(body, &tokenResponse)
		if err != nil {
			fmt.Printf("cannot unmarshall: %v\n", err)
			http.Error(w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}

		accessToken, ok := tokenResponse["access_token"].(string)
		if !ok {
			fmt.Printf("could not get access token from response: %+v\n", tokenResponse)
			http.Error(w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}

		//generate a nice opague token
		opague, err := util.GetOpagueToken(32)
		if err != nil {
			fmt.Printf("cannot get opague token: %v\n", err)
			http.Error(w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}
		tokens.Set(opague, CachedToken{
			accessToken: accessToken,
			expires:     time.Now().Add(time.Hour),
		})
		tokenResponse["access_token"] = opague
		fmt.Println("Stored access token", opague)

		w.WriteHeader(resp.StatusCode)

		byteRes, err := json.Marshal(tokenResponse)
		if err != nil {
			fmt.Printf("cannot marshall tokenresponse: %v\n", err)
			http.Error(w, "cannot marshall tokenresponse", http.StatusInternalServerError)
			return
		}

		w.Write(byteRes)
		return
	}

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func cleanup() {
	for {
		for _, key := range tokens.Keys() {
			token, _ := tokens.Get(key)

			if token.expires.Before(time.Now()) {
				tokens.Delete(key)
			}
		}

		time.Sleep(time.Hour)
	}
}
