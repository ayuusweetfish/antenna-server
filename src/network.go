package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func createAuthToken(userId int) string {
	key := make([]byte, 64)
retry:
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	keyStr := base64.StdEncoding.EncodeToString(key)
	val, err := rcli.SetNX(context.Background(),
		"auth-key:"+keyStr, userId, 7*24*time.Hour).Result()
	if !val {
		// Collision is almost impossible
		// but handle anyway to stay theoretically sound
		goto retry
	}
	if err != nil {
		panic(err)
	}
	return keyStr
}
func validateAuthToken(token string) int {
	val, err := rcli.GetEx(context.Background(),
		"auth-key:"+token, 7*24*time.Hour).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0
		}
		panic(err)
	}
	userId, err := strconv.Atoi(val)
	if err != nil {
		panic(err)
	}
	return userId
}
func auth(w http.ResponseWriter, r *http.Request) User {
	cookies := r.Cookies()
	var cookieValue string
	for _, cookie := range cookies {
		if cookie.Name == "auth" {
			cookieValue = cookie.Value
			break
		}
	}
	if cookieValue == "" {
		panic("401 Authentication required")
	}
	userId := validateAuthToken(cookieValue)
	if userId == 0 {
		panic("401 Authentication required")
	}
	user := User{Id: userId}
	if !user.LoadById() {
		panic("500 Inconsistent databases")
	}
	return user
}

func signUpHandler(w http.ResponseWriter, r *http.Request) {
	nickname := r.PostFormValue("nickname")
	password := r.PostFormValue("password")
	if nickname == "" {
		panic("400 Missing nickname")
	}
	if password == "" {
		panic("400 Missing password")
	}
	user := User{
		Nickname: nickname,
		Password: password,
	}
	user.Save()
	fmt.Fprintf(w, "%v\n", user)
}

func logInHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PostFormValue("id")
	password := r.PostFormValue("password")
	if id == "" {
		panic("400 Missing id")
	}
	idN, err := strconv.Atoi(id)
	if err != nil {
		panic("400 Incorrect id format")
	}
	if password == "" {
		panic("400 Missing password")
	}
	user := User{Id: idN}
	if !user.LoadById() {
		panic("401 No such user")
	}
	if !user.VerifyPassword(password) {
		panic("401 Incorrect password")
	}

	token := createAuthToken(idN)

	w.Header().Add("Set-Cookie",
		"auth="+token+"; SameSite=Strict; Path=/; Secure; Max-Age=604800")

	fmt.Fprintf(w, "%v\n", user)
}

func profileCreateHandler(w http.ResponseWriter, r *http.Request) {
	user := auth(w, r)

	details := r.PostFormValue("details")
	stats, err := parseProfileStats(r.PostFormValue("stats"))
	if err != nil {
		panic("400 " + err.Error())
	}
	traits := parseProfileTraits(r.PostFormValue("traits"))

	profile := Profile{
		Creator: user.Id,
		Avatar:  "",
		Details: details,
		Stats:   stats,
		Traits:  traits,
	}
	profile.Save()

	fmt.Fprintf(w, "%v\n", profile)
}

func profileGetHandler(w http.ResponseWriter, r *http.Request) {
	user := auth(w, r)

	profileId, err := strconv.Atoi(r.PathValue("profile_id"))
	if err != nil {
		panic("400 Incorrect profile_id")
	}
	profile := Profile{
		Id: profileId,
	}
	if !profile.Load() {
		panic("404 No such profile")
	}
	if profile.Creator != user.Id {
		panic("401 Not creator")
	}

	fmt.Fprintf(w, "%v\n", profile)
}

func avatarHandler(w http.ResponseWriter, r *http.Request) {
	handle := r.PathValue("profile_id")
	fmt.Fprintln(w, "avatar "+handle)
}

func roomCreateHandler(w http.ResponseWriter, r *http.Request) {
	room := Room{
		Title:       r.PostFormValue("title"),
		Tags:        r.PostFormValue("tags"),
		Description: r.PostFormValue("description"),
	}
	room.Save()
	fmt.Fprintf(w, "%v\n", room)
}

func roomGetHandler(w http.ResponseWriter, r *http.Request) {
	room := Room{
		Id: r.PathValue("room_id"),
	}
	room.Load()
	fmt.Fprintf(w, "%v\n", room)
}

// A handler that captures panics and return the error message as 500
type errCaptureHandler struct {
	Handler http.Handler
}

func (h *errCaptureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if obj := recover(); obj != nil {
			if err, ok := obj.(error); ok {
				http.Error(w, err.Error(), 500)
			} else if str, ok := obj.(string); ok {
				status := 500
				message := str
				// Try parsing the string `str` into status + message
				index := strings.Index(str, " ")
				if index != -1 {
					if n, err := strconv.Atoi(str[:index]); err == nil {
						status = n
						message = str[(index + 1):]
					}
				}
				http.Error(w, message, status)
			} else {
				message := fmt.Sprint("%v", obj)
				http.Error(w, message, 500)
			}
		}
	}()
	h.Handler.ServeHTTP(w, r)
}

func ServerListen() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /sign-up", signUpHandler)
	mux.HandleFunc("POST /log-in", logInHandler)

	mux.HandleFunc("POST /profile/create", profileCreateHandler)
	mux.HandleFunc("GET /profile/{profile_id}", profileGetHandler)
	mux.HandleFunc("GET /avatar/{profile_id}", avatarHandler)

	mux.HandleFunc("POST /room/create", roomCreateHandler)
	mux.HandleFunc("GET /room/{room_id}", roomGetHandler)

	port := Config.Port
	log.Printf("Listening on http://localhost:%d/\n", port)
	if Config.Debug {
		log.Printf("Visit http://localhost:%d/test for testing\n", port)
	}
	server := &http.Server{
		Handler:      &errCaptureHandler{Handler: mux},
		Addr:         fmt.Sprintf("localhost:%d", port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print(err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Print(err)
	}
	log.Print("Shutting down")
}
