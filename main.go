package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/c00/keycloak-opague-token-proxy/util"
	"github.com/spf13/viper"
)

type Middleware func(http.Handler) http.Handler

// Apply the middlewares in order
func chainMiddlewares(h http.Handler, middlewares []Middleware) http.Handler {
	for _, m := range middlewares {
		h = m(h)
	}
	return h
}

func PrintRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		util.PrintRequest(r, printLevel)

		next.ServeHTTP(w, r)
	})
}

func filterIpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := util.GetIp(r)
		if !slices.Contains(allowedIps, ip) {
			slog.Info("[filterip] unauthorized IP", "ip", ip, "allowed", allowedIps)
			returnHttpError(r, w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type CachedToken struct {
	accessToken string
	expires     time.Time
}

// token cache
var tokens util.SimpleKeyValueStore[CachedToken]

var forwardTo string
var listenPort string
var filterIps bool
var allowedIps []string
var printLevel int

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

	viper.SetDefault("FILTER_IP", false)
	filterIps = viper.GetBool("FILTER_IP")
	viper.SetDefault("ALLOWED_IPS", []string{})
	allowedIps = util.SplitString(viper.GetString("ALLOWED_IPS"))

	viper.SetDefault("PRINT_REQUEST_LEVEL", 0)
	printLevel = viper.GetInt("PRINT_REQUEST_LEVEL")

	go cleanup()

	//setup middlewares
	middlewares := []Middleware{}
	if filterIps {
		if len(allowedIps) == 0 {
			slog.Error("filterIps is set to true, but allowedIps is empty.")
			panic("filterIps is set to true, but allowedIps is empty.")
		}
		middlewares = append(middlewares, filterIpMiddleware)
	}

	if printLevel > 0 {
		middlewares = append(middlewares, PrintRequestMiddleware)
	}

	//Start listening
	slog.Info("Starting server", "port", listenPort)
	http.Handle("/", chainMiddlewares(http.HandlerFunc(handler), middlewares))
	http.ListenAndServe(listenPort, nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	// util.PrintRequest(r, debug)
	expectToken := false

	//Detect if there is an auth token
	header := r.Header.Get("Authorization")
	if header != "" {
		parts := strings.Split(header, " ")
		if len(parts) != 2 {
			slog.Error("malformed auth header", "header", header)
			returnHttpError(r, w, "Malformed Auth Header", http.StatusBadRequest)
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
				slog.Error("got auth header, but no matching tokens")
			}
		}
	}

	// Forward the request
	client := &http.Client{}
	req, err := http.NewRequest(r.Method, forwardTo+r.RequestURI, r.Body)
	if err != nil {
		slog.Error("cannot create request", "error", err)
		returnHttpError(r, w, "Request creation failed", http.StatusInternalServerError)
		return
	}

	// Copy headers
	req.Header = r.Header

	// Forward the request and response
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("cannot forward request", "error", err)
		returnHttpError(r, w, "Request forwarding failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy the response headers
	maps.Copy(w.Header(), resp.Header)

	if expectToken {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Error reading body", "error", err)
			returnHttpError(r, w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}

		var tokenResponse map[string]any
		err = json.Unmarshal(body, &tokenResponse)
		if err != nil {
			slog.Error("cannot unmarshall", "error", err)
			returnHttpError(r, w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}

		accessToken, ok := tokenResponse["access_token"].(string)
		if !ok {
			slog.Error("could not get access token from response", "response", tokenResponse)
			returnHttpError(r, w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}

		//generate a nice opague token
		opague, err := util.GetOpagueToken(32)
		if err != nil {
			slog.Error("cannot get opague token", "error", err)
			returnHttpError(r, w, "Request forwarding failed", http.StatusInternalServerError)
			return
		}
		tokens.Set(opague, CachedToken{
			accessToken: accessToken,
			expires:     time.Now().Add(time.Hour),
		})
		tokenResponse["access_token"] = opague

		setStatus(r, w, resp.StatusCode)

		byteRes, err := json.Marshal(tokenResponse)
		if err != nil {
			slog.Error("cannot marshall tokenresponse", "error", err)
			returnHttpError(r, w, "cannot marshall tokenresponse", http.StatusInternalServerError)
			return
		}

		w.Write(byteRes)
		return
	}

	// Set the status code
	setStatus(r, w, resp.StatusCode)

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

func setStatus(r *http.Request, w http.ResponseWriter, status int) {
	slog.Info("request ok", "method", r.Method, "path", r.URL.Path, "status", status)
	w.WriteHeader(status)
}

func returnHttpError(r *http.Request, w http.ResponseWriter, msg string, status int) {
	slog.Info("request failed", "method", r.Method, "path", r.URL.Path, "status", status)
	http.Error(w, msg, status)
}
